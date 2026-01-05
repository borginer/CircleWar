package netmsg

import (
	"CircleWar/core/geom"
	pb "CircleWar/core/network/protobuf"
	"errors"

	"google.golang.org/protobuf/proto"
)

type PlayerHealth float32
type Direction int32

const (
	NONE  Direction = 0
	LEFT  Direction = 1
	RIGHT Direction = 2
	UP    Direction = 3
	DOWN  Direction = 4
)

func Deserialize(msg []byte, n uint32) (GameMessage, error) {
	gameMsg := &pb.GameMessage{}
	err := proto.Unmarshal(msg[:n], gameMsg)
	if err != nil {
		return nil, err
	}

	switch payload := gameMsg.Payload.(type) {
	case *pb.GameMessage_DeathNote:
		return NewDeathNote(payload.DeathNote.PlayerId), nil
	case *pb.GameMessage_ConnectAck:
		return NewConnectAck(payload.ConnectAck.PlayerId), nil
	case *pb.GameMessage_ConnectRequest:
		return NewConnectRequest(payload.ConnectRequest.GameName), nil
	case *pb.GameMessage_ReconnectRequest:
		return NewReconnectRequest(payload.ReconnectRequest.OldPlayerId), nil
	case *pb.GameMessage_World:
		return worldStateFromProtobuf(payload), nil
	case *pb.GameMessage_PlayerInput:
		return playerInputFromProtobuf(payload), nil
	default:
		return nil, errors.New("Unrecognized game message")
	}
}

type GameMessage interface {
	Serialize() ([]byte, error)
}

type pbConvertible interface {
	ToProtobuf() *pb.GameMessage
}

func marshal(pc pbConvertible) ([]byte, error) {
	pbMsg := pc.ToProtobuf()
	data, err := proto.Marshal(pbMsg)
	if err != nil {
		return []byte{}, err
	}
	return data, nil
}

type PlayerAction interface {
	IsPlayerAction()
}

type MoveAction struct {
	Dir Direction
}

func (*MoveAction) IsPlayerAction() {}

func BuildPlayerMoveAction(ma *MoveAction) *pb.PlayerAction {
	return &pb.PlayerAction{
		Action: &pb.PlayerAction_Move{
			Move: &pb.MoveAction{
				Dir: pb.Direction(ma.Dir),
			},
		},
	}
}

type ShootAction struct {
	Target geom.Vector2
}

func (*ShootAction) IsPlayerAction() {}

func BuildPlayerShootAction(sa *ShootAction) *pb.PlayerAction {
	return &pb.PlayerAction{Action: &pb.PlayerAction_Shoot{
		Shoot: &pb.ShootAction{
			Target: &pb.Position{
				X: sa.Target.X,
				Y: sa.Target.Y,
			},
		},
	}}
}

type PlayerState struct {
	Id     uint32
	Pos    geom.Vector2
	Health float32
}

func NewPlayerState(id uint32, pos geom.Vector2, health float32) *PlayerState {
	return &PlayerState{id, pos, health}
}

func BuildPlayerState(pos geom.Vector2, health PlayerHealth, playerId uint32) pb.PlayerState {
	return pb.PlayerState{
		Pos:      &pb.Position{X: pos.X, Y: pos.Y},
		Health:   float32(health),
		PlayerId: playerId,
	}
}

type BulletState struct {
	OwnerId uint32
	Pos     geom.Vector2
	Size    float32
}

func NewBulletState(ownerId uint32, pos geom.Vector2, size float32) *BulletState {
	return &BulletState{ownerId, pos, size}
}

func BuildBulletState(pos geom.Vector2, size float32, ownerId uint32) pb.BulletState {
	return pb.BulletState{
		Pos:     &pb.Position{X: pos.X, Y: pos.Y},
		Size:    size,
		OwnerId: ownerId,
	}
}

type PlayerInput struct {
	Actions  []PlayerAction
	PlayerId uint32
}

func (*PlayerInput) IsGameMessage() {}

func (pi *PlayerInput) ToProtobuf() *pb.GameMessage {
	playerInput := &pb.PlayerInput{
		PlayerId:      pi.PlayerId,
		PlayerActions: []*pb.PlayerAction{},
	}

	for _, act := range pi.Actions {
		switch inner := act.(type) {
		case *MoveAction:
			playerInput.PlayerActions = append(playerInput.PlayerActions, BuildPlayerMoveAction(inner))
		case *ShootAction:
			playerInput.PlayerActions = append(playerInput.PlayerActions, BuildPlayerShootAction(inner))
		}
	}

	return &pb.GameMessage{
		Payload: &pb.GameMessage_PlayerInput{PlayerInput: playerInput},
	}
}

