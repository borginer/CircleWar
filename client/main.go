package main

import (
	"CircleWar/config"
	"CircleWar/core/hitboxes"
	"CircleWar/core/protobuf"
	sharedtypes "CircleWar/core/types"
	"fmt"
	"log"
	"net"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"google.golang.org/protobuf/proto"
)

const (
	port     = config.Port
	serverIP = config.ServerIP
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", serverIP, port))
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(1320, 860, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(400)

	buf := make([]byte, 2048)
	var world protobuf.WorldState
	i := 0

	for !rl.WindowShouldClose() {
		playerInput := &protobuf.PlayerInput{}

		// ### WASD ###
		moveAction := &protobuf.MoveAction{
			Vert: protobuf.Direction_NONE,
			Hori: protobuf.Direction_NONE,
		}
		if rl.IsKeyDown(rl.KeyW) {
			moveAction.Vert = protobuf.Direction_UP
		}
		if rl.IsKeyDown(rl.KeyS) {
			moveAction.Vert = protobuf.Direction_DOWN
		}
		if rl.IsKeyDown(rl.KeyA) {
			moveAction.Hori = protobuf.Direction_LEFT
		}
		if rl.IsKeyDown(rl.KeyD) {
			moveAction.Hori = protobuf.Direction_RIGHT
		}
		playerInput.PlayerActions = append(
			playerInput.PlayerActions,
			&protobuf.PlayerAction{
				Action: &protobuf.PlayerAction_Move{
					Move: moveAction,
				}},
		)

		// ### Shoot with mouse ###
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			mousePos := rl.GetMousePosition()
			playerInput.PlayerActions = append(playerInput.PlayerActions, &protobuf.PlayerAction{Action: &protobuf.PlayerAction_Shoot{
				Shoot: &protobuf.ShootAction{
					Pos: &protobuf.Position{
						X: mousePos.X,
						Y: mousePos.Y,
					},
				},
			}})
		}

		// TODO: add check after client is sending the first connect packet so the server knows it exists
		// if moveAction.Hori != protobuf.Direction_NONE || moveAction.Vert != protobuf.Direction_NONE {}

		data, err := proto.Marshal(playerInput)
		if err != nil {
			log.Fatal(err)
		}
		conn.Write(data)

		n, _, err := conn.ReadFromUDP(buf)
		if err == nil {
			fmt.Println("bytes from udp: ", n)
			err = proto.Unmarshal(buf[:n], &world)
			i++
			fmt.Printf("server update number %d at time: %s\n", i, time.Now().String())
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		if world.Players != nil {
			for _, player := range world.Players {
				fmt.Println("player health:", player.Health, "size: ", hitboxes.PlayerSize(sharedtypes.PlayerHealth(player.Health)))

				rl.DrawCircle(
					int32(player.Pos.X),
					int32(player.Pos.Y),
					hitboxes.PlayerSize(sharedtypes.PlayerHealth(player.Health)),
					rl.Blue,
				)
			}
		}

		if world.Bullets != nil {
			fmt.Println("bullets not null")
			for _, bullet := range world.Bullets {
				fmt.Println("bullet size: ", bullet.Size)
				rl.DrawCircle(
					int32(bullet.Pos.X),
					int32(bullet.Pos.Y),
					bullet.Size,
					rl.Blue,
				)
			}
		}

		rl.EndDrawing()
	}
}
