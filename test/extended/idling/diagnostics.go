package idling

import (
	"context"
	"fmt"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	appsv1 "github.com/openshift/api/apps/v1"
	"github.com/openshift/library-go/pkg/apps/appsutil"

	exutil "github.com/openshift/origin/test/extended/util"
)

// The pre-idle endpoint wait in the idling setup occasionally times out, and the
// spec then fails with nothing but "timed out waiting for the condition". The
// framework still dumps the namespace, including the deployer log, at teardown,
// but nothing records what the rollout and the service endpoints looked like at
// the moment the wait gave up, which is what separates a rollout that never
// started from one that failed and one that merely lagged.
//
// The helpers below log that snapshot, on the failure path only. Every value is
// an integer or a token from a fixed vocabulary, so the snapshot cannot carry
// object names, addresses or API error text. Each group of values is prefixed by
// the outcome of the read that produced it, so a read that failed or never
// happened is never mistaken for an observed zero.

const (
	// deploymentConfigResource is the resource key `oc create -o name` reports for
	// the DeploymentConfig created by the DeploymentConfig idling fixtures. The
	// other fixtures do not create one, which scopes the rollout reads to the
	// fixtures they can say anything about.
	deploymentConfigResource = "deploymentconfig.apps.openshift.io"

	// diagnosticsBudget bounds the time all of the reads together may take, so a
	// failing setup never becomes a much slower failing setup.
	diagnosticsBudget = 15 * time.Second

	// deployerContainerName is the name OpenShift gives the deployer pod's container.
	deployerContainerName = "deployment"
)

// The deployer container's state is reported as one of these tokens.
const (
	containerStateNone       = "none"
	containerStateWaiting    = "waiting"
	containerStateRunning    = "running"
	containerStateTerminated = "terminated"
)

// readOutcome records why a group of values is present or missing.
type readOutcome string

const (
	readOK          readOutcome = "ok"
	readNotFound    readOutcome = "notfound"
	readUnavailable readOutcome = "unavailable"
	readSkipped     readOutcome = "skipped"
)

// readOutcomeFor folds a read result into an outcome token; the error itself is
// deliberately discarded. An object that is nil without an error counts as
// unavailable, so a read that produced nothing is never rendered as zeroes.
func readOutcomeFor(haveObject bool, err error) readOutcome {
	switch {
	case kapierrors.IsNotFound(err):
		return readNotFound
	case err != nil, !haveObject:
		return readUnavailable
	default:
		return readOK
	}
}

// deploymentConfigGetter is the one DeploymentConfig read used here, satisfied by
// the generated typed client.
type deploymentConfigGetter interface {
	Get(ctx context.Context, name string, opts metav1.GetOptions) (*appsv1.DeploymentConfig, error)
}

// idlingDiagnosticsClients reads with the test's own clients, in the test's own
// namespace, and asks for no permission the idling tests do not already hold.
type idlingDiagnosticsClients struct {
	kube      kubernetes.Interface
	namespace string
	// deploymentConfigs is nil for the fixtures that create no DeploymentConfig.
	deploymentConfigs deploymentConfigGetter
}

// deploymentConfigNameFor returns the DeploymentConfig this fixture created, or
// the empty string for the fixtures that create none.
func deploymentConfigNameFor(resources map[string][]string) string {
	if names := resources[deploymentConfigResource]; len(names) > 0 {
		return names[0]
	}
	return ""
}

type deploymentConfigSnapshot struct {
	read                readOutcome
	latestVersion       int64
	specReplicas        int32
	replicas            int32
	readyReplicas       int32
	availableReplicas   int32
	updatedReplicas     int32
	unavailableReplicas int32
	// conditions holds the normalized status of the allowlisted condition types
	// the object reports.
	conditions map[appsv1.DeploymentConditionType]string
}

type replicationControllerSnapshot struct {
	read              readOutcome
	phase             string
	specReplicas      int32
	replicas          int32
	readyReplicas     int32
	availableReplicas int32
}

type deployerPodSnapshot struct {
	read             readOutcome
	phase            string
	containerState   string
	terminatedReason string
	exitCode         int32
}

type endpointsSnapshot struct {
	read              readOutcome
	subsets           int
	readyAddresses    int
	notReadyAddresses int
}

