package kurento

import (
	"encoding/json"
	"fmt"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type Endpoint interface {
	ProcessSDPOffer(string) (string, error)
	AddICECandidate(dto.IceCandidate) error
}

type BaseEndpoint struct {
	MediaElement
}

func (endpoint *BaseEndpoint) SubscribeOnEvent(eventName string) error {
	result, err := endpoint.kurentoClient.Call("subscribe", map[string]interface{}{
		"object":    endpoint.ID,
		"type":      eventName,
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return err
	}

	var response dto.OnSubscribeEventResult
	if err := json.Unmarshal(result, &response); err != nil {
		return err
	}

	endpoint.subscriptionIds = append(endpoint.subscriptionIds, response.Value)

	return nil
}

func (endpoint *BaseEndpoint) GatherICECandidates() error {
	_, err := endpoint.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    endpoint.ID,
		"operation": "gatherCandidates",
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (endpoint *BaseEndpoint) AddICECandidate(iceCandidate dto.IceCandidate) error {
	iceCandidateJSON, err := json.Marshal(iceCandidate)
	if err != nil {
		return err
	}

	fmt.Println("Adding ice candidate: " + string(iceCandidateJSON))

	_, err = endpoint.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    endpoint.ID,
		"operation": "addIceCandidate",
		"operationParams": map[string]interface{}{
			"candidate": iceCandidate,
		},
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (endpoint *BaseEndpoint) GenerateSDPOffer() (string, error) {
	result, err := endpoint.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    endpoint.ID,
		"operation": "generateOffer",
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return "", err
	}

	var response dto.OnGenerateSDPOfferResult
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}

	return response.SDPOffer, nil
}

func (endpoint *BaseEndpoint) ProcessSDPOffer(clientSDPOffer string) (string, error) {
	result, err := endpoint.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    endpoint.ID,
		"operation": "processOffer",
		"operationParams": map[string]interface{}{
			"offer": clientSDPOffer,
		},
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return "", err
	}

	var response dto.OnSDPOfferProcessResult
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}

	return response.RemoteSDPOffer, nil
}

func (endpoint *BaseEndpoint) ProcessSDPAnswer(clientSDPAnswer string) error {
	_, err := endpoint.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    endpoint.ID,
		"operation": "processAnswer",
		"operationParams": map[string]interface{}{
			"answer": clientSDPAnswer,
		},
		"sessionId": endpoint.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}
