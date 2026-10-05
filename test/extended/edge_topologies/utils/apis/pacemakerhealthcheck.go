package apis

import (
	"context"
	"fmt"
	"strings"
	"time"

	etcdv1 "github.com/openshift/api/etcd/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const (
	PacemakerHealthCheckDegradedCondition = "PacemakerHealthCheckDegraded"
	pacemakerUnhealthyReason              = "PacemakerUnhealthy"
	insufficientNodesHealthMessage        = "Cluster is unhealthy: Insufficient nodes in cluster (expected 2, found 1)"
	insufficientNodesCountMessage         = "Expected 2 nodes, found 1"

	healthCheckPollInterval = 10 * time.Second

	// statusCollectorCronJobName is the CronJob that snapshots pacemaker status
	// into the PacemakerCluster CR (see cluster-etcd-operator).
	statusCollectorCronJobName = "pacemaker-status-collector"

	statusCollectorWriteBlockPolicyName = "tnf-e2e-block-pacemaker-status-collector"

	statusCollectorWriteBlockMessage = "PacemakerCluster writes are blocked for TNF e2e testing"

	// PacemakerDegradedDetectionTimeout must exceed the operator's worst-case
	// detection latency. When a node drops, the etcd/API/CronJob pipeline is
	// disrupted, so degraded is often reached via the staleness path:
	// StatusStalenessThreshold (5m) -> status Unknown, then
	// StatusUnknownDegradedThreshold (5m) -> PacemakerHealthCheckDegraded=True
	// (see cluster-etcd-operator pkg/tnf/pkg/pacemaker/constants.go). Both
	// thresholds are measured from the same frozen previous.CRLastUpdated
	// timestamp (only advanced on non-Unknown syncs), not chained, so degraded
	// follows within roughly the same ~5m window staleness first fires in, not
	// 5m+5m. That is a 5m minimum even with a healthy controller; the extra
	// margin covers status-collector CronJob scheduling jitter.
	PacemakerDegradedDetectionTimeout = 15 * time.Minute
)

// PacemakerTargetFailureMode identifies the target-specific failure state
// accepted by WaitForPacemakerTargetFailure.
type PacemakerTargetFailureMode string

const (
	// PacemakerTargetOffline means the target remains in Pacemaker status and
	// is explicitly offline while the survivor is online.
	PacemakerTargetOffline PacemakerTargetFailureMode = "target-offline"
	// PacemakerTargetRemoved means the target is absent and the survivor is the
	// sole online Pacemaker member with the exact insufficient-node state.
	PacemakerTargetRemoved PacemakerTargetFailureMode = "target-removed"
)

type pacemakerTargetFailureReader struct {
	getPacemakerCluster func() (*etcdv1.PacemakerCluster, error)
	getEtcdOperator     func() (*operatorv1.Etcd, error)
}

func getEtcdOperator(oc *exutil.CLI) (*operatorv1.Etcd, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return oc.AdminOperatorClient().OperatorV1().Etcds().Get(ctx, "cluster", metav1.GetOptions{})
}

func getStatusCollectorServiceAccountName(oc *exutil.CLI) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cronJob, err := oc.AdminKubeClient().BatchV1().CronJobs(EtcdNamespace).Get(ctx, statusCollectorCronJobName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get status collector CronJob: %w", err)
	}
	serviceAccountName := cronJob.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName
	if serviceAccountName == "" {
		return "", fmt.Errorf("status collector CronJob %s/%s pod template has empty serviceAccountName", EtcdNamespace, statusCollectorCronJobName)
	}
	return serviceAccountName, nil
}