type idlingSetupDiagnostics struct {
	deploymentConfig      deploymentConfigSnapshot
	replicationController replicationControllerSnapshot
	deployerPod           deployerPodSnapshot
	endpoints             endpointsSnapshot
}

// allowedDeploymentConditionTypes is the fixed, ordered set of DeploymentConfig
// condition types reported. Ordering keeps the summary deterministic.
var allowedDeploymentConditionTypes = []appsv1.DeploymentConditionType{
	appsv1.DeploymentAvailable,
	appsv1.DeploymentProgressing,
	appsv1.DeploymentReplicaFailure,
}

// allowedDeploymentPhases are the rollout phases OpenShift records on a deployer
// ReplicationController. Anything else is reported as "other".
var allowedDeploymentPhases = map[string]bool{
	string(appsv1.DeploymentStatusNew):      true,
	string(appsv1.DeploymentStatusPending):  true,
	string(appsv1.DeploymentStatusRunning):  true,
	string(appsv1.DeploymentStatusComplete): true,
	string(appsv1.DeploymentStatusFailed):   true,
}

// allowedPodPhases are the pod phases reported verbatim.
var allowedPodPhases = map[corev1.PodPhase]bool{
	corev1.PodPending:   true,
	corev1.PodRunning:   true,
	corev1.PodSucceeded: true,
	corev1.PodFailed:    true,
	corev1.PodUnknown:   true,
}

// allowedTerminationReasons are the container termination reasons reported
// verbatim; anything else becomes "other", since the field can carry arbitrary
// runtime text.
var allowedTerminationReasons = map[string]bool{
	"Completed":              true,
	"Error":                  true,
	"OOMKilled":              true,
	"DeadlineExceeded":       true,
	"Evicted":                true,
	"ContainerStatusUnknown": true,
}

func allowedValue(allowed bool, value string) string {
	if allowed {
		return value
	}
	return "other"
}

func summarizeDeploymentConfig(dc *appsv1.DeploymentConfig) deploymentConfigSnapshot {
	snapshot := deploymentConfigSnapshot{
		read:                readOK,
		latestVersion:       dc.Status.LatestVersion,
		specReplicas:        dc.Spec.Replicas,
		replicas:            dc.Status.Replicas,
		readyReplicas:       dc.Status.ReadyReplicas,
		availableReplicas:   dc.Status.AvailableReplicas,
		updatedReplicas:     dc.Status.UpdatedReplicas,
		unavailableReplicas: dc.Status.UnavailableReplicas,
		conditions:          map[appsv1.DeploymentConditionType]string{},
	}
	for _, condition := range dc.Status.Conditions {
		for _, allowed := range allowedDeploymentConditionTypes {
			if condition.Type == allowed {
				status := condition.Status
				known := status == corev1.ConditionTrue || status == corev1.ConditionFalse || status == corev1.ConditionUnknown
				snapshot.conditions[allowed] = allowedValue(known, string(status))
				break
			}
		}
	}
	return snapshot
}

func (d deploymentConfigSnapshot) fields() []string {
	fields := []string{
		fmt.Sprintf("dc.latestVersion=%d", d.latestVersion),
		fmt.Sprintf("dc.spec.replicas=%d", d.specReplicas),
		fmt.Sprintf("dc.status.replicas=%d", d.replicas),
		fmt.Sprintf("dc.status.ready=%d", d.readyReplicas),
		fmt.Sprintf("dc.status.available=%d", d.availableReplicas),
		fmt.Sprintf("dc.status.updated=%d", d.updatedReplicas),
		fmt.Sprintf("dc.status.unavailable=%d", d.unavailableReplicas),
	}
	for _, conditionType := range allowedDeploymentConditionTypes {
		status, ok := d.conditions[conditionType]
		if !ok {
			status = "absent"
		}
		fields = append(fields, fmt.Sprintf("dc.condition.%s=%s", conditionType, status))
	}
	return fields
}

func summarizeReplicationController(rc *corev1.ReplicationController) replicationControllerSnapshot {
	snapshot := replicationControllerSnapshot{
		read:              readOK,
		phase:             "absent",
		replicas:          rc.Status.Replicas,
		readyReplicas:     rc.Status.ReadyReplicas,
		availableReplicas: rc.Status.AvailableReplicas,
	}
	if rc.Spec.Replicas != nil {
		snapshot.specReplicas = *rc.Spec.Replicas
	}
	if phase, ok := rc.Annotations[appsv1.DeploymentStatusAnnotation]; ok {
		snapshot.phase = allowedValue(allowedDeploymentPhases[phase], phase)
	}
	return snapshot
}

