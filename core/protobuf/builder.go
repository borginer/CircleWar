package protobuf

import (
	sharedtypes "CircleWar/core/types"
)

func BuildPlayerState(x, y float32, health sharedtypes.PlayerHealth) PlayerState {
	return PlayerState{
		Pos:    &Position{X: x, Y: y},
		Health: uint32(health),
	}
}

func BuildBulletState(x, y, size float32) BulletState {
	return BulletState{
		Pos:  &Position{X: x, Y: y},
		Size: size,
	}
}