// BlockStatusCollectorWrites creates an admission policy that denies the status
// collector's PacemakerCluster writes, so the CR's lastUpdated stops advancing.
// CEO reconciles the CronJob spec, so suspend/schedule edits do not hold; denying
// the write at admission is the stable test lever.
//
// The collector runs as the service account from its CronJob pod template
// (currently tnf-setup-manager), which it shares with the TNF setup Jobs, so the
// policy denies PacemakerCluster create/update from all of them. That is safe for
// these specs: setup Jobs run during install, node replacement and fencing
// credential changes, not on the healthy steady-state cluster the specs require
// (see SkipIfClusterIsNotHealthy).
func BlockStatusCollectorWrites(oc *exutil.CLI) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	serviceAccountName, err := getStatusCollectorServiceAccountName(oc)
	if err != nil {
		return err
	}

	policy := &admissionregistrationv1.ValidatingAdmissionPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: statusCollectorWriteBlockPolicyName},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicySpec{
			MatchConstraints: &admissionregistrationv1.MatchResources{
				ResourceRules: []admissionregistrationv1.NamedRuleWithOperations{{
					RuleWithOperations: admissionregistrationv1.RuleWithOperations{
						Operations: []admissionregistrationv1.OperationType{admissionregistrationv1.Create, admissionregistrationv1.Update},
						Rule: admissionregistrationv1.Rule{
							APIGroups:   []string{PacemakerClusterGVR.Group},
							APIVersions: []string{"*"},
							Resources:   []string{"pacemakerclusters", "pacemakerclusters/status"},
						},
					},
				}},
			},
			MatchConditions: []admissionregistrationv1.MatchCondition{{
				Name:       "status-collector-service-account",
				Expression: fmt.Sprintf("request.userInfo.username == 'system:serviceaccount:%s:%s'", EtcdNamespace, serviceAccountName),
			}},
			Validations: []admissionregistrationv1.Validation{{
				Expression: "false",
				Message:    statusCollectorWriteBlockMessage,
			}},
		},
	}
	if _, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Create(ctx, policy, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create status collector write-block ValidatingAdmissionPolicy: %w", err)
	}

	binding := &admissionregistrationv1.ValidatingAdmissionPolicyBinding{
		ObjectMeta: metav1.ObjectMeta{Name: statusCollectorWriteBlockPolicyName},
		Spec: admissionregistrationv1.ValidatingAdmissionPolicyBindingSpec{
			PolicyName:        statusCollectorWriteBlockPolicyName,
			ValidationActions: []admissionregistrationv1.ValidationAction{admissionregistrationv1.Deny},
		},
	}
	if _, err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Create(ctx, binding, metav1.CreateOptions{}); err != nil && !apierrors.IsAlreadyExists(err) {
		return fmt.Errorf("create status collector write-block ValidatingAdmissionPolicyBinding: %w", err)
	}
	return nil
}

// WaitForStatusCollectorWritesBlocked waits until the admission policy denies
// the status collector service account's dry-run PacemakerCluster status update.
// Matching the policy message prevents a generic RBAC Forbidden response from
// being mistaken for proof that the admission policy has propagated.
func WaitForStatusCollectorWritesBlocked(oc *exutil.CLI, timeout time.Duration) error {
	serviceAccountName, err := getStatusCollectorServiceAccountName(oc)
	if err != nil {
		return err
	}

	config := rest.CopyConfig(oc.AdminConfig())
	config.Impersonate = rest.ImpersonationConfig{
		UserName: fmt.Sprintf("system:serviceaccount:%s:%s", EtcdNamespace, serviceAccountName),
	}
	client, err := dynamic.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("create status collector impersonation client: %w", err)
	}

	checker := func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pc, err := oc.AdminDynamicClient().Resource(PacemakerClusterGVR).Get(ctx, "cluster", metav1.GetOptions{})
		if err != nil {
			framework.Logf("WaitForStatusCollectorWritesBlocked: get PacemakerCluster: %v", err)
			return false, nil
		}
		_, err = client.Resource(PacemakerClusterGVR).UpdateStatus(ctx, pc, metav1.UpdateOptions{DryRun: []string{metav1.DryRunAll}})
		if err != nil {
			if (apierrors.IsForbidden(err) || apierrors.IsInvalid(err)) && strings.Contains(err.Error(), statusCollectorWriteBlockMessage) {
				return true, nil
			}
			framework.Logf("WaitForStatusCollectorWritesBlocked: status update not blocked by admission policy: %v", err)
		}
		return false, nil
	}

	if err := core.PollUntil(checker, timeout, 5*time.Second, "status collector writes blocked by admission policy"); err != nil {
		return fmt.Errorf("wait for admission policy to block status collector writes: %w", err)
	}
	return nil
}

// UnblockStatusCollectorWrites removes the test admission policy and its binding.
func UnblockStatusCollectorWrites(oc *exutil.CLI) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicyBindings().Delete(ctx, statusCollectorWriteBlockPolicyName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete status collector write-block ValidatingAdmissionPolicyBinding: %w", err)
	}
	if err := oc.AdminKubeClient().AdmissionregistrationV1().ValidatingAdmissionPolicies().Delete(ctx, statusCollectorWriteBlockPolicyName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("delete status collector write-block ValidatingAdmissionPolicy: %w", err)
	}
	return nil
}

// RemoveLeftoverStatusCollectorWriteBlock deletes a status collector write-block
// policy left behind by an earlier run that was killed before its DeferCleanup
// ran (for example, by a CI step timeout). It reports whether one was found.
func RemoveLeftoverStatusCollectorWriteBlock(oc *exutil.CLI) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	admission := oc.AdminKubeClient().AdmissionregistrationV1()
	_, policyErr := admission.ValidatingAdmissionPolicies().Get(ctx, statusCollectorWriteBlockPolicyName, metav1.GetOptions{})
	_, bindingErr := admission.ValidatingAdmissionPolicyBindings().Get(ctx, statusCollectorWriteBlockPolicyName, metav1.GetOptions{})
	for _, err := range []error{policyErr, bindingErr} {
		if err != nil && !apierrors.IsNotFound(err) {
			return false, fmt.Errorf("check for leftover status collector write-block policy: %w", err)
		}
	}
	if apierrors.IsNotFound(policyErr) && apierrors.IsNotFound(bindingErr) {
		return false, nil
	}
	return true, UnblockStatusCollectorWrites(oc)
}

