package centrifugo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lightlink/stream-service/pkg/room/domain/entity"
)

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

func (c *CentrifugoClient) PublishToUser(groupID, userID string, data interface{}) error {
	return c.Publish(entity.UserChannel(groupID, userID), data)
}

func (c *CentrifugoClient) PublishToRoom(roomID string, data interface{}) error {
	return c.Publish(entity.RoomChannel(roomID), data)
}
