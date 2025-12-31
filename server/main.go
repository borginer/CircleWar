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
	id      uint
	actions []playerAction
}

func (playerInput) ClientReqName() string {
	return "player input"
}

type ClientMsg interface {
	ClientReqName() string
}

// func (clientInput) ClientReqName() string {
// 	return "client input"
// }

type clientJoinReq struct {
	addrStr  stypes.UDPAddrStr
	gameName string
}

func (clientJoinReq) ClientReqName() string {
	return "join request"
}

func moveDelta(inputs map[moveDirection]bool, delta float32) geom.Vector2 {
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

	return geom.NewVector(float32(dx)*delta, float32(dy)*delta)
}

func extractPlayerInput(pi *protobuf.PlayerInput) *playerInput {
	playerIn := playerInput{
		actions: []playerAction{},
		id:      uint(pi.PlayerId),
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
			fmt.Println("player input:", payload)
			playerIn := extractPlayerInput(payload.PlayerInput)
			fmt.Println("player in:", playerIn)
			inputChan <- playerIn
		case *protobuf.GameMessage_ConnectRequest:
			joinReq := payload.ConnectRequest
			inputChan <- &clientJoinReq{
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
					serverWorld.RemovePlayerState(player.Id)
				} else {
					serverWorld.AddPlayerState(player)
				}
				serverWorld.RemoveBullet(bulletId)
			}
		}
	}
}

func movePlayer(serverWorld *worldstate.ServerWorld, id uint, delta geom.Vector2) {
	player := serverWorld.PlayerSnapshot(id)
	playerSize := hitboxes.PlayerSize(player.Health())
	player.Pos = player.Pos.Add(delta).Limited(
		playerSize,
		playerSize,
		serverWorld.Width()-playerSize,
		serverWorld.Height()-playerSize,
	)
	serverWorld.AddPlayerState(player)
}

func handleClientInputs(serverWorld *worldstate.ServerWorld, clientInput *playerInput) {
	dirMap := make(map[moveDirection]bool)
	for _, action := range clientInput.actions {
		fmt.Println("player id:", clientInput.id)
		switch act := action.(type) {
		case moveAction:
			dirMap[act.dir] = true
			break
		case shootAction:
			playerId := clientInput.id
			if serverWorld.DurSinceLastBullet(playerId) > time.Duration(config.BulletCooldownMS)*time.Millisecond {
				playerState := serverWorld.PlayerSnapshot(playerId)
				serverWorld.StartPlayerBulletCD(playerId)
				serverWorld.AddBulletState(worldstate.NewBulletState(playerState, act.target))
			}
			break
		}
	}
	delta := moveDelta(dirMap, 1.0/ticksPerSecond)
	movePlayer(serverWorld, clientInput.id, delta)
}

func updateWorldState(serverWorld *worldstate.ServerWorld, playerInputs map[uint]playerInput) {
	for i, bullet := range serverWorld.BulletSnapshots() {
		if time.Since(bullet.Born) > time.Duration(config.BulletTimeToLiveSec*float64(time.Second)) {
			serverWorld.RemoveBullet(i)
		}

		bullet.Pos = bullet.Pos.Add(
			bullet.MoveDir.ScalarMult(bulletSpeed / ticksPerSecond),
		)
		if !bullet.Pos.InsideSquare(0, 0, serverWorld.Width(), serverWorld.Height(), config.InitialBulletSize) {
			serverWorld.RemoveBullet(i)
		}
	}

	for _, ci := range playerInputs {
		if serverWorld.HasPlayer(ci.id) {
			handleClientInputs(serverWorld, &ci)
		}
	}

	calculateHits(serverWorld)
}

func buildPBWorldState(serverWorld *worldstate.ServerWorld) *protobuf.WorldState {
	pbWorld := &protobuf.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		pbPlayer := protobuf.BuildPlayerState(player.Pos, player.Health())
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	for _, bullet := range serverWorld.BulletSnapshots() {
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

	playerInputs := make(map[uint]playerInput)
	for {
		select {
		case <-clock:
			// fmt.Println(tick)

			updateWorldState(&serverWorld, playerInputs)
			playerInputs = make(map[uint]playerInput)

			pbWorld := buildPBWorldState(&serverWorld)
			// if len(pbWorld.Players) > 0 {
			// 	fmt.Println("player health: ", pbWorld.Players[0].Health)
			// }
			worldMsg := protobuf.BuildGameMessage(&protobuf.GameMessage_World{
				World: pbWorld,
			})
			data, err := proto.Marshal(worldMsg)
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
			case *playerInput:
				playerInputs[in.id] = *in
			case *clientJoinReq:
				serverWorld.AddAddress(in.addrStr)
				// if !serverWorld.HasPlayerState(in.Id) {
				// TODO: add udp addr to player id mapping to check if player is reconnecting
				// instead of always creating new player
				// }
				newPlayer := worldstate.NewPlayerState(geom.NewVector(500, 500), in.addrStr)
				fmt.Println("new player:", newPlayer)
				serverWorld.AddPlayerState(newPlayer)
				connectMsg := protobuf.BuildConnectAckMsg(newPlayer.Id)
				data, err := proto.Marshal(connectMsg)
				if err != nil {
					log.Fatal(err)
				}
				fmt.Println("player addr:", in.addrStr)
				netAddr, _ := net.ResolveUDPAddr("udp", string(in.addrStr))
				conn.WriteToUDP(data, netAddr)
			default:
				fmt.Println("player input didn't match any case", input)
			}
		}
	}
}
