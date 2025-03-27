package entity

import "fmt"

func UserChannel(groupID, userID string) string {
	return fmt.Sprintf("group:%s:user:%s", groupID, userID)
}

func RoomChannel(roomID string) string {
	return fmt.Sprintf("room:%s", roomID)
}
