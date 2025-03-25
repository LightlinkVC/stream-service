package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/tinyzimmer/go-glib/glib"
	"github.com/tinyzimmer/go-gst/gst"
	"github.com/tinyzimmer/go-gst/gst/app"
)

var rooms = sync.Map{} // roomID -> *Room
var endpointToRoom = sync.Map{}

type IceCandidateCallback func(candidate IceCandidate)

type KurentoClient struct {
	conn             *websocket.Conn
	pending          map[uint]chan Response
	pendingMutex     sync.Mutex
	nextID           uint
	messageQueue     chan []byte
	done             chan struct{}
	centrifugoClient *CentrifugoClient
}

type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      uint            `json:"id"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type OnCreateMediaElementResult struct {
	ElementID string `json:"value"`
	SessionID string `json:"sessionId"`
}

type OnSDPOfferProcessResult struct {
	RemoteSDPOffer string `json:"value"`
}

type OnSubscribeEventResult struct {
	Value     string `json:"value"`
	SessionID string `json:"sessionId"`
}

type MediaElement struct {
	ID              string
	SessionID       string
	KurentoClient   *KurentoClient
	SubscriptionIds []string
}

type MediaPipeline struct {
	MediaElement
}

type WebRTCEndpoint struct {
	MediaElement
}

type IceCandidate struct {
	Candidate     string `json:"candidate"`
	SdpMid        string `json:"sdpMid"`
	SdpMLineIndex uint   `json:"sdpMLineIndex"`
}

func NewKurentoClient(url string, centrifugoClient *CentrifugoClient) (*KurentoClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		return nil, err
	}

	client := &KurentoClient{
		conn:             conn,
		pending:          make(map[uint]chan Response),
		nextID:           1,
		messageQueue:     make(chan []byte, 100),
		done:             make(chan struct{}),
		centrifugoClient: centrifugoClient,
	}

	go client.readMessages()
	return client, nil
}

func (c *KurentoClient) readMessages() {
	defer close(c.done)
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			fmt.Println("error due reading messages")
			return
		}

		var rawResponse map[string]interface{}
		if err := json.Unmarshal(message, &rawResponse); err != nil {
			fmt.Println("error unmarshalling rawResponse message due reading messages")
			continue
		}

		fmt.Println("Debug: ", rawResponse)

		if _, ok := rawResponse["id"]; ok {
			var resp Response
			if err := json.Unmarshal(message, &resp); err != nil {
				fmt.Println("error unmarshalling response message due reading messages")
				continue
			}

			c.pendingMutex.Lock()
			if ch, ok := c.pending[resp.ID]; ok {
				ch <- resp
				delete(c.pending, resp.ID)
			}
			c.pendingMutex.Unlock()
		} else if method, ok := rawResponse["method"].(string); ok {
			fmt.Println("Event handled")
			c.handleKurentoEvent(method, message)
		} else {
			fmt.Println("Undefined raw resp: ", rawResponse)
		}
	}
}

func (c *KurentoClient) handleKurentoEvent(method string, message []byte) {
	switch method {
	case "onEvent":
		var event struct {
			JSONRPC string `json:"jsonrpc"`
			Method  string `json:"method"`
			Params  struct {
				Object string `json:"object"`
				Type   string `json:"type"`
				Value  struct {
					Data struct {
						Candidate struct {
							Module          string   `json:"__module__"`
							Type            string   `json:"__type__"`
							Candidate       string   `json:"candidate"`
							SdpMid          string   `json:"sdpMid"`
							SdpMLineIndex   int      `json:"sdpMLineIndex"`
							Tags            []string `json:"tags"`
							Timestamp       int      `json:"timestamp"`
							TimestampMillis int64    `json:"timestampMillis"`
						} `json:"candidate"`
						Source string `json:"source"`
					} `json:"data"`
					Type string `json:"type"`
				} `json:"value"`
			} `json:"params"`
		}

		if err := json.Unmarshal(message, &event); err != nil {
			fmt.Println("Ошибка обработки события onEvent:", err)
			return
		}

		if event.Params.Value.Data.Candidate.Type == "IceCandidate" {
			endpointID := event.Params.Value.Data.Source

			rawRoomID, ok := endpointToRoom.Load(endpointID)
			if !ok {
				log.Printf("Room not found for endpoint: %s", endpointID)
				return
			}
			roomID := rawRoomID.(string)

			rawRoom, ok := rooms.Load(roomID)
			if !ok {
				log.Printf("Room %s not found", roomID)
				return
			}
			room := rawRoom.(*Room)

			room.mu.RLock()
			userID := room.EndpointToUser[endpointID]
			room.mu.RUnlock()

			isPublisher := false
			client := room.Clients[userID]

			room.mu.RLock()
			isPublisher = client.PublisherEndpoint.ID == endpointID
			room.mu.RUnlock()

			msgType := "subscriber_ice"
			target := ""
			if isPublisher {
				msgType = "publisher_ice"
			} else {
				room.mu.RLock()
				for targetUserId, subEndpoint := range client.SubscriberEndpoints {
					if subEndpoint.ID == endpointID {
						target = targetUserId
						break
					}
				}
				room.mu.RUnlock()
			}

			candidate := IceCandidate{
				Candidate:     event.Params.Value.Data.Candidate.Candidate,
				SdpMid:        event.Params.Value.Data.Candidate.SdpMid,
				SdpMLineIndex: uint(event.Params.Value.Data.Candidate.SdpMLineIndex),
			}
			fmt.Printf("📡 Новый ICE-кандидат: %+v\n", candidate)

			// TODO: Тут можно обработать и использовать кандидата, например, отправить его дальше или добавить в RTC соединение
			c.centrifugoClient.Publish(
				userChannel(roomID, userID),
				map[string]interface{}{
					"type": msgType,
					"payload": map[string]interface{}{
						"candidate": candidate,
						"target":    target,
					},
				},
			)
		}

	case "IceGatheringDone":
		var event struct {
			JSONRPC string `json:"jsonrpc"`
			Method  string `json:"method"`
			Params  struct {
				Object string `json:"object"`
			} `json:"params"`
		}

		if err := json.Unmarshal(message, &event); err != nil {
			fmt.Println("Ошибка обработки IceGatheringDone:", err)
			return
		}

		/*TODO*/
		fmt.Println("✅ Сбор ICE-кандидатов завершён!")

	default:
		/*TODO*/
		fmt.Println("⚠ Неизвестное событие от Kurento:", method)
	}
}

func (c *KurentoClient) Call(method string, params interface{}) (json.RawMessage, error) {
	id := c.nextID
	c.nextID++

	request := struct {
		JSONRPC string      `json:"jsonrpc"`
		ID      uint        `json:"id"`
		Method  string      `json:"method"`
		Params  interface{} `json:"params"`
	}{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}

	msg, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	responseChan := make(chan Response, 1)

	c.pendingMutex.Lock()
	c.pending[id] = responseChan

	if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
		return nil, err
	}
	c.pendingMutex.Unlock()

	select {
	case resp := <-responseChan:
		if resp.Error != nil {
			return nil, errors.New(resp.Error.Message)
		}
		return resp.Result, nil
	case <-c.done:
		return nil, errors.New("connection closed")
	}
}

func (c *KurentoClient) Close() error {
	close(c.messageQueue)
	return c.conn.Close()
}

func (c *KurentoClient) CreateMediaPipeline() (*MediaPipeline, error) {
	result, err := c.Call("create", map[string]interface{}{
		"type": "MediaPipeline",
	})
	if err != nil {
		return nil, err
	}

	var response OnCreateMediaElementResult
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	return &MediaPipeline{
		MediaElement: MediaElement{
			ID:              response.ElementID,
			SessionID:       response.SessionID,
			KurentoClient:   c,
			SubscriptionIds: []string{},
		},
	}, nil
}

func (mp *MediaPipeline) CreateWebRtcEndpoint() (*WebRTCEndpoint, error) {
	result, err := mp.KurentoClient.Call("create", map[string]interface{}{
		"type": "WebRtcEndpoint",
		"constructorParams": map[string]interface{}{
			"mediaPipeline": mp.ID,
		},
	})
	if err != nil {
		return nil, err
	}

	var response OnCreateMediaElementResult
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	return &WebRTCEndpoint{
		MediaElement: MediaElement{
			ID:              response.ElementID,
			SessionID:       response.SessionID,
			KurentoClient:   mp.KurentoClient,
			SubscriptionIds: []string{},
		},
	}, nil
}

func (rtcEndpoint *WebRTCEndpoint) ConnectToEndpoint(sub *WebRTCEndpoint) error {
	_, err := rtcEndpoint.KurentoClient.Call("invoke", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"operation": "connect",
		"operationParams": map[string]interface{}{
			"sink": sub.ID,
		},
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (rtcEndpoint *WebRTCEndpoint) ProcessSDPOffer(clientSDPOffer string) (string, error) {
	result, err := rtcEndpoint.KurentoClient.Call("invoke", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"operation": "processOffer",
		"operationParams": map[string]interface{}{
			"offer": clientSDPOffer,
		},
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return "", err
	}

	var response OnSDPOfferProcessResult
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}

	return response.RemoteSDPOffer, nil
}

func (rtcEndpoint *WebRTCEndpoint) AddICECandidate(iceCandidate IceCandidate) error {
	iceCandidateJSON, err := json.Marshal(iceCandidate)
	if err != nil {
		return err
	}

	fmt.Println("Adding ice candidate: " + string(iceCandidateJSON))

	_, err = rtcEndpoint.KurentoClient.Call("invoke", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"operation": "addIceCandidate",
		"operationParams": map[string]interface{}{
			"candidate": iceCandidate,
		},
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (rtcEndpoint *WebRTCEndpoint) GatherICECandidates() error {
	_, err := rtcEndpoint.KurentoClient.Call("invoke", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"operation": "gatherCandidates",
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (me *MediaElement) UnSubscribeOnEvent(eventId string) error {
	_, err := me.KurentoClient.Call("unsubscribe", map[string]interface{}{
		"object":       me.ID,
		"subscription": eventId,
		"sessionId":    me.SessionID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (me *MediaElement) SelfRelease() error {
	for _, subscriptionId := range me.SubscriptionIds {
		err := me.UnSubscribeOnEvent(subscriptionId)
		if err != nil {
			return err
		}
	}

	_, err := me.KurentoClient.Call("release", map[string]interface{}{
		"object":    me.ID,
		"sessionId": me.SessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (rtcEndpoint *WebRTCEndpoint) SubscribeOnEvent(eventName string) error {
	result, err := rtcEndpoint.KurentoClient.Call("subscribe", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"type":      eventName,
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return err
	}

	var response OnSubscribeEventResult
	if err := json.Unmarshal(result, &response); err != nil {
		return err
	}

	rtcEndpoint.SubscriptionIds = append(rtcEndpoint.SubscriptionIds, response.Value)

	return nil
}

func kurentoBasicTest() {
	fmt.Printf("Attempting to connect to Kurento Media Server at ws://%s:%s/kurento\n",
		os.Getenv("KURENTO_MEDIA_SERVER_HOST"), os.Getenv("KURENTO_MEDIA_SERVER_PORT"))

	client, err := NewKurentoClient(
		fmt.Sprintf(
			"ws://%s:%s/kurento",
			os.Getenv("KURENTO_MEDIA_SERVER_HOST"),
			os.Getenv("KURENTO_MEDIA_SERVER_PORT"),
		),
		&CentrifugoClient{},
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	fmt.Println("connected")

	pipeline, err := client.CreateMediaPipeline()
	if err != nil {
		panic(err)
	}

	endpoint1, err := pipeline.CreateWebRtcEndpoint()
	if err != nil {
		panic(err)
	}

	endpoint2, err := pipeline.CreateWebRtcEndpoint()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Using session with id: %s\n", endpoint1.SessionID)
	fmt.Printf("Created WebRTC endpoint: %s\n", endpoint1.ID)

	err = endpoint1.ConnectToEndpoint(endpoint2)
	if err != nil {
		panic(err)
	}
	fmt.Println("Endpoints connected")

	offer := "v=0\r\no=- 1113439286307267998 3 IN IP4 127.0.0.1\r\ns=-\r\nt=0 0\r\na=extmap-allow-mixed\r\na=msid-semantic: WMS\r\n"

	income, err := endpoint1.ProcessSDPOffer(offer)
	if err != nil {
		panic(err)
	}

	fmt.Println("Remote SDP: ", income)

	err = endpoint1.SubscribeOnEvent("OnIceCandidate")
	if err != nil {
		panic(err)
	}

	err = endpoint1.GatherICECandidates()
	if err != nil {
		panic(err)
	}

	fmt.Println("Start gathering")

	time.Sleep(100 * time.Second)
}

///////////

type Client struct {
	UserID              string
	PublisherEndpoint   *WebRTCEndpoint
	SubscriberEndpoints map[string]*WebRTCEndpoint
}

type Room struct {
	ID             string
	Clients        map[string]*Client // key = userID
	EndpointToUser map[string]string
	Pipeline       *MediaPipeline
	mu             *sync.RWMutex
}

type CentrifugoClient struct {
	httpClient *http.Client
	apiURL     string
	apiKey     string
}

func NewCentrifugoClient(apiURL, apiKey string) *CentrifugoClient {
	return &CentrifugoClient{
		httpClient: &http.Client{Timeout: 5 * time.Second},
		apiURL:     apiURL,
		apiKey:     apiKey,
	}
}

type PublishResponse struct {
	Error  *PublishErrorResponse   `json:"error,omitempty"`
	Result *PublishSuccessResponse `json:"result,omitempty"`
}

type PublishErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type PublishSuccessResponse struct {
	Offset int    `json:"offset"`
	Epoch  string `json:"epoch"`
}

func (c *CentrifugoClient) Publish(channel string, data interface{}) error {
	payload := map[string]interface{}{
		"channel": channel,
		"data":    data,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", c.apiURL+"/api/publish", bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("centrifugo error: %s", resp.Status)
	}

	var publishResponse PublishResponse
	err = json.NewDecoder(resp.Body).Decode(&publishResponse)
	if err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	if publishResponse.Error != nil {
		return fmt.Errorf("error: Code %d, Message: %s", publishResponse.Error.Code, publishResponse.Error.Message)
	}

	fmt.Println("Successfully published in channel")

	return nil
}

type SignalingHandler struct {
	centrifugo *CentrifugoClient
	kurento    *KurentoClient
}

func NewSignalingHandler(centrifugoURL, centrifugoToken, kurentoURL string) (*SignalingHandler, error) {
	centrifugoClient := NewCentrifugoClient(centrifugoURL, centrifugoToken)

	kurentoClient, err := NewKurentoClient(kurentoURL, centrifugoClient)
	if err != nil {
		return nil, err
	}

	return &SignalingHandler{
		centrifugo: centrifugoClient,
		kurento:    kurentoClient,
	}, nil
}

// Создана обвязка для куренты и центрифуги
//
// Метод для получения/создания румы
// Подписка на евенты румы
func (h *SignalingHandler) GetOrCreateRoom(roomID string) (*Room, error) {
	// Пытаемся получить существующую комнату
	if roomIface, ok := rooms.Load(roomID); ok {
		return roomIface.(*Room), nil
	}

	// Создаем новую медиапайплайн через Kurento
	pipeline, err := h.kurento.CreateMediaPipeline()
	if err != nil {
		return nil, fmt.Errorf("failed to create media pipeline: %v", err)
	}

	// Инициализируем новую комнату
	newRoom := &Room{
		ID:             roomID,
		Clients:        make(map[string]*Client),
		Pipeline:       pipeline,
		EndpointToUser: make(map[string]string),
		mu:             &sync.RWMutex{},
	}

	// Сохраняем комнату с проверкой на конкурентное создание
	roomIface, loaded := rooms.LoadOrStore(roomID, newRoom)
	if loaded {
		// Если кто-то уже создал комнату параллельно - возвращаем существующую
		return roomIface.(*Room), nil
	}

	return newRoom, nil
}

func roomChannel(roomID string) string {
	return fmt.Sprintf("room:%s", roomID)
}
func userChannel(groupID, userID string) string {
	return fmt.Sprintf("group:%s:user:%s", groupID, userID)
}

// Хендлер на присоединение юзера к руме
func (h *SignalingHandler) HandleUserJoin(roomID, userID string) (*Client, error) {
	room, err := h.GetOrCreateRoom(roomID)
	if err != nil {
		return nil, err
	}

	publisherEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
	if err != nil {
		return nil, err
	}
	endpointToRoom.Store(publisherEndpoint.ID, roomID)

	// try in rtp
	data, err := startRTPEndpoint(room.Pipeline, publisherEndpoint)
	if err != nil {
		return nil, err
	}
	go startAudioPipeline(data.AudioPort)
	fmt.Println("DBG: Started listening through rtp with data: %v", data)

	room.mu.RLock()
	subscriberEndpoints := make(map[string]*WebRTCEndpoint)
	existingUserIds := []string{}
	for existingUserID, existingClient := range room.Clients {
		existingUserIds = append(existingUserIds, existingUserID)

		subscriberEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
		if err != nil {
			return nil, err
		}
		endpointToRoom.Store(subscriberEndpoint.ID, roomID)
		room.EndpointToUser[subscriberEndpoint.ID] = userID

		existingClientSubscriberEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
		if err != nil {
			return nil, err
		}
		endpointToRoom.Store(existingClientSubscriberEndpoint.ID, roomID)
		room.EndpointToUser[existingClientSubscriberEndpoint.ID] = existingUserID

		err = existingClient.PublisherEndpoint.ConnectToEndpoint(subscriberEndpoint)
		if err != nil {
			return nil, err
		}

		err = publisherEndpoint.ConnectToEndpoint(existingClientSubscriberEndpoint)
		if err != nil {
			return nil, err
		}

		existingClient.SubscriberEndpoints[userID] = existingClientSubscriberEndpoint
		subscriberEndpoints[existingUserID] = subscriberEndpoint
	}
	room.mu.RUnlock()

	client := &Client{
		UserID:              userID,
		PublisherEndpoint:   publisherEndpoint,
		SubscriberEndpoints: subscriberEndpoints,
	}

	room.mu.Lock()
	room.EndpointToUser[publisherEndpoint.ID] = userID
	room.Clients[userID] = client
	existingUserIds = append(existingUserIds, userID)
	room.mu.Unlock()

	log.Printf("INFO: Sent user_joined event to room with id: %s", roomID)
	h.centrifugo.Publish(
		roomChannel(roomID),
		map[string]interface{}{
			"type": "user_joined",
			"payload": map[string]interface{}{
				"existing_user_ids": existingUserIds,
			},
		},
	)

	return client, nil
}

type SignalingRequest struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (h *BackgroundHandler) SingalHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling incoming client signal")

	userID := r.Header.Get("X-User-ID")
	roomID := mux.Vars(r)["roomID"]

	var req SignalingRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	roomIface, ok := rooms.Load(roomID)
	if !ok {
		http.Error(w, "room not found", http.StatusNotFound)
		return
	}
	room := roomIface.(*Room)
	client, ok := room.Clients[userID]
	fmt.Println("Handling signal for client: ", client)
	if !ok {
		http.Error(w, "client not found", http.StatusNotFound)
		return
	}

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
			fmt.Println("Cant gen ans")
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

		h.handler.centrifugo.Publish(
			userChannel(roomID, userID),
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
			TargetUserId string       `json:"targetUserId"`
			Candidate    IceCandidate `json:"candidate"`
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
		err = h.handler.centrifugo.Publish(
			userChannel(roomID, userID),
			map[string]interface{}{
				"type":    "answer",
				"payload": answer,
			},
		)
		if err != nil {
			log.Printf("Centrifugo error: %v", err)
		}

	case "ice":
		var candidate IceCandidate
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

func generateUserToken(secret, userID, roomID string) (string, error) {
	claims := jwt.MapClaims{
		"sub": userID,
		"exp": time.Now().Add(time.Hour * 10).Unix(),
		"channels": []string{
			roomChannel(roomID),
			userChannel(roomID, userID),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

type BackgroundHandler struct {
	handler *SignalingHandler
}

func (h *BackgroundHandler) JoinHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling incoming join request")
	userIDString := r.Header.Get("X-User-ID")
	roomID := mux.Vars(r)["roomID"]

	_, err := h.handler.HandleUserJoin(roomID, userIDString)
	if err != nil {
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
}

func (h *BackgroundHandler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Handling incoming info request")
	userIDString := r.Header.Get("X-User-ID")
	roomID := mux.Vars(r)["roomID"]

	token, err := generateUserToken(os.Getenv("TOKEN_KEY"), userIDString, roomID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]interface{}{
		"token": token,
		"channels": map[string]string{
			"room": roomChannel(roomID),
			"user": userChannel(roomID, userIDString),
		},
	})
}

func (h *SignalingHandler) handleUserLeft(roomID, userID string) error {
	roomIface, ok := rooms.Load(roomID)
	if !ok {
		return errors.New("can't find such room")
	}
	room := roomIface.(*Room)

	client, ok := room.Clients[userID]
	if !ok {
		return errors.New("can't find such user")
	}

	endpointToRoom.Delete(client.PublisherEndpoint.ID)
	room.mu.Lock()
	delete(room.EndpointToUser, client.PublisherEndpoint.ID)

	err := client.PublisherEndpoint.SelfRelease()
	if err != nil {
		return err
	}

	for endpointID, endpoint := range client.SubscriberEndpoints {
		endpointToRoom.Delete(endpointID)
		delete(room.EndpointToUser, endpointID)

		err := endpoint.SelfRelease()
		if err != nil {
			return err
		}
	}
	delete(room.Clients, userID)
	room.mu.Unlock()

	if len(room.Clients) == 0 {
		fmt.Printf("Room with id: %s is emty now\n", roomID)
		rooms.Delete(roomID)

		return nil
	}

	h.centrifugo.Publish(
		roomChannel(roomID),
		map[string]interface{}{
			"type": "user_left",
			"payload": map[string]interface{}{
				"userId": userID,
			},
		},
	)

	return nil
}

func (h *BackgroundHandler) ConnectionStateHandler(w http.ResponseWriter, r *http.Request) {
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
			h.handler.handleUserLeft(roomID, userIDString)
			conn.Close()

			break
		}

		var payload map[string]string
		if err := json.Unmarshal(msg, &payload); err == nil {
			if status, ok := payload["status"]; ok && status == "inactive" {
				fmt.Printf("INFO: User with id: %s left room %s\n", userIDString, roomID)
				h.handler.handleUserLeft(roomID, userIDString)
				conn.Close()
				break
			}
		}
	}
}

func testGST() {
	fmt.Printf("Attempting to connect to Kurento Media Server at ws://%s:%s/kurento\n",
		os.Getenv("KURENTO_MEDIA_SERVER_HOST"), os.Getenv("KURENTO_MEDIA_SERVER_PORT"))

	client, err := NewKurentoClient(
		fmt.Sprintf(
			"ws://%s:%s/kurento",
			os.Getenv("KURENTO_MEDIA_SERVER_HOST"),
			os.Getenv("KURENTO_MEDIA_SERVER_PORT"),
		),
		&CentrifugoClient{},
	)
	if err != nil {
		panic(err)
	}
	defer client.Close()
	fmt.Println("connected")

	pipeline, err := client.CreateMediaPipeline()
	if err != nil {
		panic(err)
	}

	rtcEndpoint, err := pipeline.CreateWebRtcEndpoint()
	if err != nil {
		panic(err)
	}

	resp, err := pipeline.KurentoClient.Call(
		"create",
		map[string]interface{}{
			"type": "RtpEndpoint",
			"constructorParams": map[string]interface{}{
				"mediaPipeline": pipeline.ID,
			},
			"sessionId": pipeline.SessionID,
		},
	)

	if err != nil {
		panic(err)
	}

	var res struct {
		Value     string `json:"value"`
		SessionID string `json:"sessionId"`
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		panic(err)
	}

	_, err = pipeline.KurentoClient.Call(
		"invoke",
		map[string]interface{}{
			"object":    rtcEndpoint.ID,
			"operation": "connect",
			"operationParams": map[string]interface{}{
				"sink": res.Value,
			},
			"sessionId": pipeline.SessionID,
		},
	)
	if err != nil {
		panic(err)
	}

	resp2, err := pipeline.KurentoClient.Call(
		"invoke",
		map[string]interface{}{
			"object":    res.Value,
			"operation": "generateOffer",
			"sessionId": pipeline.SessionID,
		},
	)
	if err != nil {
		panic(err)
	}

	var res2 struct {
		Value     string `json:"value"`
		SessionID string `json:"sessionId"`
	}

	err = json.Unmarshal(resp2, &res2)
	if err != nil {
		panic(err)
	}

	fmt.Println(res2.Value)

	ipRegex := regexp.MustCompile(`c=IN IP4 (\S+)`)
	audioPortRegex := regexp.MustCompile(`m=audio (\d+)`)
	videoPortRegex := regexp.MustCompile(`m=video (\d+)`)

	// Извлекаем данные
	rtpHost := ipRegex.FindStringSubmatch(res2.Value)[1]
	audioPort := audioPortRegex.FindStringSubmatch(res2.Value)[1]
	videoPort := videoPortRegex.FindStringSubmatch(res2.Value)[1]

	// Выводим данные
	fmt.Printf("rtpHost := \"%s\"\n", rtpHost)
	fmt.Printf("audioPort := %s\n", audioPort)
	fmt.Printf("videoPort := %s\n", videoPort)

	gst.Init(nil)

	mainLoop := glib.NewMainLoop(glib.MainContextDefault(), false)

	pipelineStr := fmt.Sprintf(
		"udpsrc port=%s caps=\"application/x-rtp, media=audio\" ! "+
			"rtpjitterbuffer ! "+
			"rtpopusdepay ! "+
			"opusdec ! "+
			"audioconvert ! "+
			"audioresample ! "+
			"appsink name=audio_sink", // Будем читать данные отсюда
		audioPort)

	pipe, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		panic(fmt.Sprintf("Failed to create pipeline: %v", err))
	}

	// Получение appsink
	el, err := pipe.GetElementByName("audio_sink")
	if err != nil {
		panic(err)
	}

	appsink := app.SinkFromElement(el)
	if err != nil {
		panic(err)
	}

	// Чтение аудиоданных
	go func() {
		for {
			// Получаем сэмпл из appsink
			sample := appsink.PullSample()
			if sample == nil {
				break
			}

			// Получаем буфер с аудиоданными
			buffer := sample.GetBuffer()
			data := buffer.Map(gst.MapRead).Data()
			defer buffer.Unmap()

			// Выводим данные в консоль (можно преобразовать в строку для удобства)
			fmt.Printf("Received audio data: %v\n", data)
		}
	}()

	pipe.GetPipelineBus().AddWatch(func(msg *gst.Message) bool {
		switch msg.Type() {
		case gst.MessageError:
			err := msg.ParseError()
			fmt.Println("ERROR:", err.Error())
			if debug := err.DebugString(); debug != "" {
				fmt.Println("DEBUG:", debug)
			}
			mainLoop.Quit()
		default:
			fmt.Println(msg)
		}
		return true
	})

	pipe.SetState(gst.StatePlaying)
	mainLoop.Run()
}

func createPipeline(audioPort string) (*gst.Pipeline, error) {
	gst.Init(nil)

	// Создаем пайплайн из строки
	// pipelineStr := fmt.Sprintf(
	// 	"udpsrc port=%s caps=\"application/x-rtp,media=audio,payload=96,encoding-name=OPUS,clock-rate=48000\" ! "+
	// 		"rtpjitterbuffer latency=200 drop-on-latency=true ! "+
	// 		"rtpopusdepay ! "+
	// 		"opusdec plc=true ! "+
	// 		"audioconvert ! "+
	// 		"audioresample ! "+
	// 		"appsink name=audio_sink sync=false async=false max-buffers=1 drop=true",
	// 	audioPort)

	pipelineStr := fmt.Sprintf(
		"udpsrc port=%s caps=\"application/x-rtp,media=audio,payload=96,encoding-name=OPUS,clock-rate=48000\" ! "+
			"rtpjitterbuffer latency=200 ! "+
			"rtpopusdepay ! "+
			"opusdec plc=true ! "+
			"audioconvert ! "+
			"audioresample ! "+
			"tee name=t ! "+
			"queue ! "+
			"appsink name=audio_sink sync=false async=false "+
			"t. ! queue ! wavenc ! filesink location=test.wav",
		audioPort)

	pipe, err := gst.NewPipelineFromString(pipelineStr)
	if err != nil {
		fmt.Println("GST: failed to create pipeline")
		return nil, fmt.Errorf("failed to create pipeline: %v", err)
	}

	// Получаем appsink
	element, err := pipe.GetElementByName("audio_sink")
	if err != nil {
		fmt.Println("GST: failed to GetElementByName")
		return nil, err
	}
	appsink := app.SinkFromElement(element)

	// Устанавливаем caps для аудио
	appsink.SetCaps(gst.NewCapsFromString(
		"audio/x-raw, format=S16LE, layout=interleaved, rate=48000, channels=2",
	))

	// Устанавливаем обработчики
	appsink.SetCallbacks(&app.SinkCallbacks{
		NewSampleFunc: func(sink *app.Sink) gst.FlowReturn {
			sample := sink.PullSample()
			if sample == nil {
				return gst.FlowEOS
			}

			buffer := sample.GetBuffer()
			if buffer == nil {
				return gst.FlowError
			}

			samples := buffer.Map(gst.MapRead).AsInt16LESlice()
			defer buffer.Unmap()

			// Обработка аудиоданных
			var square float64
			for _, i := range samples {
				square += float64(i * i)
			}
			rms := math.Sqrt(square / float64(len(samples)))
			fmt.Println("rms:", rms)

			return gst.FlowOK
		},
	})

	return pipe, nil
}

func handleMessage(msg *gst.Message) error {
	switch msg.Type() {
	// case gst.MessageEOS:
	// 	return fmt.Errorf("end of stream")
	case gst.MessageError:
		return msg.ParseError()
	case gst.MessageWarning:
		warn := msg.ParseWarning()
		fmt.Printf("WARNING: %s\nDebug: %s\n", warn.Error(), warn.DebugString())
	}
	return nil
}

func startAudioPipeline(audioPort string) error {
	// Создаем пайплайн
	pipeline, err := createPipeline(audioPort)
	if err != nil {
		fmt.Println("GST: failed to create pipe")
		return err
	}

	// Запускаем пайплайн
	pipeline.SetState(gst.StatePlaying)

	// Настраиваем обработку сигналов
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		fmt.Println("\nReceived interrupt, shutting down...")
		pipeline.SendEvent(gst.NewEOSEvent())
	}()

	// Получаем bus для обработки сообщений
	bus := pipeline.GetPipelineBus()

	// Основной цикл обработки сообщений
	for {
		msg := bus.TimedPop(time.Duration(-1))
		if msg == nil {
			break
		}
		if err := handleMessage(msg); err != nil {
			// Остановка пайплайна при ошибке
			pipeline.SetState(gst.StateNull)
			fmt.Println("GST: error msg: ", err.Error())
			return err
		}
	}

	// Корректное завершение
	pipeline.SetState(gst.StateNull)
	return nil
}

type StreamData struct {
	RtpHost   string
	AudioPort string
	VideoPort string
}

// Функция для работы с Kurento API и получения необходимых данных
func startRTPEndpoint(pipeline *MediaPipeline, rtcEndpoint *WebRTCEndpoint) (*StreamData, error) {
	// 1. Создание RtpEndpoint
	resp, err := pipeline.KurentoClient.Call(
		"create",
		map[string]interface{}{
			"type": "RtpEndpoint",
			"constructorParams": map[string]interface{}{
				"mediaPipeline": pipeline.ID,
			},
			"sessionId": pipeline.SessionID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create RtpEndpoint: %v", err)
	}

	var res struct {
		Value     string `json:"value"`
		SessionID string `json:"sessionId"`
	}

	err = json.Unmarshal(resp, &res)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %v", err)
	}

	// 2. Подключаем sink
	_, err = pipeline.KurentoClient.Call(
		"invoke",
		map[string]interface{}{
			"object":    rtcEndpoint.ID,
			"operation": "connect",
			"operationParams": map[string]interface{}{
				"sink": res.Value,
			},
			"sessionId": pipeline.SessionID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %v", err)
	}

	// 3. Генерация offer
	resp2, err := pipeline.KurentoClient.Call(
		"invoke",
		map[string]interface{}{
			"object":    res.Value,
			"operation": "generateOffer",
			"sessionId": pipeline.SessionID,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to generate offer: %v", err)
	}

	var res2 struct {
		Value     string `json:"value"`
		SessionID string `json:"sessionId"`
	}

	err = json.Unmarshal(resp2, &res2)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal response2: %v", err)
	}

	// 4. Извлечение данных из offer
	ipRegex := regexp.MustCompile(`c=IN IP4 (\S+)`)
	audioPortRegex := regexp.MustCompile(`m=audio (\d+)`)
	videoPortRegex := regexp.MustCompile(`m=video (\d+)`)

	// Извлекаем данные
	rtpHost := ipRegex.FindStringSubmatch(res2.Value)[1]
	audioPort := audioPortRegex.FindStringSubmatch(res2.Value)[1]
	videoPort := videoPortRegex.FindStringSubmatch(res2.Value)[1]

	sdpAnswer := fmt.Sprintf(`v=0
		o=- 0 0 IN IP4 %s
		s=-
		c=IN IP4 %s
		t=0 0
		m=audio %s RTP/AVP 96
		a=rtpmap:96 opus/48000/2
		a=sendrecv`, "stream_service", "stream_service", audioPort)

	fmt.Println("CHECK: ", sdpAnswer)

	// Отправляем SDP Answer обратно в Kurento
	_, err = pipeline.KurentoClient.Call("invoke", map[string]interface{}{
		"object":    res.Value,
		"operation": "processAnswer",
		"operationParams": map[string]interface{}{
			"answer": sdpAnswer,
		},
		"sessionId": pipeline.SessionID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to process answer: %v", err)
	}

	// Возвращаем структуру с результатами
	return &StreamData{
		RtpHost:   rtpHost,
		AudioPort: audioPort,
		VideoPort: videoPort,
	}, nil
}

/*Пофиксить gst работу, кажется, она блочит поток*/

func main() {
	centrifugoKey := os.Getenv("CENTRIFUGO_HTTP_API_KEY")

	handler, err := NewSignalingHandler(
		"http://centrifugo:8000",
		centrifugoKey,
		fmt.Sprintf(
			"ws://%s:%s/kurento",
			os.Getenv("KURENTO_MEDIA_SERVER_HOST"),
			os.Getenv("KURENTO_MEDIA_SERVER_PORT"),
		),
	)
	if err != nil {
		panic(err)
	}

	backHandler := BackgroundHandler{
		handler: handler,
	}

	router := mux.NewRouter()

	router.HandleFunc("/api/room/{roomID}", backHandler.JoinHandler).Methods("POST")
	router.HandleFunc("/api/room/{roomID}/info", backHandler.InfoHandler).Methods("GET")
	router.HandleFunc("/api/room/{roomID}/signal", backHandler.SingalHandler).Methods("POST")
	router.HandleFunc("/ws/api/room/{roomID}/trace", backHandler.ConnectionStateHandler).Methods("GET")

	log.Println("starting server at http://127.0.0.1:8085")
	log.Fatal(http.ListenAndServe(":8085", router))
}
