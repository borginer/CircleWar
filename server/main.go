package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	"CircleWar/core/protobuf"
	stypes "CircleWar/core/types"
	worldstate "CircleWar/server/world_state"
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
	target geom.Vector2
}

func (shootAction) ActionType() actionType {
	return SHOOT
}

type playerInput struct {
	actions []playerAction
}

type ClientMsg interface {
	ClientReqName() string
}

type clientInput struct {
	addrStr stypes.UDPAddrStr
	input   playerInput
}

func (clientInput) ClientReqName() string {
	return "client input"
}

type clientJoinReq struct {
	addrStr  stypes.UDPAddrStr
	gameName string
}

func (clientJoinReq) ClientReqName() string {
	return "join request"
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

func extractPlayerInput(pi *protobuf.PlayerInput) *playerInput {
	playerIn := playerInput{
		actions: []playerAction{},
	}

	for _, playerAct := range pi.PlayerActions {
		switch act := playerAct.Action.(type) {
		case *protobuf.PlayerAction_Move:
			if act.Move.Dir == protobuf.Direction_DOWN {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_DOWN})
			}
			if act.Move.Dir == protobuf.Direction_UP {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_UP})
			}
			if act.Move.Dir == protobuf.Direction_RIGHT {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_RIGHT})
			}
			if act.Move.Dir == protobuf.Direction_LEFT {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_LEFT})
			}
			break
		case *protobuf.PlayerAction_Shoot:
			playerIn.actions = append(playerIn.actions, shootAction{
				geom.NewVector(
					playerAct.GetShoot().Target.X,
					playerAct.GetShoot().Target.Y,
				),
			})
			break
		}
	}

	return &playerIn
}

func clientInputHandler(conn *net.UDPConn, inputChan chan ClientMsg) {
	buf := make([]byte, 1024)
	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var gameMsg protobuf.GameMessage
		if err := proto.Unmarshal(buf[:n], &gameMsg); err != nil {
			fmt.Println(err)
			continue
		}

		clientAddrStr := stypes.UDPAddrStr(clientAddr.String())

		switch payload := gameMsg.Payload.(type) {
		case *protobuf.GameMessage_PlayerInput:
			// fmt.Println("player input recieved from:", clientAddrStr)
			playerIn := extractPlayerInput(payload.PlayerInput)
			inputChan <- clientInput{
				addrStr: clientAddrStr,
				input:   *playerIn,
			}
		case *protobuf.GameMessage_ConnectRequest:
			joinReq := payload.ConnectRequest
			inputChan <- clientJoinReq{
				addrStr:  clientAddrStr,
				gameName: joinReq.GameName,
			}
		}
	}
}

