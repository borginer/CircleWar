package protobuf

import (
	"CircleWar/core/geom"
	sharedtypes "CircleWar/core/types"
)

func BuildPlayerMoveAction(dir Direction) PlayerAction {
	return PlayerAction{
		Action: &PlayerAction_Move{
			Move: &MoveAction{
				Dir: dir,
			},
		},
	}
}

func BuildPlayerShootAction(target geom.Vector2) PlayerAction {
	return PlayerAction{Action: &PlayerAction_Shoot{
		Shoot: &ShootAction{
			Target: &Position{
				X: target.X,
				Y: target.Y,
			},
		},
	}}
}

func BuildPlayerState(pos geom.Vector2, health sharedtypes.PlayerHealth) PlayerState {
	return PlayerState{
		Pos:    &Position{X: pos.X, Y: pos.Y},
		Health: uint32(health),
	}
}

func BuildBulletState(pos geom.Vector2, size float32) BulletState {
	return BulletState{
		Pos:  &Position{X: pos.X, Y: pos.Y},
		Size: size,
	}
}
