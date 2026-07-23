package node

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	ote "github.com/openshift-eng/openshift-tests-extension/pkg/ginkgo"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-node] Probe configuration", func() {
	var (
		oc = exutil.NewCLIWithoutNamespace("probe-termination")
	)

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}

		nodeutils.EnsureNodesReady(ctx, oc)
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Liveness probe should respect probe-level terminationGracePeriodSeconds [OCP-44493]", ote.Informing(), func() {
		ctx := context.Background()

		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("Create pod with liveness probe having probe-level terminationGracePeriodSeconds=10s")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liveness-probe-level",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: ptr.To[int64](60),
				Containers: []corev1.Container{
					{
						Name:    "test",
						Image:   image.ShellImage(),
						Command: []string{"sh", "-c", "sleep 100000000"},
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds:           5,
							FailureThreshold:              1,
							PeriodSeconds:                 60,
							TerminationGracePeriodSeconds: ptr.To[int64](10),
						},
					},
				},
			},
		}

		_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create liveness probe pod")

		g.By("Verify probe-level terminationGracePeriodSeconds is honored (10s)")
		expectedSec := 10
		// Allow asymmetric tolerance: -3s for event timing precision, +10s for container cleanup overhead
		minSec := expectedSec - 3
		maxSec := expectedSec + 10
		timeDiff, err := verifyProbeTermination(ctx, oc, namespace, "liveness-probe-level", "test", expectedSec)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get probe termination events")
		o.Expect(timeDiff).To(o.BeNumerically(">=", minSec), fmt.Sprintf("time difference %ds is less than expected minimum %ds", timeDiff, minSec))
		o.Expect(timeDiff).To(o.BeNumerically("<=", maxSec), fmt.Sprintf("time difference %ds is greater than expected maximum %ds", timeDiff, maxSec))
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Startup probe should respect probe-level terminationGracePeriodSeconds [OCP-44493]", ote.Informing(), func() {
		ctx := context.Background()

		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("Create pod with startup probe having probe-level terminationGracePeriodSeconds=10s")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "startup-probe-level",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: ptr.To[int64](60),
				Containers: []corev1.Container{
					{
						Name:    "teststartup",
						Image:   image.ShellImage(),
						Command: []string{"sh", "-c", "sleep 100000000"},
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080},
						},
						StartupProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds:           5,
							FailureThreshold:              1,
							PeriodSeconds:                 60,
							TerminationGracePeriodSeconds: ptr.To[int64](10),
						},
					},
				},
			},
		}

		_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create startup probe pod")

		g.By("Verify probe-level terminationGracePeriodSeconds is honored (10s)")
		expectedSec := 10
		// Allow asymmetric tolerance: -3s for event timing precision, +10s for container cleanup overhead
		minSec := expectedSec - 3
		maxSec := expectedSec + 10
		timeDiff, err := verifyProbeTermination(ctx, oc, namespace, "startup-probe-level", "teststartup", expectedSec)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get probe termination events")
		o.Expect(timeDiff).To(o.BeNumerically(">=", minSec), fmt.Sprintf("time difference %ds is less than expected minimum %ds", timeDiff, minSec))
		o.Expect(timeDiff).To(o.BeNumerically("<=", maxSec), fmt.Sprintf("time difference %ds is greater than expected maximum %ds", timeDiff, maxSec))
	})

	//author: bgudi@redhat.com
	g.It("[OTP] Liveness probe should fall back to pod-level terminationGracePeriodSeconds when probe-level is not set [OCP-44493]", ote.Informing(), func() {
		ctx := context.Background()

		oc.SetupProject()
		namespace := oc.Namespace()

		g.By("Create pod with liveness probe without probe-level terminationGracePeriodSeconds")
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "liveness-pod-level",
				Namespace: namespace,
			},
			Spec: corev1.PodSpec{
				TerminationGracePeriodSeconds: ptr.To[int64](60),
				Containers: []corev1.Container{
					{
						Name:    "test",
						Image:   image.ShellImage(),
						Command: []string{"sh", "-c", "sleep 100000000"},
						Ports: []corev1.ContainerPort{
							{ContainerPort: 8080},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8080),
								},
							},
							InitialDelaySeconds: 5,
							FailureThreshold:    1,
							PeriodSeconds:       60,
							// No TerminationGracePeriodSeconds - should use pod-level (60s)
						},
					},
				},
			},
		}

		_, err := oc.KubeClient().CoreV1().Pods(namespace).Create(ctx, pod, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create liveness probe pod without probe-level termination")

		g.By("Verify pod-level terminationGracePeriodSeconds is used (60s)")
		expectedSec := 60
		// Allow asymmetric tolerance: -3s for event timing precision, +10s for container cleanup overhead
		minSec := expectedSec - 3
		maxSec := expectedSec + 10
		timeDiff, err := verifyProbeTermination(ctx, oc, namespace, "liveness-pod-level", "test", expectedSec)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to get probe termination events")
		o.Expect(timeDiff).To(o.BeNumerically(">=", minSec), fmt.Sprintf("time difference %ds is less than expected minimum %ds", timeDiff, minSec))
		o.Expect(timeDiff).To(o.BeNumerically("<=", maxSec), fmt.Sprintf("time difference %ds is greater than expected maximum %ds", timeDiff, maxSec))
	})
})