// GetStatusCollectorPinnedNode returns the nodeName currently pinned in the
// status collector CronJob pod template. An empty result means it is not pinned.
func GetStatusCollectorPinnedNode(oc *exutil.CLI) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cronJob, err := oc.AdminKubeClient().BatchV1().CronJobs(EtcdNamespace).Get(ctx, statusCollectorCronJobName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("get status collector CronJob: %w", err)
	}
	return cronJob.Spec.JobTemplate.Spec.Template.Spec.NodeName, nil
}

// GetPacemakerClusterLastUpdated returns the most recent PacemakerCluster status
// collection timestamp and errors when the collector has not populated it yet.
func GetPacemakerClusterLastUpdated(oc *exutil.CLI) (time.Time, error) {
	pc, err := GetPacemakerCluster(oc)
	if err != nil {
		return time.Time{}, err
	}
	lastUpdated := pc.Status.LastUpdated.Time
	if lastUpdated.IsZero() {
		return time.Time{}, fmt.Errorf("PacemakerCluster CR lastUpdated is zero (status never populated)")
	}
	return lastUpdated, nil
}

// DumpHealthCheckDiagnostics logs the state most useful for triaging a
// PacemakerHealthCheck test failure: the etcd operator condition, the
// PacemakerCluster CR staleness and conditions, the status-collector CronJob's
// last run, and node readiness. Together these show whether the data pipeline
// was flowing (CR fresh, CronJob running, nodes Ready) or broken. Every step is
// best-effort — this runs on an already-failing path and must never itself fail
// or panic. Called both internally on wait timeouts and by callers wiring it up
// as a failure-time DeferCleanup so any failing assertion gets the dump, not
// just polling timeouts.
func DumpHealthCheckDiagnostics(oc *exutil.CLI, reason string) {
	framework.Logf("========== PACEMAKER HEALTHCHECK DIAGNOSTICS (%s) ==========", reason)

	// 1. etcd operator PacemakerHealthCheckDegraded condition
	if etcd, err := getEtcdOperator(oc); err != nil {
		framework.Logf("diagnostics: get etcd operator: %v", err)
	} else if cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition); cond == nil {
		framework.Logf("diagnostics: etcd operator %s condition absent", PacemakerHealthCheckDegradedCondition)
	} else {
		framework.Logf("diagnostics: etcd operator %s: Status=%s reason=%s message=%q lastTransition=%s",
			PacemakerHealthCheckDegradedCondition, cond.Status, cond.Reason, cond.Message,
			cond.LastTransitionTime.Format(time.RFC3339))
	}

	// 2. PacemakerCluster CR staleness and conditions
	if pc, err := GetPacemakerCluster(oc); err != nil {
		framework.Logf("diagnostics: get PacemakerCluster CR: %v", err)
	} else {
		lastUpdated := pc.Status.LastUpdated.Time
		if lastUpdated.IsZero() {
			framework.Logf("diagnostics: PacemakerCluster CR lastUpdated is zero (status never populated)")
		} else {
			framework.Logf("diagnostics: PacemakerCluster CR lastUpdated=%s (age %v)",
				lastUpdated.Format(time.RFC3339), time.Since(lastUpdated).Round(time.Second))
		}
		for i := range pc.Status.Conditions {
			c := &pc.Status.Conditions[i]
			framework.Logf("diagnostics: PacemakerCluster condition %s=%s reason=%s message=%q",
				c.Type, c.Status, c.Reason, c.Message)
		}
	}

	// 3. status-collector CronJob last run
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	cronJob, err := oc.AdminKubeClient().BatchV1().CronJobs(EtcdNamespace).Get(ctx, statusCollectorCronJobName, metav1.GetOptions{})
	cancel()
	if err != nil {
		framework.Logf("diagnostics: get CronJob %s/%s: %v", EtcdNamespace, statusCollectorCronJobName, err)
	} else {
		lastSchedule := "never"
		if cronJob.Status.LastScheduleTime != nil {
			lastSchedule = cronJob.Status.LastScheduleTime.Format(time.RFC3339)
		}
		lastSuccess := "never"
		if cronJob.Status.LastSuccessfulTime != nil {
			lastSuccess = cronJob.Status.LastSuccessfulTime.Format(time.RFC3339)
		}
		framework.Logf("diagnostics: CronJob %s lastSchedule=%s lastSuccessful=%s activeJobs=%d",
			statusCollectorCronJobName, lastSchedule, lastSuccess, len(cronJob.Status.Active))
	}

	// 4. Node readiness
	ctx2, cancel2 := context.WithTimeout(context.Background(), 30*time.Second)
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx2, metav1.ListOptions{})
	cancel2()
	if err != nil {
		framework.Logf("diagnostics: list nodes: %v", err)
	} else {
		for i := range nodes.Items {
			n := &nodes.Items[i]
			ready := "Unknown"
			for _, c := range n.Status.Conditions {
				if c.Type == corev1.NodeReady {
					ready = string(c.Status)
					break
				}
			}
			framework.Logf("diagnostics: node %s Ready=%s", n.Name, ready)
		}
	}

	framework.Logf("========== END PACEMAKER HEALTHCHECK DIAGNOSTICS ==========")
}

