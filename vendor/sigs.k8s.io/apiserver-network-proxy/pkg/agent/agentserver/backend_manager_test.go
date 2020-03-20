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
	"reflect"
	"testing"

	"sigs.k8s.io/apiserver-network-proxy/proto/agent"
)

type fakeAgentService_ConnectServer struct {
	agent.AgentService_ConnectServer
}

func TestAddRemoveBackends(t *testing.T) {
	conn1 := new(fakeAgentService_ConnectServer)
	conn12 := new(fakeAgentService_ConnectServer)
	conn2 := new(fakeAgentService_ConnectServer)
	conn22 := new(fakeAgentService_ConnectServer)
	conn3 := new(fakeAgentService_ConnectServer)

	p := NewDefaultBackendManager()

	p.AddBackend("agent1", conn1)
	p.RemoveBackend("agent1", conn1)
	expectedBackends := make(map[string][]agent.AgentService_ConnectServer)
	expectedAgentIDs := []string{}
	if e, a := expectedBackends, p.backends; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := expectedAgentIDs, p.agentIDs; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}

	p = NewDefaultBackendManager()
	p.AddBackend("agent1", conn1)
	p.AddBackend("agent1", conn12)
	// Adding the same connection again should be a no-op.
	p.AddBackend("agent1", conn12)
	p.AddBackend("agent2", conn2)
	p.AddBackend("agent2", conn22)
	p.AddBackend("agent3", conn3)
	p.RemoveBackend("agent2", conn22)
	p.RemoveBackend("agent2", conn2)
	p.RemoveBackend("agent1", conn1)
	// This is invalid. agent1 doesn't have conn3. This should be a no-op.
	p.RemoveBackend("agent1", conn3)
	expectedBackends = map[string][]agent.AgentService_ConnectServer{
		"agent1": []agent.AgentService_ConnectServer{conn12},
		"agent3": []agent.AgentService_ConnectServer{conn3},
	}
	expectedAgentIDs = []string{"agent1", "agent3"}
	if e, a := expectedBackends, p.backends; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := expectedAgentIDs, p.agentIDs; !reflect.DeepEqual(e, a) {
		t.Errorf("expected %v, got %v", e, a)
	}
}
