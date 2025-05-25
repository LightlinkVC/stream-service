package entity

import (
	"sync"

	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
)

type Room struct {
	ID                string
	Clients           map[string]*Client // key = userID
	EndpointToUser    map[string]string
	Pipeline          *kurento.MediaPipeline
	MU                *sync.RWMutex
	CompositeHub      *kurento.Composite
	OutputHubPort     *kurento.HubPort
	OutputRTPEndpoint *kurento.RTPEndpoint
}

func (r *Room) SelfRelease() error {
	err := r.Pipeline.SelfRelease()
	if err != nil {
		return err
	}

	err = r.OutputHubPort.SelfRelease()
	if err != nil {
		return err
	}

	err = r.OutputRTPEndpoint.SelfRelease()
	if err != nil {
		return err
	}

	err = r.CompositeHub.SelfRelease()
	if err != nil {
		return err
	}

	return nil
}
