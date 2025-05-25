package kurento

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type EventHandler interface {
	HandleIceCandidate(candidate dto.IceCandidate, endpointID string)
	HandleIceGatheringDone(endpointID string)
	HandleGenericEvent(method string)
}

type KurentoClient struct {
	conn         *websocket.Conn
	pending      map[uint]chan dto.Response
	pendingMutex sync.Mutex
	nextID       uint
	messageQueue chan []byte
	done         chan struct{}
	eventHandler EventHandler
}

func NewKurentoClient(kurentoURL string, eventHandler EventHandler) (*KurentoClient, error) {
	conn, _, err := websocket.DefaultDialer.Dial(kurentoURL, nil)
	if err != nil {
		return nil, err
	}

	client := &KurentoClient{
		conn:         conn,
		pending:      make(map[uint]chan dto.Response),
		nextID:       1,
		messageQueue: make(chan []byte, 100),
		done:         make(chan struct{}),
		eventHandler: eventHandler,
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
			var resp dto.Response
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
			candidate := dto.IceCandidate{
				Candidate:     event.Params.Value.Data.Candidate.Candidate,
				SdpMid:        event.Params.Value.Data.Candidate.SdpMid,
				SdpMLineIndex: uint(event.Params.Value.Data.Candidate.SdpMLineIndex),
			}

			endpointID := event.Params.Value.Data.Source

			c.eventHandler.HandleIceCandidate(
				candidate,
				endpointID,
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

		c.eventHandler.HandleIceGatheringDone(
			event.Params.Object,
		)

	default:
		c.eventHandler.HandleGenericEvent(
			method,
		)
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

	responseChan := make(chan dto.Response, 1)

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

	var response dto.OnCreateMediaElementResult
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	return &MediaPipeline{
		MediaElement: MediaElement{
			ID:              response.ElementID,
			sessionID:       response.SessionID,
			kurentoClient:   c,
			subscriptionIds: []string{},
		},
	}, nil
}
