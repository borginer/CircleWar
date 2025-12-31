package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	"CircleWar/core/protobuf"
	sharedtypes "CircleWar/core/types"
	"fmt"
	"log"
	"net"
	// "time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"google.golang.org/protobuf/proto"
)

const (
	port     = config.Port
	serverIP = config.ServerIP
)

func addPlayerDirAction(pi *protobuf.PlayerInput, dir protobuf.Direction) {
	moveAction := protobuf.BuildPlayerMoveAction(dir)
	pi.PlayerActions = append(pi.PlayerActions, &moveAction)
}

func addPlayerShootAction(pi *protobuf.PlayerInput, target geom.Vector2) {
	shootAction := protobuf.BuildPlayerShootAction(target)
	pi.PlayerActions = append(pi.PlayerActions, &shootAction)
}

func drawWorld(world *protobuf.WorldState) {
	for _, player := range world.Players {
		// fmt.Println("player health:", player.Health, "size:", hitboxes.PlayerSize(sharedtypes.PlayerHealth(player.Health)))
		rl.DrawCircle(
			int32(player.Pos.X),
			int32(player.Pos.Y),
			hitboxes.PlayerSize(sharedtypes.PlayerHealth(player.Health)),
			rl.Blue,
		)
	}

	for _, bullet := range world.Bullets {
		// fmt.Println("bullet size:", bullet.Size)
		rl.DrawCircle(
			int32(bullet.Pos.X),
			int32(bullet.Pos.Y),
			bullet.Size,
			rl.Blue,
		)
	}
}

func getPlayerInput() *protobuf.PlayerInput {
	playerInput := &protobuf.PlayerInput{}

	// ### WASD ###
	if rl.IsKeyDown(rl.KeyW) {
		addPlayerDirAction(playerInput, protobuf.Direction_UP)
	}
	if rl.IsKeyDown(rl.KeyS) {
		addPlayerDirAction(playerInput, protobuf.Direction_DOWN)
	}
	if rl.IsKeyDown(rl.KeyA) {
		addPlayerDirAction(playerInput, protobuf.Direction_LEFT)
	}
	if rl.IsKeyDown(rl.KeyD) {
		addPlayerDirAction(playerInput, protobuf.Direction_RIGHT)
	}

	// ### Shoot with mouse ###
	if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
		mousePos := rl.GetMousePosition()
		addPlayerShootAction(playerInput, geom.Vector2(mousePos))
	}

	return playerInput
}

func serverInputHandler(conn *net.UDPConn, serverInput chan *protobuf.GameMessage) {
	buf := make([]byte, 1024)
	i := 0
	servMsg := &protobuf.GameMessage{}
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		// fmt.Println("bytes from udp:", n)
		err = proto.Unmarshal(buf[:n], servMsg)
		if err != nil {
			continue
		}
		i++
		// fmt.Printf("server update number %d at time: %s\n", i, time.Now().String())
		serverInput <- servMsg
		servMsg = &protobuf.GameMessage{}
	}
}

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverIP, port))
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(config.WorldWidth, config.WorldHeight, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(100)

	world := &protobuf.WorldState{}
	serverMsg := make(chan *protobuf.GameMessage)
	go serverInputHandler(conn, serverMsg)

	data, err := proto.Marshal(&protobuf.GameMessage{
		Payload: &protobuf.GameMessage_ConnectRequest{
			ConnectRequest: &protobuf.ConnectRequest{
				GameName: "default",
			},
		},
	})
	conn.Write(data)

	var playerId uint32

	for !rl.WindowShouldClose() {
		playerInput := getPlayerInput()
		playerInput.PlayerId = playerId
		inputMsg := protobuf.BuildGameMessage(&protobuf.GameMessage_PlayerInput{
			PlayerInput: playerInput,
		})

		if len(playerInput.PlayerActions) > 0 {
			data, err := proto.Marshal(inputMsg)
			// fmt.Println("player input:", playerInput)
			if err != nil {
				log.Fatal(err)
			}
			conn.Write(data)
		}

		select {
		case msg := <-serverMsg:
			switch payload := msg.Payload.(type) {
			case *protobuf.GameMessage_World:
				world = payload.World
			case *protobuf.GameMessage_ConnectAck:
				playerId = payload.ConnectAck.PlayerId
			}
		default:
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)
		drawWorld(world)
		rl.EndDrawing()
	}
}
