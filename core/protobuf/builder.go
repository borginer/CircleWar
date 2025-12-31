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

func BuildPlayerState(pos geom.Vector2, health sharedtypes.PlayerHealth, playerId uint32) PlayerState {
	return PlayerState{
		Pos:      &Position{X: pos.X, Y: pos.Y},
		Health:   uint32(health),
		PlayerId: playerId,
	}
}

func BuildBulletState(pos geom.Vector2, size float32, ownerId uint32) BulletState {
	return BulletState{
		Pos:     &Position{X: pos.X, Y: pos.Y},
		Size:    size,
		OwnerId: ownerId,
	}
}

func BuildConnectAckMsg(playerId uint) *GameMessage {
	return BuildGameMessage(
		&GameMessage_ConnectAck{&ConnectAck{PlayerId: uint32(playerId)}},
	)
}

func BuildGameMessage(payload isGameMessage_Payload) *GameMessage {
	return &GameMessage{
		Payload: payload,
	}
}
