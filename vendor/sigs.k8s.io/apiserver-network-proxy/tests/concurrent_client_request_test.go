package tests

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"google.golang.org/grpc"
	"sigs.k8s.io/apiserver-network-proxy/konnectivity-client/pkg/client"
	"sigs.k8s.io/apiserver-network-proxy/pkg/agent/agentserver"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

type simpleServer struct {
	receivedSecondReq chan struct{}
}

// ServeHTTP blocks the response to the request whose body is "1" until a
// request whose body is "2" is handled.
func (s *simpleServer) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	bytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		w.Write([]byte(err.Error()))
	}
	if string(bytes) == "2" {
		close(s.receivedSecondReq)
		w.Write([]byte("2"))
	}
	if string(bytes) == "1" {
		<-s.receivedSecondReq
		w.Write([]byte("1"))
	}
}

// TODO: test http-connect as well.
func getTestClient(front string, t *testing.T) *http.Client {
	tunnel, err := client.CreateGrpcTunnel(front, grpc.WithInsecure())
	if err != nil {
		t.Fatal(err)
	}

	return &http.Client{
		Transport: &http.Transport{
			Dial: tunnel.Dial,
		},
		Timeout: 2 * time.Second,
	}
}

// singleTimeManager makes sure that a backend only serves one request.
type singleTimeManager struct {
	mu       sync.Mutex
	backends map[string]agent.AgentService_ConnectServer
	used     map[string]struct{}
}

func (s *singleTimeManager) AddBackend(agentID string, conn agent.AgentService_ConnectServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backends[agentID] = conn
}

func (s *singleTimeManager) RemoveBackend(agentID string, conn agent.AgentService_ConnectServer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.backends[agentID]
	if !ok {
		panic(fmt.Errorf("no backends found for %s", agentID))
	}
	if v != conn {
		panic(fmt.Errorf("recorded connection %v does not match conn %v", v, conn))
	}
	delete(s.backends, agentID)
}

func (s *singleTimeManager) Backend() (agent.AgentService_ConnectServer, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.backends {
		if _, ok := s.used[k]; !ok {
			s.used[k] = struct{}{}
			return v, nil
		}
	}
	return nil, fmt.Errorf("cannot find backend to a new agent")
}

func newSingleTimeGetter(m *agentserver.DefaultBackendManager) *singleTimeManager {
	return &singleTimeManager{
		used:     make(map[string]struct{}),
		backends: make(map[string]agent.AgentService_ConnectServer),
	}
}

var _ agentserver.BackendManager = &singleTimeManager{}

func TestConcurrentClientRequest(t *testing.T) {
	server := httptest.NewServer(&simpleServer{receivedSecondReq: make(chan struct{})})
	defer server.Close()

	proxy, ps, cleanup, err := runGRPCProxyServerWithServerCount(1)
	ps.BackendManager = newSingleTimeGetter(agentserver.NewDefaultBackendManager())

	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	stopCh := make(chan struct{})
	defer close(stopCh)
	// Run two agents
	runAgent(proxy.agent, stopCh)
	runAgent(proxy.agent, stopCh)

	client1 := getTestClient(proxy.front, t)
	client2 := getTestClient(proxy.front, t)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		r, err := client1.Post(server.URL, "text/plain", bytes.NewBufferString("1"))
		if err != nil {
			t.Error(err)
			return
		}
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		if string(data) != "1" {
			t.Errorf("expect %v; got %v", "1", string(data))
		}
	}()
	// give client1 some time to establish the connection.
	time.Sleep(1 * time.Second)
	go func() {
		defer wg.Done()
		r, err := client2.Post(server.URL, "text/plain", bytes.NewBufferString("2"))
		if err != nil {
			t.Error(err)
			return
		}
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			t.Error(err)
		}

		if string(data) != "2" {
			t.Errorf("expect %v; got %v", "2", string(data))
		}
	}()
	wg.Wait()
}
