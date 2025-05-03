package http

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/lightlink/stream-service/pkg/room/application/roommanager"
	"github.com/lightlink/stream-service/pkg/room/domain/dto"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/ws"
)

type RoomHandler struct {
	roomManager     roommanager.RoomManager
	messagingServer ws.MessagingServer
}

func NewRoomHandler(roomManager roommanager.RoomManager, messagingServer ws.MessagingServer) *RoomHandler {
	return &RoomHandler{
		roomManager:     roomManager,
		messagingServer: messagingServer,
	}
}

type SignalingRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (h *RoomHandler) SingalHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling incoming client signal")

	userID := r.Header.Get("X-User-ID")
	roomID := mux.Vars(r)["roomID"]

	var req SignalingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	client, err := h.roomManager.GetClientByUserRoomIds(roomID, userID)
	if err != nil {
		fmt.Println("Err: ", err)
		http.Error(w, "client not found", http.StatusNotFound)
		return
	}

	fmt.Println("Handling signal for client: ", client)

	switch req.Type {
	case "subscribe":
		var data struct {
			TargetUserId string `json:"targetUserId"`
			Offer        string `json:"offer"`
		}
		if err := json.Unmarshal(req.Payload, &data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		subscriberEndpoint := client.SubscriberEndpoints[data.TargetUserId]
		sdpAnswer, err := subscriberEndpoint.ProcessSDPOffer(data.Offer)
		if err != nil {
			fmt.Println("Cant get ans")
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		err = subscriberEndpoint.SubscribeOnEvent("OnIceCandidate")
		if err != nil {
			log.Printf("Error due subscribing on icecandidate event: %v", err)
		}

		err = subscriberEndpoint.GatherICECandidates()
		if err != nil {
			log.Printf("Error due gathering candidates: %v", err)
		}

		fmt.Println("Published sdp answer")

		h.messagingServer.PublishToUser(
			roomID,
			userID,
			map[string]interface{}{
				"type": "answer_sub",
				"payload": map[string]interface{}{
					"targetUserId": data.TargetUserId,
					"sdpAnswer":    sdpAnswer,
				},
			},
		)
	case "remoteIce":
		var data struct {
			TargetUserId string           `json:"targetUserId"`
			Candidate    dto.IceCandidate `json:"candidate"`
		}
		if err := json.Unmarshal(req.Payload, &data); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		subscriberEndpoint := client.SubscriberEndpoints[data.TargetUserId]
		if subscriberEndpoint == nil {
			fmt.Printf("ERROR: Nil sub for target: %s for user with id: %s", data.TargetUserId, userID)
		}
		subscriberEndpoint.AddICECandidate(data.Candidate)
	case "offer":
		var offer string
		if err := json.Unmarshal(req.Payload, &offer); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		answer, err := client.PublisherEndpoint.ProcessSDPOffer(offer)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		err = client.PublisherEndpoint.SubscribeOnEvent("OnIceCandidate")
		if err != nil {
			log.Printf("Error due subscribing on icecandidate event: %v", err)
		}

		err = client.PublisherEndpoint.GatherICECandidates()
		if err != nil {
			log.Printf("Error due gathering candidates: %v", err)
		}

		fmt.Println("Published sdp answer")
		err = h.messagingServer.PublishToUser(
			roomID,
			userID,
			map[string]interface{}{
				"type":    "answer",
				"payload": answer,
			},
		)
		if err != nil {
			log.Printf("Centrifugo error: %v", err)
		}

	case "ice":
		var candidate dto.IceCandidate
		if err := json.Unmarshal(req.Payload, &candidate); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if err := client.PublisherEndpoint.AddICECandidate(candidate); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RoomHandler) JoinHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling incoming join request")
	userIDString := r.Header.Get("X-User-ID")
	roomID := mux.Vars(r)["roomID"]

	_, err := h.roomManager.HandleUserJoin(roomID, userIDString)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *RoomHandler) ConnectionStateHandler(w http.ResponseWriter, r *http.Request) {
	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return true },
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("Ошибка WebSocket:", err)
		return
	}
	defer conn.Close()

	roomID := mux.Vars(r)["roomID"]
	userIDString := r.URL.Query().Get("userID")
	if userIDString == "" {
		fmt.Println("Ошибка: userID отсутствует")
		http.Error(w, "Ошибка: userID отсутствует", http.StatusBadRequest)
		return
	}

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Printf("INFO: User with id: %s left room %s with error: %s\n", userIDString, roomID, err.Error())
			h.roomManager.HandleUserLeft(roomID, userIDString)
			conn.Close()

			break
		}

		var payload map[string]string
		if err := json.Unmarshal(msg, &payload); err == nil {
			if status, ok := payload["status"]; ok && status == "inactive" {
				fmt.Printf("INFO: User with id: %s left room %s\n", userIDString, roomID)
				h.roomManager.HandleUserLeft(roomID, userIDString)
				conn.Close()
				break
			}
		}
	}
}
