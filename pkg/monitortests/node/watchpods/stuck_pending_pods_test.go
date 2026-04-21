package watchpods

import (
	"fmt"
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func podLocator(namespace, name string) monitorapi.Locator {
	return monitorapi.NewLocator().PodFromNames(namespace, name, "")
}

func TestStuckPendingPodsJunit(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name       string
		intervals  monitorapi.Intervals
		wantFail   bool
		wantCount  int
		wantSubstr string
	}{
		{
			name:      "no intervals produces a pass",
			intervals: monitorapi.Intervals{},
			wantFail:  false,
		},
		{
			name: "pod pending that completed is not a failure",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("openshift-etcd", "etcd-master-0")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("pod has been pending longer than a minute")).
					Build(now.Add(-10*time.Minute), now.Add(-5*time.Minute)),
			},
			wantFail: false,
		},
		{
			name: "single stuck pending pod is a failure",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("my-namespace", "stuck-pod")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("never completed")).
					Build(now.Add(-30*time.Minute), now),
			},
			wantFail:   true,
			wantCount:  1,
			wantSubstr: "ns/my-namespace pod/stuck-pod",
		},
		{
			name: "multiple stuck pending pods are reported",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("ns-a", "pod-a")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("never completed")).
					Build(now.Add(-20*time.Minute), now),
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("ns-b", "pod-b")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("never completed")).
					Build(now.Add(-15*time.Minute), now),
			},
			wantFail:  true,
			wantCount: 2,
		},
		{
			name: "different source is ignored",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
					Locator(podLocator("ns", "pod")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("never completed")).
					Build(now.Add(-10*time.Minute), now),
			},
			wantFail: false,
		},
		{
			name: "different reason is ignored",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("ns", "pod")).
					Message(monitorapi.NewMessage().Reason("NodeUpdate").HumanMessage("never completed")).
					Build(now.Add(-10*time.Minute), now),
			},
			wantFail: false,
		},
		{
			name: "stuck pod mixed with normal intervals",
			intervals: monitorapi.Intervals{
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Info).
					Locator(podLocator("ns-ok", "healthy-pod")).
					Message(monitorapi.NewMessage().Reason("Created")).
					Build(now.Add(-30*time.Minute), now.Add(-29*time.Minute)),
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("ns-stuck", "image-pull-stuck")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("never completed")).
					Build(now.Add(-25*time.Minute), now),
				monitorapi.NewInterval(monitorapi.SourcePodState, monitorapi.Warning).
					Locator(podLocator("ns-ok", "slow-pod")).
					Message(monitorapi.NewMessage().Reason("PodWasPending").HumanMessage("pod has been pending longer than a minute")).
					Build(now.Add(-5*time.Minute), now.Add(-3*time.Minute)),
			},
			wantFail:   true,
			wantCount:  1,
			wantSubstr: "ns/ns-stuck pod/image-pull-stuck",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			junits := stuckPendingPodsJunit(tt.intervals)
			require.NotEmpty(t, junits, "should always produce at least one JUnit test case")

			if tt.wantFail {
				// When failing, we expect a failure entry + a pass entry (flake pattern)
				var failCase *junitapi.JUnitTestCase
				for _, tc := range junits {
					if tc.FailureOutput != nil {
						failCase = tc
						break
					}
				}
				require.NotNil(t, failCase, "expected a failing test case")
				assert.Contains(t, failCase.FailureOutput.Output, "stuck in Pending state")
				if tt.wantSubstr != "" {
					assert.Contains(t, failCase.FailureOutput.Output, tt.wantSubstr)
				}
				if tt.wantCount > 0 {
					assert.Contains(t, failCase.FailureOutput.Output,
						fmt.Sprintf("%d pod(s)", tt.wantCount))
				}

				// Verify the flake pattern: both a failure and a pass with the same test name
				var hasPass bool
				for _, tc := range junits {
					if tc.FailureOutput == nil && tc.Name == failCase.Name {
						hasPass = true
					}
				}
				assert.True(t, hasPass, "expected a matching pass entry for flake pattern")
			} else {
				for _, tc := range junits {
					assert.Nil(t, tc.FailureOutput, "expected no failure output for passing test")
				}
			}
		})
	}
}
