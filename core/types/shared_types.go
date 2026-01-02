package stypes

import (
	"CircleWar/core/geom"
)

type UDPAddrStr string
type PlayerHealth float32
type Direction int32

const (
	NONE  Direction = 0
	LEFT  Direction = 1
	RIGHT Direction = 2
	UP    Direction = 3
	DOWN  Direction = 4
)

// func marshal(msg *protobuf.GameMessage) ([]byte, error) {
// 	return []byte{}, nil
// }

type PlayerAction interface {
	// Serialize() ([]byte, error)
	IsPlayerAction()
}

type MoveAction struct {
	Dir Direction
}

func (*MoveAction) IsPlayerAction() {}

type ShootAction struct {
	Target geom.Vector2
}

func (*ShootAction) IsPlayerAction() {}

type PlayerState struct {
	Pos    geom.Vector2
	Health float32
	Id     uint32
}

type BulletState struct {
	Pos     geom.Vector2
	Size    float32
	OwnerId uint32
}

type PlayerInput struct {
	Actions  []PlayerAction
	PlayerId uint32
}

func (*PlayerInput) IsGameMessage() {}

type WorldState struct {
	TickNum uint32
	Players []*PlayerState
	Bullets []*BulletState
}

func (*WorldState) IsGameMessage() {}

type ConnectRequest struct {
	GameName string
}

func (*ConnectRequest) IsGameMessage() {}

type ConnectAck struct {
	PlayerId uint32
}

func (*ConnectAck) IsGameMessage() {}

type DeathNote struct {
	PlayerId uint32
}

func (*DeathNote) IsGameMessage() {}

type ReconnectRequest struct {
	OldPlayerId uint32
}

func (*ReconnectRequest) IsGameMessage() {}

type GameMessage interface {
	IsGameMessage()
}
