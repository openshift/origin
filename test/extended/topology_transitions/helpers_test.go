package topology_transitions

import "testing"

func TestDetectChain(t *testing.T) {
	t.Cleanup(func() { exercisedTransitionName = "" })

	if err := detectChain("a"); err != nil {
		t.Fatalf("first call for a new transition should succeed, got: %v", err)
	}
	if err := detectChain("a"); err != nil {
		t.Fatalf("repeat calls for the same transition should succeed, got: %v", err)
	}
	if err := detectChain("b"); err == nil {
		t.Fatal("expected an error when a different transition tries to run in the same invocation, got nil")
	}
}
