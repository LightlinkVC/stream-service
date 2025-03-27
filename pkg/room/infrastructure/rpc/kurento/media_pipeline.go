package kurento

import (
	"encoding/json"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type MediaPipeline struct {
	MediaElement
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

	var response dto.OnCreateMediaElementResult
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
