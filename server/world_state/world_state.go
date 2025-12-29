package worldstate

import (
	"CircleWar/config"
	"CircleWar/geom"
	sharedtypes "CircleWar/shared/types"
	stypes "CircleWar/shared/types"
	"time"
)

// type StateObject interface {
// 	ObjectType() string
// }

type PlayerState struct {
	LastBulletShot time.Time
	Pos            geom.Position
	Health         sharedtypes.PlayerHealth
	Addr           stypes.UDPAddrStr
	Id             uint
}

var nextPlayerId uint = uint(1)

func NewPlayerState(pos geom.Position, addr stypes.UDPAddrStr) PlayerState {
	ps := PlayerState{time.Now(), pos, config.InitialPlayerHealth, addr, nextPlayerId}
	nextPlayerId += 1
	return ps
}

// func (playerState) ObjectType() string {
// 	return "player"
// }

type BulletState struct {
	PlayerId uint
	Born     time.Time
	Pos      geom.Position
	MoveDir  geom.Direction
	Size     float32
}

// func (bulletState) ObjectType() string {
// 	return "bullet"
// }

type ServerWorld struct {
	nextBulletId int
	players      map[string]PlayerState
	bullets      map[int]*BulletState
	addresses    map[stypes.UDPAddrStr]bool
}

func NewServerWorld() ServerWorld {
	return ServerWorld{
		nextBulletId: 0,
		players:      make(map[string]PlayerState),
		bullets:      make(map[int]*BulletState),
		addresses:    make(map[stypes.UDPAddrStr]bool),
	}

}

func (p *PlayerState) ChangePlayerHealth(by int) {
	p.Health += stypes.PlayerHealth(by)
}

func (sw *ServerWorld) AddAddress(addr stypes.UDPAddrStr) {
	sw.addresses[addr] = true
}

func (sw *ServerWorld) AddressSnapshots() []stypes.UDPAddrStr {
	snapshot := []stypes.UDPAddrStr{}
	for addr := range sw.addresses {
		snapshot = append(snapshot, addr)
	}
	return snapshot
}

func (sw *ServerWorld) StartPlayerBulletCD(addr stypes.UDPAddrStr) {
	playerState := sw.players[string(addr)]
	playerState.LastBulletShot = time.Now()
	sw.players[string(addr)] = playerState
}

func (sw *ServerWorld) DurSinceLastBullet(addr stypes.UDPAddrStr) time.Duration {
	now := time.Now()
	return now.Sub(sw.players[string(addr)].LastBulletShot)
}

func (sw *ServerWorld) PlayerSnapshots() []PlayerState {
	snapshot := []PlayerState{}
	for _, state := range sw.players {
		snapshot = append(snapshot, state)
	}
	return snapshot
}

func (sw *ServerWorld) AddPlayerState(key string, state PlayerState) {
	sw.players[key] = state
}

func (sw *ServerWorld) RemovePlayerState(key string) {
	delete(sw.players, key)
}

func (sw *ServerWorld) PlayerSnapshot(key string) PlayerState {
	return sw.players[key]
}

func (sw *ServerWorld) HasPlayerState(key string) bool {
	_, ok := sw.players[key]
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