// findLatestEventByReason finds the latest event matching the given reason and message filter
func findLatestEventByReason(events *corev1.EventList, reason string, msgFilter func(string) bool) *corev1.Event {
	var latestEvent *corev1.Event
	for i := range events.Items {
		event := &events.Items[i]
		if event.Reason == reason && msgFilter(event.Message) {
			if latestEvent == nil || event.LastTimestamp.Time.After(latestEvent.LastTimestamp.Time) {
				latestEvent = event
			}
		}
	}
	return latestEvent
}

// findEarliestEventAfter finds the earliest event matching the reason and filter that occurred after the given time
// Uses LastTimestamp to properly detect repeated events (container restarts)
func findEarliestEventAfter(events *corev1.EventList, reason string, msgFilter func(string) bool, afterTime time.Time) *corev1.Event {
	var earliestEvent *corev1.Event
	for i := range events.Items {
		event := &events.Items[i]
		// Use LastTimestamp instead of FirstTimestamp to catch repeated events (e.g., container restarts)
		// FirstTimestamp stays at the original event time, but LastTimestamp updates on each occurrence
		if event.Reason == reason && msgFilter(event.Message) && event.LastTimestamp.Time.After(afterTime) {
			if earliestEvent == nil || event.LastTimestamp.Time.Before(earliestEvent.LastTimestamp.Time) {
				earliestEvent = event
			}
		}
	}
	return earliestEvent
}

// verifyProbeTermination verifies that the probe termination grace period is honored
// by checking the time difference between probe failure (Killing) and container restart (Started) events
// Returns the time difference in seconds, or an error if events are not found
func verifyProbeTermination(ctx context.Context, oc *exutil.CLI, namespace, podName, containerName string, expectedTerminationSec int) (int, error) {
	var timeDiff int
	// Timeout needs to account for: pod start (~30s) + probe period (60s) + termination (up to 60s) + restart (~30s) = ~3 minutes minimum
	// Use 6 minutes to be safe for tests with 60s termination grace period
	err := wait.PollUntilContextTimeout(ctx, 10*time.Second, 6*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Get events using the Events API
		events, err := oc.KubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod", podName),
		})
		if err != nil {
			e2e.Logf("Error getting events: %v", err)
			return false, nil
		}

		// Find probe failure (Killing) event for the container
		killingEvent := findLatestEventByReason(events, "Killing", func(msg string) bool {
			return strings.Contains(msg, containerName) &&
				strings.Contains(msg, "failed") &&
				strings.Contains(msg, "probe")
		})

		if killingEvent == nil {
			e2e.Logf("Waiting for probe failure (Killing) event")
			return false, nil
		}

		// Find container restart (Started) event that occurred after the Killing event
		startedEvent := findEarliestEventAfter(events, "Started", func(msg string) bool {
			return strings.Contains(msg, "Container started")
		}, killingEvent.LastTimestamp.Time)

		if startedEvent == nil {
			e2e.Logf("Waiting for container restart (Started) event after Killing event")
			return false, nil
		}

		e2e.Logf("Killing event: %s at %v", killingEvent.Message, killingEvent.LastTimestamp)
		e2e.Logf("Started event: %s at %v", startedEvent.Message, startedEvent.LastTimestamp)

		// Calculate time difference using the helper function
		timeDiff = int(nodeutils.CalculateEventTimeDiff(killingEvent, startedEvent).Seconds())
		e2e.Logf("Time difference: %d seconds (expected: %d ±10 seconds)", timeDiff, expectedTerminationSec)

		return true, nil
	})
	if err != nil {
		return 0, err
	}
	return timeDiff, nil
}
