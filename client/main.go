package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	conn "CircleWar/core/network/gameConn"
	stypes "CircleWar/core/stypes"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
)

const (
	port     = config.Port
	serverIP = config.ServerIP
)

func drawWorld(world *stypes.WorldState, myId uint32) {
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
	for _, player := range world.Players {
		color = rl.Red
		if player.Id == myId {
			color = rl.Blue
		}
		rl.DrawCircle(
			int32(player.Pos.X),
			int32(player.Pos.Y),
			hitboxes.PlayerSize(stypes.PlayerHealth(player.Health)),
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

func addPlayerDirAction(pi *stypes.PlayerInput, dir stypes.Direction) {
	pi.Actions = append(pi.Actions, &stypes.MoveAction{Dir: dir})
}

func addPlayerShootAction(pi *stypes.PlayerInput, target geom.Vector2) {
	pi.Actions = append(pi.Actions, &stypes.ShootAction{Target: target})
}

func getPlayerInput() *stypes.PlayerInput {
	playerInput := &stypes.PlayerInput{}

	// ### WASD ###
	if rl.IsKeyDown(rl.KeyW) {
		addPlayerDirAction(playerInput, stypes.UP)
	}
	if rl.IsKeyDown(rl.KeyS) {
		addPlayerDirAction(playerInput, stypes.DOWN)
	}
	if rl.IsKeyDown(rl.KeyA) {
		addPlayerDirAction(playerInput, stypes.LEFT)
	}
	if rl.IsKeyDown(rl.KeyD) {
		addPlayerDirAction(playerInput, stypes.RIGHT)
	}

	// ### Shoot with mouse ###
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
		addPlayerShootAction(playerInput, geom.Vector2(mousePos))
	}

	return playerInput
}

func serverInputHandler(conn *conn.ClientConn, serverInput chan stypes.GameMessage) {
	for {
		servMsg, err := conn.Recieve()
		if err != nil {
			fmt.Println("error receiving from server:", err)
			continue
		}
		serverInput <- servMsg
	}
}

func getMyHealth(ws *stypes.WorldState, myId uint32) (float32, error) {
	for _, player := range ws.Players {
		if player.Id == myId {
			return player.Health, nil
		}
	}
	return 0, errors.New("player not found in world")
}

type Status uint

const (
	NONE = iota
	ALIVE
	DEAD
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverIP, port))
	conn, err := conn.NewClientConn(serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(config.WorldWidth, config.WorldHeight, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(200)

	serverMsg := make(chan stypes.GameMessage)
	go serverInputHandler(conn, serverMsg)

	curWorld := &stypes.WorldState{}
	var playerId uint32
	var lastServerTick uint32 = 0
	status := NONE

	err = conn.Send(stypes.NewConnectRequest("default"))
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

		select {
		case msg := <-serverMsg:
			switch payload := msg.(type) {
			case *stypes.WorldState:
				// fmt.Println("world from server:", *payload, "tick:", payload.TickNum)
				if payload.TickNum >= lastServerTick {
					lastServerTick = payload.TickNum
					curWorld = payload
				}
			case *stypes.ConnectAck:
				fmt.Println("got ack")
				playerId = payload.PlayerId
				status = ALIVE
			case *stypes.DeathNote:
				status = DEAD
			}
		default:
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
				err := conn.Send(stypes.NewReconnectRequest(playerId))
				if err != nil {
					fmt.Println("failed to send reconnect request:", err)
				}
				status = NONE
			}
		}

		rl.EndDrawing()
	}
}
