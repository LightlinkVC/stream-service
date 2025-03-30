package kurento

import (
	"encoding/json"
	"errors"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
)

type HubPortType string

const (
	SourceType HubPortType = "Source"
	SinkType   HubPortType = "Sink"
)

type Composite struct {
	MediaElement
	mediaPipelineID string
	sinkHubPort     *HubPort
	sourceHubPorts  []HubPort
}

func (c *Composite) CreateHubPort(hubPortType HubPortType) (*HubPort, error) {
	result, err := c.kurentoClient.Call("create", map[string]interface{}{
		"type": "HubPort",
		"constructorParams": map[string]interface{}{
			"hub":           c.ID,
			"mediaPipeline": c.mediaPipelineID,
		},
	})
	if err != nil {
		return nil, err
	}

	var response dto.OnCreateMediaElementResult
	if err := json.Unmarshal(result, &response); err != nil {
		return nil, err
	}

	hubPort := HubPort{
		MediaElement: MediaElement{
			ID:              response.ElementID,
			sessionID:       response.SessionID,
			kurentoClient:   c.kurentoClient,
			subscriptionIds: []string{},
		},
		mediaPipelineID: c.mediaPipelineID,
		hubID:           c.ID,
	}

	switch hubPortType {
	case SinkType:
		if c.sinkHubPort != nil {
			return nil, errors.New("err: sink hub port is already created")
		}

		c.sinkHubPort = &hubPort
	case SourceType:
		c.sourceHubPorts = append(c.sourceHubPorts, hubPort)
	}

	return &hubPort, nil
}
