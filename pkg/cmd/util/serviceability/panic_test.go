package serviceability

import (
	"testing"
	"time"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

func TestPanicDelayingDeath(t *testing.T) {
	BehaviorOnPanic(`["crash-after-delay:10s"]`)

	utilruntime.ReallyCrash = false
	go func() {
		defer utilruntime.HandleCrash()
		panic("not dead yet!")
	}()

	select {
	case <-time.After(5 * time.Second):
		t.Log("beat death!")
	}
}
