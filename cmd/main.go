package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
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

type MediaPipeline struct {
	ID            string
	SessionID     string
	Endpoints     []WebRTCEndpoint
	KurentoClient *KurentoClient
}

type WebRTCEndpoint struct {
	ID            string
	SessionID     string
	KurentoClient *KurentoClient
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
		ID:            response.ElementID,
		SessionID:     response.SessionID,
		Endpoints:     []WebRTCEndpoint{},
		KurentoClient: c,
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
		ID:            response.ElementID,
		SessionID:     response.SessionID,
		KurentoClient: mp.KurentoClient,
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

func (rtcEndpoint *WebRTCEndpoint) SubscribeOnEvent(eventName string) error {
	_, err := rtcEndpoint.KurentoClient.Call("subscribe", map[string]interface{}{
		"object":    rtcEndpoint.ID,
		"type":      eventName,
		"sessionId": rtcEndpoint.SessionID,
	})
	if err != nil {
		return err
	}

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

func (h *BackgroundHandler) HandleSignal(w http.ResponseWriter, r *http.Request) {
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
	router.HandleFunc("/api/room/{roomID}/signal", backHandler.HandleSignal).Methods("POST")

	log.Println("starting server at http://127.0.0.1:8085")
	log.Fatal(http.ListenAndServe(":8085", router))
}
