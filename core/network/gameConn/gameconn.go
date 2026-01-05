package gameConn

import (
	stypes "CircleWar/core/netmsg"
	"net"
	"sync"
)

type ClientConn struct {
	conn *net.UDPConn
}

func NewClientConn(servAddr *net.UDPAddr) (*ClientConn, error) {
	conn, err := net.DialUDP("udp", nil, servAddr)
	if err != nil {
		return &ClientConn{}, err
	}
	return &ClientConn{conn}, nil
}

func (cc *ClientConn) Close() error {
	return cc.conn.Close()
}

func (cc *ClientConn) Send(msg stypes.GameMessage) error {
	bytes, err := msg.Serialize()
	if err != nil {
		return err
	} else {
		_, err := cc.conn.Write(bytes)
		if err != nil {
			return err
		}
		return nil
	}
}

// calls ReadFromUDP aka blocks
func (cc *ClientConn) Recieve() (stypes.GameMessage, error) {
	buf := make([]byte, 1024)
	n, _, err := cc.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, err
	} else {
		gameMsg, err := stypes.Deserialize(buf, uint32(n))
		if err != nil {
			return nil, err
		}
		return gameMsg, nil
	}
}

type ServerConn struct {
	conn    *net.UDPConn
	clients []net.UDPAddr
	cmu     sync.Mutex // client lock
}

func NewServerConn(ip net.IP, port int) (*ServerConn, error) {
	addr := net.UDPAddr{
		Port: port,
		IP:   ip,
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return &ServerConn{}, err
	}
	return &ServerConn{conn, []net.UDPAddr{}, sync.Mutex{}}, nil
}

func (sc *ServerConn) AddListener(newListener net.UDPAddr) {
	sc.cmu.Lock()
	defer sc.cmu.Unlock()
	sc.clients = append(sc.clients, newListener)
}

func (sc *ServerConn) RemoveListener(listener net.UDPAddr) {
	sc.cmu.Lock()
	defer sc.cmu.Unlock()
}

func (sc *ServerConn) Close() error {
	return sc.conn.Close()
}

func (sc *ServerConn) Broadcast(msg stypes.GameMessage) error {
	sc.cmu.Lock()
	defer sc.cmu.Unlock()
	for _, addr := range sc.clients {
		err := sc.SendTo(msg, addr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cc *ServerConn) SendTo(msg stypes.GameMessage, addr net.UDPAddr) error {
	bytes, err := msg.Serialize()
	if err != nil {
		return err
	} else {
		_, err := cc.conn.WriteToUDP(bytes, &addr)
		if err != nil {
			return err
		}
	}
	return nil
}

func (cc *ServerConn) Recieve() (stypes.GameMessage, net.UDPAddr, error) {
	buf := make([]byte, 1024)
	n, addr, err := cc.conn.ReadFromUDP(buf)
	if err != nil {
		return nil, net.UDPAddr{}, err
	} else {
		gameMsg, err := stypes.Deserialize(buf, uint32(n))
		if err != nil {
			return nil, net.UDPAddr{}, err
		}
		return gameMsg, *addr, nil
	}
}
