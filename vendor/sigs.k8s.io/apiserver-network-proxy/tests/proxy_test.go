package tests

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent/agentclient"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent/agentserver"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
	clientproto "sigs.k8s.io/apiserver-network-proxy/konnectivity-client/proto/client"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

// test remote server
type testServer struct {
	echo   []byte
	chunks int
}

func newEchoServer(echo string) *testServer {
	return &testServer{
		echo:   []byte(echo),
		chunks: 1,
	}
}

func newSizedServer(length, chunks int) *testServer {
	return &testServer{
		echo:   make([]byte, length),
		chunks: chunks,
	}
}

func (s *testServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for i := 0; i < s.chunks; i++ {
		w.Write(s.echo)
	}
}

func TestBasicProxy_GRPC(t *testing.T) {
	server := httptest.NewServer(newEchoServer("hello"))
	defer server.Close()

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

	c := &http.Client{
		Transport: &http.Transport{
			Dial: tunnel.Dial,
		},
	}

	r, err := c.Get(server.URL)
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}

	if string(data) != "hello" {
		t.Errorf("expect %v; got %v", "hello", string(data))
	}
}

func TestProxy_LargeResponse(t *testing.T) {
	length := 1 << 20 // 1M
	chunks := 10
	server := httptest.NewServer(newSizedServer(length, chunks))
	defer server.Close()

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

	c := &http.Client{
		Transport: &http.Transport{
			Dial: tunnel.Dial,
		},
	}

	r, err := c.Get(server.URL)
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}

	if len(data) != length*chunks {
		t.Errorf("expect data length %d; got %d", length*chunks, len(data))
	}
}

func TestBasicProxy_HTTPCONN(t *testing.T) {
	server := httptest.NewServer(newEchoServer("hello"))
	defer server.Close()

	stopCh := make(chan struct{})
	defer close(stopCh)

	proxy, cleanup, err := runHTTPConnProxyServer()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	runAgent(proxy.agent, stopCh)

	// Wait for agent to register on proxy server
	time.Sleep(time.Second)

	conn, err := net.Dial("tcp", proxy.front)
	if err != nil {
		t.Error(err)
	}

	serverURL, _ := url.Parse(server.URL)

	// Send HTTP-Connect request
	_, err = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", serverURL.Host, "127.0.0.1")
	if err != nil {
		t.Error(err)
	}

	// Parse the HTTP response for Connect
	br := bufio.NewReader(conn)
	res, err := http.ReadResponse(br, nil)
	if err != nil {
		t.Errorf("reading HTTP response from CONNECT: %v", err)
	}
	if res.StatusCode != 200 {
		t.Errorf("expect 200; got %d", res.StatusCode)
	}
	if br.Buffered() > 0 {
		t.Error("unexpected extra buffer")
	}

	dialer := func(network, addr string) (net.Conn, error) {
		return conn, nil
	}

	c := &http.Client{
		Transport: &http.Transport{
			Dial: dialer,
		},
	}

	r, err := c.Get(server.URL)
	if err != nil {
		t.Error(err)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}

	if string(data) != "hello" {
		t.Errorf("expect %v; got %v", "hello", string(data))
	}

}

func localAddr(addr net.Addr) string {
	return addr.String()
}

type proxy struct {
	front string
	agent string
}

func runGRPCProxyServer() (proxy, func(), error) {
	p, _, cleanup, err := runGRPCProxyServerWithServerCount(1)
	return p, cleanup, err
}

func runGRPCProxyServerWithServerCount(serverCount int) (proxy, *agentserver.ProxyServer, func(), error) {
	var proxy proxy
	var err error
	var lis, lis2 net.Listener

	server := agentserver.NewProxyServer(uuid.New().String(), serverCount, &agentserver.AgentTokenAuthenticationOptions{})
	grpcServer := grpc.NewServer()
	agentServer := grpc.NewServer()
	cleanup := func() {
		if lis != nil {
			lis.Close()
		}
		if lis2 != nil {
			lis2.Close()
		}
		agentServer.Stop()
		grpcServer.Stop()
	}

	clientproto.RegisterProxyServiceServer(grpcServer, server)
	lis, err = net.Listen("tcp", "")
	if err != nil {
		return proxy, server, cleanup, err
	}
	go grpcServer.Serve(lis)
	proxy.front = localAddr(lis.Addr())

	agent.RegisterAgentServiceServer(agentServer, server)
	lis2, err = net.Listen("tcp", "")
	if err != nil {
		return proxy, server, cleanup, err
	}
	go func() {
		agentServer.Serve(lis2)
	}()
	proxy.agent = localAddr(lis2.Addr())

	return proxy, server, cleanup, nil
}

func runHTTPConnProxyServer() (proxy, func(), error) {
	var proxy proxy
	server := agentserver.NewProxyServer(uuid.New().String(), 0, &agentserver.AgentTokenAuthenticationOptions{})
	agentServer := grpc.NewServer()

	agent.RegisterAgentServiceServer(agentServer, server)
	lis, err := net.Listen("tcp", "")
	if err != nil {
		return proxy, func() {}, err
	}
	go func() {
		agentServer.Serve(lis)
	}()
	proxy.agent = localAddr(lis.Addr())

	// http-connect
	httpServer := &http.Server{
		Handler: &agentserver.Tunnel{
			Server: server,
		},
	}
	lis2, err := net.Listen("tcp", "")
	if err != nil {
		return proxy, func() {}, err
	}
	proxy.front = localAddr(lis2.Addr())

	go func() {
		err := httpServer.Serve(lis2)
		if err != nil {
			fmt.Println("http connect server error: ", err)
		}
	}()

	cleanup := func() {
		lis.Close()
		lis2.Close()
		httpServer.Shutdown(context.Background())
	}

	return proxy, cleanup, nil
}

func runAgent(addr string, stopCh <-chan struct{}) *agentclient.ClientSet {
	cc := agentclient.ClientSetConfig{
		Address:           addr,
		AgentID:           uuid.New().String(),
		SyncInterval:      1 * time.Millisecond,
		ProbeInterval:     10 * time.Millisecond,
		ReconnectInterval: 10 * time.Millisecond,
		DialOption:        grpc.WithInsecure(),
	}
	client := cc.NewAgentClientSet()
	go client.Serve()
	return client
}
