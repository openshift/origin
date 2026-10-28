package utils

import (
	"context"
	"fmt"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/test/e2e/framework"
)

// RecoveryEssentialOperators are the cluster operators that must be healthy
// for TNF recovery tests to execute. Tests will skip when any of these are
// unhealthy.  Non-essential operators (e.g. image-registry) are audited
// separately: their failures are reported as primary diagnostics without
// blocking independent recovery tests.
var RecoveryEssentialOperators = []string{
	"etcd",
	"kube-apiserver",
	"openshift-apiserver",
	"network",
	"kube-controller-manager",
	"dns",
	"kube-scheduler",
	"machine-config",
}

// UnhealthyOperator captures the state of a cluster operator that is not
// fully healthy (unavailable or degraded).
type UnhealthyOperator struct {
	Name       string
	Available  bool
	Degraded   bool
	Conditions []v1.ClusterOperatorStatusCondition
}

// NonEssentialOperatorStatus captures the health state of all non-essential
// operators at a point in time.
type NonEssentialOperatorStatus struct {
	Unhealthy []UnhealthyOperator
	Healthy   int
	Total     int
}

// HasFailures returns true if any non-essential operators are unhealthy.
func (s *NonEssentialOperatorStatus) HasFailures() bool {
	return len(s.Unhealthy) > 0
}

// Summary returns a human-readable summary suitable for JUnit/CI output.
func (s *NonEssentialOperatorStatus) Summary() string {
	if !s.HasFailures() {
		return fmt.Sprintf("all %d non-essential operators healthy", s.Healthy)
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d of %d non-essential operator(s) unhealthy:\n", len(s.Unhealthy), s.Total)
	for _, op := range s.Unhealthy {
		fmt.Fprintf(&sb, "  operator %q: Available=%v Degraded=%v\n", op.Name, op.Available, op.Degraded)
		for _, c := range op.Conditions {
			if (c.Type == v1.OperatorAvailable && c.Status != v1.ConditionTrue) ||
				(c.Type == v1.OperatorDegraded && c.Status == v1.ConditionTrue) {
				fmt.Fprintf(&sb, "    %s=%s reason=%s: %s\n", c.Type, c.Status, c.Reason, c.Message)
				if !c.LastTransitionTime.IsZero() {
					fmt.Fprintf(&sb, "    since %s\n", c.LastTransitionTime.UTC().Format(time.RFC3339))
				}
			}
		}
	}
	return sb.String()
}

// isRecoveryEssentialOperator returns true if the operator name is in the
// essential set for recovery tests.
func isRecoveryEssentialOperator(name string) bool {
	for _, essential := range RecoveryEssentialOperators {
		if name == essential {
			return true
		}
	}
	return false
}

// AuditNonEssentialOperators lists all cluster operators and returns the
// health status of those not in RecoveryEssentialOperators. Essential
// operators are excluded because they are validated separately by
// ValidateEssentialOperatorsAvailable.
func AuditNonEssentialOperators(ctx context.Context, oc *exutil.CLI) (*NonEssentialOperatorStatus, error) {
	status := &NonEssentialOperatorStatus{}

	cos, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(
		ctx, metav1.ListOptions{})
	if err != nil {
		framework.Logf("WARNING: cannot list cluster operators for audit: %v", err)
		return status, err
	}

	for i := range cos.Items {
		co := &cos.Items[i]
		if isRecoveryEssentialOperator(co.Name) {
			continue
		}
		status.Total++

		available := IsClusterOperatorAvailable(co)
		degraded := IsClusterOperatorDegraded(co)

		if !available || degraded {
			status.Unhealthy = append(status.Unhealthy, UnhealthyOperator{
				Name:       co.Name,
				Available:  available,
				Degraded:   degraded,
				Conditions: co.Status.Conditions,
			})
		} else {
			status.Healthy++
		}
	}

	return status, nil
}

// WaitForEssentialOperatorsHealthy polls until all RecoveryEssentialOperators
// are Available=True and Degraded=False, or the timeout expires.
func WaitForEssentialOperatorsHealthy(oc *exutil.CLI, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			return fmt.Errorf("essential operators not healthy after %v: %w", timeout, lastErr)
		}
		callCtx, cancel := context.WithTimeout(context.Background(), remaining)
		lastErr = ValidateEssentialOperatorsAvailable(callCtx, oc)
		cancel()
		if lastErr == nil {
			return nil
		}
		if time.Until(deadline) <= 0 {
			return fmt.Errorf("essential operators not healthy after %v: %w", timeout, lastErr)
		}
		time.Sleep(FiveSecondPollInterval)
	}
}

const evidenceTimeout = 30 * time.Second

// GatherOperatorEvidence collects diagnostic evidence for a set of unhealthy
// operators: pod status, recent events, and (for image-registry) PV/PVC
// state. Collection is time-bounded to avoid blocking teardown.
func GatherOperatorEvidence(oc *exutil.CLI, unhealthy []UnhealthyOperator) string {
	var sb strings.Builder

	for _, op := range unhealthy {
		fmt.Fprintf(&sb, "\n=== Evidence: operator %q ===\n", op.Name)

		ns := operatorNamespace(op.Name)
		if ns == "" {
			sb.WriteString("  (namespace unknown — skipping pod/event collection)\n")
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), evidenceTimeout)

		gatherPodEvidence(oc, ctx, ns, &sb)
		gatherEventEvidence(oc, ctx, ns, &sb)
		if op.Name == "image-registry" {
			gatherStorageEvidence(oc, ctx, ns, &sb)
		}

		cancel()
	}

	return sb.String()
}