func calculateHits(serverWorld *worldstate.ServerWorld) {
	for _, player := range serverWorld.PlayerSnapshots() {
		for bulletId, bullet := range serverWorld.BulletSnapshots() {
			if bullet.PlayerId == player.Id {
				continue
			}
			playerRad := hitboxes.PlayerSize(player.Health())
			bulletRad := hitboxes.BulletSize(player.Health())
			playerPos := player.Pos
			bulletPos := bullet.Pos

			if playerPos.DistTo(bulletPos) < (playerRad+bulletRad)*0.9 {
				player.ChangeHealth(-1)
				// fmt.Println("player health:", player.Health())
				if int(player.Health()) <= 0 {
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

func movePlayer(serverWorld *worldstate.ServerWorld, addr stypes.UDPAddrStr, dx, dy float32) {
	player := serverWorld.PlayerSnapshot(string(addr))
	playerSize := hitboxes.PlayerSize(player.Health())
	player.Pos = player.Pos.Add(dx, dy).Limited(
		playerSize,
		playerSize,
		serverWorld.Width()-playerSize,
		serverWorld.Height()-playerSize,
	)
	serverWorld.AddPlayerState(string(addr), player)
}

func handleClientInputs(serverWorld *worldstate.ServerWorld, clientInput *clientInput) {
	dirMap := make(map[moveDirection]bool)
	for _, action := range clientInput.input.actions {
		switch act := action.(type) {
		case moveAction:
			dirMap[act.dir] = true
			break
		case shootAction:
			// fmt.Println("since last bullet: ", serverWorld.DurSinceLastBullet(ci.addrStr))
			if serverWorld.DurSinceLastBullet(clientInput.addrStr) > time.Duration(config.BulletCooldownMS)*time.Millisecond {
				playerState := serverWorld.PlayerSnapshot(string(clientInput.addrStr))
				serverWorld.StartPlayerBulletCD(clientInput.addrStr)
				// fmt.Println("shoot!", act.target)
				serverWorld.AddBulletState(worldstate.NewBulletState(playerState, act.target))
			}
			break
		}
	}
	dx, dy := moveDelta(dirMap, 1.0/ticksPerSecond)
	movePlayer(serverWorld, clientInput.addrStr, dx, dy)
}

func updateWorldState(serverWorld *worldstate.ServerWorld, clientsInputs map[stypes.UDPAddrStr]clientInput) {
	for i, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("vector: (%f, %f)", bullet.moveVec.x, bullet.moveVec.y)
		if time.Since(bullet.Born) > time.Duration(config.BulletTimeToLiveSec*float64(time.Second)) {
			serverWorld.RemoveBullet(i)
		}
		// fmt.Println("shooting", bullet.MoveDir.X*bulletSpeed/ticksPerSecond, bullet.MoveDir.Y*bulletSpeed/ticksPerSecond)

		bullet.Pos = bullet.Pos.Add(
			bullet.MoveDir.ScalarMult(bulletSpeed / ticksPerSecond).Coord(),
		)
		if !bullet.Pos.InsideSquare(0, 0, serverWorld.Width(), serverWorld.Height(), config.InitialBulletSize) {
			serverWorld.RemoveBullet(i)
		}
	}

	for _, ci := range clientsInputs {
		if serverWorld.HasPlayer(string(ci.addrStr)) {
			handleClientInputs(serverWorld, &ci)
		}
	}

	calculateHits(serverWorld)
}

func buildPBWorldState(serverWorld *worldstate.ServerWorld) *protobuf.WorldState {
	pbWorld := &protobuf.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		// fmt.Println("sending player: ", player)
		pbPlayer := protobuf.BuildPlayerState(player.Pos, player.Health())
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	// fmt.Printf("bullets amount: %d\n", len(serverWorld.BulletSnapshots()))
	for _, bullet := range serverWorld.BulletSnapshots() {
		// fmt.Printf("building bullet at: (%f, %f)\n", bullet.pos.x, bullet.pos.y)
		pbBullet := protobuf.BuildBulletState(bullet.Pos, bullet.Size)
		pbWorld.Bullets = append(pbWorld.Bullets, &pbBullet)
	}

	return pbWorld
}

func openUDPConn() *net.UDPConn {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP(config.ServerIP),
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

	inputChan := make(chan ClientMsg)
	go clientInputHandler(conn, inputChan)

	clientInputs := make(map[stypes.UDPAddrStr]clientInput)
	for {
		select {
		case <-clock:
			// fmt.Println(tick)

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
			// fmt.Printf("data size: %d\n", len(data))
			for _, addr := range serverWorld.AddressSnapshots() {
				netAddr, _ := net.ResolveUDPAddr("udp", string(addr))
				conn.WriteToUDP(data, netAddr)
			}

		case input := <-inputChan:
			switch in := input.(type) {
			case clientInput:
				clientInputs[in.addrStr] = in
			case clientJoinReq:
				serverWorld.AddAddress(in.addrStr)
				if !serverWorld.HasPlayerState(string(in.addrStr)) {
					serverWorld.AddPlayerState(string(in.addrStr), worldstate.NewPlayerState(
						geom.NewVector(500, 500),
						in.addrStr,
					))
				}
			}
		}
	}
}