// describeNotDegradedCondition formats a not-yet-True PacemakerHealthCheckDegraded
// condition for logging. The operator's clearPacemakerDegradedCondition intentionally
// leaves Reason/Message empty when setting Status=False (see healthcheck.go), so an
// empty reason/message here reflects the healthy baseline, not a missing field bug.
func describeNotDegradedCondition(cond *operatorv1.OperatorCondition) string {
	if cond.Reason == "" && cond.Message == "" {
		return fmt.Sprintf("Status=%s (healthy)", cond.Status)
	}
	return fmt.Sprintf("Status=%s reason=%s message=%q", cond.Status, cond.Reason, cond.Message)
}

func expectPacemakerNodes(pc *etcdv1.PacemakerCluster, names ...string) error {
	if pc == nil {
		return fmt.Errorf("PacemakerCluster is nil")
	}
	if pc.Status.Nodes == nil {
		return fmt.Errorf("PacemakerCluster status nodes are nil")
	}

	expected := make(map[string]struct{}, len(names))
	for _, name := range names {
		if name == "" {
			return fmt.Errorf("expected node name is empty")
		}
		if _, exists := expected[name]; exists {
			return fmt.Errorf("expected node name %q is duplicated", name)
		}
		expected[name] = struct{}{}
	}

	nodes := *pc.Status.Nodes
	if len(nodes) != len(expected) {
		return fmt.Errorf("PacemakerCluster has %d nodes, expected exactly %d", len(nodes), len(expected))
	}

	byName := make(map[string]*etcdv1.PacemakerClusterNodeStatus, len(nodes))
	for i := range nodes {
		node := &nodes[i]
		if node.NodeName == "" {
			return fmt.Errorf("PacemakerCluster contains a node with an empty name")
		}
		if _, exists := byName[node.NodeName]; exists {
			return fmt.Errorf("PacemakerCluster contains duplicate node %q", node.NodeName)
		}
		if _, exists := expected[node.NodeName]; !exists {
			return fmt.Errorf("PacemakerCluster contains unexpected node %q", node.NodeName)
		}
		byName[node.NodeName] = node
	}
	return nil
}

func validatePacemakerTargetFailureArgs(targetNode, survivingNode, baselineResourceVersion string) error {
	if targetNode == "" || survivingNode == "" || targetNode == survivingNode {
		return fmt.Errorf("target and surviving node names must be distinct and non-empty")
	}
	if baselineResourceVersion == "" {
		return fmt.Errorf("PacemakerCluster baseline resourceVersion must be non-empty")
	}
	return nil
}

func readStablePacemakerTargetFailureSnapshot(reader pacemakerTargetFailureReader) (*etcdv1.PacemakerCluster, *operatorv1.Etcd, error) {
	pcBefore, err := reader.getPacemakerCluster()
	if err != nil {
		return nil, nil, fmt.Errorf("get PacemakerCluster before Etcd condition: %w", err)
	}
	if pcBefore == nil {
		return nil, nil, fmt.Errorf("get PacemakerCluster before Etcd condition: empty response")
	}
	if pcBefore.ResourceVersion == "" {
		return nil, nil, fmt.Errorf("PacemakerCluster before Etcd condition has an empty resourceVersion")
	}

	etcd, err := reader.getEtcdOperator()
	if err != nil {
		return nil, nil, fmt.Errorf("get Etcd operator: %w", err)
	}
	if etcd == nil {
		return nil, nil, fmt.Errorf("get Etcd operator: empty response")
	}

	pcAfter, err := reader.getPacemakerCluster()
	if err != nil {
		return nil, nil, fmt.Errorf("get PacemakerCluster after Etcd condition: %w", err)
	}
	if pcAfter == nil {
		return nil, nil, fmt.Errorf("get PacemakerCluster after Etcd condition: empty response")
	}
	if pcAfter.ResourceVersion == "" {
		return nil, nil, fmt.Errorf("PacemakerCluster after Etcd condition has an empty resourceVersion")
	}
	if pcBefore.ResourceVersion != pcAfter.ResourceVersion {
		return nil, nil, fmt.Errorf("PacemakerCluster changed while reading Etcd condition (resourceVersion %q to %q)",
			pcBefore.ResourceVersion, pcAfter.ResourceVersion)
	}
	return pcAfter, etcd, nil
}