func gatherPodEvidence(oc *exutil.CLI, ctx context.Context, ns string, sb *strings.Builder) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(sb, "  Pods: failed to list: %v\n", err)
		return
	}
	fmt.Fprintf(sb, "  Pods (%d):\n", len(pods.Items))
	for i := range pods.Items {
		pod := &pods.Items[i]
		fmt.Fprintf(sb, "    %s  Phase=%s  Node=%s\n", pod.Name, pod.Status.Phase, pod.Spec.NodeName)
		for _, cs := range pod.Status.ContainerStatuses {
			fmt.Fprintf(sb, "      container %s: Ready=%v Restarts=%d\n", cs.Name, cs.Ready, cs.RestartCount)
			if cs.State.Waiting != nil {
				fmt.Fprintf(sb, "        Waiting: %s — %s\n", cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
			if cs.State.Terminated != nil {
				fmt.Fprintf(sb, "        Terminated: %s — %s\n", cs.State.Terminated.Reason, cs.State.Terminated.Message)
			}
		}
		for _, cond := range pod.Status.Conditions {
			if cond.Status == corev1.ConditionFalse && cond.Message != "" {
				fmt.Fprintf(sb, "      condition %s=False: %s\n", cond.Type, cond.Message)
			}
		}
	}
}

func gatherEventEvidence(oc *exutil.CLI, ctx context.Context, ns string, sb *strings.Builder) {
	events, err := oc.AdminKubeClient().CoreV1().Events(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(sb, "  Events: failed to list: %v\n", err)
		return
	}
	cutoff := time.Now().Add(-30 * time.Minute)
	var recent []corev1.Event
	for _, ev := range events.Items {
		t := ev.LastTimestamp.Time
		if t.IsZero() {
			t = ev.CreationTimestamp.Time
		}
		if t.After(cutoff) {
			recent = append(recent, ev)
		}
	}
	if len(recent) > 20 {
		recent = recent[len(recent)-20:]
	}
	if len(recent) > 0 {
		fmt.Fprintf(sb, "  Recent events (last 30 min, showing %d):\n", len(recent))
		for _, ev := range recent {
			t := ev.LastTimestamp.Time
			if t.IsZero() {
				t = ev.CreationTimestamp.Time
			}
			fmt.Fprintf(sb, "    %s %s/%s: %s (%s)\n",
				t.UTC().Format(time.RFC3339),
				ev.InvolvedObject.Kind, ev.InvolvedObject.Name,
				ev.Message, ev.Reason)
		}
	}
}

func gatherStorageEvidence(oc *exutil.CLI, ctx context.Context, ns string, sb *strings.Builder) {
	pvcs, err := oc.AdminKubeClient().CoreV1().PersistentVolumeClaims(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		fmt.Fprintf(sb, "  PVCs: failed to list: %v\n", err)
		return
	}
	if len(pvcs.Items) == 0 {
		return
	}
	fmt.Fprintf(sb, "  PersistentVolumeClaims (%d):\n", len(pvcs.Items))
	for i := range pvcs.Items {
		pvc := &pvcs.Items[i]
		fmt.Fprintf(sb, "    %s: Phase=%s Volume=%s\n", pvc.Name, pvc.Status.Phase, pvc.Spec.VolumeName)
		if pvc.Spec.VolumeName == "" {
			continue
		}
		pv, pvErr := oc.AdminKubeClient().CoreV1().PersistentVolumes().Get(ctx, pvc.Spec.VolumeName, metav1.GetOptions{})
		if pvErr != nil {
			fmt.Fprintf(sb, "      PV %s: get failed: %v\n", pvc.Spec.VolumeName, pvErr)
			continue
		}
		fmt.Fprintf(sb, "      PV %s: Phase=%s ReclaimPolicy=%s\n",
			pv.Name, pv.Status.Phase, pv.Spec.PersistentVolumeReclaimPolicy)
		if pv.Spec.NFS != nil {
			fmt.Fprintf(sb, "      NFS server=%s path=%s\n", pv.Spec.NFS.Server, pv.Spec.NFS.Path)
		}
		if pv.Status.Message != "" {
			fmt.Fprintf(sb, "      Message: %s\n", pv.Status.Message)
		}
	}
}

func operatorNamespace(name string) string {
	known := map[string]string{
		"image-registry":               "openshift-image-registry",
		"console":                      "openshift-console",
		"monitoring":                   "openshift-monitoring",
		"authentication":               "openshift-authentication",
		"ingress":                      "openshift-ingress",
		"marketplace":                  "openshift-marketplace",
		"cloud-credential":             "openshift-cloud-credential-operator",
		"operator-lifecycle-manager":   "openshift-operator-lifecycle-manager",
		"dns":                          "openshift-dns",
		"machine-config":               "openshift-machine-config-operator",
		"openshift-controller-manager": "openshift-controller-manager",
	}
	if ns, ok := known[name]; ok {
		return ns
	}
	return ""
}
