package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	stypes "CircleWar/core/netmsg"
	"CircleWar/core/network/gameConn"
	wstate "CircleWar/server/world_state"
	"errors"
	"fmt"
	"log"
	"math"
	"net"
	"time"
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

type clientInput struct {
	addr    net.UDPAddr
	gameMsg stypes.GameMessage
}

type TickResults struct {
	playersDied []uint
}

func moveDelta(inputs map[stypes.Direction]bool, delta float32) geom.Vector2 {
	dx, dy := 0.0, 0.0
	for act := range inputs {
		switch act {
		case stypes.LEFT:
			dx = -playerSpeed
		case stypes.RIGHT:
			dx = playerSpeed
		case stypes.UP:
			dy = -playerSpeed
		case stypes.DOWN:
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

func clientInputHandler(conn *gameConn.ServerConn, inputChan chan clientInput) {
	for {
		clientMsg, clientAddr, err := conn.Recieve()
		if err != nil {
			continue
		}
		inputChan <- clientInput{clientAddr, clientMsg}
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
	player := serverWorld.Player(id)
	playerSize := hitboxes.PlayerSize(player.Health())
	player.Pos = player.Pos.Add(delta).Limited(
		playerSize,
		playerSize,
		serverWorld.Width()-playerSize,
		serverWorld.Height()-playerSize,
	)
}

func handleClientInputs(serverWorld *wstate.ServerWorld, clientInput *stypes.PlayerInput) {
	playerId := uint(clientInput.PlayerId)
	for _, action := range clientInput.Actions {
		switch act := action.(type) {
		case *stypes.MoveAction:
			serverWorld.PlayerWants(playerId).MoveDirs[act.Dir] = true
			break
		case *stypes.ShootAction:
			if serverWorld.DurSinceLastBullet(playerId) > time.Duration(config.BulletCooldownMS)*time.Millisecond {
				playerState := serverWorld.Player(playerId)
				serverWorld.StartPlayerBulletCD(playerId)
				serverWorld.AddBulletState(wstate.NewBulletState(*playerState, act.Target))
			}
			break
		default:
			fmt.Println("unrecognized player action!")
		}
	}
}

func changeEntityStates(serverWorld *wstate.ServerWorld) {
	for _, player := range serverWorld.PlayerSnapshots() {
		playerId := player.Id
		delta := moveDelta(serverWorld.PlayerWants(playerId).MoveDirs, 1.0/ticksPerSecond)
		movePlayer(serverWorld, playerId, delta)
	}
}

func updateWorldState(serverWorld *wstate.ServerWorld, playerInputs map[uint]stypes.PlayerInput) []uint {
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
		fmt.Println("input from pid:", ci.PlayerId)
		clear(serverWorld.PlayerWants(uint(ci.PlayerId)).MoveDirs)

		if serverWorld.HasPlayer(uint(ci.PlayerId)) {
			handleClientInputs(serverWorld, &ci)
		}
	}

	changeEntityStates(serverWorld)

	return calculateHits(serverWorld)
}

func buildNetworkWorldState(serverWorld *wstate.ServerWorld) *stypes.WorldState {
	netWorld := &stypes.WorldState{}

	players := serverWorld.PlayerSnapshots()
	for _, player := range players {
		netWorld.Players = append(netWorld.Players, stypes.NewPlayerState(
			uint32(player.Id),
			player.Pos,
			float32(player.Health()),
		))
	}

	for _, bullet := range serverWorld.BulletSnapshots() {
		netWorld.Bullets = append(netWorld.Bullets, stypes.NewBulletState(
			uint32(bullet.OwnerId),
			bullet.Pos,
			bullet.Size,
		))
	}

	netWorld.TickNum = serverWorld.Tick()

	return netWorld
}

func handleWorldTick(sw *wstate.ServerWorld, playerInputs map[uint]stypes.PlayerInput) *TickResults {
	playersDied := updateWorldState(sw, playerInputs)
	return &TickResults{
		playersDied: playersDied,
	}
}

func handlePlayerConnect(sw *wstate.ServerWorld, req *stypes.ConnectRequest, addr net.UDPAddr) *stypes.ConnectAck {
	newPlayer := wstate.NewPlayerState(geom.NewVector(500, 500), addr)
	sw.AddAddress(newPlayer.Id, addr)
	fmt.Println("new player:", newPlayer)
	sw.AddPlayerState(newPlayer)
	return &stypes.ConnectAck{PlayerId: uint32(newPlayer.Id)}
}

func handlePlayerReconnect(sw *wstate.ServerWorld, req *stypes.ReconnectRequest, addr net.UDPAddr) (*stypes.ConnectAck, error) {
	oldAddr := sw.GetAddress(uint(req.OldPlayerId))
	if oldAddr.String() != addr.String() {
		return nil, errors.New("didn't find player")
	}
	sw.RemovePlayerAddress(uint(req.OldPlayerId))
	newPlayer := wstate.NewPlayerState(geom.NewVector(500, 500), addr)
	sw.AddAddress(newPlayer.Id, newPlayer.Addr)
	fmt.Println("new player:", newPlayer)
	sw.AddPlayerState(newPlayer)
	connectAck := &stypes.ConnectAck{PlayerId: uint32(newPlayer.Id)}
	return connectAck, nil
}

func notifyDeadPlayers(sw *wstate.ServerWorld, conn *gameConn.ServerConn, playerIds []uint) {
	for _, id := range playerIds {
		conn.SendTo(stypes.NewDeathNote(uint32(id)), sw.GetAddress(id))
	}
}

func main() {
	conn, err := gameConn.NewServerConn(net.ParseIP(config.ServerIP), port)
	if err != nil {
		log.Fatal("whoops")
	}
	defer conn.Close()
	fmt.Printf("Listening on udp port %d\n", port)

	serverWorld := wstate.NewServerWorld()
	clock := time.Tick(time.Second / ticksPerSecond)
	playerInputs := make(map[uint]stypes.PlayerInput)

	inputChan := make(chan clientInput, 10)
	go clientInputHandler(conn, inputChan)

	for {
		select {
		case <-clock:
			tickResults := handleWorldTick(&serverWorld, playerInputs)
			// fmt.Println("server world:", serverWorld)
			netWorld := buildNetworkWorldState(&serverWorld)
			serverWorld.NextTick()
			notifyDeadPlayers(&serverWorld, conn, tickResults.playersDied)
			fmt.Println("sending tick#", netWorld.TickNum)
			conn.Broadcast(netWorld)
			playerInputs = make(map[uint]stypes.PlayerInput) // reset inputs for next tick
		case input := <-inputChan:
			switch in := input.gameMsg.(type) {
			case *stypes.PlayerInput:
				// fmt.Println("player input gotten:", *in)
				playerInputs[uint(in.PlayerId)] = *in
			case *stypes.ConnectRequest:
				conn.AddListener(input.addr)
				ackMsg := handlePlayerConnect(&serverWorld, in, input.addr)
				conn.SendTo(ackMsg, input.addr)
			case *stypes.ReconnectRequest:
				fmt.Println("sending ack msg")
				ackMsg, err := handlePlayerReconnect(&serverWorld, in, input.addr)
				if err != nil {
					break
				}
				conn.SendTo(ackMsg, input.addr)
			default:
				fmt.Println("player input didn't match any case", input)
			}
		}
	}
}
