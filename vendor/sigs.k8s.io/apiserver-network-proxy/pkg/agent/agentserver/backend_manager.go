/*
Copyright 2020 The Kubernetes Authors.

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

package agentserver

import (
	"math/rand"
	"sync"
	"time"

	"k8s.io/klog"
	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

// BackendManager is an interface to manage backend connections, i.e.,
// connection to the proxy agents.
type BackendManager interface {
	// Backend returns a single backend.
	Backend() (agent.AgentService_ConnectServer, error)
	// AddBackend adds a backend.
	AddBackend(agentID string, conn agent.AgentService_ConnectServer)
	// RemoveBackend removes a backend.
	RemoveBackend(agentID string, conn agent.AgentService_ConnectServer)
}

var _ BackendManager = &DefaultBackendManager{}

// DefaultBackendManager is the default backend manager.
type DefaultBackendManager struct {
	mu sync.RWMutex //protects the following
	// A map between agentID and its grpc connections.
	// For a given agent, ProxyServer prefers backends[agentID][0] to send
	// traffic, because backends[agentID][1:] are more likely to be closed
	// by the agent to deduplicate connections to the same server.
	backends map[string][]agent.AgentService_ConnectServer
	// agentID is tracked in this slice to enable randomly picking an
	// agentID in the Backend() method. There is no reliable way to
	// randomly pick a key from a map (in this case, the backends) in
	// Golang.
	agentIDs []string
	random   *rand.Rand
}

// NewDefaultBackendManager returns a DefaultBackendManager.
func NewDefaultBackendManager() *DefaultBackendManager {
	return &DefaultBackendManager{
		backends: make(map[string][]agent.AgentService_ConnectServer),
		random:   rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// AddBackend adds a backend.
func (s *DefaultBackendManager) AddBackend(agentID string, conn agent.AgentService_ConnectServer) {
	klog.Infof("register Backend %v for agentID %s", conn, agentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.backends[agentID]
	if ok {
		for _, v := range s.backends[agentID] {
			if v == conn {
				klog.Warningf("this should not happen. Adding existing connection %v for agentID %s", conn, agentID)
				return
			}
		}
		s.backends[agentID] = append(s.backends[agentID], conn)
		return
	}
	s.backends[agentID] = []agent.AgentService_ConnectServer{conn}
	s.agentIDs = append(s.agentIDs, agentID)
}

// RemoveBackend removes a backend.
func (s *DefaultBackendManager) RemoveBackend(agentID string, conn agent.AgentService_ConnectServer) {
	klog.Infof("remove Backend %v for agentID %s", conn, agentID)
	s.mu.Lock()
	defer s.mu.Unlock()
	backends, ok := s.backends[agentID]
	if !ok {
		klog.Warningf("can't find agentID %s in the backends", agentID)
		return
	}
	var found bool
	for i, c := range backends {
		if c == conn {
			s.backends[agentID] = append(s.backends[agentID][:i], s.backends[agentID][i+1:]...)
			if i == 0 && len(s.backends) != 0 {
				klog.Warningf("this should not happen. Removed connection %v that is not the first connection, remaining connections are %v", conn, s.backends[agentID])
			}
			found = true
		}
	}
	if len(s.backends[agentID]) == 0 {
		delete(s.backends, agentID)
		for i := range s.agentIDs {
			if s.agentIDs[i] == agentID {
				s.agentIDs[i] = s.agentIDs[len(s.agentIDs)-1]
				s.agentIDs = s.agentIDs[:len(s.agentIDs)-1]
				break
			}
		}
	}
	if !found {
		klog.Errorf("can't find conn %v for agentID %s in the backends", conn, agentID)
	}
}

// ErrNotFound indicates that no backend can be found.
type ErrNotFound struct{}

// Error returns the error message.
func (e *ErrNotFound) Error() string {
	return "No backend available"
}

// Backend returns a random backend.
func (s *DefaultBackendManager) Backend() (agent.AgentService_ConnectServer, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.backends) == 0 {
		return nil, &ErrNotFound{}
	}
	agentID := s.agentIDs[s.random.Intn(len(s.agentIDs))]
	// always return the first connection to an agent, because the agent
	// will close later connections if there are multiple.
	return s.backends[agentID][0], nil
}
