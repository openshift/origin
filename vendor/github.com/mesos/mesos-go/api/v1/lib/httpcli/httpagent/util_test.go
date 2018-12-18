package httpagent

import (
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/agent"
)

func TestClassifyResponse(t *testing.T) {
	_, err := classifyResponse(nil)
	if err == nil {
		t.Fatal("expected error instead of nil")
	}
	for _, v := range agent.Call_Type_value {
		ct := agent.Call_Type(v)
		_, err = classifyResponse(&agent.Call{Type: ct})
		if ct == agent.Call_UNKNOWN {
			if err == nil {
				t.Fatal("expected error instead of nil")
			}
		} else {
			if err != nil {
				t.Fatalf("unexpected error %v", err)
			}
		}
	}
}
