package entity

import "github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"

type Client struct {
	UserID              string
	PublisherEndpoint   *kurento.WebRTCEndpoint
	SubscriberEndpoints map[string]*kurento.WebRTCEndpoint
	InputHubPort        *kurento.HubPort
}

func (c *Client) SelfRelease() ([]string, error) {
	releasedEndpointIDs := []string{}

	err := c.PublisherEndpoint.SelfRelease()
	if err != nil {
		return nil, err
	}

	err = c.InputHubPort.SelfRelease()
	if err != nil {
		return nil, err
	}

	for endpointID, endpoint := range c.SubscriberEndpoints {
		err := endpoint.SelfRelease()
		if err != nil {
			return nil, err
		}

		releasedEndpointIDs = append(releasedEndpointIDs, endpointID)
	}

	return releasedEndpointIDs, nil
}
