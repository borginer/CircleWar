package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	"CircleWar/core/netmsg"
	conn "CircleWar/core/network/gameConn"
	envdata "CircleWar/env/env_data"
	envloader "CircleWar/env/env_loader"
	"errors"
	"fmt"
	"log"
	"net"
	"sort"
	"strconv"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	port = config.Port
)

func drawWorld(world *netmsg.WorldState, myId uint32) {
	for i := int32(0); i < config.WorldWidth/100+1; i++ {
		for j := int32(0); j < config.WorldHeight/100+1; j++ {
			if i%2 == 1 {
				continue
			}
			if j%2 == 0 {
				rl.DrawRectangle(i*100, j*100, 100, 100, rl.Brown)
			} else {
				rl.DrawRectangle((i+1)*100, j*100, 100, 100, rl.Brown)
			}
		}
	}
	var color rl.Color
	sort.Slice(world.Players, func(i, j int) bool {
		return world.Players[i].Id < world.Players[j].Id
	})
	for _, player := range world.Players {
		color = rl.Red
		if player.Id == myId {
			color = rl.Blue
		}
		rl.DrawCircle(
			int32(player.Pos.X),
			int32(player.Pos.Y),
			hitboxes.PlayerSize(netmsg.PlayerHealth(player.Health)),
			color,
		)
	}

	for _, bullet := range world.Bullets {
		color = rl.Red
		if bullet.OwnerId == myId {
			color = rl.Blue
		}
		rl.DrawCircle(
			int32(bullet.Pos.X),
			int32(bullet.Pos.Y),
			bullet.Size,
			color,
		)
	}
}

func addPlayerDirAction(pi *netmsg.PlayerInput, dir netmsg.Direction) {
	pi.Actions = append(pi.Actions, &netmsg.MoveAction{Dir: dir})
}

func addPlayerShootAction(pi *netmsg.PlayerInput, target geom.Vector2) {
	pi.Actions = append(pi.Actions, &netmsg.ShootAction{Target: target})
}

func getPlayerInput() *netmsg.PlayerInput {
	playerInput := &netmsg.PlayerInput{}

	// ### WASD ###
	if rl.IsKeyDown(rl.KeyW) {
		addPlayerDirAction(playerInput, netmsg.UP)
	}
	if rl.IsKeyDown(rl.KeyS) {
		addPlayerDirAction(playerInput, netmsg.DOWN)
	}
	if rl.IsKeyDown(rl.KeyA) {
		addPlayerDirAction(playerInput, netmsg.LEFT)
	}
	if rl.IsKeyDown(rl.KeyD) {
		addPlayerDirAction(playerInput, netmsg.RIGHT)
	}

	// ### Shoot with mouse ###
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
		addPlayerShootAction(playerInput, geom.Vector2(mousePos))
	}

	return playerInput
}

func serverInputHandler(conn *conn.ClientConn, serverInput chan netmsg.GameMessage) {
	for {
		servMsg, err := conn.Recieve()
		if err != nil {
			fmt.Println("error receiving from server:", err)
			continue
		}
		serverInput <- servMsg
	}
}

func getMyHealth(ws *netmsg.WorldState, myId uint32) (float32, error) {
	for _, player := range ws.Players {
		if player.Id == myId {
			return player.Health, nil
		}
	}
	return 0, errors.New("player not found in world")
}

func gatherServerMsgs(serverInput chan netmsg.GameMessage) []netmsg.GameMessage {
	msgs := []netmsg.GameMessage{}
	const limit = 30

	for range limit {
		select {
		case msg := <-serverInput:
			fmt.Println("input from server")
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}

	return msgs
}

type Status uint

const (
	NONE = iota
	ALIVE
	DEAD
)

func main() {
	envloader.LoadFile(envdata.EnvfilePath())
	serverIp := envloader.GetEnv("SERVER_IP", "127.0.0.1")

	serverAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverIp, port))
	conn, err := conn.NewClientConn(serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(config.WorldWidth, config.WorldHeight, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(60)

	serverInput := make(chan netmsg.GameMessage, 100)
	go serverInputHandler(conn, serverInput)

	curWorld := &netmsg.WorldState{}
	var playerId uint32
	var lastServerTick uint32 = 0
	status := NONE

	err = conn.Send(netmsg.NewConnectRequest("default"))
	if err != nil {
		fmt.Println("error sending connect request:", err)
	}

	for !rl.WindowShouldClose() {
		if status == ALIVE {
			playerInput := getPlayerInput()
			playerInput.PlayerId = playerId
			err := conn.Send(playerInput)
			if err != nil {
				fmt.Println("error sending player input:", err)
			}
		}

		servMsgs := gatherServerMsgs(serverInput)
		// fmt.Println("#servmsgs:", len(servMsgs))

		for _, msg := range servMsgs {
			switch payload := msg.(type) {
			case *netmsg.WorldState:
				// fmt.Println("world from server:", *payload, "tick:", payload.TickNum)
				if payload.TickNum >= lastServerTick {
					lastServerTick = payload.TickNum
					curWorld = payload
				}
			case *netmsg.ConnectAck:
				fmt.Println("got ack")
				playerId = payload.PlayerId
				status = ALIVE
			case *netmsg.DeathNote:
				status = DEAD
			}
		}

		myHealth, _ := getMyHealth(curWorld, playerId)

		rl.BeginDrawing()
		rl.ClearBackground(rl.NewColor(253, 245, 203, 100))
		drawWorld(curWorld, playerId)
		rl.DrawText("HP : "+strconv.FormatInt(int64(myHealth), 10), 10, 10, 32, rl.Black)

		if status == DEAD {
			bx, by := float32(180), float32(60)
			if gui.Button(rl.Rectangle{
				X: (config.CameraWidth - bx) / 2, Y: (config.CameraHeight - by) / 2,
				Width: bx, Height: by,
			}, "Reconnect") {
				err := conn.Send(netmsg.NewReconnectRequest(playerId))
				if err != nil {
					fmt.Println("failed to send reconnect request:", err)
				}
				status = NONE
			}
		}

		rl.EndDrawing()
	}
}
