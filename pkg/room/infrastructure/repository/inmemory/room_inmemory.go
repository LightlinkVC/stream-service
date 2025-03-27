package inmemory

import (
	"sync"

	"github.com/lightlink/stream-service/pkg/room/domain/entity"
)

type InMemoryRoomRepository struct {
	endpointToRoom *sync.Map
	rooms          *sync.Map
}

func NewInMemoryRoomRepository(endpointToRoom *sync.Map, rooms *sync.Map) *InMemoryRoomRepository {
	return &InMemoryRoomRepository{
		endpointToRoom: endpointToRoom,
		rooms:          rooms,
	}
}

func (repo *InMemoryRoomRepository) GetRoomIDByEndpointID(endpointID string) (string, error) {
	rawRoomID, ok := repo.endpointToRoom.Load(endpointID)
	if !ok {
		return "", entity.ErrRoomNotFound
	}

	roomID, ok := rawRoomID.(string)
	if !ok {
		return "", entity.ErrRoomTypeCast
	}

	return roomID, nil
}

func (repo *InMemoryRoomRepository) GetRoomByID(roomID string) (*entity.Room, error) {
	rawRoom, ok := repo.rooms.Load(roomID)
	if !ok {
		return nil, entity.ErrRoomNotFound
	}

	room, ok := rawRoom.(*entity.Room)
	if !ok {
		return nil, entity.ErrRoomTypeCast
	}

	return room, nil
}

func (repo *InMemoryRoomRepository) LoadOrStore(newRoom *entity.Room) *entity.Room {
	roomIface, loaded := repo.rooms.LoadOrStore(newRoom.ID, newRoom)
	if loaded {
		return roomIface.(*entity.Room)
	}

	return newRoom
}

func (repo *InMemoryRoomRepository) CreateEndpointToRoomMapping(endpointID, roomID string) {
	repo.endpointToRoom.Store(endpointID, roomID)
}

func (repo *InMemoryRoomRepository) DeleteEndpointToRoomMapping(endpointID string) {
	repo.endpointToRoom.Delete(endpointID)
}

func (repo *InMemoryRoomRepository) DeleteRoom(roomID string) {
	repo.rooms.Delete(roomID)
}
