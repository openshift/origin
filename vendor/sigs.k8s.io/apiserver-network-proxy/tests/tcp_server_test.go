package tests

import (
	"net"
	"testing"
	"time"

	"google.golang.org/grpc"
	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
)

func echo(conn net.Conn) {
	var data [256]byte

	for {
		n, err := conn.Read(data[:])
		if err != nil {
			klog.Info(err)
			return
		}

		_, err = conn.Write(data[:n])
		if err != nil {
			klog.Info(err)
			return
		}
	}
}

func TestEchoServer(t *testing.T) {
	ln, err := net.Listen("tcp", "")
	if err != nil {
		t.Error(err)
	}

	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				klog.Info(err)
				break
			}
			go echo(conn)
		}
	}()

	stopCh := make(chan struct{})
	defer close(stopCh)

	proxy, cleanup, err := runGRPCProxyServer()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	runAgent(proxy.agent, stopCh)

	// Wait for agent to register on proxy server
	time.Sleep(time.Second)

	// run test client
	tunnel, err := client.CreateGrpcTunnel(proxy.front, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	conn, err := tunnel.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Error(err)
	}

	msg := "1234567890123456789012345"
	n, err := conn.Write([]byte(msg))
	if err != nil {
		t.Error(err)
	}
	if n != len(msg) {
		t.Errorf("expect write %d; got %d", len(msg), n)
	}

	var data [10]byte

	n, err = conn.Read(data[:])
	if err != nil {
		t.Error(err)
	}
	if string(data[:n]) != msg[:10] {
		t.Errorf("expect %s; got %s", msg[:10], string(data[:n]))
	}

	n, err = conn.Read(data[:])
	if err != nil {
		t.Error(err)
	}
	if string(data[:n]) != msg[10:20] {
		t.Errorf("expect %s; got %s", msg[10:20], string(data[:n]))
	}

	msg2 := "1234567"
	n, err = conn.Write([]byte(msg2))
	if err != nil {
		t.Error(err)
	}
	if n != len(msg2) {
		t.Errorf("expect write %d; got %d", len(msg2), n)
	}

	n, err = conn.Read(data[:])
	if err != nil {
		t.Error(err)
	}
	if string(data[:n]) != msg[20:] {
		t.Errorf("expect %s; got %s", msg[20:], string(data[:n]))
	}

	n, err = conn.Read(data[:])
	if err != nil {
		t.Error(err)
	}
	if string(data[:n]) != msg2 {
		t.Errorf("expect %s; got %s", msg, string(data[:n]))
	}

	if err := conn.Close(); err != nil {
		t.Error(err)
	}
}