func capturePacemakerTargetFailureBaseline(reader pacemakerTargetFailureReader) (string, error) {
	pc, etcd, err := readStablePacemakerTargetFailureSnapshot(reader)
	if err != nil {
		return "", err
	}
	cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return "", fmt.Errorf("healthy baseline is missing %s condition", PacemakerHealthCheckDegradedCondition)
	}
	if cond.Status != operatorv1.ConditionFalse {
		return "", fmt.Errorf("healthy baseline requires %s=False, observed %s",
			PacemakerHealthCheckDegradedCondition, describeNotDegradedCondition(cond))
	}
	return pc.ResourceVersion, nil
}

func classifyPacemakerTargetFailureMessage(message, targetNode string) (PacemakerTargetFailureMode, error) {
	offlineMessage := fmt.Sprintf("Node %s is offline", targetNode)
	offlineMatches := 0
	removedMatches := 0

	for _, component := range strings.Split(message, "; ") {
		switch component {
		case offlineMessage:
			offlineMatches++
		case insufficientNodesHealthMessage:
			removedMatches++
		default:
			if strings.HasPrefix(component, "Node ") && strings.HasSuffix(component, " is offline") {
				return "", fmt.Errorf("%s message contains conflicting offline component %q", PacemakerHealthCheckDegradedCondition, component)
			}
			if strings.HasPrefix(component, "Cluster is unhealthy: Insufficient nodes in cluster ") {
				return "", fmt.Errorf("%s message contains conflicting insufficient-node component %q", PacemakerHealthCheckDegradedCondition, component)
			}
		}
	}

	if offlineMatches > 1 || removedMatches > 1 {
		return "", fmt.Errorf("%s message repeats a target-failure component: %q", PacemakerHealthCheckDegradedCondition, message)
	}
	if offlineMatches == 1 && removedMatches == 1 {
		return "", fmt.Errorf("%s message contains conflicting offline and removed components: %q", PacemakerHealthCheckDegradedCondition, message)
	}
	if offlineMatches == 1 {
		return PacemakerTargetOffline, nil
	}
	if removedMatches == 1 {
		return PacemakerTargetRemoved, nil
	}
	return "", fmt.Errorf("%s message %q does not contain the exact target %s offline or one-node removal component",
		PacemakerHealthCheckDegradedCondition, message, targetNode)
}

func classifyPacemakerTargetFailure(
	cond *operatorv1.OperatorCondition,
	pc *etcdv1.PacemakerCluster,
	targetNode, survivingNode string,
) (PacemakerTargetFailureMode, error) {
	if targetNode == "" || survivingNode == "" || targetNode == survivingNode {
		return "", fmt.Errorf("target and surviving node names must be distinct and non-empty")
	}
	if cond == nil {
		return "", fmt.Errorf("%s condition is absent", PacemakerHealthCheckDegradedCondition)
	}
	if cond.Status != operatorv1.ConditionTrue {
		return "", fmt.Errorf("%s condition is %s, expected True", PacemakerHealthCheckDegradedCondition, cond.Status)
	}
	if cond.Reason != pacemakerUnhealthyReason {
		return "", fmt.Errorf("%s reason is %q, expected %q", PacemakerHealthCheckDegradedCondition, cond.Reason, pacemakerUnhealthyReason)
	}
	if pc == nil {
		return "", fmt.Errorf("PacemakerCluster is nil")
	}
	if pc.Status.LastUpdated.IsZero() {
		return "", fmt.Errorf("PacemakerCluster lastUpdated is zero")
	}

	mode, err := classifyPacemakerTargetFailureMessage(cond.Message, targetNode)
	if err != nil {
		return "", err
	}
	switch mode {
	case PacemakerTargetOffline:
		if err := expectPacemakerNodes(pc, targetNode, survivingNode); err != nil {
			return "", err
		}
		if err := ExpectNodeCondition(pc, targetNode, etcdv1.NodeOnlineConditionType, metav1.ConditionFalse); err != nil {
			return "", err
		}
		if err := ExpectNodeCondition(pc, survivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionTrue); err != nil {
			return "", err
		}
		if err := ExpectNodeCondition(pc, survivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionTrue); err != nil {
			return "", err
		}
		if err := ExpectClusterCondition(pc, etcdv1.ClusterNodeCountAsExpectedConditionType, metav1.ConditionTrue); err != nil {
			return "", err
		}
		return mode, nil

	case PacemakerTargetRemoved:
		if err := expectPacemakerNodes(pc, survivingNode); err != nil {
			return "", err
		}
		if err := ExpectNodeCondition(pc, survivingNode, etcdv1.NodeOnlineConditionType, metav1.ConditionTrue); err != nil {
			return "", err
		}
		if err := ExpectNodeCondition(pc, survivingNode, etcdv1.NodeMemberConditionType, metav1.ConditionTrue); err != nil {
			return "", err
		}
		nodeCount := meta.FindStatusCondition(pc.Status.Conditions, etcdv1.ClusterNodeCountAsExpectedConditionType)
		if nodeCount == nil {
			return "", fmt.Errorf("PacemakerCluster is missing %s condition", etcdv1.ClusterNodeCountAsExpectedConditionType)
		}
		if nodeCount.Status != metav1.ConditionFalse ||
			nodeCount.Reason != etcdv1.ClusterNodeCountAsExpectedReasonInsufficientNodes ||
			nodeCount.Message != insufficientNodesCountMessage {
			return "", fmt.Errorf("PacemakerCluster %s is %s reason=%q message=%q, expected False reason=%q message=%q",
				etcdv1.ClusterNodeCountAsExpectedConditionType, nodeCount.Status, nodeCount.Reason, nodeCount.Message,
				etcdv1.ClusterNodeCountAsExpectedReasonInsufficientNodes, insufficientNodesCountMessage)
		}
		return mode, nil
	default:
		return "", fmt.Errorf("unsupported Pacemaker target failure mode %q", mode)
	}
}

