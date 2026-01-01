package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	pb "CircleWar/core/protobuf"
	stypes "CircleWar/core/types"
	wstate "CircleWar/server/world_state"
	"errors"
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

type TickResults struct {
	playersDied []uint
}

// func (clientInput) ClientReqName() string {
// 	return "client input"
// }

type clientJoinReq struct {
	addr     stypes.UDPAddrStr
	gameName string
}

func (clientJoinReq) ClientReqName() string {
	return "join request"
}

type clientRejoinReq struct {
	addr     stypes.UDPAddrStr
	gameName string
	prevId   uint
}

func (clientRejoinReq) ClientReqName() string {
	return "rejoin request"
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

func extractPlayerInput(pi *pb.PlayerInput) *playerInput {
	playerIn := playerInput{
		actions: []playerAction{},
		id:      uint(pi.PlayerId),
	}

	for _, playerAct := range pi.PlayerActions {
		switch act := playerAct.Action.(type) {
		case *pb.PlayerAction_Move:
			if act.Move.Dir == pb.Direction_DOWN {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_DOWN})
			}
			if act.Move.Dir == pb.Direction_UP {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_UP})
			}
			if act.Move.Dir == pb.Direction_RIGHT {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_RIGHT})
			}
			if act.Move.Dir == pb.Direction_LEFT {
				playerIn.actions = append(playerIn.actions, moveAction{DIR_LEFT})
			}
			break
		case *pb.PlayerAction_Shoot:
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

		var gameMsg pb.GameMessage
		if err := proto.Unmarshal(buf[:n], &gameMsg); err != nil {
			fmt.Println(err)
			continue
		}

		clientAddrStr := stypes.UDPAddrStr(clientAddr.String())

		switch payload := gameMsg.Payload.(type) {
		case *pb.GameMessage_PlayerInput:
			// fmt.Println("player input:", payload)
			playerIn := extractPlayerInput(payload.PlayerInput)
			// fmt.Println("player in:", playerIn)
			inputChan <- playerIn
		case *pb.GameMessage_ConnectRequest:
			joinReq := payload.ConnectRequest
			inputChan <- &clientJoinReq{
				addr:     clientAddrStr,
				gameName: joinReq.GameName,
			}
		case *pb.GameMessage_ReconnectRequest:
			rejoinReq := payload.ReconnectRequest
			inputChan <- &clientRejoinReq{
				addr:   clientAddrStr,
				prevId: uint(rejoinReq.OldPlayerId),
			}
		}
	}
}

func calculateHits(serverWorld *wstate.ServerWorld) []uint {
	deadPlayers := []uint{}
	for _, player := range serverWorld.PlayerSnapshots() {
		for bulletId, bullet := range serverWorld.BulletSnapshots() {
			if bullet.OwnerId == player.Id {
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
					deadPlayers = append(deadPlayers, player.Id)
					serverWorld.RemovePlayerState(player.Id)
				} else {
					serverWorld.AddPlayerState(player)
				}
				serverWorld.RemoveBullet(bulletId)
			}
		}
	}
	return deadPlayers
}