func (r replicationControllerSnapshot) fields() []string {
	return []string{
		fmt.Sprintf("rc.phase=%s", r.phase),
		fmt.Sprintf("rc.spec.replicas=%d", r.specReplicas),
		fmt.Sprintf("rc.status.replicas=%d", r.replicas),
		fmt.Sprintf("rc.status.ready=%d", r.readyReplicas),
		fmt.Sprintf("rc.status.available=%d", r.availableReplicas),
	}
}

func summarizeDeployerPod(pod *corev1.Pod) deployerPodSnapshot {
	snapshot := deployerPodSnapshot{
		read:           readOK,
		phase:          allowedValue(allowedPodPhases[pod.Status.Phase], string(pod.Status.Phase)),
		containerState: containerStateNone,
	}
	for _, status := range pod.Status.ContainerStatuses {
		if status.Name != deployerContainerName {
			continue
		}
		switch {
		case status.State.Terminated != nil:
			terminated := status.State.Terminated
			snapshot.containerState = containerStateTerminated
			snapshot.exitCode = terminated.ExitCode
			snapshot.terminatedReason = "none"
			if terminated.Reason != "" {
				snapshot.terminatedReason = allowedValue(allowedTerminationReasons[terminated.Reason], terminated.Reason)
			}
		case status.State.Running != nil:
			snapshot.containerState = containerStateRunning
		case status.State.Waiting != nil:
			snapshot.containerState = containerStateWaiting
		}
		break
	}
	return snapshot
}

func (p deployerPodSnapshot) fields() []string {
	fields := []string{
		fmt.Sprintf("deployer.phase=%s", p.phase),
		fmt.Sprintf("deployer.container.state=%s", p.containerState),
	}
	if p.containerState == containerStateTerminated {
		fields = append(fields,
			fmt.Sprintf("deployer.container.reason=%s", p.terminatedReason),
			fmt.Sprintf("deployer.container.exitCode=%d", p.exitCode),
		)
	}
	return fields
}

func summarizeEndpoints(endpoints *corev1.Endpoints) endpointsSnapshot {
	snapshot := endpointsSnapshot{read: readOK, subsets: len(endpoints.Subsets)}
	for _, subset := range endpoints.Subsets {
		snapshot.readyAddresses += len(subset.Addresses)
		snapshot.notReadyAddresses += len(subset.NotReadyAddresses)
	}
	return snapshot
}

// fields reports address counts summed over every subset, which is why they are
// named totals: the wait itself only looks at the first subset.
func (e endpointsSnapshot) fields() []string {
	return []string{
		fmt.Sprintf("endpoints.subsets=%d", e.subsets),
		fmt.Sprintf("endpoints.readyTotal=%d", e.readyAddresses),
		fmt.Sprintf("endpoints.notReadyTotal=%d", e.notReadyAddresses),
	}
}

