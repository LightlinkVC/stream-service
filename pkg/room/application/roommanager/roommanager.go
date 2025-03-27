package roommanager

import (
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/lightlink/stream-service/pkg/room/domain/entity"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/repository"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/ws"
)

type RoomManager interface {
	HandleUserJoin(roomID, userID string) (*entity.Client, error)
	HandleUserLeft(roomID, userID string) error
	GetClientByUserRoomIds(roomID, userID string) (*entity.Client, error)
}

type DefaultRoomManager struct {
	kurentoClient   rpc.RPCClient
	roomRepo        repository.RoomRepository
	messagingServer ws.MessagingServer
}

func NewDefaultRoomManager(
	kurentoClient rpc.RPCClient,
	roomRepo repository.RoomRepository,
	messagingServer ws.MessagingServer,
) *DefaultRoomManager {
	return &DefaultRoomManager{
		kurentoClient:   kurentoClient,
		roomRepo:        roomRepo,
		messagingServer: messagingServer,
	}
}

func (rm *DefaultRoomManager) getOrCreateRoom(roomID string) (*entity.Room, error) {
	room, err := rm.roomRepo.GetRoomByID(roomID)
	if err == nil {
		return room, nil
	}

	if err != entity.ErrRoomNotFound {
		return nil, err
	}

	pipeline, err := rm.kurentoClient.CreateMediaPipeline()
	if err != nil {
		return nil, fmt.Errorf("failed to create media pipeline: %v", err)
	}

	newRoom := &entity.Room{
		ID:             roomID,
		Clients:        make(map[string]*entity.Client),
		Pipeline:       pipeline,
		EndpointToUser: make(map[string]string),
		MU:             &sync.RWMutex{},
	}

	return rm.roomRepo.LoadOrStore(newRoom), nil
}

func (rm *DefaultRoomManager) HandleUserJoin(roomID, userID string) (*entity.Client, error) {
	room, err := rm.getOrCreateRoom(roomID)
	if err != nil {
		return nil, err
	}

	publisherEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
	if err != nil {
		return nil, err
	}
	rm.roomRepo.CreateEndpointToRoomMapping(publisherEndpoint.ID, roomID)

	// try in rtp
	// data, err := startRTPEndpoint(room.Pipeline, publisherEndpoint)
	// if err != nil {
	// 	return nil, err
	// }
	// go startAudioPipeline(data.AudioPort)
	// fmt.Println("DBG: Started listening through rtp with data: %v", data)
	//

	room.MU.RLock()
	subscriberEndpoints := make(map[string]*kurento.WebRTCEndpoint)
	existingUserIds := []string{}
	for existingUserID, existingClient := range room.Clients {
		existingUserIds = append(existingUserIds, existingUserID)

		subscriberEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
		if err != nil {
			return nil, err
		}
		rm.roomRepo.CreateEndpointToRoomMapping(subscriberEndpoint.ID, roomID)
		room.EndpointToUser[subscriberEndpoint.ID] = userID

		existingClientSubscriberEndpoint, err := room.Pipeline.CreateWebRtcEndpoint()
		if err != nil {
			return nil, err
		}
		rm.roomRepo.CreateEndpointToRoomMapping(existingClientSubscriberEndpoint.ID, roomID)
		room.EndpointToUser[existingClientSubscriberEndpoint.ID] = existingUserID

		err = existingClient.PublisherEndpoint.ConnectToEndpoint(subscriberEndpoint)
		if err != nil {
			return nil, err
		}

		err = publisherEndpoint.ConnectToEndpoint(existingClientSubscriberEndpoint)
		if err != nil {
			return nil, err
		}

		existingClient.SubscriberEndpoints[userID] = existingClientSubscriberEndpoint
		subscriberEndpoints[existingUserID] = subscriberEndpoint
	}
	room.MU.RUnlock()

	client := &entity.Client{
		UserID:              userID,
		PublisherEndpoint:   publisherEndpoint,
		SubscriberEndpoints: subscriberEndpoints,
	}

	room.MU.Lock()
	room.EndpointToUser[publisherEndpoint.ID] = userID
	room.Clients[userID] = client
	existingUserIds = append(existingUserIds, userID)
	room.MU.Unlock()

	log.Printf("INFO: Sent user_joined event to room with id: %s", roomID)
	rm.messagingServer.PublishToRoom(
		roomID,
		map[string]interface{}{
			"type": "user_joined",
			"payload": map[string]interface{}{
				"existing_user_ids": existingUserIds,
			},
		},
	)

	return client, nil
}

func (rm *DefaultRoomManager) HandleUserLeft(roomID, userID string) error {
	room, err := rm.getOrCreateRoom(roomID)
	if err != nil {
		return err
	}

	client, ok := room.Clients[userID]
	if !ok {
		return errors.New("can't find such user")
	}

	rm.roomRepo.DeleteEndpointToRoomMapping(client.PublisherEndpoint.ID)
	room.MU.Lock()
	delete(room.EndpointToUser, client.PublisherEndpoint.ID)

	err = client.PublisherEndpoint.SelfRelease()
	if err != nil {
		return err
	}

	for endpointID, endpoint := range client.SubscriberEndpoints {
		rm.roomRepo.DeleteEndpointToRoomMapping(endpointID)
		delete(room.EndpointToUser, endpointID)

		err := endpoint.SelfRelease()
		if err != nil {
			return err
		}
	}
	delete(room.Clients, userID)
	room.MU.Unlock()

	if len(room.Clients) == 0 {
		fmt.Printf("Room with id: %s is emty now\n", roomID)
		rm.roomRepo.DeleteRoom(roomID)

		return nil
	}

	rm.messagingServer.PublishToRoom(
		roomID,
		map[string]interface{}{
			"type": "user_left",
			"payload": map[string]interface{}{
				"userId": userID,
			},
		},
	)

	return nil
}

func (rm *DefaultRoomManager) GetClientByUserRoomIds(roomID, userID string) (*entity.Client, error) {
	room, err := rm.roomRepo.GetRoomByID(roomID)
	if err != nil {
		return nil, err
	}

	client, ok := room.Clients[userID]
	if !ok {
		return nil, errors.New("user not found")
	}

	return client, nil
}
