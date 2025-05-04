package roommanager

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"

	"github.com/lightlink/stream-service/pkg/room/domain/entity"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/repository"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/rpc/kurento"
	"github.com/lightlink/stream-service/pkg/room/infrastructure/ws"
	proto "github.com/lightlink/stream-service/protogen/rtpproxy"
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
	rtpProxyClient  proto.RtpProxyServiceClient
}

func NewDefaultRoomManager(
	kurentoClient rpc.RPCClient,
	roomRepo repository.RoomRepository,
	messagingServer ws.MessagingServer,
	rtpProxyClient proto.RtpProxyServiceClient,
) *DefaultRoomManager {
	return &DefaultRoomManager{
		kurentoClient:   kurentoClient,
		roomRepo:        roomRepo,
		messagingServer: messagingServer,
		rtpProxyClient:  rtpProxyClient,
	}
}

func (rm *DefaultRoomManager) getRoomByID(roomID string) (*entity.Room, error) {
	room, err := rm.roomRepo.GetRoomByID(roomID)
	if err != nil {
		return room, err
	}

	return room, nil
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

	hub, err := pipeline.CreateComposite()
	if err != nil {
		return nil, fmt.Errorf("failed to create hub: %v", err)
	}

	outputRTPEndpoint, err := pipeline.CreateRTPEndpoint()
	if err != nil {
		return nil, fmt.Errorf("failed to create output rtp: %v", err)
	}

	rptEndpointSDPOffer, err := outputRTPEndpoint.GenerateSDPOffer()
	if err != nil {
		return nil, fmt.Errorf("failed to create sdp offer from rtp: %v", err)
	}
	sdpAnwerProto, err := rm.rtpProxyClient.ExchangeSdp(context.Background(),
		&proto.SdpOffer{
			RoomId: roomID,
			Sdp:    rptEndpointSDPOffer,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to exchange sdp: %v", err)
	}
	err = outputRTPEndpoint.ProcessSDPAnswer(sdpAnwerProto.Sdp)
	if err != nil {
		return nil, fmt.Errorf("failed to apply sdp answer for rtp: %v", err)
	}

	fmt.Printf("HUBMETA:%v\n", hub)

	outputHubPort, err := hub.CreateHubPort(kurento.SinkType)
	if err != nil {
		return nil, fmt.Errorf("failed to create output hub port: %v", err)
	}

	err = outputHubPort.ConnectTo(outputRTPEndpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect output hub port to rtp: %v", err)
	}

	newRoom := &entity.Room{
		ID:                roomID,
		Clients:           make(map[string]*entity.Client),
		Pipeline:          pipeline,
		EndpointToUser:    make(map[string]string),
		MU:                &sync.RWMutex{},
		CompositeHub:      hub,
		OutputHubPort:     outputHubPort,
		OutputRTPEndpoint: outputRTPEndpoint,
	}

	return rm.roomRepo.LoadOrStore(newRoom), nil
}

func (rm *DefaultRoomManager) HandleUserJoin(roomID, userID string) (*entity.Client, error) {
	fmt.Println("INFO-BUG: Handling User Joined")
	room, err := rm.getOrCreateRoom(roomID)
	if err != nil {
		return nil, err
	}

	publisherEndpoint, err := room.Pipeline.CreateWebRTCEndpoint()
	if err != nil {
		return nil, err
	}
	inputHubPort, err := room.CompositeHub.CreateHubPort(kurento.SourceType)
	if err != nil {
		return nil, err
	}
	publisherEndpoint.ConnectTo(inputHubPort)
	rm.roomRepo.CreateEndpointToRoomMapping(publisherEndpoint.ID, roomID)

	room.MU.RLock()
	subscriberEndpoints := make(map[string]*kurento.WebRTCEndpoint)
	existingUserIds := []string{}
	for existingUserID, existingClient := range room.Clients {
		existingUserIds = append(existingUserIds, existingUserID)

		subscriberEndpoint, err := room.Pipeline.CreateWebRTCEndpoint()
		if err != nil {
			return nil, err
		}
		rm.roomRepo.CreateEndpointToRoomMapping(subscriberEndpoint.ID, roomID)
		room.EndpointToUser[subscriberEndpoint.ID] = userID

		existingClientSubscriberEndpoint, err := room.Pipeline.CreateWebRTCEndpoint()
		if err != nil {
			return nil, err
		}
		rm.roomRepo.CreateEndpointToRoomMapping(existingClientSubscriberEndpoint.ID, roomID)
		room.EndpointToUser[existingClientSubscriberEndpoint.ID] = existingUserID

		err = existingClient.PublisherEndpoint.ConnectTo(subscriberEndpoint)
		if err != nil {
			return nil, err
		}

		err = publisherEndpoint.ConnectTo(existingClientSubscriberEndpoint)
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
		InputHubPort:        inputHubPort,
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
	room, err := rm.getRoomByID(roomID)
	if err == entity.ErrRoomNotFound {
		fmt.Println("PANIC ERR FATAL: User left not allocated room")
		return err
	}
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

	releasedEndpointIDs, err := client.SelfRelease()
	if err != nil {
		return err
	}

	for _, endpointID := range releasedEndpointIDs {
		rm.roomRepo.DeleteEndpointToRoomMapping(endpointID)
		delete(room.EndpointToUser, endpointID)
	}

	delete(room.Clients, userID)
	room.MU.Unlock()

	isEmpty := len(room.Clients) == 0
	if isEmpty {
		fmt.Printf("Room with id: %s is emty now\n", roomID)
		room.SelfRelease()
		rm.roomRepo.DeleteRoom(roomID)

		stopStreamStatusProto, err := rm.rtpProxyClient.StopStream(context.Background(),
			&proto.RoomIdRequest{
				RoomId: roomID,
			},
		)
		if err != nil {
			return err
		}

		if !stopStreamStatusProto.Status {
			fmt.Printf("ERR: Failed to stop active Gstreamer stream for room with id: %s", roomID)
			return fmt.Errorf("failed to stop active Gstreamer stream for room with id: %s", roomID)
		}

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
