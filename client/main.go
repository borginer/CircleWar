package main

import (
	"CircleWar/protob"
	"fmt"
	"log"
	"net"
	"time"

	rl "github.com/gen2brain/raylib-go/raylib"
	"google.golang.org/protobuf/proto"
)

func main() {
	serverAddr, _ := net.ResolveUDPAddr("udp", "127.0.0.1:4000")
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(1080, 640, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(400)

	buf := make([]byte, 1024)
	var world protob.WorldState
	i := 0

	for !rl.WindowShouldClose() {
		playerInput := &protob.PlayerInput{}

		// ### WASD ###
		moveAction := &protob.MoveAction{
			Vert: protob.Direction_NONE,
			Hori: protob.Direction_NONE,
		}
		if rl.IsKeyDown(rl.KeyW) {
			moveAction.Vert = protob.Direction_UP
		}
		if rl.IsKeyDown(rl.KeyS) {
			moveAction.Vert = protob.Direction_DOWN
		}
		if rl.IsKeyDown(rl.KeyA) {
			moveAction.Hori = protob.Direction_LEFT
		}
		if rl.IsKeyDown(rl.KeyD) {
			moveAction.Hori = protob.Direction_RIGHT
		}
		playerInput.PlayerActions = append(playerInput.PlayerActions, &protob.PlayerAction{Action: &protob.PlayerAction_Move{
			Move: moveAction,
		}})

		// ### Shoot with mouse ###
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			mousePos := rl.GetMousePosition()
			playerInput.PlayerActions = append(playerInput.PlayerActions, &protob.PlayerAction{Action: &protob.PlayerAction_Shoot{
				Shoot: &protob.ShootAction{
					Pos: &protob.Position{
						X: mousePos.X,
						Y: mousePos.Y,
					},
				},
			}})
		}

		// Todo: add check later when server ticks happen independently of client packets
		// if moveAction.Hori != protob.Direction_NONE && moveAction.Vert != protob.Direction_NONE {

		// }

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
				// fmt.Printf("x: %f, y: %f\n", player.Pos.X, player.Pos.Y)
				rl.DrawCircle(
					int32(player.Pos.X),
					int32(player.Pos.Y),
					26,
					rl.Blue,
				)
			}
		}

		if world.Bullets != nil {
			fmt.Println("bullets not null")
			for _, bullet := range world.Bullets {
				// fmt.Printf("x: %f, y: %f\n", player.Pos.X, player.Pos.Y)
				rl.DrawCircle(
					int32(bullet.Pos.X),
					int32(bullet.Pos.Y),
					14,
					rl.Green,
				)
			}
		}

		rl.EndDrawing()
	}
}
