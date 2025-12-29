package main

import (
	"CircleWar/config"
	"CircleWar/geom"
	"CircleWar/server/world_state"
	"CircleWar/shared/hitboxes"
	"CircleWar/shared/protobuf"
	stypes "CircleWar/shared/types"
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
	ticksPerSecond = 60
)

type actionType int16
type moveDirection int8

const (
	DIR_LEFT  moveDirection = 0
	DIR_RIGHT moveDirection = 1
	DIR_UP    moveDirection = 2
	DIR_DOWN  moveDirection = 3
)
const (
	MOVE  actionType = 0
	SHOOT actionType = 1
)

type playerAction interface {
	ActionType() actionType
}

type moveAction struct {
	dir moveDirection
}

func (moveAction) ActionType() actionType {
	return MOVE
}

type shootAction struct {
	target geom.Position
}

func (shootAction) ActionType() actionType {
	return SHOOT
}

type playerInput struct {
	actions []playerAction
}

type clientInput struct {
	addrStr stypes.UDPAddrStr
	input   playerInput
}

func moveDelta(inputs map[moveDirection]bool, delta float32) (float32, float32) {
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

	return float32(dx) * delta, float32(dy) * delta
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
			actions: []playerAction{},
		}

		for _, playerAct := range pbInput.PlayerActions {
			switch act := playerAct.Action.(type) {
			case *protobuf.PlayerAction_Move:
				if act.Move.Vert == protobuf.Direction_DOWN {
					playerIn.actions = append(playerIn.actions, moveAction{DIR_DOWN})
				}
				if act.Move.Vert == protobuf.Direction_UP {
					playerIn.actions = append(playerIn.actions, moveAction{DIR_UP})
				}
				if act.Move.Hori == protobuf.Direction_RIGHT {
					playerIn.actions = append(playerIn.actions, moveAction{DIR_RIGHT})
				}
				if act.Move.Hori == protobuf.Direction_LEFT {
					playerIn.actions = append(playerIn.actions, moveAction{DIR_LEFT})
				}
				break
			case *protobuf.PlayerAction_Shoot:
				playerIn.actions = append(playerIn.actions, shootAction{
					geom.NewPosition(
						playerAct.GetShoot().Pos.X,
						playerAct.GetShoot().Pos.Y,
					),
				})
				break
			}
		}

		inputChan <- clientInput{
			clientAddrStr,
			playerIn,
		}
	}
}

func calculateHits(serverWorld *worldstate.ServerWorld) {
	for _, player := range serverWorld.PlayerSnapshots() {
		for bulletId, bullet := range serverWorld.BulletSnapshots() {
			if bullet.PlayerId == player.Id {
				continue
			}
			playerRad := hitboxes.PlayerSize(player.Health)
			bulletRad := hitboxes.BulletSize(player.Health)
			playerPos := player.Pos
			bulletPos := bullet.Pos
			if playerPos.DistTo(bulletPos) < (playerRad+bulletRad)*0.85 {
				player.ChangePlayerHealth(-1)
				fmt.Println("player health:", player.Health)
				if int(player.Health) <= 0 {
					fmt.Println("removing player")
					serverWorld.RemovePlayerState(string(player.Addr))
				} else {
					serverWorld.AddPlayerState(string(player.Addr), player)
				}
				serverWorld.RemoveBullet(bulletId)
			}
		}
	}
}

func updateWorldState(serverWorld *worldstate.ServerWorld, clientInputs map[stypes.UDPAddrStr]clientInput) {
	for i, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("vector: (%f, %f)", bullet.moveVec.x, bullet.moveVec.y)
		if time.Since(bullet.Born) > time.Duration(config.BulletTimeToLiveSec*float64(time.Second)) {
			serverWorld.RemoveBullet(i)
		}
		fmt.Println("shooting", bullet.MoveDir.X*bulletSpeed/ticksPerSecond, bullet.MoveDir.Y*bulletSpeed/ticksPerSecond)

		bullet.Pos = bullet.Pos.Add(
			bullet.MoveDir.X*bulletSpeed/ticksPerSecond,
			bullet.MoveDir.Y*bulletSpeed/ticksPerSecond,
		)
	}

	for _, ci := range clientInputs {
		dirMap := make(map[moveDirection]bool)
		for _, action := range ci.input.actions {
			switch act := action.(type) {
			case moveAction:
				dirMap[act.dir] = true
				break
			case shootAction:
				fmt.Println("since last bullet: ", serverWorld.DurSinceLastBullet(ci.addrStr))
				if serverWorld.DurSinceLastBullet(ci.addrStr) > time.Duration(config.BulletCooldownMS)*time.Millisecond {
					playerState := serverWorld.PlayerSnapshot(string(ci.addrStr))
					playerPos := playerState.Pos
					serverWorld.StartPlayerBulletCD(ci.addrStr)
					fmt.Println("shoot!", act.target)
					serverWorld.AddBulletState(
						worldstate.BulletState{
							PlayerId: playerState.Id,	
							Born:    time.Now(),
							Pos:     geom.NewPosition(playerPos.X, playerPos.Y),
							MoveDir: geom.NewDir(act.target.X-playerPos.X, act.target.Y-playerPos.Y),
							Size:    hitboxes.BulletSize(playerState.Health),
						})
				}
				break
			}
		}
		dx, dy := moveDelta(dirMap, 1.0/ticksPerSecond)
		state := serverWorld.PlayerSnapshot(string(ci.addrStr))
		state.Pos = state.Pos.Add(dx, dy)
		serverWorld.AddPlayerState(string(ci.addrStr), state)
	}

	calculateHits(serverWorld)
}

func buildPBWorldState(serverWorld *worldstate.ServerWorld) *protobuf.WorldState {
	pbWorld := &protobuf.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		fmt.Println("sending player: ", player)
		pbPlayer := protobuf.BuildPlayerState(player.Pos.X, player.Pos.Y, player.Health)
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	fmt.Printf("bullets amount: %d\n", len(serverWorld.BulletSnapshots()))
	for _, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("building bullet at: (%f, %f)\n", bullet.pos.x, bullet.pos.y)
		pbBullet := protobuf.BuildBulletState(bullet.Pos.X, bullet.Pos.Y, bullet.Size)
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
			// if len(pbWorld.Players) > 0 {
			// 	fmt.Println("player health: ", pbWorld.Players[0].Health)
			// }
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
				serverWorld.AddPlayerState(string(input.addrStr), worldstate.NewPlayerState(
					geom.NewPosition(500, 500),
					input.addrStr,
				))
			}
			clientInputs[input.addrStr] = input
		}
	}
}
