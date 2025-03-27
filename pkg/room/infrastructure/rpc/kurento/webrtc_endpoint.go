package kurento

import (
	"encoding/json"
	"fmt"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type WebRTCEndpoint struct {
	MediaElement
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

	var response dto.OnSubscribeEventResult
	if err := json.Unmarshal(result, &response); err != nil {
		return err
	}

	rtcEndpoint.SubscriptionIds = append(rtcEndpoint.SubscriptionIds, response.Value)

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

func (rtcEndpoint *WebRTCEndpoint) AddICECandidate(iceCandidate dto.IceCandidate) error {
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

	var response dto.OnSDPOfferProcessResult
	if err := json.Unmarshal(result, &response); err != nil {
		return "", err
	}

	return response.RemoteSDPOffer, nil
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
