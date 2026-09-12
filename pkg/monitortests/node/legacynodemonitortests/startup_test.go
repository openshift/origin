package legacynodemonitortests

import (
	"testing"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/stretchr/testify/assert"
)

func TestFilterExternalTopologyStartupNodeIntervals(t *testing.T) {
	beginning := time.Date(2026, 9, 11, 12, 0, 0, 0, time.UTC)
	externalCluster := platformidentification.ClusterData{JobType: platformidentification.JobType{Topology: "external"}}
	standaloneCluster := platformidentification.ClusterData{JobType: platformidentification.JobType{Topology: "ha"}}

	buildInterval := func(source monitorapi.IntervalSource, reason monitorapi.IntervalReason, namespace, humanMessage string, from, to time.Duration) monitorapi.Interval {
		return monitorapi.NewInterval(source, monitorapi.Info).
			Locator(monitorapi.Locator{
				Type: monitorapi.LocatorTypePod,
				Keys: map[monitorapi.LocatorKey]string{
					monitorapi.LocatorNamespaceKey: namespace,
					monitorapi.LocatorNodeKey:      "node-0",
				},
			}).
			Message(monitorapi.NewMessage().Reason(reason).HumanMessage(humanMessage)).
			Build(beginning.Add(from), beginning.Add(to))
	}

	tests := []struct {
		name     string
		cluster  platformidentification.ClusterData
		start    time.Time
		interval monitorapi.Interval
		expected monitorapi.Intervals
	}{
		{
			name:     "force delete during startup is ignored",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{},
		},
		{
			name:     "anonymous kubelet access during startup is ignored",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourceKubeletLog, monitorapi.FailedToAuthenticateWithOpenShiftUser, "", "system:anonymous cannot get CSINodes", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{},
		},
		{
			name:     "image pull QPS error during startup is ignored",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourceKubeletLog, monitorapi.ContainerErrImagePull, "openshift-dns", "ErrImagePull: pull QPS exceeded", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{},
		},
		{
			name:     "matching interval crossing startup window is clipped",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourceKubeletLog, monitorapi.FailedToAuthenticateWithOpenShiftUser, "", "system:anonymous cannot get CSINodes", time.Minute, 7*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourceKubeletLog, monitorapi.FailedToAuthenticateWithOpenShiftUser, "", "system:anonymous cannot get CSINodes", 5*time.Minute, 7*time.Minute)},
		},
		{
			name:     "matching interval after startup window is unchanged",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", 6*time.Minute, 7*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", 6*time.Minute, 7*time.Minute)},
		},
		{
			name:     "force delete in another namespace is unchanged",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "openshift-dns", "", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "openshift-dns", "", time.Minute, 2*time.Minute)},
		},
		{
			name:     "other image pull error is unchanged",
			cluster:  externalCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourceKubeletLog, monitorapi.ContainerErrImagePull, "openshift-dns", "ErrImagePull: manifest unknown", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourceKubeletLog, monitorapi.ContainerErrImagePull, "openshift-dns", "ErrImagePull: manifest unknown", time.Minute, 2*time.Minute)},
		},
		{
			name:     "non-external topology is unchanged",
			cluster:  standaloneCluster,
			start:    beginning,
			interval: buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", time.Minute, 2*time.Minute)},
		},
		{
			name:     "zero collection start is unchanged",
			cluster:  externalCluster,
			start:    time.Time{},
			interval: buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", time.Minute, 2*time.Minute),
			expected: monitorapi.Intervals{buildInterval(monitorapi.SourcePodMonitor, monitorapi.PodReasonForceDelete, "kube-system", "", time.Minute, 2*time.Minute)},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := filterExternalTopologyStartupNodeIntervals(monitorapi.Intervals{tt.interval}, tt.cluster, tt.start)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
