package main

import (
	"CircleWar/gamepb"
	"fmt"
	"log"
	"math"
	"net"
	"sync"
	"time"

	"google.golang.org/protobuf/proto"
)

type position struct {
	x, y float32
}

type action int16
type udpAddrStr string

const (
	DIR_LEFT  action = 0
	DIR_RIGHT action = 1
	DIR_UP    action = 2
	DIR_DOWN  action = 3
	SHOOT     action = 4
)

type playerInput struct {
	actions map[action]bool
}

type clientState struct {
	pos position
}

type clientsInfo struct {
	states    map[string]*clientState
	addresses map[udpAddrStr]bool
	mut       sync.Mutex
}

func (ci *clientsInfo) setAddress(addr udpAddrStr) {
	ci.mut.Lock()
	defer ci.mut.Unlock()
	ci.addresses[addr] = true
}

func (ci *clientsInfo) getAddressesCopy() []udpAddrStr {
	copy := []udpAddrStr{}
	ci.mut.Lock()
	defer ci.mut.Unlock()
	for addr := range ci.addresses {
		copy = append(copy, addr)
	}
	return copy
}

func (ci *clientsInfo) getStatesCopy() []clientState {
	copy := []clientState{}
	ci.mut.Lock()
	defer ci.mut.Unlock()
	for _, state := range ci.states {
		copy = append(copy, *state)
	}
	return copy
}

func (ci *clientsInfo) setState(key string, state *clientState) {
	ci.mut.Lock()
	defer ci.mut.Unlock()
	ci.states[key] = state
}

func (ci *clientsInfo) getStateCopy(key string) clientState {
	ci.mut.Lock()
	defer ci.mut.Unlock()
	return *ci.states[key]
}

func (ci *clientsInfo) hasState(key string) bool {
	ci.mut.Lock()
	defer ci.mut.Unlock()
	_, ok := ci.states[key]
	return ok
}

func moveDelta(inputs map[action]bool, delta float64) (float64, float64) {
	const speed = 700
	dx, dy := 0.0, 0.0
	for act, _ := range inputs {
		switch act {
		case DIR_LEFT:
			dx = -speed
		case DIR_RIGHT:
			dx = speed
		case DIR_UP:
			dy = -speed
		case DIR_DOWN:
			dy = speed
		default:
			continue
		}
	}

	if dx != 0 && dy != 0 {
		norm := math.Sqrt(2)
		dx /= norm
		dy /= norm
	}

	return dx * delta, dy * delta
}

type clientInput struct {
	addrStr udpAddrStr
	input   playerInput
}

func readHandler(conn *net.UDPConn, inputChan chan clientInput) {
	buf := make([]byte, 1024)

	for {
		n, clientAddr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var pbInput gamepb.PlayerInput
		if err := proto.Unmarshal(buf[:n], &pbInput); err != nil {
			continue
		}

		clientAddrStr := udpAddrStr(clientAddr.String())
		playerIn := playerInput{
			actions: make(map[action]bool),
		}

		for _, playerAct := range pbInput.PlayerActions {
			switch act := playerAct.Action.(type) {
			case *gamepb.PlayerAction_Move:
				if act.Move.Vert == gamepb.Direction_DOWN {
					playerIn.actions[DIR_DOWN] = true
				}
				if act.Move.Vert == gamepb.Direction_UP {
					playerIn.actions[DIR_UP] = true
				}
				if act.Move.Hori == gamepb.Direction_RIGHT {
					playerIn.actions[DIR_RIGHT] = true
				}
				if act.Move.Hori == gamepb.Direction_LEFT {
					playerIn.actions[DIR_LEFT] = true
				}
				break
			case *gamepb.PlayerAction_Shoot:
				playerIn.actions[SHOOT] = true
				break
			}
		}

		inputChan <- clientInput{
			addrStr: clientAddrStr,
			input:   playerIn,
		}
	}
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

	clients_info := clientsInfo{
		states:    make(map[string]*clientState),
		addresses: make(map[udpAddrStr]bool),
	}

	const ticksPerSecond = 40
	clock := time.Tick(time.Second / ticksPerSecond)

	inputChan := make(chan clientInput)
	go readHandler(conn, inputChan)

	clientInputs := make(map[udpAddrStr]clientInput)
	for {
		select {
		case tick := <-clock:
			fmt.Println(tick)

			for _, input := range clientInputs {
				dx, dy := moveDelta(input.input.actions, float64(1)/ticksPerSecond)
				state := clients_info.getStateCopy(string(input.addrStr))
				state.pos.x += float32(dx)
				state.pos.y += float32(dy)
				clients_info.setState(string(input.addrStr), &state)
			}

			for uas := range clientInputs {
				delete(clientInputs, uas)
			}			

			world := &gamepb.WorldState{}

			client_states := clients_info.getStatesCopy()
			for _, client := range client_states {
				player := gamepb.BuildPlayerState(client.pos.x, client.pos.y)
				fmt.Printf("pos before sending: %f, %f\n", player.Pos.X, player.Pos.Y)
				world.Players = append(world.Players, &player)
			}

			data, _ := proto.Marshal(world)
			client_addressses := clients_info.getAddressesCopy()
			for _, addr := range client_addressses {
				netAddr, _ := net.ResolveUDPAddr("udp", string(addr))
				conn.WriteToUDP(data, netAddr)
			}
		case input := <-inputChan:
			clients_info.setAddress(input.addrStr)
			if !clients_info.hasState(string(input.addrStr)) {
				clients_info.setState(string(input.addrStr), &clientState{
					pos: position{x: 500, y: 500},
				})
			}

			clientInputs[input.addrStr] = input
		}
	}
}
