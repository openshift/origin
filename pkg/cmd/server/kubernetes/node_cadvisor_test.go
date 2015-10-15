package kubernetes

import "testing"

func TestDefaultCadvisorInterface(t *testing.T) {
	// Make sure no one changes the default cadvisor interface
	if defaultCadvisorInterface != nil {
		t.Errorf("Expected nil default for cadvisor interface")
	}
}
