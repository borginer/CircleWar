package protobuf

import (
	"CircleWar/core/geom"
	stypes "CircleWar/core/types"
)

func BuildPlayerMoveAction(ma *stypes.MoveAction) *PlayerAction {
	return &PlayerAction{
		Action: &PlayerAction_Move{
			Move: &MoveAction{
				Dir: Direction(ma.Dir),
			},
		},
	}
}

func BuildPlayerShootAction(sa *stypes.ShootAction) *PlayerAction {
	return &PlayerAction{Action: &PlayerAction_Shoot{
		Shoot: &ShootAction{
			Target: &Position{
				X: sa.Target.X,
				Y: sa.Target.Y,
			},
		},
	}}
}

func BuildPlayerState(pos geom.Vector2, health stypes.PlayerHealth, playerId uint32) PlayerState {
	return PlayerState{
		Pos:      &Position{X: pos.X, Y: pos.Y},
		Health:   float32(health),
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

func BuildWorldState(ws *stypes.WorldState) *GameMessage {
	worldState := &WorldState{}

	for _, player := range ws.Players {
		pbPlayer := BuildPlayerState(player.Pos, stypes.PlayerHealth(player.Health), uint32(player.Id))
		worldState.Players = append(worldState.Players, &pbPlayer)
	}

	for _, bullet := range ws.Bullets {
		pbBullet := BuildBulletState(bullet.Pos, bullet.Size, uint32(bullet.OwnerId))
		worldState.Bullets = append(worldState.Bullets, &pbBullet)
	}

	worldState.TickNum = ws.TickNum

	return WrapGameMessage(
		&GameMessage_World{worldState},
	)
}

func BuildPlayerInput(pi *stypes.PlayerInput) *GameMessage {
	playerInput := &PlayerInput{
		PlayerId:      pi.PlayerId,
		PlayerActions: []*PlayerAction{},
	}
	for _, act := range pi.Actions {
		switch inner := act.(type) {
		case *stypes.MoveAction:
			playerInput.PlayerActions = append(playerInput.PlayerActions, BuildPlayerMoveAction(inner))
		case *stypes.ShootAction:
			playerInput.PlayerActions = append(playerInput.PlayerActions, BuildPlayerShootAction(inner))
		}
	}
	return WrapGameMessage(
		&GameMessage_PlayerInput{PlayerInput: playerInput},
	)
}

func BuildConnectAck(ca *stypes.ConnectAck) *GameMessage {
	return WrapGameMessage(
		&GameMessage_ConnectAck{&ConnectAck{PlayerId: ca.PlayerId}},
	)
}

func BuildConnectRequest(cr *stypes.ConnectRequest) *GameMessage {
	return WrapGameMessage(
		&GameMessage_ConnectRequest{&ConnectRequest{GameName: cr.GameName}},
	)
}

func BuildReconnectRequest(rr *stypes.ReconnectRequest) *GameMessage {
	return WrapGameMessage(
		&GameMessage_ReconnectRequest{&ReconnectRequest{OldPlayerId: rr.OldPlayerId}},
	)
}

func BuildDeathNote(dn *stypes.DeathNote) *GameMessage {
	return WrapGameMessage(
		&GameMessage_DeathNote{&DeathNote{PlayerId: dn.PlayerId}},
	)
}

func WrapGameMessage(payload isGameMessage_Payload) *GameMessage {
	return &GameMessage{
		Payload: payload,
	}
}
func BuildGameMessage(msg stypes.GameMessage) *GameMessage {
	switch payload := msg.(type) {
	case *stypes.ConnectAck:
		return BuildConnectAck(payload)
	case *stypes.ReconnectRequest:
		return BuildReconnectRequest(payload)
	case *stypes.ConnectRequest:
		return BuildConnectRequest(payload)
	case *stypes.DeathNote:
		return BuildDeathNote(payload)
	case *stypes.PlayerInput:
		return BuildPlayerInput(payload)
	case *stypes.WorldState:
		return BuildWorldState(payload)
	}
	return &GameMessage{}
}
