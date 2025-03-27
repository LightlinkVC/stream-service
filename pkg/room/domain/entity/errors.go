package entity

import "errors"

var (
	ErrRoomNotFound = errors.New("room not found")
	ErrRoomTypeCast = errors.New("room type cast error")
)
