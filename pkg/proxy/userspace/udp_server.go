package userspace

import (
	"fmt"
	"net"
)

// udpEchoServer is a simple echo server in UDP, intended for testing the proxy.
type udpEchoServer struct {
	net.PacketConn
}

func (r *udpEchoServer) Loop() {
	var buffer [4096]byte
	for {
		n, cliAddr, err := r.ReadFrom(buffer[0:])
		if err != nil {
			fmt.Printf("ReadFrom failed: %v\n", err)
			continue
		}
		r.WriteTo(buffer[0:n], cliAddr)
	}
}

func newUDPEchoServer() (*udpEchoServer, error) {
	packetconn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		return nil, err
	}
	return &udpEchoServer{packetconn}, nil
}

/*
func main() {
	r,_ := newUDPEchoServer()
	r.Loop()
}
*/
