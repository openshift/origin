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

package agentclient

import (
	"fmt"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
)

// ClientSet consists of clients connected to each instance of an HA proxy server.
type ClientSet struct {
	mu      sync.Mutex              //protects the clients.
	clients map[string]*AgentClient // map between serverID and the client
	// connects to this server.

	agentID     string // ID of this agent
	address     string // proxy server address. Assuming HA proxy server
	serverCount int    // number of proxy server instances, should be 1
	// unless it is an HA server. Initialized when the ClientSet creates
	// the first client.
	syncInterval time.Duration // The interval by which the agent
	// periodically checks that it has connections to all instances of the
	// proxy server.
	probeInterval time.Duration // The interval by which the agent
	// periodically checks if its connections to the proxy server is ready.
	reconnectInterval time.Duration // The interval by which the agent
	// tries to reconnect.
	dialOption grpc.DialOption
	// file path contains service account token
	serviceAccountTokenPath string
}

func (cs *ClientSet) ClientsCount() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return len(cs.clients)
}
func (cs *ClientSet) HealthyClientsCount() int {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	var count int
	for _, c := range cs.clients {
		if c.stream.conn.GetState() == connectivity.Ready {
			count++
		}
	}
	return count

}

func (cs *ClientSet) hasIDLocked(serverID string) bool {
	_, ok := cs.clients[serverID]
	return ok
}

func (cs *ClientSet) HasID(serverID string) bool {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.hasIDLocked(serverID)
}

func (cs *ClientSet) addClientLocked(serverID string, c *AgentClient) error {
	if cs.hasIDLocked(serverID) {
		return fmt.Errorf("client for proxy server %s already exists", serverID)
	}
	cs.clients[serverID] = c
	return nil

}

func (cs *ClientSet) AddClient(serverID string, c *AgentClient) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.addClientLocked(serverID, c)
}

func (cs *ClientSet) RemoveClient(serverID string) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	delete(cs.clients, serverID)
}

type ClientSetConfig struct {
	Address                 string
	AgentID                 string
	SyncInterval            time.Duration
	ProbeInterval           time.Duration
	ReconnectInterval       time.Duration
	DialOption              grpc.DialOption
	ServiceAccountTokenPath string
}

func (cc *ClientSetConfig) NewAgentClientSet() *ClientSet {
	return &ClientSet{
		clients:                 make(map[string]*AgentClient),
		agentID:                 cc.AgentID,
		address:                 cc.Address,
		syncInterval:            cc.SyncInterval,
		probeInterval:           cc.ProbeInterval,
		reconnectInterval:       cc.ReconnectInterval,
		dialOption:              cc.DialOption,
		serviceAccountTokenPath: cc.ServiceAccountTokenPath,
	}

}

func (cs *ClientSet) newAgentClient() (*AgentClient, error) {
	return newAgentClient(cs.address, cs.agentID, cs, cs.dialOption)
}

func (cs *ClientSet) resetBackoff() *wait.Backoff {
	return &wait.Backoff{
		Steps:    3,
		Jitter:   0.1,
		Factor:   1.5,
		Duration: cs.syncInterval,
		Cap:      60 * time.Second,
	}
}

// sync makes sure that #clients >= #proxy servers
func (cs *ClientSet) sync() {
	backoff := cs.resetBackoff()
	var duration time.Duration
	for {
		if err := cs.syncOnce(); err != nil {
			klog.Error(err)
			duration = backoff.Step()
		} else {
			backoff = cs.resetBackoff()
			duration = wait.Jitter(backoff.Duration, backoff.Jitter)
		}
		time.Sleep(duration)
	}
}

func (cs *ClientSet) syncOnce() error {
	if cs.serverCount != 0 && cs.ClientsCount() >= cs.serverCount {
		return nil
	}
	c, err := cs.newAgentClient()
	if err != nil {
		return err
	}
	cs.serverCount = c.stream.serverCount
	if err := cs.AddClient(c.stream.serverID, c); err != nil {
		klog.Infof("closing connection: %v", err)
		c.Close()
		return nil
	}
	klog.Infof("sync added client connecting to proxy server %s", c.stream.serverID)
	go c.Serve()
	return nil
}

func (cs *ClientSet) Serve() {
	go cs.sync()
}
