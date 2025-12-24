package main

import (
	"CircleWar/gamepb"
	"fmt"
	"log"
	"math"
	"net"
	"time"

	"google.golang.org/protobuf/proto"
)

type ClientState struct {
	Player gamepb.PlayerState
}

func tickDist(vert, hori gamepb.Direction, delta float64) (float64, float64) {
	const speed = 700
	dx, dy := 0.0, 0.0
	if vert == gamepb.Direction_DOWN {
		dy = speed
	} else if vert == gamepb.Direction_UP {
		dy = -speed
	}

	if hori == gamepb.Direction_LEFT {
		dx = -speed
	} else if hori == gamepb.Direction_RIGHT {
		dx = speed
	}

	if dx != 0 && dy != 0 {
		norm := math.Sqrt(2)
		dx /= norm
		dy /= norm
	}
	return dx * delta, dy * delta
}

func main() {
	const port = 4000
	addr := net.UDPAddr{
		Port: port,
		IP:   net.ParseIP("0.0.0.0"),
	}
	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	fmt.Printf("Listening on udp port %d\n", port)

	clients := make(map[string]*ClientState)
	buf := make([]byte, 1024)
	last_tick_time := time.Now()

	for {
		delta := float64(time.Since(last_tick_time).Microseconds()) / 1e6
		fmt.Println("delta: ", delta)
		last_tick_time = time.Now()

		n, remoteAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		client_addr := remoteAddr.String()
		if _, ok := clients[client_addr]; !ok {
			clients[client_addr] =
				&ClientState{
					Player: gamepb.PlayerState{
						Pos: &gamepb.Position{X: 500, Y: 500},
					},
				}
		}

		var playerInput gamepb.PlayerInput
		if err := proto.Unmarshal(buf[:n], &playerInput); err != nil {
			continue
		}

		for _, playerAct := range playerInput.PlayerActions {
			switch act := playerAct.Action.(type) {
			case *gamepb.PlayerAction_Move:
				player := &clients[client_addr].Player
				dx, dy := tickDist(act.Move.Vert, act.Move.Hori, delta)
				player.Pos.X += float32(dx)
				player.Pos.Y += float32(dy)

				world := &gamepb.WorldState{}

				for _, client := range clients {
					world.Players = append(world.Players, &client.Player)
				}

				data, _ := proto.Marshal(world)
				conn.WriteToUDP(data, remoteAddr)
				break
			case *gamepb.PlayerAction_Shoot:
				fmt.Println("shoot!")
				break
			}
		}
	}
}
