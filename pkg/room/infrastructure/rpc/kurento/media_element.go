package kurento

type MediaElement struct {
	ID              string
	SessionID       string
	KurentoClient   *KurentoClient
	SubscriptionIds []string
}

func (me *MediaElement) UnSubscribeOnEvent(eventId string) error {
	_, err := me.KurentoClient.Call("unsubscribe", map[string]interface{}{
		"object":       me.ID,
		"subscription": eventId,
		"sessionId":    me.SessionID,
	})
	if err != nil {
		return err
	}
	return nil
}

func (me *MediaElement) SelfRelease() error {
	for _, subscriptionId := range me.SubscriptionIds {
		err := me.UnSubscribeOnEvent(subscriptionId)
		if err != nil {
			return err
		}
	}

	_, err := me.KurentoClient.Call("release", map[string]interface{}{
		"object":    me.ID,
		"sessionId": me.SessionID,
	})
	if err != nil {
		return err
	}

	return nil
}
