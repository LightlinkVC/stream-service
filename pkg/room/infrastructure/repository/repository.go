package repository

import "github.com/lightlink/stream-service/pkg/room/domain/entity"

type RoomRepository interface {
	GetRoomIDByEndpointID(endpointID string) (string, error)
	GetRoomByID(roomID string) (*entity.Room, error)
	LoadOrStore(room *entity.Room) *entity.Room
	DeleteRoom(roomID string)
	CreateEndpointToRoomMapping(endpointID, roomID string)
	DeleteEndpointToRoomMapping(endpointID string)
}