func movePlayer(serverWorld *wstate.ServerWorld, id uint, delta geom.Vector2) {
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

func handleClientInputs(serverWorld *wstate.ServerWorld, clientInput *playerInput) {
	dirMap := make(map[moveDirection]bool)
	for _, action := range clientInput.actions {
		// fmt.Println("player id:", clientInput.id)
		switch act := action.(type) {
		case moveAction:
			dirMap[act.dir] = true
			break
		case shootAction:
			playerId := clientInput.id
			if serverWorld.DurSinceLastBullet(playerId) > time.Duration(config.BulletCooldownMS)*time.Millisecond {
				playerState := serverWorld.PlayerSnapshot(playerId)
				serverWorld.StartPlayerBulletCD(playerId)
				serverWorld.AddBulletState(wstate.NewBulletState(playerState, act.target))
			}
			break
		}
	}
	delta := moveDelta(dirMap, 1.0/ticksPerSecond)
	movePlayer(serverWorld, clientInput.id, delta)
}

func updateWorldState(serverWorld *wstate.ServerWorld, playerInputs map[uint]playerInput) []uint {
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

	return calculateHits(serverWorld)
}

func buildPBWorldState(serverWorld *wstate.ServerWorld) *pb.WorldState {
	pbWorld := &pb.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		pbPlayer := pb.BuildPlayerState(player.Pos, player.Health(), uint32(player.Id))
		pbWorld.Players = append(pbWorld.Players, &pbPlayer)
	}

	for _, bullet := range serverWorld.BulletSnapshots() {
		pbBullet := pb.BuildBulletState(bullet.Pos, bullet.Size, uint32(bullet.OwnerId))
		pbWorld.Bullets = append(pbWorld.Bullets, &pbBullet)
	}

	pbWorld.TickNum = serverWorld.Tick()

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

func sendWorldToClients(sw wstate.ServerWorld, conn *net.UDPConn, pbWorld *pb.WorldState) {
	worldMsg := pb.BuildGameMessage(&pb.GameMessage_World{
		World: pbWorld,
	})
	fmt.Println("game message:", worldMsg)
	data, err := proto.Marshal(worldMsg)
	if err != nil {
		log.Fatal(err)
	}

	// fmt.Println("tick num:", sw.Tick())
	// ms := rand.Intn(10) + 35
	// fmt.Println("ms:", ms)
	// time.Sleep(time.Duration(ms) * time.Millisecond)
	// fmt.Printf("data size: %d\n", len(data))
	for _, addr := range sw.AddressSnapshots() {
		netAddr, _ := net.ResolveUDPAddr("udp", string(addr))
		conn.WriteToUDP(data, netAddr)
	}
}

func handleWorldTick(sw *wstate.ServerWorld, playerInputs map[uint]playerInput) *TickResults {
	playersDied := updateWorldState(sw, playerInputs)
	return &TickResults{
		playersDied: playersDied,
	}
}

func handlePlayerConnect(sw *wstate.ServerWorld, req *clientJoinReq) *pb.GameMessage {
	newPlayer := wstate.NewPlayerState(geom.NewVector(500, 500), req.addr)
	sw.AddAddress(newPlayer.Id, newPlayer.Addr)
	fmt.Println("new player:", newPlayer)
	sw.AddPlayerState(newPlayer)
	return pb.BuildConnectAckMsg(newPlayer.Id)
}

func handlePlayerReconnect(sw *wstate.ServerWorld, req *clientRejoinReq) (*pb.GameMessage, error) {
	if sw.GetAddress(req.prevId) != req.addr {
		return nil, errors.New("didn't find player")
	}
	sw.RemovePlayerAddress(req.prevId)
	newPlayer := wstate.NewPlayerState(geom.NewVector(500, 500), req.addr)
	sw.AddAddress(newPlayer.Id, newPlayer.Addr)
	fmt.Println("new player:", newPlayer)
	sw.AddPlayerState(newPlayer)
	return pb.BuildConnectAckMsg(newPlayer.Id), nil
}

func sendConnectAck(conn *net.UDPConn, ack *pb.GameMessage, clientAddr stypes.UDPAddrStr) {
	data, err := proto.Marshal(ack)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("player addr:", clientAddr)
	netAddr, _ := net.ResolveUDPAddr("udp", string(clientAddr))
	conn.WriteToUDP(data, netAddr)
}

func notifyDeadPlayers(sw *wstate.ServerWorld, conn *net.UDPConn, playerIds []uint) {
	for _, id := range playerIds {
		data, err := proto.Marshal(pb.BuildDeathNote(id))
		if err != nil {
			log.Fatal(err)
		}
		netAddr, _ := net.ResolveUDPAddr("udp", string(sw.GetAddress(id)))
		conn.WriteToUDP(data, netAddr)
	}
}

func main() {
	conn := openUDPConn()
	defer conn.Close()
	fmt.Printf("Listening on udp port %d\n", port)

	serverWorld := wstate.NewServerWorld()
	clock := time.Tick(time.Second / ticksPerSecond)
	playerInputs := make(map[uint]playerInput)

	inputChan := make(chan ClientMsg)
	go clientInputHandler(conn, inputChan)

	for {
		select {
		case <-clock:
			// fmt.Println(tick)
			tickResults := handleWorldTick(&serverWorld, playerInputs)
			pbWorld := buildPBWorldState(&serverWorld)
			serverWorld.IncTick()
			notifyDeadPlayers(&serverWorld, conn, tickResults.playersDied)
			go sendWorldToClients(serverWorld, conn, pbWorld)
			playerInputs = make(map[uint]playerInput) // reset inputs for next tick
		case input := <-inputChan:
			switch in := input.(type) {
			case *playerInput:
				playerInputs[in.id] = *in
			case *clientJoinReq:
				ackMsg := handlePlayerConnect(&serverWorld, in)
				sendConnectAck(conn, ackMsg, in.addr)
			case *clientRejoinReq:
				ackMsg, err := handlePlayerReconnect(&serverWorld, in)
				if err != nil {
					break
				}
				sendConnectAck(conn, ackMsg, in.addr)
			default:
				fmt.Println("player input didn't match any case", input)
			}
		}
	}
}
