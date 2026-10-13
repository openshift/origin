package pathologicaleventlibrary

import (
	"testing"
	"time"

	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/stretchr/testify/assert"
)

func TestVsphereConfigurationMatcherUsesLastOccurrence(t *testing.T) {
	testStart := time.Date(2026, 6, 28, 22, 30, 0, 0, time.UTC)
	testEnd := testStart.Add(30 * time.Minute)
	firstTimestamp := testStart.Add(-4 * time.Hour)
	lastTimestamp := testStart.Add(15 * time.Minute)

	testInterval := monitorapi.NewInterval(monitorapi.SourceE2ETest, monitorapi.Info).
		Locator(monitorapi.NewLocator().E2ETest("[sig-storage] snapshot options in clusterCSIDriver should restore configuration")).
		Message(monitorapi.NewMessage().HumanMessage("test interval")).
		Build(testStart, testEnd)

	tests := []struct {
		name    string
		locator monitorapi.Locator
		reason  monitorapi.IntervalReason
		message string
	}{
		{
			name:    "daemonset update",
			locator: monitorapi.NewLocator().DeploymentFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-operator"),
			reason:  "DaemonSetUpdated",
			message: "Updated DaemonSet.apps/vmware-vsphere-csi-driver-node because it changed",
		},
		{
			name:    "daemonset pod creation",
			locator: monitorapi.NewLocator().DaemonSetFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-node"),
			reason:  "SuccessfulCreate",
			message: "Created pod: vmware-vsphere-csi-driver-node-abcde",
		},
		{
			name:    "configuration secret update",
			locator: monitorapi.NewLocator().DeploymentFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-operator"),
			reason:  "SecretUpdated",
			message: "Updated Secret/vmware-vsphere-csi-driver-config",
		},
		{
			name:    "deployment scaling",
			locator: monitorapi.NewLocator().DeploymentFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-controller"),
			reason:  "ScalingReplicaSet",
			message: "Scaled down replica set vmware-vsphere-csi-driver-controller-abcde from 2 to 1",
		},
	}

	matcher := newVsphereConfigurationTestsRollOutTooOftenEventMatcher(monitorapi.Intervals{testInterval})
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
				Locator(test.locator).
				Message(monitorapi.NewMessage().
					Reason(test.reason).
					HumanMessage(test.message).
					WithAnnotation(monitorapi.AnnotationCount, "25").
					WithAnnotation("firstTimestamp", firstTimestamp.Format(time.RFC3339)).
					WithAnnotation("lastTimestamp", lastTimestamp.Format(time.RFC3339))).
				Build(lastTimestamp, lastTimestamp.Add(time.Second))

			assert.True(t, matcher.Allows(event, v1.HighlyAvailableTopologyMode),
				"expected the rollout event's last occurrence to be attributed to the configuration test")
		})
	}

	// The daemonset rollout triggered by the test (or by the configuration restore at test
	// teardown) can continue past the end of the test itself; events from that tail must
	// still be attributed to the configuration test.
	rolloutTailEvent := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
		Locator(monitorapi.NewLocator().DaemonSetFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-node")).
		Message(monitorapi.NewMessage().
			Reason("SuccessfulCreate").
			HumanMessage("Created pod: vmware-vsphere-csi-driver-node-tlp4z").
			WithAnnotation(monitorapi.AnnotationCount, "43").
			WithAnnotation("firstTimestamp", firstTimestamp.Format(time.RFC3339)).
			WithAnnotation("lastTimestamp", lastTimestamp.Format(time.RFC3339))).
		Build(testEnd.Add(20*time.Minute), testEnd.Add(20*time.Minute+time.Second))
	assert.True(t, matcher.Allows(rolloutTailEvent, v1.HighlyAvailableTopologyMode),
		"expected an event from the post-test rollout tail to be attributed to the configuration test")

	outsideTestWindow := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
		Locator(monitorapi.NewLocator().DeploymentFromName("openshift-cluster-csi-drivers", "vmware-vsphere-csi-driver-controller")).
		Message(monitorapi.NewMessage().
			Reason("ScalingReplicaSet").
			HumanMessage("Scaled down replica set vmware-vsphere-csi-driver-controller-abcde from 2 to 1").
			WithAnnotation(monitorapi.AnnotationCount, "25").
			WithAnnotation("firstTimestamp", firstTimestamp.Format(time.RFC3339))).
		Build(testEnd.Add(45*time.Minute), testEnd.Add(45*time.Minute+time.Second))
	assert.False(t, matcher.Allows(outsideTestWindow, v1.HighlyAvailableTopologyMode),
		"expected an event outside the buffered test interval to remain pathological")

	unrelatedScalingEvent := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
		Locator(monitorapi.NewLocator().DeploymentFromName("openshift-cluster-csi-drivers", "unrelated-controller")).
		Message(monitorapi.NewMessage().
			Reason("ScalingReplicaSet").
			HumanMessage("some unrelated scaling operation").
			WithAnnotation(monitorapi.AnnotationCount, "25")).
		Build(lastTimestamp, lastTimestamp.Add(time.Second))
	assert.False(t, matcher.Allows(unrelatedScalingEvent, v1.HighlyAvailableTopologyMode),
		"expected an unrelated scaling event not to match the rollout allowance")
}

func Test_singleEventThresholdCheck_getNamespacedFailuresAndFlakes(t *testing.T) {
	namespace := "openshift-etcd-operator"
	samplePod := "etcd-operator-6f9b4d9d4f-4q9q8"

	testName := "[sig-cluster-lifecycle] pathological event should not see excessive Back-off restarting failed containers"
	backoffMatcher := NewSingleEventThresholdCheck(testName, AllowBackOffRestartingFailedContainer,
		DuplicateEventThreshold, BackoffRestartingFlakeThreshold)
	type fields struct {
		testName       string
		matcher        *SimplePathologicalEventMatcher
		failThreshold  int
		flakeThreshold int
	}
	type args struct {
		events monitorapi.Intervals
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		expectedKeyCount int
	}{
		{
			name: "Successful test yields no keys",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						5),
				},
			},
			expectedKeyCount: 0,
		},
		{
			name: "Failing test yields one key",
			fields: fields{
				testName:       testName,
				matcher:        backoffMatcher.matcher,
				failThreshold:  DuplicateEventThreshold,
				flakeThreshold: BackoffRestartingFlakeThreshold,
			},
			args: args{
				events: monitorapi.Intervals{
					BuildTestDupeKubeEvent(namespace, samplePod,
						"BackOff",
						"Back-off restarting failed container",
						21),
				},
			},
			expectedKeyCount: 1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &singleEventThresholdCheck{
				testName:       tt.fields.testName,
				matcher:        tt.fields.matcher,
				failThreshold:  tt.fields.failThreshold,
				flakeThreshold: tt.fields.flakeThreshold,
			}
			got := s.getNamespacedFailuresAndFlakes(tt.args.events)
			assert.Equal(t, tt.expectedKeyCount, len(got))
		})
	}
}
