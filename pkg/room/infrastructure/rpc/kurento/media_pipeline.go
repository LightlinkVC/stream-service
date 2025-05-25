package kurento

import (
	"encoding/json"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type ElementType string

const (
	WebRTCEndpointName ElementType = "WebRtcEndpoint"
	RTPEndpointName    ElementType = "RtpEndpoint"
	CompositeName      ElementType = "Composite"
)

type MediaPipeline struct {
	MediaElement
}

func (mp *MediaPipeline) createMediaElement(elementType ElementType) (*dto.OnCreateMediaElementResult, error) {
	result, err := mp.kurentoClient.Call("create", map[string]interface{}{
		"type": elementType,
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

	return &response, nil
}

func (mp *MediaPipeline) CreateWebRTCEndpoint() (*WebRTCEndpoint, error) {
	response, err := mp.createMediaElement(WebRTCEndpointName)
	if err != nil {
		return nil, err
	}

	return &WebRTCEndpoint{
		BaseEndpoint: BaseEndpoint{
			MediaElement: MediaElement{
				ID:              response.ElementID,
				sessionID:       response.SessionID,
				kurentoClient:   mp.kurentoClient,
				subscriptionIds: []string{},
			},
		},
	}, nil
}

func (mp *MediaPipeline) CreateRTPEndpoint() (*RTPEndpoint, error) {
	response, err := mp.createMediaElement(RTPEndpointName)
	if err != nil {
		return nil, err
	}

	return &RTPEndpoint{
		BaseEndpoint: BaseEndpoint{
			MediaElement: MediaElement{
				ID:              response.ElementID,
				sessionID:       response.SessionID,
				kurentoClient:   mp.kurentoClient,
				subscriptionIds: []string{},
			},
		},
	}, nil
}

func (mp *MediaPipeline) CreateComposite() (*Composite, error) {
	response, err := mp.createMediaElement(CompositeName)
	if err != nil {
		return nil, err
	}

	return &Composite{
		MediaElement: MediaElement{
			ID:              response.ElementID,
			sessionID:       response.SessionID,
			kurentoClient:   mp.kurentoClient,
			subscriptionIds: []string{},
		},
		mediaPipelineID: mp.ID,
	}, nil
}
