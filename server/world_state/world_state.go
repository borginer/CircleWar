package worldstate

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	"CircleWar/core/netmsg"
	stypes "CircleWar/core/netmsg"
	"net"
	"time"
)

type PlayerState struct {
	LastBulletShot time.Time
	Pos            geom.Vector2
	health         stypes.PlayerHealth
	Addr           net.UDPAddr
	Id             uint
}

func (ps PlayerState) Health() stypes.PlayerHealth {
	return ps.health
}

func (ps *PlayerState) ChangeHealth(by int) {
	ps.health += stypes.PlayerHealth(by)
}

var nextPlayerId uint = uint(1)

func NewPlayerState(pos geom.Vector2, addr net.UDPAddr) PlayerState {
	ps := PlayerState{time.Now(), pos, config.InitialPlayerHealth, addr, nextPlayerId}
	nextPlayerId += 1
	return ps
}

type BulletState struct {
	OwnerId uint
	Born    time.Time
	Pos     geom.Vector2
	MoveDir geom.Direction
	Size    float32
}

func NewBulletState(player PlayerState, target geom.Vector2) BulletState {
	return BulletState{
		OwnerId: player.Id,
		Born:    time.Now(),
		Pos:     player.Pos,
		MoveDir: geom.NewDir(target.Sub(player.Pos)),
		Size:    hitboxes.BulletSize(player.Health()),
	}
}

type PlayerWants struct {
	// TODO: think of a better way to do player inputs
	MoveDirs map[netmsg.Direction]bool //toggle
}

type ServerWorld struct {
	nextBulletId  int
	players       map[uint]*PlayerState
	playerWants   map[uint]*PlayerWants
	bullets       map[int]*BulletState
	addresses     map[uint]net.UDPAddr
	height, width float32
	tickNum       uint32
}

func (sw *ServerWorld) InitPlayerWants(pid uint) {
	sw.playerWants[pid] = &PlayerWants{make(map[stypes.Direction]bool)}
}

func (sw *ServerWorld) PlayerWants(pid uint) *PlayerWants {
	return sw.playerWants[pid]
}

func NewServerWorld() ServerWorld {
	return ServerWorld{
		nextBulletId: 0,
		tickNum:      0,
		players:      make(map[uint]*PlayerState),
		playerWants:  make(map[uint]*PlayerWants),
		bullets:      make(map[int]*BulletState),
		addresses:    make(map[uint]net.UDPAddr),
		height:       config.WorldHeight,
		width:        config.WorldWidth,
	}
}

func (sw *ServerWorld) Width() float32 {
	return sw.width
}

func (sw *ServerWorld) Height() float32 {
	return sw.height
}

func (sw *ServerWorld) NextTick() {
	sw.tickNum++
}

func (sw *ServerWorld) Tick() uint32 {
	return sw.tickNum
}

func (sw *ServerWorld) AddAddress(playerId uint, addr net.UDPAddr) {
	sw.addresses[playerId] = addr
}

func (sw *ServerWorld) AddressSnapshots() map[uint]net.UDPAddr {
	return sw.addresses
}

func (sw *ServerWorld) GetAddress(playerId uint) net.UDPAddr {
	return sw.addresses[playerId]
}

func (sw *ServerWorld) RemovePlayerAddress(playerId uint) {
	delete(sw.addresses, playerId)
}

func (sw *ServerWorld) StartPlayerBulletCD(id uint) {
	playerState := sw.players[id]
	playerState.LastBulletShot = time.Now()
	sw.players[id] = playerState
}

func (sw *ServerWorld) DurSinceLastBullet(id uint) time.Duration {
	now := time.Now()
	return now.Sub(sw.players[id].LastBulletShot)
}

func (sw *ServerWorld) PlayerSnapshots() []PlayerState {
	snapshot := []PlayerState{}
	for _, state := range sw.players {
		snapshot = append(snapshot, *state)
	}
	return snapshot
}

func (sw *ServerWorld) AddPlayerState(player PlayerState) {
	sw.players[player.Id] = &player
	sw.InitPlayerWants(player.Id)
}

func (sw *ServerWorld) HasPlayer(id uint) bool {
	_, ok := sw.players[id]
	return ok
}

func (sw *ServerWorld) RemovePlayerState(id uint) {
	delete(sw.players, id)
}

func (sw *ServerWorld) Player(id uint) *PlayerState {
	return sw.players[id]
}

func (sw *ServerWorld) HasPlayerState(id uint) bool {
	_, ok := sw.players[id]
	return ok
}

func (sw *ServerWorld) AddBulletState(bullet BulletState) {
	sw.bullets[sw.nextBulletId] = &bullet
	sw.nextBulletId++
}

func (sw *ServerWorld) BulletSnapshots() map[int]*BulletState {
	return sw.bullets
}

func (sw *ServerWorld) RemoveBullet(id int) {
	delete(sw.bullets, id)
}
