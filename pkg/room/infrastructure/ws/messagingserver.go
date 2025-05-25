package ws

type MessagingServer interface {
	Publish(channel string, data interface{}) error
	PublishToUser(groupID, userID string, data interface{}) error
	PublishToRoom(roomID string, data interface{}) error
}
