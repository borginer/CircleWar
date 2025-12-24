package main

import (
	"CircleWar/gamepb"
	"log"
	"net"

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
	rl.SetTargetFPS(60)

	buf := make([]byte, 1024)
	var world gamepb.WorldState

	for !rl.WindowShouldClose() {
		playerInput := &gamepb.PlayerInput{}

		// ### WASD ###
		moveAction := &gamepb.MoveAction{
			Vert: gamepb.Direction_NONE,
			Hori: gamepb.Direction_NONE,
		}
		if rl.IsKeyDown(rl.KeyW) {
			moveAction.Vert = gamepb.Direction_UP
		}
		if rl.IsKeyDown(rl.KeyS) {
			moveAction.Vert = gamepb.Direction_DOWN
		}
		if rl.IsKeyDown(rl.KeyA) {
			moveAction.Hori = gamepb.Direction_LEFT
		}
		if rl.IsKeyDown(rl.KeyD) {
			moveAction.Hori = gamepb.Direction_RIGHT
		}
		playerInput.PlayerActions = append(playerInput.PlayerActions, &gamepb.PlayerAction{Action: &gamepb.PlayerAction_Move{
			Move: moveAction,
		}})

		// ### Shoot with mouse ###
		if rl.IsMouseButtonDown(rl.MouseButtonLeft) {
			mousePos := rl.GetMousePosition()
			playerInput.PlayerActions = append(playerInput.PlayerActions, &gamepb.PlayerAction{Action: &gamepb.PlayerAction_Shoot{
				Shoot: &gamepb.ShootAction{
					Pos: &gamepb.Position{
						X: mousePos.X,
						Y: mousePos.Y,
					},
				},
			}})
		}

		// Todo: add check later when server ticks happen independently of client packets
		// if moveAction.Hori != gamepb.Direction_NONE && moveAction.Vert != gamepb.Direction_NONE {

		// }

		data, _ := proto.Marshal(playerInput)
		conn.Write(data)

		n, _, err := conn.ReadFromUDP(buf)
		if err == nil {
			proto.Unmarshal(buf[:n], &world)
		}

		rl.BeginDrawing()
		rl.ClearBackground(rl.RayWhite)

		if world.Players != nil {
			for _, player := range world.Players {
				rl.DrawCircle(
					int32(player.Pos.X),
					int32(player.Pos.Y),
					20,
					rl.Blue,
				)
			}

		}

		rl.EndDrawing()
	}
}
