/*
Copyright 2019 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"bytes"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
)

func TestDial(t *testing.T) {
	s, ps := pipe()
	ts := testServer(ps, 100)

	defer ps.Close()
	defer s.Close()

	tunnel := &grpcTunnel{
		stream:      s,
		pendingDial: make(map[int64]chan<- dialResult),
		conns:       make(map[int64]*conn),
	}

	go tunnel.serve()
	go ts.serve()

	_, err := tunnel.Dial("tcp", "127.0.0.1:80")
	if err != nil {
		t.Fatalf("expect nil; got %v", err)
	}

	if ts.packets[0].Type != client.PacketType_DIAL_REQ {
		t.Fatalf("expect packet.type %v; got %v", client.PacketType_CLOSE_REQ, ts.packets[0].Type)
	}

	if ts.packets[0].GetDialRequest().Address != "127.0.0.1:80" {
		t.Errorf("expect packet.address %v; got %v", "127.0.0.1:80", ts.packets[0].GetDialRequest().Address)
	}
}

func TestData(t *testing.T) {
	s, ps := pipe()
	ts := testServer(ps, 100)

	defer ps.Close()
	defer s.Close()

	tunnel := &grpcTunnel{
		stream:      s,
		pendingDial: make(map[int64]chan<- dialResult),
		conns:       make(map[int64]*conn),
	}

	go tunnel.serve()
	go ts.serve()

	conn, err := tunnel.Dial("tcp", "127.0.0.1:80")
	if err != nil {
		t.Fatalf("expect nil; got %v", err)
	}

	datas := [][]byte{
		[]byte("hello"),
		[]byte(", "),
		[]byte("world."),
	}

	// send data using conn.Write
	for _, data := range datas {
		n, err := conn.Write(data)
		if err != nil {
			t.Error(err)
		}
		if n != len(data) {
			t.Errorf("expect n=%d len(%q); got %d", len(data), string(data), n)
		}
	}

	// test server should echo data back
	var buf [64]byte
	for _, data := range datas {
		n, err := conn.Read(buf[:])
		if err != nil {
			t.Error(err)
		}

		if string(buf[:n]) != "echo: "+string(data) {
			t.Errorf("expect 'echo: %s'; got %s", string(data), string(buf[:n]))
		}
	}

	// verify test server received data
	if ts.data.String() != "hello, world." {
		t.Errorf("expect server received %v; got %v", "hello, world.", ts.data.String())
	}
}

func TestClose(t *testing.T) {
	s, ps := pipe()
	ts := testServer(ps, 100)

	defer ps.Close()
	defer s.Close()

	tunnel := &grpcTunnel{
		stream:      s,
		pendingDial: make(map[int64]chan<- dialResult),
		conns:       make(map[int64]*conn),
	}

	go tunnel.serve()
	go ts.serve()

	conn, err := tunnel.Dial("tcp", "127.0.0.1:80")
	if err != nil {
		t.Fatalf("expect nil; got %v", err)
	}

	if err := conn.Close(); err != nil {
		t.Error(err)
	}

	if ts.packets[1].Type != client.PacketType_CLOSE_REQ {
		t.Fatalf("expect packet.type %v; got %v", client.PacketType_CLOSE_REQ, ts.packets[1].Type)
	}
	if ts.packets[1].GetCloseRequest().ConnectID != 100 {
		t.Errorf("expect connectID=100; got %d", ts.packets[1].GetCloseRequest().ConnectID)
	}
}

// TODO: Move to common testing library

// fakeStream implements ProxyService_ProxyClient
type fakeStream struct {
	grpc.ClientStream
	r <-chan *client.Packet
	w chan<- *client.Packet
}

var _ client.ProxyService_ProxyClient = &fakeStream{}

func pipe() (*fakeStream, *fakeStream) {
	r, w := make(chan *client.Packet, 2), make(chan *client.Packet, 2)
	s1, s2 := &fakeStream{}, &fakeStream{}
	s1.r, s1.w = r, w
	s2.r, s2.w = w, r
	return s1, s2
}

func (s *fakeStream) Send(packet *client.Packet) error {
	klog.Infof("[DEBUG] send packet %+v", packet)
	if packet == nil {
		return nil
	}
	s.w <- packet
	return nil
}

func (s *fakeStream) Recv() (*client.Packet, error) {
	select {
	case pkt := <-s.r:
		klog.Infof("[DEBUG] recv packet %+v", pkt)
		return pkt, nil
	case <-time.After(5 * time.Second):
		return nil, errors.New("timeout recv")
	}
}

func (s *fakeStream) Close() {
	close(s.w)
}

type proxyServer struct {
	t        testing.T
	s        client.ProxyService_ProxyClient
	handlers map[client.PacketType]handler
	connid   int64
	data     bytes.Buffer
	packets  []*client.Packet
}

func testServer(s client.ProxyService_ProxyClient, connid int64) *proxyServer {
	server := &proxyServer{
		s:        s,
		connid:   connid,
		handlers: make(map[client.PacketType]handler),
		packets:  []*client.Packet{},
	}

	server.handlers[client.PacketType_CLOSE_REQ] = server.handleClose
	server.handlers[client.PacketType_DIAL_REQ] = server.handleDial
	server.handlers[client.PacketType_DATA] = server.handleData

	return server
}

func (s *proxyServer) serve() {
	for {
		pkt, err := s.s.Recv()
		if err != nil {
			s.t.Error(err)
			return
		}

		if pkt == nil {
			return
		}

		if handler, ok := s.handlers[pkt.Type]; ok {
			if err := s.s.Send(handler(pkt)); err != nil {
				s.t.Error(err)
			}
		}
	}

}

func (s *proxyServer) handle(t client.PacketType, h handler) *proxyServer {
	s.handlers[t] = h
	return s
}

type handler func(pkt *client.Packet) *client.Packet

func (s *proxyServer) handleDial(pkt *client.Packet) *client.Packet {
	s.packets = append(s.packets, pkt)
	return &client.Packet{
		Type: client.PacketType_DIAL_RSP,
		Payload: &client.Packet_DialResponse{
			DialResponse: &client.DialResponse{
				Random:    pkt.GetDialRequest().Random,
				ConnectID: s.connid,
			},
		},
	}
}

func (s *proxyServer) handleClose(pkt *client.Packet) *client.Packet {
	s.packets = append(s.packets, pkt)
	return &client.Packet{
		Type: client.PacketType_CLOSE_RSP,
		Payload: &client.Packet_CloseResponse{
			CloseResponse: &client.CloseResponse{
				ConnectID: pkt.GetCloseRequest().ConnectID,
			},
		},
	}
}

func (s *proxyServer) handleData(pkt *client.Packet) *client.Packet {
	s.packets = append(s.packets, pkt)
	s.data.Write(pkt.GetData().Data)

	return &client.Packet{
		Type: client.PacketType_DATA,
		Payload: &client.Packet_Data{
			Data: &client.Data{
				ConnectID: pkt.GetData().ConnectID,
				Data:      append([]byte("echo: "), pkt.GetData().Data...),
			},
		},
	}
}
