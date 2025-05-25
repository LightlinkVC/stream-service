package rpc

import (
	"encoding/json"

	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
)

type RPCClient interface {
	Call(method string, params interface{}) (json.RawMessage, error)
	Close() error
	CreateMediaPipeline() (*kurento.MediaPipeline, error)
}
