package controlplane

import (
	"context"
	"strings"
	"time"

	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/test/extended/util/disruption"
)

// AvailableTest tests that the control plane remains is available
// before and after a cluster upgrade.
type AvailableTest struct {
}

func (AvailableTest) Name() string        { return "control-plane-available" }
func (AvailableTest) DisplayName() string { return "Kubernetes and OpenShift APIs remain available" }

// Setup does nothing
func (t *AvailableTest) Setup(f *framework.Framework) {
}

// Test runs a connectivity check to the core APIs.
func (t *AvailableTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	config, err := framework.LoadConfig()
	framework.ExpectNoError(err)

	ctx, cancel := context.WithCancel(context.Background())
	m := monitor.NewMonitorWithInterval(time.Second)
	err = monitor.StartAPIMonitoring(ctx, m, config, 15*time.Second)
	framework.ExpectNoError(err, "unable to monitor API")
	m.StartSampling(ctx)

	// wait to ensure API is still up after the test ends
	<-done
	time.Sleep(15 * time.Second)
	cancel()

	var duration time.Duration
	var describe []string
	for _, interval := range m.Events(time.Time{}, time.Time{}) {
		describe = append(describe, interval.String())
		i := interval.To.Sub(interval.From)
		if i < time.Second {
			i = time.Second
		}
		if interval.Condition.Level > monitor.Info {
			duration += i
		}
	}
	if duration > 120*time.Second {
		framework.Failf("API was unreachable during upgrade for at least %s:\n\n%s", duration.Truncate(time.Second), strings.Join(describe, "\n"))
	} else if duration > 0 {
		disruption.Flakef(f, "API was unreachable during upgrade for at least %s:\n\n%s", duration.Truncate(time.Second), strings.Join(describe, "\n"))
	}
}

// Teardown cleans up any remaining resources.
func (t *AvailableTest) Teardown(f *framework.Framework) {
}