func observePacemakerTargetFailure(
	reader pacemakerTargetFailureReader,
	targetNode, survivingNode string,
	baselineResourceVersion string,
) (PacemakerTargetFailureMode, error) {
	if err := validatePacemakerTargetFailureArgs(targetNode, survivingNode, baselineResourceVersion); err != nil {
		return "", err
	}
	pc, etcd, err := readStablePacemakerTargetFailureSnapshot(reader)
	if err != nil {
		return "", err
	}
	if pc.ResourceVersion == baselineResourceVersion {
		return "", fmt.Errorf("PacemakerCluster resourceVersion %q is unchanged from the pre-destruction baseline", pc.ResourceVersion)
	}

	cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
	return classifyPacemakerTargetFailure(cond, pc, targetNode, survivingNode)
}

func waitForPacemakerTargetFailure(
	reader pacemakerTargetFailureReader,
	targetNode, survivingNode string,
	baselineResourceVersion string,
	timeout, pollInterval time.Duration,
) (PacemakerTargetFailureMode, error) {
	if err := validatePacemakerTargetFailureArgs(targetNode, survivingNode, baselineResourceVersion); err != nil {
		return "", err
	}
	var acceptedMode PacemakerTargetFailureMode
	var lastErr error
	checker := func() (bool, error) {
		mode, err := observePacemakerTargetFailure(reader, targetNode, survivingNode, baselineResourceVersion)
		if err != nil {
			lastErr = err
			framework.Logf("WaitForPacemakerTargetFailure: %v", err)
			return false, nil
		}
		acceptedMode = mode
		framework.Logf("Coherent Pacemaker target failure confirmed: mode=%s target=%s survivor=%s", mode, targetNode, survivingNode)
		return true, nil
	}

	if err := core.PollUntil(checker, timeout, pollInterval, "coherent Pacemaker target failure"); err != nil {
		return "", fmt.Errorf("timed out after %v waiting for target %s offline or removed (last: %v)", timeout, targetNode, lastErr)
	}
	return acceptedMode, nil
}

// CapturePacemakerTargetFailureBaseline validates that the live
// PacemakerHealthCheckDegraded condition is healthy and returns the stable
// PacemakerCluster resourceVersion immediately before target destruction.
func CapturePacemakerTargetFailureBaseline(oc *exutil.CLI) (string, error) {
	reader := pacemakerTargetFailureReader{
		getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return GetPacemakerCluster(oc) },
		getEtcdOperator:     func() (*operatorv1.Etcd, error) { return getEtcdOperator(oc) },
	}
	resourceVersion, err := capturePacemakerTargetFailureBaseline(reader)
	if err != nil {
		DumpHealthCheckDiagnostics(oc, "CapturePacemakerTargetFailureBaseline failed")
		return "", fmt.Errorf("capture healthy pre-destruction Pacemaker baseline: %w", err)
	}
	return resourceVersion, nil
}

// WaitForPacemakerTargetFailure accepts either the named target explicitly
// offline or the named target removed with the named survivor as the sole
// online member. The PacemakerCluster read is stable around the Etcd condition
// read and must have a resourceVersion different from the pre-destruction
// baseline. ResourceVersions are opaque and are compared only for equality.
// The two resources are not transactionally atomic, so the classifier also
// requires their structured states to agree. Target absence can still result
// from collector filtering as well as removal; unchanged final recovery checks
// remain responsible for proving restoration of the complete two-node state.
func WaitForPacemakerTargetFailure(
	oc *exutil.CLI,
	targetNode, survivingNode string,
	baselineResourceVersion string,
	timeout time.Duration,
) (PacemakerTargetFailureMode, error) {
	if err := validatePacemakerTargetFailureArgs(targetNode, survivingNode, baselineResourceVersion); err != nil {
		return "", err
	}
	reader := pacemakerTargetFailureReader{
		getPacemakerCluster: func() (*etcdv1.PacemakerCluster, error) { return GetPacemakerCluster(oc) },
		getEtcdOperator:     func() (*operatorv1.Etcd, error) { return getEtcdOperator(oc) },
	}

	mode, err := waitForPacemakerTargetFailure(reader, targetNode, survivingNode, baselineResourceVersion, timeout, healthCheckPollInterval)
	if err != nil {
		DumpHealthCheckDiagnostics(oc, "WaitForPacemakerTargetFailure timeout")
	}
	return mode, err
}

