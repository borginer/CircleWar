package main

import (
	"CircleWar/config"
	"CircleWar/geom"
	"CircleWar/shared/protobuf"
	"CircleWar/server/world_state"
	stypes "CircleWar/shared/shared_types"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"google.golang.org/protobuf/proto"
)

const (
	port        = config.Port
	bulletSpeed = config.BulletSpeed
	playerSpeed = config.PlayerSpeed
)

const (
	ticksPerSecond   = 60
	bulletCooldownMS = 250
)

type actionType int16

const (
	DIR_LEFT  actionType = 0
	DIR_RIGHT actionType = 1
	DIR_UP    actionType = 2
	DIR_DOWN  actionType = 3
	SHOOT     actionType = 4
)

type playerInput struct {
	actions map[actionType]bool
	pos     geom.Position
}

type clientInput struct {
	addrStr stypes.UDPAddrStr
	input   playerInput
}

func moveDelta(inputs map[actionType]bool, delta float64) (float64, float64) {
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

func clientInputHandler(conn *net.UDPConn, inputChan chan clientInput) {
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		fmt.Println("player input recieved")

		var pbInput protobuf.PlayerInput
		if err := proto.Unmarshal(buf[:n], &pbInput); err != nil {
			continue
		}

		clientAddrStr := stypes.UDPAddrStr(clientAddr.String())
		playerIn := playerInput{
			actions: make(map[actionType]bool),
		}

		for _, playerAct := range pbInput.PlayerActions {
			switch act := playerAct.Action.(type) {
			case *protobuf.PlayerAction_Move:
				if act.Move.Vert == protobuf.Direction_DOWN {
					playerIn.actions[DIR_DOWN] = true
				}
				if act.Move.Vert == protobuf.Direction_UP {
					playerIn.actions[DIR_UP] = true
				}
				if act.Move.Hori == protobuf.Direction_RIGHT {
					playerIn.actions[DIR_RIGHT] = true
				}
				if act.Move.Hori == protobuf.Direction_LEFT {
					playerIn.actions[DIR_LEFT] = true
				}
				break
			case *protobuf.PlayerAction_Shoot:
				playerIn.actions[SHOOT] = true
				playerIn.pos = geom.NewPosition(
					playerAct.GetShoot().Pos.X,
					playerAct.GetShoot().Pos.Y,
				)
				break
			}
		}

		inputChan <- clientInput{
			clientAddrStr,
			playerIn,
		}
	}
}

func updateWorldState(serverWorld *worldstate.ServerWorld, clientInputs map[stypes.UDPAddrStr]clientInput) {
	for _, ci := range clientInputs {
		dx, dy := moveDelta(ci.input.actions, float64(1)/ticksPerSecond)
		state := serverWorld.PlayerSnapshot(string(ci.addrStr))
		state.Pos = state.Pos.Add(float32(dx), float32(dy))
		serverWorld.AddPlayerState(string(ci.addrStr), state)

		for act := range ci.input.actions {
			if act == SHOOT {
				fmt.Println("since last bullet: ", serverWorld.DurSinceLastBullet(ci.addrStr))
				if serverWorld.DurSinceLastBullet(ci.addrStr) >
					time.Duration(bulletCooldownMS)*time.Millisecond {
					serverWorld.StartPlayerBulletCD(ci.addrStr)
					serverWorld.AddBulletState(
						worldstate.BulletState{
							Born:    time.Now(),
							Pos:     geom.NewPosition(state.Pos.X, state.Pos.Y),
							MoveDir: geom.NewDir(ci.input.pos.X-state.Pos.X, ci.input.pos.Y-state.Pos.Y),
						})
				}
			}
		}
	}

	for i, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("vector: (%f, %f)", bullet.moveVec.x, bullet.moveVec.y)
		if time.Since(bullet.Born) > 3*time.Second {
			serverWorld.RemoveBullet(i)
		}
		bullet.Pos = bullet.Pos.Add(
			bullet.MoveDir.X*bulletSpeed/ticksPerSecond,
			bullet.MoveDir.Y*bulletSpeed/ticksPerSecond,
		)
	}
}

func buildPBWorldState(serverWorld *worldstate.ServerWorld) *protobuf.WorldState {
	pbWorld := &protobuf.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		pbPlayer := protobuf.BuildPlayerState(player.Pos.X, player.Pos.Y)
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	fmt.Printf("bullets amount: %d\n", len(serverWorld.BulletSnapshots()))
	for _, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("building bullet at: (%f, %f)\n", bullet.pos.x, bullet.pos.y)
		pbBullet := protobuf.BuildBulletState(bullet.Pos.X, bullet.Pos.Y)
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

	serverWorld := worldstate.NewServerWorld()
	clock := time.Tick(time.Second / ticksPerSecond)

	inputChan := make(chan clientInput)
	go clientInputHandler(conn, inputChan)

	clientInputs := make(map[stypes.UDPAddrStr]clientInput)
	for {
		select {
		case tick := <-clock:
			fmt.Println(tick)

			updateWorldState(&serverWorld, clientInputs)
			clientInputs = make(map[stypes.UDPAddrStr]clientInput)

			pbWorld := buildPBWorldState(&serverWorld)
			data, err := proto.Marshal(pbWorld)
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("data size: %d\n", len(data))
			for _, addr := range serverWorld.AddressSnapshots() {
				netAddr, _ := net.ResolveUDPAddr("udp", string(addr))
				conn.WriteToUDP(data, netAddr)
			}

		case input := <-inputChan:
			serverWorld.AddAddress(input.addrStr)
			if !serverWorld.HasPlayerState(string(input.addrStr)) {
				serverWorld.AddPlayerState(string(input.addrStr), worldstate.PlayerState{
					Pos: geom.NewPosition(500, 500),
				})
			}
			clientInputs[input.addrStr] = input
		}
	}
}
