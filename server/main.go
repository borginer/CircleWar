package main

import (
	"CircleWar/protob"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"google.golang.org/protobuf/proto"
)

const port = 4000
const ticksPerSecond = 60
const bulletCooldownMS = 200
const bulletSpeed = 1200
const playerSpeed = 700

const (
	DIR_LEFT  playerAction = 0
	DIR_RIGHT playerAction = 1
	DIR_UP    playerAction = 2
	DIR_DOWN  playerAction = 3
	SHOOT     playerAction = 4
)

type position struct{ x, y float32 }
type vector2 struct{ x, y float32 }
type playerAction int16
type udpAddrStr string

func (v vector2) normalized() vector2 {
	length := math.Sqrt(math.Pow(float64(v.x), 2) + math.Pow(float64(v.y), 2))
	return vector2{v.x / float32(length), v.y / float32(length)}
}

type playerInput struct {
	actions map[playerAction]bool
	pos     position
}

type playerState struct {
	lastBulletShot time.Time
	pos            position
}

type bulletState struct {
	born    time.Time
	pos     position
	moveVec vector2
}

type clientInput struct {
	addrStr udpAddrStr
	input   playerInput
}

type serverWorld struct {
	nextBulletId int
	players      map[string]playerState
	bullets      map[int]*bulletState
	addresses    map[udpAddrStr]bool
}

func newServerWorld() serverWorld {
	return serverWorld{
		nextBulletId: 0,
		players:      make(map[string]playerState),
		bullets:      make(map[int]*bulletState),
		addresses:    make(map[udpAddrStr]bool),
	}

}

func (sw *serverWorld) addAddress(addr udpAddrStr) {
	sw.addresses[addr] = true
}

func (sw *serverWorld) addressSnapshots() []udpAddrStr {
	snapshot := []udpAddrStr{}
	for addr := range sw.addresses {
		snapshot = append(snapshot, addr)
	}
	return snapshot
}

func (sw *serverWorld) startPlayerBulletCD(addr udpAddrStr) {
	playerState := sw.players[string(addr)]
	playerState.lastBulletShot = time.Now()
	sw.players[string(addr)] = playerState
}

func (sw *serverWorld) durSinceLastBullet(addr udpAddrStr) time.Duration {
	now := time.Now()
	return now.Sub(sw.players[string(addr)].lastBulletShot)
}

func (sw *serverWorld) playerSnapshots() []playerState {
	snapshot := []playerState{}
	for _, state := range sw.players {
		snapshot = append(snapshot, state)
	}
	return snapshot
}

func (sw *serverWorld) addPlayerState(key string, state playerState) {
	sw.players[key] = state
}

func (sw *serverWorld) playerSnapshot(key string) playerState {
	return sw.players[key]
}

func (sw *serverWorld) hasPlayerState(key string) bool {
	_, ok := sw.players[key]
	return ok
}

func (sw *serverWorld) addBulletState(bullet bulletState) {
	sw.bullets[sw.nextBulletId] = &bullet
	sw.nextBulletId++
}

func (sw *serverWorld) bulletSnapshots() map[int]*bulletState {
	return sw.bullets
}

func (sw *serverWorld) removeBullet(id int) {
	delete(sw.bullets, id)
}

func moveDelta(inputs map[playerAction]bool, delta float64) (float64, float64) {
	dx, dy := 0.0, 0.0
	for act := range inputs {
		switch act {
		case DIR_LEFT:
			dx = -playerSpeed
		case DIR_RIGHT:
			dx = playerSpeed
		case DIR_UP:
			dy = -playerSpeed
		case DIR_DOWN:
			dy = playerSpeed
		default:
			continue
		}
	}

	if dx != 0 && dy != 0 {
		norm := math.Sqrt(2)
		dx /= norm
		dy /= norm
	}

	return dx * delta, dy * delta
}