func playerInputFromProtobuf(pbPlayerInput *pb.GameMessage_PlayerInput) *PlayerInput {
	playerInput := &PlayerInput{}

	for _, playerAct := range pbPlayerInput.PlayerInput.PlayerActions {
		switch act := playerAct.Action.(type) {
		case *pb.PlayerAction_Move:
			playerInput.Actions = append(playerInput.Actions, &MoveAction{Direction(act.Move.Dir)})
		case *pb.PlayerAction_Shoot:
			playerInput.Actions = append(playerInput.Actions, &ShootAction{geom.NewVector(act.Shoot.Target.X, act.Shoot.Target.Y)})
		}
	}

	playerInput.PlayerId = pbPlayerInput.PlayerInput.PlayerId

	return playerInput
}

func (pi *PlayerInput) Serialize() ([]byte, error) {
	return marshal(pi)
}

type WorldState struct {
	Players []*PlayerState
	Bullets []*BulletState
	TickNum uint32
}

func NewWorldState(players []*PlayerState, bullets []*BulletState, tickNum uint32) *WorldState {
	return &WorldState{players, bullets, tickNum}
}

func (*WorldState) IsGameMessage() {}

func (ws *WorldState) ToProtobuf() *pb.GameMessage {
	worldState := &pb.WorldState{}

	for _, player := range ws.Players {
		pbPlayer := BuildPlayerState(player.Pos, PlayerHealth(player.Health), uint32(player.Id))
		worldState.Players = append(worldState.Players, &pbPlayer)
	}

	for _, bullet := range ws.Bullets {
		pbBullet := BuildBulletState(bullet.Pos, bullet.Size, uint32(bullet.OwnerId))
		worldState.Bullets = append(worldState.Bullets, &pbBullet)
	}

	worldState.TickNum = ws.TickNum

	return &pb.GameMessage{
		Payload: &pb.GameMessage_World{World: worldState},
	}
}

func worldStateFromProtobuf(pbWorld *pb.GameMessage_World) *WorldState {
	worldState := &WorldState{}

	for _, player := range pbWorld.World.Players {
		worldState.Players = append(worldState.Players, NewPlayerState(
			player.PlayerId,
			geom.NewVector(player.Pos.X, player.Pos.Y),
			player.Health,
		))
	}

	for _, bullet := range pbWorld.World.Bullets {
		worldState.Bullets = append(worldState.Bullets, NewBulletState(
			bullet.OwnerId,
			geom.NewVector(bullet.Pos.X, bullet.Pos.Y),
			bullet.Size,
		))
	}
	worldState.TickNum = pbWorld.World.TickNum

	return worldState
}

func (ws *WorldState) Serialize() ([]byte, error) {
	return marshal(ws)
}

type ConnectRequest struct {
	GameName string
}

func NewConnectRequest(gameName string) *ConnectRequest {
	return &ConnectRequest{gameName}
}

func (*ConnectRequest) IsGameMessage() {}

func (cr *ConnectRequest) ToProtobuf() *pb.GameMessage {
	return &pb.GameMessage{
		Payload: &pb.GameMessage_ConnectRequest{
			ConnectRequest: &pb.ConnectRequest{GameName: cr.GameName},
		},
	}
}

func (cr *ConnectRequest) Serialize() ([]byte, error) {
	return marshal(cr)
}

type ConnectAck struct {
	PlayerId uint32
}

func NewConnectAck(playerId uint32) *ConnectAck {
	return &ConnectAck{playerId}
}

func (*ConnectAck) IsGameMessage() {}

func (ca *ConnectAck) ToProtobuf() *pb.GameMessage {
	return &pb.GameMessage{
		Payload: &pb.GameMessage_ConnectAck{
			ConnectAck: &pb.ConnectAck{PlayerId: ca.PlayerId},
		},
	}
}

func (ca *ConnectAck) Serialize() ([]byte, error) {
	return marshal(ca)
}

type DeathNote struct {
	PlayerId uint32
}

func NewDeathNote(playerId uint32) *DeathNote {
	return &DeathNote{playerId}
}

func (*DeathNote) IsGameMessage() {}

func (dn *DeathNote) ToProtobuf() *pb.GameMessage {
	return &pb.GameMessage{
		Payload: &pb.GameMessage_DeathNote{
			DeathNote: &pb.DeathNote{PlayerId: dn.PlayerId},
		},
	}
}

func (dn *DeathNote) Serialize() ([]byte, error) {
	return marshal(dn)
}

type ReconnectRequest struct {
	OldPlayerId uint32
}

func NewReconnectRequest(oldPlayerId uint32) *ReconnectRequest {
	return &ReconnectRequest{oldPlayerId}
}

func (*ReconnectRequest) IsGameMessage() {}

func (rr *ReconnectRequest) ToProtobuf() *pb.GameMessage {
	return &pb.GameMessage{
		Payload: &pb.GameMessage_ReconnectRequest{
			ReconnectRequest: &pb.ReconnectRequest{OldPlayerId: rr.OldPlayerId},
		},
	}
}

func (rr *ReconnectRequest) Serialize() ([]byte, error) {
	return marshal(rr)
}
