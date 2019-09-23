package httpmaster

import (
	"testing"

	"github.com/mesos/mesos-go/api/v1/lib/master"
)

func TestClassifyResponse(t *testing.T) {
	_, err := classifyResponse(nil)
	if err == nil {
		t.Fatal("expected error instead of nil")
	}
	for _, v := range master.Call_Type_value {
		ct := master.Call_Type(v)
		_, err = classifyResponse(&master.Call{Type: ct})
		if ct == master.Call_UNKNOWN {
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
