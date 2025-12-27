package worldstate

import (
	"CircleWar/geom"
	stypes "CircleWar/shared/shared_types"
	"time"
)

// type StateObject interface {
// 	ObjectType() string
// }

type PlayerState struct {
	LastBulletShot time.Time
	Pos            geom.Position
}

// func (playerState) ObjectType() string {
// 	return "player"
// }

type BulletState struct {
	Born    time.Time
	Pos     geom.Position
	MoveDir geom.Direction
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
