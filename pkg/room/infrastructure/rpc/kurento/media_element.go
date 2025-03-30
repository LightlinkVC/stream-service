package kurento

type ConnectableMediaElement interface {
	ConnectTo(subscriber ConnectableMediaElement) error
	GetID() string
}

type MediaElement struct {
	ID              string
	sessionID       string
	kurentoClient   *KurentoClient
	subscriptionIds []string
}

func (me *MediaElement) GetID() string {
	return me.ID
}

func (me *MediaElement) UnSubscribeOnEvent(eventId string) error {
	_, err := me.kurentoClient.Call("unsubscribe", map[string]interface{}{
		"object":       me.ID,
		"subscription": eventId,
		"sessionId":    me.sessionID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (me *MediaElement) SelfRelease() error {
	for _, subscriptionId := range me.subscriptionIds {
		err := me.UnSubscribeOnEvent(subscriptionId)
		if err != nil {
			return err
		}
	}

	_, err := me.kurentoClient.Call("release", map[string]interface{}{
		"object":    me.ID,
		"sessionId": me.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}

func (me *MediaElement) ConnectTo(subscriber ConnectableMediaElement) error {
	_, err := me.kurentoClient.Call("invoke", map[string]interface{}{
		"object":    me.ID,
		"operation": "connect",
		"operationParams": map[string]interface{}{
			"sink": subscriber.GetID(),
		},
		"sessionId": me.sessionID,
	})
	if err != nil {
		return err
	}

	return nil
}