// WaitForPacemakerHealthCheckDegraded polls the etcd operator resource until
// PacemakerHealthCheckDegraded=True with a message containing expectedSubstring.
// Pass an empty expectedSubstring to accept any message.
func WaitForPacemakerHealthCheckDegraded(oc *exutil.CLI, expectedSubstring string, timeout time.Duration) error {
	var lastErr string
	checker := func() (bool, error) {
		etcd, err := getEtcdOperator(oc)
		if err != nil {
			lastErr = fmt.Sprintf("get etcd operator: %v", err)
			framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
			return false, nil
		}

		cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
		if cond == nil {
			lastErr = "condition not found"
			framework.Logf("WaitForPacemakerHealthCheckDegraded: condition not yet present on etcd operator")
			return false, nil
		}

		if cond.Status != operatorv1.ConditionTrue {
			lastErr = describeNotDegradedCondition(cond)
			framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
			return false, nil
		}

		if expectedSubstring != "" && !strings.Contains(cond.Message, expectedSubstring) {
			lastErr = fmt.Sprintf("True but message %q does not contain %q", cond.Message, expectedSubstring)
			framework.Logf("WaitForPacemakerHealthCheckDegraded: %s", lastErr)
			return false, nil
		}

		framework.Logf("PacemakerHealthCheckDegraded=True confirmed (reason=%s, message=%q)", cond.Reason, cond.Message)
		return true, nil
	}

	if err := core.PollUntil(checker, timeout, healthCheckPollInterval, "PacemakerHealthCheckDegraded=True"); err != nil {
		DumpHealthCheckDiagnostics(oc, "WaitForPacemakerHealthCheckDegraded timeout")
		return fmt.Errorf("timed out after %v waiting for PacemakerHealthCheckDegraded=True (last: %s)", timeout, lastErr)
	}
	return nil
}

// WaitForPacemakerHealthCheckCleared polls the etcd operator resource until
// PacemakerHealthCheckDegraded=False. An absent condition is not treated as
// cleared: clearPacemakerDegradedCondition in cluster-etcd-operator always
// writes an explicit False on the controller's first healthy sync, so an
// absent condition means the health check controller never ran at all.
func WaitForPacemakerHealthCheckCleared(oc *exutil.CLI, timeout time.Duration) error {
	var lastErr string
	checker := func() (bool, error) {
		etcd, err := getEtcdOperator(oc)
		if err != nil {
			lastErr = fmt.Sprintf("get etcd operator: %v", err)
			framework.Logf("WaitForPacemakerHealthCheckCleared: %s", lastErr)
			return false, nil
		}

		cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
		if cond == nil {
			lastErr = "condition not yet present on etcd operator"
			framework.Logf("WaitForPacemakerHealthCheckCleared: %s", lastErr)
			return false, nil
		}

		if cond.Status == operatorv1.ConditionFalse {
			framework.Logf("PacemakerHealthCheckDegraded=False confirmed")
			return true, nil
		}

		lastErr = fmt.Sprintf("Status=%s reason=%s message=%q", cond.Status, cond.Reason, cond.Message)
		framework.Logf("WaitForPacemakerHealthCheckCleared: still degraded — %s", lastErr)
		return false, nil
	}

	if err := core.PollUntil(checker, timeout, healthCheckPollInterval, "PacemakerHealthCheckDegraded=False"); err != nil {
		DumpHealthCheckDiagnostics(oc, "WaitForPacemakerHealthCheckCleared timeout")
		return fmt.Errorf("timed out after %v waiting for PacemakerHealthCheckDegraded to clear (last: %s)", timeout, lastErr)
	}
	return nil
}

// PacemakerHealthCheckEventNamespace is where the healthcheck controller's
// library-go event recorder writes events — the operator's own pod namespace
// (openshift-etcd-operator), not the target namespace (openshift-etcd) that
// the status collector uses for its own events (e.g. PacemakerFencingEvent,
// PacemakerFailedResourceAction, PacemakerStatusCollectionError).
const PacemakerHealthCheckEventNamespace = "openshift-etcd-operator"

// eventTime returns the most recent activity timestamp for an event, preferring
// EventTime, then LastTimestamp, then the object's creation timestamp. This is
// used to distinguish freshly-emitted events from stale ones left over from an
// earlier reconcile or a previous test run.
func eventTime(ev *corev1.Event) time.Time {
	if !ev.EventTime.IsZero() {
		return ev.EventTime.Time
	}
	if !ev.LastTimestamp.IsZero() {
		return ev.LastTimestamp.Time
	}
	return ev.CreationTimestamp.Time
}

