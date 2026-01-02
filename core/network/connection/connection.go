package conn

import (
	stypes "CircleWar/core/types"
	"net"
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

func (cc *ClientConn) Send(msg stypes.GameMessage) {

}

func (cc *ClientConn) Recieve() stypes.GameMessage {
	return &stypes.PlayerInput{}
}

type ServerConn struct {
	conn *net.UDPConn
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
	return &ServerConn{conn}, nil
}

func (sc *ServerConn) Close() error {
	return sc.conn.Close()
}

func (cc *ServerConn) Send(msg stypes.GameMessage) {
	
}

func (cc *ServerConn) Recieve() stypes.GameMessage {
	return &stypes.PlayerInput{}
}
