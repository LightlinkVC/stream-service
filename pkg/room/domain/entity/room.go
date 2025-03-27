package entity

import (
	"sync"

	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
)

type Room struct {
	ID             string
	Clients        map[string]*Client // key = userID
	EndpointToUser map[string]string
	Pipeline       *kurento.MediaPipeline
	MU             *sync.RWMutex
}
