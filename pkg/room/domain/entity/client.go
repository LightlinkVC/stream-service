package entity

import "github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"

type Client struct {
	UserID              string
	PublisherEndpoint   *kurento.WebRTCEndpoint
	SubscriberEndpoints map[string]*kurento.WebRTCEndpoint
}
