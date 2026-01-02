package main

import (
	"CircleWar/config"
	"CircleWar/core/geom"
	"CircleWar/core/hitboxes"
	pb "CircleWar/core/network/protobuf"
	stypes "CircleWar/core/types"
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"

	gui "github.com/gen2brain/raylib-go/raygui"
	rl "github.com/gen2brain/raylib-go/raylib"
	"google.golang.org/protobuf/proto"
)

const (
	port     = config.Port
	serverIP = config.ServerIP
)

func drawWorld(world *pb.WorldState, myId uint32) {
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
		// fmt.Println("player health:", player.Health, "size:", hitboxes.PlayerSize(sharedtypes.PlayerHealth(player.Health)))
		if player.PlayerId == myId {
			color = rl.Blue
		} else {
			color = rl.Red
		}
		rl.DrawCircle(
			int32(player.Pos.X),
			int32(player.Pos.Y),
			hitboxes.PlayerSize(stypes.PlayerHealth(player.Health)),
			color,
		)
	}

	for _, bullet := range world.Bullets {
		// fmt.Println("bullet size:", bullet.Size)
		if bullet.OwnerId == myId {
			color = rl.Blue
		} else {
			color = rl.Red
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

func serverInputHandler(conn *net.UDPConn, serverInput chan *pb.GameMessage) {
	buf := make([]byte, 1024)
	i := 0
	servMsg := &pb.GameMessage{}
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			fmt.Println("read err:", err)
			continue
		}
		fmt.Println("bytes from udp:", n, "addr:", addr)
		err = proto.Unmarshal(buf[:n], servMsg)
		if err != nil {
			fmt.Println("unmarshal err:", err)
			continue
		}
		i++
		// fmt.Printf("server update number %d at time: %s\n", i, time.Now().String())
		serverInput <- servMsg
		servMsg = &pb.GameMessage{}
	}
}

func getMyHealth(ws *pb.WorldState, myId uint32) (float32, error) {
	for _, player := range ws.Players {
		if player.PlayerId == myId {
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
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	rl.InitWindow(config.WorldWidth, config.WorldHeight, "CircleWar Client")
	defer rl.CloseWindow()
	rl.SetTargetFPS(100)

	serverMsg := make(chan *pb.GameMessage)
	go serverInputHandler(conn, serverMsg)

	curWorld := &pb.WorldState{}
	var playerId uint32
	var lastServerTick uint32 = 0
	status := NONE

	// send connect

	data, err := proto.Marshal(&pb.GameMessage{
		Payload: &pb.GameMessage_ConnectRequest{
			ConnectRequest: &pb.ConnectRequest{
				GameName: "default",
			},
		},
	})
	if err != nil {
		log.Fatal("fuck")
	}
	conn.Write(data)

	for !rl.WindowShouldClose() {
		if status == ALIVE {
			playerInput := getPlayerInput()
			playerInput.PlayerId = playerId
			// fmt.Println("player input:", playerInput)

			if len(playerInput.Actions) > 0 {
				inputMsg := pb.BuildPlayerInput(playerInput)
				data, err := proto.Marshal(inputMsg)
				if err != nil {
					log.Fatal(err)
				}
				conn.Write(data)
			}
		}

		select {
		case msg := <-serverMsg:
			// fmt.Println("msg:", msg)
			switch payload := msg.Payload.(type) {
			case *pb.GameMessage_World:
				// fmt.Println("tick num:", curWorld.TickNum, "last tick:", lastServerTick)
				if payload.World.TickNum >= lastServerTick {
					lastServerTick = payload.World.TickNum
					curWorld = payload.World
				}
			case *pb.GameMessage_ConnectAck:
				playerId = payload.ConnectAck.PlayerId
				// fmt.Println("got connectAck, id:", playerId)
				status = ALIVE
			case *pb.GameMessage_DeathNote:
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
				data, err := proto.Marshal(&pb.GameMessage{
					Payload: &pb.GameMessage_ReconnectRequest{
						ReconnectRequest: &pb.ReconnectRequest{
							OldPlayerId: playerId,
						},
					},
				})
				if err != nil {
					log.Fatal("fuck")
				}
				conn.Write(data)
				status = NONE
			}
		}

		rl.EndDrawing()
	}
}
