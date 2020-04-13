package tests

import (
	"io"
	"io/ioutil"
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
)

type tcpLB struct {
	t        *testing.T
	mu       sync.RWMutex
	backends []string
}

func copy(wc io.WriteCloser, r io.Reader) {
	defer wc.Close()
	io.Copy(wc, r)
}

func (lb *tcpLB) handleConnection(in net.Conn, backend string) {
	out, err := net.Dial("tcp", backend)
	if err != nil {
		lb.t.Log(err)
		return
	}
	go copy(out, in)
	go copy(in, out)
}

func (lb *tcpLB) serve(stopCh chan struct{}) {
	ln, err := net.Listen("tcp", ":8000")
	if err != nil {
		log.Fatalf("failed to bind: %s", err)
	}
	for {
		select {
		case <-stopCh:
			return
		default:
		}
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("failed to accept: %s", err)
			continue
		}
		// go lb.handleConnection(conn, lb.randomBackend())
		back := lb.randomBackend()
		go lb.handleConnection(conn, back)
	}
}

func (lb *tcpLB) addBackend(backend string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.backends = append(lb.backends, backend)
}

func (lb *tcpLB) removeBackend(backend string) {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	for i := range lb.backends {
		if lb.backends[i] == backend {
			lb.backends = append(lb.backends[:i], lb.backends[i+1:]...)
			return
		}
	}
}

func (lb *tcpLB) randomBackend() string {
	lb.mu.RLock()
	defer lb.mu.RUnlock()
	i := rand.Intn(len(lb.backends))
	return lb.backends[i]
}

const haServerCount = 3

func setupHAProxyServer(t *testing.T) ([]proxy, []func()) {
	proxy1, _, cleanup1, err := runGRPCProxyServerWithServerCount(haServerCount)
	if err != nil {
		t.Fatal(err)
	}

	proxy2, _, cleanup2, err := runGRPCProxyServerWithServerCount(haServerCount)
	if err != nil {
		t.Fatal(err)
	}

	proxy3, _, cleanup3, err := runGRPCProxyServerWithServerCount(haServerCount)
	if err != nil {
		t.Fatal(err)
	}
	return []proxy{proxy1, proxy2, proxy3}, []func(){cleanup1, cleanup2, cleanup3}
}

func TestBasicHAProxyServer_GRPC(t *testing.T) {
	server := httptest.NewServer(newEchoServer("hello"))
	defer server.Close()

	stopCh := make(chan struct{})
	defer close(stopCh)

	proxy, cleanups := setupHAProxyServer(t)

	lb := tcpLB{
		backends: []string{
			proxy[0].agent,
			proxy[1].agent,
			proxy[2].agent,
		},
		t: t,
	}
	go lb.serve(stopCh)

	clientset := runAgent(":8000", stopCh)

	var ready bool
	var hc, cc int
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		hc, cc = clientset.HealthyClientsCount(), clientset.ClientsCount()
		t.Logf("got %d clients, %d of them are healthy", hc, cc)
		if hc == 3 && cc == 3 {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("expected to get 3 clients, got %d clients, %d healthy clients", hc, cc)
	}

	// run test client
	testProxyServer(t, proxy[0].front, server.URL)
	testProxyServer(t, proxy[1].front, server.URL)
	testProxyServer(t, proxy[2].front, server.URL)

	t.Logf("basic HA proxy server test passed")

	// interrupt the HA server
	lb.removeBackend(proxy[0].agent)
	cleanups[0]()

	proxy4, _, cleanup4, err := runGRPCProxyServerWithServerCount(haServerCount)
	if err != nil {
		t.Fatal(err)
	}
	lb.addBackend(proxy4.agent)
	defer func() {
		cleanups[1]()
		cleanups[2]()
		cleanup4()
	}()
	// give the agent some time to detect the disconnection
	time.Sleep(1 * time.Second)

	ready = false
	for i := 0; i < 3; i++ {
		time.Sleep(1 * time.Second)
		hc, cc = clientset.HealthyClientsCount(), clientset.ClientsCount()
		t.Logf("got %d clients, %d of them are healthy", hc, cc)
		if hc == 3 && (cc == 3 || cc == 4) {
			ready = true
			break
		}
	}
	if !ready {
		t.Fatalf("expected to get 3 clients, got %d clients, %d healthy clients", hc, cc)
	}
	// run test client
	testProxyServer(t, proxy[1].front, server.URL)
	testProxyServer(t, proxy[2].front, server.URL)
	testProxyServer(t, proxy4.front, server.URL)
}

func testProxyServer(t *testing.T, front string, target string) {
	tunnel, err := client.CreateGrpcTunnel(front, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	c := &http.Client{
		Transport: &http.Transport{
			Dial: tunnel.Dial,
		},
		Timeout: 1 * time.Second,
	}

	r, err := c.Get(target)
	if err != nil {
		t.Fatal(err)
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		t.Error(err)
	}

	if string(data) != "hello" {
		t.Errorf("expect %v; got %v", "hello", string(data))
	}
}
