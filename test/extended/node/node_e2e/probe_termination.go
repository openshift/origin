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
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[Suite:openshift/disruptive-longrunning][sig-node][NodeResource:numNodes=1,label=probe_termination] Probe configuration", func() {
	var (
		oc       = exutil.NewCLIWithoutNamespace("probe-termination")
		testNode string
	)

	g.BeforeEach(func(ctx context.Context) {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster")
		}

		nodeutils.EnsureNodeResourceNodesReady(ctx, oc, "probe_termination")
		testNode, err = nodeutils.GetNodeResource(ctx, oc, "probe_termination")
		o.Expect(err).NotTo(o.HaveOccurred(), "Error getting NodeResource node")
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
				NodeName:                      testNode,
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
				NodeName:                      testNode,
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
				NodeName:                      testNode,
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

// verifyProbeTermination returns the seconds between the "Killing" event (FirstTimestamp) and
// the container's restart (pod.Status, gated on RestartCount==1 for the same cycle) - avoids
// matching the "Started" event's message text, which varies across kubelet versions.
func verifyProbeTermination(ctx context.Context, oc *exutil.CLI, namespace, podName, containerName string, expectedTerminationSec int) (int, error) {
	var timeDiff int
	// Timeout needs to account for: pod start (~30s) + probe period (60s) + termination (up to 60s) + restart (~30s) = ~3 minutes minimum
	// Use 6 minutes to be safe for tests with 60s termination grace period
	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 6*time.Minute, true, func(ctx context.Context) (bool, error) {
		pod, err := oc.KubeClient().CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Error getting pod %s: %v", podName, err)
			return false, nil
		}

		status := e2epod.FindContainerStatusInPod(pod, containerName)
		if status == nil {
			e2e.Logf("Waiting for container status of %q to appear", containerName)
			return false, nil
		}

		// Gate strictly on the first restart so the "Killing" event's FirstTimestamp below
		// is guaranteed to correspond to the same cycle as this restart, not a later one.
		if status.RestartCount != 1 {
			e2e.Logf("Waiting for the first restart of %q (restartCount=%d)", containerName, status.RestartCount)
			return false, nil
		}

		if status.State.Running == nil {
			e2e.Logf("Waiting for container %q to be running again after restart", containerName)
			return false, nil
		}

		events, err := oc.KubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=Pod,reason=Killing", podName),
		})
		if err != nil {
			e2e.Logf("Error listing events for pod %s: %v", podName, err)
			return false, nil
		}

		killingEvent := findProbeKillingEvent(events, containerName)
		if killingEvent == nil {
			e2e.Logf("Waiting for probe-failure (Killing) event for container %q", containerName)
			return false, nil
		}

		killedAt := killingEvent.FirstTimestamp.Time
		startedAt := status.State.Running.StartedAt.Time
		if !startedAt.After(killedAt) {
			// The status hasn't caught up yet (e.g. reporting a stale running instance); keep polling.
			e2e.Logf("Waiting for a fresh Running state after kill decision at %v", killedAt)
			return false, nil
		}

		timeDiff = int(startedAt.Sub(killedAt).Seconds())
		e2e.Logf("Container %q: probe failure detected at %v, restarted at %v, time difference: %d seconds (expected: %d seconds)",
			containerName, killedAt, startedAt, timeDiff, expectedTerminationSec)

		return true, nil
	})
	if err != nil {
		return 0, fmt.Errorf("failed to observe probe termination and restart for container %q: %w", containerName, err)
	}
	return timeDiff, nil
}

// findProbeKillingEvent finds the "Killing" event kubelet records when a probe failure
// triggers container termination. The message format is "Container <name> failed
// liveness/startup probe, will be restarted" (see kuberuntime_manager.go).
func findProbeKillingEvent(events *corev1.EventList, containerName string) *corev1.Event {
	for i := range events.Items {
		event := &events.Items[i]
		if strings.Contains(event.Message, containerName) && strings.Contains(event.Message, "failed") && strings.Contains(event.Message, "probe") {
			return event
		}
	}
	return nil
}