// WaitForPacemakerEvent polls events in the given namespace until one with
// the given Reason emitted at or after the provided lower bound (since)
// appears. Use PacemakerHealthCheckEventNamespace for healthcheck-controller
// reasons (e.g. PacemakerHealthy, PacemakerClusterInMaintenance,
// PacemakerNodeOffline); use the status collector's target namespace
// (openshift-etcd) for reasons it emits (e.g. PacemakerFencingEvent). The
// since bound prevents a stale event from a prior reconcile or test from
// satisfying the wait.
func WaitForPacemakerEvent(oc *exutil.CLI, namespace, reason string, since time.Time, timeout time.Duration) error {
	checker := func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		events, err := oc.AdminKubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
			FieldSelector: fmt.Sprintf("reason=%s", reason),
		})
		cancel()
		if err != nil {
			framework.Logf("WaitForPacemakerEvent: list events: %v", err)
			return false, nil
		}
		for i := range events.Items {
			ev := &events.Items[i]
			if eventTime(ev).Before(since) {
				continue
			}
			framework.Logf("Found event reason=%s message=%q at %s (baseline %s)",
				ev.Reason, ev.Message, eventTime(ev).Format(time.RFC3339), since.Format(time.RFC3339))
			return true, nil
		}
		return false, nil
	}

	if err := core.PollUntil(checker, timeout, healthCheckPollInterval, fmt.Sprintf("event with reason %q in %s", reason, namespace)); err != nil {
		return fmt.Errorf("timed out after %v waiting for event with reason %q emitted at or after %s in %s",
			timeout, reason, since.Format(time.RFC3339), namespace)
	}
	return nil
}

// ExpectNoPacemakerEventSince performs one event list and returns an error when
// an event with reason was emitted at or after since. Its timestamp filtering
// matches WaitForPacemakerEvent so callers can assert an event did not occur in
// the same bounded interval.
func ExpectNoPacemakerEventSince(oc *exutil.CLI, namespace, reason string, since time.Time) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	events, err := oc.AdminKubeClient().CoreV1().Events(namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fmt.Sprintf("reason=%s", reason),
	})
	if err != nil {
		return fmt.Errorf("list events with reason %q in %s: %w", reason, namespace, err)
	}
	for i := range events.Items {
		ev := &events.Items[i]
		if eventTime(ev).Before(since) {
			continue
		}
		return fmt.Errorf("unexpected event with reason %q at %s in %s", reason, eventTime(ev).Format(time.RFC3339), namespace)
	}
	return nil
}

// IsPacemakerHealthCheckDegraded performs a single check of the etcd operator
// resource and reports whether PacemakerHealthCheckDegraded is currently True,
// along with the condition message. A missing condition is reported as not
// degraded. Unlike WaitForPacemakerHealthCheckDegraded, this does not poll, so
// callers can interleave it with other actions (e.g. re-inducing a failure that
// Pacemaker would otherwise auto-recover before the next status snapshot).
func IsPacemakerHealthCheckDegraded(oc *exutil.CLI) (bool, string, error) {
	etcd, err := getEtcdOperator(oc)
	if err != nil {
		return false, "", fmt.Errorf("get etcd operator: %w", err)
	}

	cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return false, "", nil
	}

	return cond.Status == operatorv1.ConditionTrue, cond.Message, nil
}

// ExpectPacemakerHealthCheckNotDegraded checks the etcd operator resource
// and returns an error if PacemakerHealthCheckDegraded is True.
func ExpectPacemakerHealthCheckNotDegraded(oc *exutil.CLI) error {
	etcd, err := getEtcdOperator(oc)
	if err != nil {
		return fmt.Errorf("get etcd operator: %w", err)
	}

	cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return nil
	}

	if cond.Status == operatorv1.ConditionTrue {
		return fmt.Errorf("PacemakerHealthCheckDegraded=True (reason=%s, message=%q)", cond.Reason, cond.Message)
	}
	return nil
}

// ExpectPacemakerHealthCheckExplicitlyNotDegraded checks that the etcd operator
// explicitly reports PacemakerHealthCheckDegraded=False. A missing or Unknown
// condition means the health check controller has not established a healthy
// baseline and is therefore returned as an error.
func ExpectPacemakerHealthCheckExplicitlyNotDegraded(oc *exutil.CLI) error {
	etcd, err := getEtcdOperator(oc)
	if err != nil {
		return fmt.Errorf("get etcd operator: %w", err)
	}

	cond := v1helpers.FindOperatorCondition(etcd.Status.Conditions, PacemakerHealthCheckDegradedCondition)
	if cond == nil {
		return fmt.Errorf("PacemakerHealthCheckDegraded condition is absent")
	}
	if cond.Status != operatorv1.ConditionFalse {
		return fmt.Errorf("PacemakerHealthCheckDegraded=%s, expected False (reason=%s, message=%q)", cond.Status, cond.Reason, cond.Message)
	}
	return nil
}