func readHandler(conn *net.UDPConn, inputChan chan clientInput) {
	buf := make([]byte, 1024)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var pbInput protob.PlayerInput
		if err := proto.Unmarshal(buf[:n], &pbInput); err != nil {
			continue
		}

		clientAddrStr := udpAddrStr(clientAddr.String())
		playerIn := playerInput{
			actions: make(map[playerAction]bool),
		}

		for _, playerAct := range pbInput.PlayerActions {
			switch act := playerAct.Action.(type) {
			case *protob.PlayerAction_Move:
				if act.Move.Vert == protob.Direction_DOWN {
					playerIn.actions[DIR_DOWN] = true
				}
				if act.Move.Vert == protob.Direction_UP {
					playerIn.actions[DIR_UP] = true
				}
				if act.Move.Hori == protob.Direction_RIGHT {
					playerIn.actions[DIR_RIGHT] = true
				}
				if act.Move.Hori == protob.Direction_LEFT {
					playerIn.actions[DIR_LEFT] = true
				}
				break
			case *protob.PlayerAction_Shoot:
				playerIn.actions[SHOOT] = true
				playerIn.pos = position{
					playerAct.GetShoot().Pos.X,
					playerAct.GetShoot().Pos.Y,
				}
				break
			}
		}

		inputChan <- clientInput{
			clientAddrStr,
			playerIn,
		}
	}
}

func updateWorldState(serverWorld *serverWorld, clientInputs map[udpAddrStr]clientInput) {
	for _, ci := range clientInputs {
		dx, dy := moveDelta(ci.input.actions, float64(1)/ticksPerSecond)
		state := serverWorld.playerSnapshot(string(ci.addrStr))
		state.pos.x += float32(dx)
		state.pos.y += float32(dy)
		serverWorld.addPlayerState(string(ci.addrStr), state)

		for act := range ci.input.actions {
			if act == SHOOT {
				fmt.Println("since last bullet: ", serverWorld.durSinceLastBullet(ci.addrStr))
				if serverWorld.durSinceLastBullet(ci.addrStr) >
					time.Duration(bulletCooldownMS)*time.Millisecond {
					serverWorld.startPlayerBulletCD(ci.addrStr)
					serverWorld.addBulletState(
						bulletState{
							time.Now(),
							position{
								state.pos.x,
								state.pos.y,
							},
							vector2{
								ci.input.pos.x - state.pos.x,
								ci.input.pos.y - state.pos.y,
							}.normalized(),
						})
				}
			}
		}
	}

	for i, bullet := range serverWorld.bulletSnapshots() {
		// fmt.Printf("vector: (%f, %f)", bullet.moveVec.x, bullet.moveVec.y)
		if time.Since(bullet.born) > 3*time.Second {
			serverWorld.removeBullet(i)
		}
		bullet.pos.x += bullet.moveVec.x * bulletSpeed / ticksPerSecond
		bullet.pos.y += bullet.moveVec.y * bulletSpeed / ticksPerSecond

	}
}

func buildPBWorldState(serverWorld *serverWorld) *protob.WorldState {
	pbWorld := &protob.WorldState{}

	players := serverWorld.playerSnapshots()
	for _, player := range players {
		pbPlayer := protob.BuildPlayerState(player.pos.x, player.pos.y)
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	fmt.Printf("bullets amount: %d\n", len(serverWorld.bulletSnapshots()))
	for _, bullet := range serverWorld.bulletSnapshots() {
		// fmt.Printf("building bullet at: (%f, %f)\n", bullet.pos.x, bullet.pos.y)
		pbBullet := protob.BuildBulletState(bullet.pos.x, bullet.pos.y)
		pbWorld.Bullets = append(pbWorld.Bullets, &pbBullet)
	}

	return pbWorld
}

func openUDPConn() *net.UDPConn {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}
	return conn
}

func main() {
	conn := openUDPConn()
	defer conn.Close()
	fmt.Printf("Listening on udp port %d\n", port)

	serverWorld := newServerWorld()
	clock := time.Tick(time.Second / ticksPerSecond)

	inputChan := make(chan clientInput)
	go readHandler(conn, inputChan)

	clientInputs := make(map[udpAddrStr]clientInput)
	for {
		select {
		case tick := <-clock:
			fmt.Println(tick)

			updateWorldState(&serverWorld, clientInputs)
			clientInputs = make(map[udpAddrStr]clientInput)

			pbWorld := buildPBWorldState(&serverWorld)
			data, err := proto.Marshal(pbWorld)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("data size: %d\n", len(data))
			for _, addr := range serverWorld.addressSnapshots() {
				netAddr, _ := net.ResolveUDPAddr("udp", string(addr))
				conn.WriteToUDP(data, netAddr)
			}

		case input := <-inputChan:
			serverWorld.addAddress(input.addrStr)
			if !serverWorld.hasPlayerState(string(input.addrStr)) {
				serverWorld.addPlayerState(string(input.addrStr), playerState{
					pos: position{x: 500, y: 500},
				})
			}
			clientInputs[input.addrStr] = input
		}
	}
}
