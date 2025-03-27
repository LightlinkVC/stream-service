package eventhandler

import (
	"fmt"

	"github.com/lightlink/stream-service/pkg/room/domain/dto"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/repository"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/ws"
)

type DefaultEventHandler struct {
	messagingServer ws.MessagingServer
	roomRepository  repository.RoomRepository
}

func NewDefaultEventHandler(messagingServer ws.MessagingServer, roomRepository repository.RoomRepository) *DefaultEventHandler {
	return &DefaultEventHandler{
		messagingServer: messagingServer,
		roomRepository:  roomRepository,
	}
}

func (h *DefaultEventHandler) HandleIceGatheringDone(endpointID string) {
	fmt.Printf("✅ Сбор ICE-кандидатов для %s завершён!\n", endpointID)
}

func (h *DefaultEventHandler) HandleGenericEvent(method string) {
	fmt.Println("⚠ Неизвестное событие от Kurento: ", method)
}

func (h *DefaultEventHandler) HandleIceCandidate(candidate dto.IceCandidate, endpointID string) {
	roomID, err := h.roomRepository.GetRoomIDByEndpointID(endpointID)
	if err != nil {
		fmt.Println("Err with ICE: ", err)
		return
	}

	room, err := h.roomRepository.GetRoomByID(roomID)
	if err != nil {
		fmt.Println("Err with ICE: ", err)
		return
	}

	room.MU.RLock()
	userID := room.EndpointToUser[endpointID]
	room.MU.RUnlock()
	isPublisher := false
	client := room.Clients[userID]

	room.MU.RLock()
	isPublisher = client.PublisherEndpoint.ID == endpointID
	room.MU.RUnlock()

	msgType := "subscriber_ice"
	target := ""
	if isPublisher {
		msgType = "publisher_ice"
	} else {
		room.MU.RLock()
		for targetUserId, subEndpoint := range client.SubscriberEndpoints {
			if subEndpoint.ID == endpointID {
				target = targetUserId
				break
			}
		}
		room.MU.RUnlock()
	}

	fmt.Printf("📡 Новый ICE-кандидат: %+v\n", candidate)

	h.messagingServer.PublishToUser(
		roomID,
		userID,
		map[string]interface{}{
			"type": msgType,
			"payload": map[string]interface{}{
				"candidate": candidate,
				"target":    target,
			},
		},
	)
}