// collectIdlingSetupDiagnostics reads the state behind a failed wait. Every read
// is best effort: one that fails is recorded as an outcome, and the reads that do
// not depend on it still run.
func collectIdlingSetupDiagnostics(ctx context.Context, clients idlingDiagnosticsClients, serviceName, deploymentConfigName string) idlingSetupDiagnostics {
	diagnostics := idlingSetupDiagnostics{
		deploymentConfig:      deploymentConfigSnapshot{read: readSkipped},
		replicationController: replicationControllerSnapshot{read: readSkipped},
		deployerPod:           deployerPodSnapshot{read: readSkipped},
	}

	// The Endpoints object the wait itself polled.
	endpoints, err := clients.kube.CoreV1().Endpoints(clients.namespace).Get(ctx, serviceName, metav1.GetOptions{})
	if outcome := readOutcomeFor(endpoints != nil, err); outcome != readOK {
		diagnostics.endpoints.read = outcome
	} else {
		diagnostics.endpoints = summarizeEndpoints(endpoints)
	}

	// Without the DeploymentConfig there is no way to name its latest
	// ReplicationController or deployer pod, so those reads stay skipped.
	if deploymentConfigName == "" || clients.deploymentConfigs == nil {
		return diagnostics
	}
	dc, err := clients.deploymentConfigs.Get(ctx, deploymentConfigName, metav1.GetOptions{})
	if outcome := readOutcomeFor(dc != nil, err); outcome != readOK {
		diagnostics.deploymentConfig.read = outcome
		return diagnostics
	}
	diagnostics.deploymentConfig = summarizeDeploymentConfig(dc)
	if dc.Status.LatestVersion == 0 {
		return diagnostics
	}

	// The deployer ReplicationController and its pod are both named after the
	// DeploymentConfig and its latest version, so a failed ReplicationController
	// read still leaves the deployer pod addressable.
	rcName := appsutil.LatestDeploymentNameForConfigAndVersion(deploymentConfigName, dc.Status.LatestVersion)
	rc, err := clients.kube.CoreV1().ReplicationControllers(clients.namespace).Get(ctx, rcName, metav1.GetOptions{})
	if outcome := readOutcomeFor(rc != nil, err); outcome != readOK {
		diagnostics.replicationController.read = outcome
	} else {
		diagnostics.replicationController = summarizeReplicationController(rc)
	}

	podName := appsutil.DeployerPodNameForDeployment(rcName)
	pod, err := clients.kube.CoreV1().Pods(clients.namespace).Get(ctx, podName, metav1.GetOptions{})
	if outcome := readOutcomeFor(pod != nil, err); outcome != readOK {
		diagnostics.deployerPod.read = outcome
		return diagnostics
	}
	diagnostics.deployerPod = summarizeDeployerPod(pod)

	return diagnostics
}

// appendSection appends "<prefix>.read=<outcome>" and, only for a read that
// succeeded, the fields body reports. A read that failed or never happened
// contributes its outcome and nothing else.
func appendSection(fields []string, prefix string, read readOutcome, body func() []string) []string {
	fields = append(fields, fmt.Sprintf("%s.read=%s", prefix, read))
	if read != readOK {
		return fields
	}
	return append(fields, body()...)
}

// summary renders the snapshot as space separated key=value pairs.
func (d idlingSetupDiagnostics) summary() string {
	var fields []string
	fields = appendSection(fields, "dc", d.deploymentConfig.read, d.deploymentConfig.fields)
	fields = appendSection(fields, "rc", d.replicationController.read, d.replicationController.fields)
	fields = appendSection(fields, "deployer", d.deployerPod.read, d.deployerPod.fields)
	fields = appendSection(fields, "endpoints", d.endpoints.read, d.endpoints.fields)
	return strings.Join(fields, " ")
}

// idlingSetupDiagnosticsOnFailure returns the summary for a failed pre-idle
// endpoint wait. When the wait succeeded it reports false, having built no
// client and issued no read.
func idlingSetupDiagnosticsOnFailure(ctx context.Context, newClients func() idlingDiagnosticsClients, waitErr error, serviceName, deploymentConfigName string) (string, bool) {
	if waitErr == nil {
		return "", false
	}
	ctx, cancel := context.WithTimeout(ctx, diagnosticsBudget)
	defer cancel()
	return collectIdlingSetupDiagnostics(ctx, newClients(), serviceName, deploymentConfigName).summary(), true
}

// logIdlingSetupDiagnostics logs the snapshot when the pre-idle endpoint wait
// failed, and does nothing otherwise. The caller still owns the assertion on
// waitErr.
func logIdlingSetupDiagnostics(ctx context.Context, oc *exutil.CLI, waitErr error, serviceName string, resources map[string][]string) {
	deploymentConfigName := deploymentConfigNameFor(resources)
	newClients := func() idlingDiagnosticsClients {
		clients := idlingDiagnosticsClients{kube: oc.KubeClient(), namespace: oc.Namespace()}
		if deploymentConfigName != "" {
			clients.deploymentConfigs = oc.AppsClient().AppsV1().DeploymentConfigs(oc.Namespace())
		}
		return clients
	}

	summary, collected := idlingSetupDiagnosticsOnFailure(ctx, newClients, waitErr, serviceName, deploymentConfigName)
	if !collected {
		return
	}
	e2e.Logf("idling setup diagnostics (pre-idle endpoint wait failed): %s", summary)
}
