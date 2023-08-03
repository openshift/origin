package pathologicaleventlibrary

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ImagePullRedhatRegEx             = `reason/[a-zA-Z]+ .*Back-off pulling image .*registry.redhat.io`
	RequiredResourcesMissingRegEx    = `reason/RequiredInstallerResourcesMissing secrets: etcd-all-certs-[0-9]+`
	BackoffRestartingFailedRegEx     = `reason/BackOff Back-off restarting failed container`
	ErrorUpdatingEndpointSlicesRegex = `reason/FailedToUpdateEndpointSlices Error updating Endpoint Slices`
	NodeHasNoDiskPressureRegExpStr   = "reason/NodeHasNoDiskPressure.*status is now: NodeHasNoDiskPressure"
	NodeHasSufficientMemoryRegExpStr = "reason/NodeHasSufficientMemory.*status is now: NodeHasSufficientMemory"
	NodeHasSufficientPIDRegExpStr    = "reason/NodeHasSufficientPID.*status is now: NodeHasSufficientPID"

	OvnReadinessRegExpStr                   = `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$)`
	ConsoleReadinessRegExpStr               = `ns/(?P<NS>openshift-console) pod/(?P<POD>console-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>ProbeError) (?P<MSG>Readiness probe error:.* connect: connection refused$)`
	MarketplaceStartupProbeFailureRegExpStr = `ns/(?P<NS>openshift-marketplace) pod/(?P<POD>(community-operators|redhat-operators)-[a-z0-9-]+).*Startup probe failed`

	ImagePullRedhatFlakeThreshold              = 5
	RequiredResourceMissingFlakeThreshold      = 10
	BackoffRestartingFlakeThreshold            = 10
	ErrorUpdatingEndpointSlicesFailedThreshold = -1 // flake only
	ErrorUpdatingEndpointSlicesFlakeThreshold  = 10

	ReadinessFailedMessageRegExpStr           = "reason/ReadinessFailed.*Get.*healthz.*net/http.*request canceled while waiting for connection.*Client.Timeout exceeded"
	ProbeErrorReadinessMessageRegExpStr       = "reason/ProbeError.*Readiness probe error.*Client.Timeout exceeded while awaiting headers"
	ProbeErrorLivenessMessageRegExpStr        = "reason/(ProbeError|Unhealthy).*Liveness probe error.*Client.Timeout exceeded while awaiting headers"
	ProbeErrorConnectionRefusedRegExpStr      = "reason/ProbeError.*Readiness probe error.*connection refused"
	SingleNodeErrorConnectionRefusedRegExpStr = "reason/.*dial tcp.*connection refused"

	ErrorReconcilingNode  = "reason/ErrorReconcilingNode roles/worker .*annotation not found for node"
	FailedScheduling      = "reason/FailedScheduling .*nodes are available.*didn't match Pod's node affinity/selector"
	OperatorStatusChanged = "reason/OperatorStatusChanged Status for clusteroperator/etcd changed.*No unhealthy members found"

	DuplicateEventThreshold           = 20
	DuplicateSingleNodeEventThreshold = 30
	PathologicalMark                  = "pathological/true"
	InterestingMark                   = "interesting/true"
)

var EventCountExtractor = regexp.MustCompile(`(?s)(.*) \((\d+) times\).*`)

var AllowedRepeatedEventPatterns = []*regexp.Regexp{
	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	// PauseNewPods intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	regexp.MustCompile(`ns/e2e-statefulset-[0-9]+ pod/ss2-[0-9] node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: HTTP probe failed with statuscode: 404`),

	// [sig-node] Probing container ***
	// these tests intentionally cause repeated probe failures to ensure good handling
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe failed: `),
	regexp.MustCompile(`ns/e2e-container-probe-[0-9]+ .* probe warning: `),

	// Kubectl Port forwarding ***
	// The same pod name is used many times for all these tests with a tight readiness check to make the tests fast.
	// This results in hundreds of events while the pod isn't ready.
	regexp.MustCompile(`ns/e2e-port-forwarding-[0-9]+ pod/pfpod node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed:`),

	// should not start app containers if init containers fail on a RestartAlways pod
	// the init container intentionally fails to start
	regexp.MustCompile(`ns/e2e-init-container-[0-9]+ pod/pod-init-[a-z0-9.-]+ node/[a-z0-9.-]+ - reason/BackOff Back-off restarting failed container`),

	// TestAllowedSCCViaRBAC and TestPodUpdateSCCEnforcement
	// The pod is shaped to intentionally not be scheduled.  Looks like an artifact of the old integration testing.
	regexp.MustCompile(`ns/e2e-test-scc-[a-z0-9]+ pod/.* - reason/FailedScheduling.*`),

	// Security Context ** should not run with an explicit root user ID
	// Security Context ** should not run without a specified user ID
	// This container should never run
	regexp.MustCompile(`ns/e2e-security-context-test-[0-9]+ pod/.*-root-uid node/[a-z0-9.-]+ - reason/Failed Error: container's runAsUser breaks non-root policy.*"`),

	// PersistentVolumes-local tests should not run the pod when there is a volume node
	// affinity and node selector conflicts.
	regexp.MustCompile(`ns/e2e-persistent-local-volumes-test-[0-9]+ pod/pod-[a-z0-9.-]+ reason/FailedScheduling`),

	// various DeploymentConfig tests trigger this by canceling multiple rollouts
	regexp.MustCompile(`reason/DeploymentAwaitingCancellation Deployment of version [0-9]+ awaiting cancellation of older running deployments`),

	// this image is used specifically to be one that cannot be pulled in our tests
	regexp.MustCompile(`.*reason/BackOff Back-off pulling image "webserver:404"`),

	// If image pulls in e2e namespaces fail catastrophically we'd expect them to lead to test failures
	// We are deliberately not ignoring image pull failures for core component namespaces
	regexp.MustCompile(`ns/e2e-.* reason/BackOff Back-off pulling image`),

	// promtail crashlooping as its being started by sideloading manifests.  per @vrutkovs
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*Readiness probe"),

	// Related to known bug below, but we do not need to report on loki: https://bugzilla.redhat.com/show_bug.cgi?id=1986370
	regexp.MustCompile("ns/openshift-e2e-loki pod/loki-promtail.*reason/NetworkNotReady"),

	// kube-apiserver guard probe failing due to kube-apiserver operands getting rolled out
	// multiple times during the bootstrapping phase of a cluster installation
	regexp.MustCompile("ns/openshift-kube-apiserver pod/kube-apiserver-guard.*ProbeError Readiness probe error"),
	// the same thing happens for kube-controller-manager and kube-scheduler
	regexp.MustCompile("ns/openshift-kube-controller-manager pod/kube-controller-manager-guard.*ProbeError Readiness probe error"),
	regexp.MustCompile("ns/openshift-kube-scheduler pod/kube-scheduler-guard.*ProbeError Readiness probe error"),

	// this is the less specific even sent by the kubelet when a probe was executed successfully but returned false
	// we ignore this event because openshift has a patch in patch_prober that sends a more specific event about
	// readiness failures in openshift-* namespaces.  We will catch the more specific ProbeError events.
	regexp.MustCompile("Unhealthy Readiness probe failed"),
	// readiness probe errors during pod termination are expected, so we do not fail on them.
	regexp.MustCompile("TerminatingPodProbeError"),

	// we have a separate test for this
	regexp.MustCompile(OvnReadinessRegExpStr),

	// Separated out in testBackoffPullingRegistryRedhatImage
	regexp.MustCompile(ImagePullRedhatRegEx),

	// Separated out in testRequiredInstallerResourcesMissing
	regexp.MustCompile(RequiredResourcesMissingRegEx),

	// Separated out in testBackoffStartingFailedContainer
	regexp.MustCompile(BackoffRestartingFailedRegEx),

	// Separated out in testErrorUpdatingEndpointSlices
	regexp.MustCompile(ErrorUpdatingEndpointSlicesRegex),

	// If you see this error, it means enough was working to get this event which implies enough retries happened to allow initial openshift
	// installation to succeed. Hence, we can ignore it.
	regexp.MustCompile(`reason/FailedCreate .* error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient .* capacity in the Availability Zone you requested`),

	// Separated out in testNodeHasNoDiskPressure
	regexp.MustCompile(NodeHasNoDiskPressureRegExpStr),

	// Separated out in testNodeHasSufficientMemory
	regexp.MustCompile(NodeHasSufficientMemoryRegExpStr),

	// Separated out in testNodeHasSufficientPID
	regexp.MustCompile(NodeHasSufficientPIDRegExpStr),

	// Separated out in testMarketplaceStartupProbeFailure
	regexp.MustCompile(MarketplaceStartupProbeFailureRegExpStr),
}

// AllowedUpgradeRepeatedEventPatterns are patterns of events that we should only allow during upgrades, not during normal execution.
var AllowedUpgradeRepeatedEventPatterns = []*regexp.Regexp{
	// Operators that use library-go can report about multiple versions during upgrades.
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-apiserver-operator deployment/kube-apiserver-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-controller-manager-operator deployment/kube-controller-manager-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),
	regexp.MustCompile(`ns/openshift-kube-scheduler-operator deployment/openshift-kube-scheduler-operator - reason/MultipleVersions multiple versions found, probably in transition: .*`),

	// etcd-quorum-guard can fail during upgrades.
	regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
	// etcd can have unhealthy members during an upgrade
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/UnhealthyEtcdMember unhealthy members: .*`),
	// etcd-operator began to version etcd-endpoints configmap in 4.10 as part of static-pod-resource. During upgrade existing revisions will not contain the resource.
	// The condition reconciles with the next revision which the result of the upgrade. TODO(hexfusion) remove in 4.11
	regexp.MustCompile(`ns/openshift-etcd-operator deployment/etcd-operator - reason/RequiredInstallerResourcesMissing configmaps: etcd-endpoints-[0-9]+`),
	// There is a separate test to catch this specific case
	regexp.MustCompile(RequiredResourcesMissingRegEx),

	// Separated out in testMarketplaceStartupProbeFailure
	regexp.MustCompile(MarketplaceStartupProbeFailureRegExpStr),
}

type KnownProblem struct {
	Regexp *regexp.Regexp
	BZ     string

	// Platform limits the exception to a specific OpenShift platform.
	Platform *v1.PlatformType

	// Topology limits the exception to a specific topology (e.g. single replica)
	Topology *v1.TopologyMode

	// TestSuite limits the exception to a specific test suite (e.g. openshift/builds)
	TestSuite *string
}

var KnownEventsBugs = []KnownProblem{
	{
		Regexp: regexp.MustCompile(`ns/openshift-multus pod/network-metrics-daemon-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-e2e-loki pod/loki-promtail-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-network-diagnostics pod/network-check-target-[a-z0-9]+ node/[a-z0-9.-]+ - reason/NetworkNotReady network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file in /etc/kubernetes/cni/net\.d/\. Has your network provider started\?`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1986370",
	},
	{
		Regexp: regexp.MustCompile(`ns/.* service/.* - reason/FailedToDeleteOVNLoadBalancer .*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1990631",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*horizontalpodautoscaler.*failed to get cpu utilization: unable to get metrics for resource cpu: no metrics returned from resource metrics API.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993985",
	},
	{
		Regexp: regexp.MustCompile(`ns/.*unable to ensure pod container exists: failed to create container.*slice already exists.*`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=1993980",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-quorum-guard-[a-z0-9-]+ node/[a-z0-9.-]+ - reason/Unhealthy Readiness probe failed: `),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2000234",
	},
	{
		Regexp: regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/.* - reason/ProbeError Readiness probe error: .* connect: connection refused`),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2075204",
	},
	{
		Regexp: regexp.MustCompile("ns/openshift-etcd-operator namespace/openshift-etcd-operator -.*rpc error: code = Canceled desc = grpc: the client connection is closing.*"),
		BZ:     "https://bugzilla.redhat.com/show_bug.cgi?id=2006975",
	},
	{
		Regexp: regexp.MustCompile("reason/TopologyAwareHintsDisabled"),
		BZ:     "https://issues.redhat.com/browse/OCPBUGS-13366",
	},
	{
		Regexp:   regexp.MustCompile("ns/.*reason/.*APICheckFailed.*503.*"),
		BZ:       "https://bugzilla.redhat.com/show_bug.cgi?id=2017435",
		Topology: TopologyPointer(v1.SingleReplicaTopologyMode),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-route-controller-manager deployment/route-controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set route-controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: StringPointer("openshift/build"),
	},
	// builds tests trigger many changes in the config which creates new rollouts -> event for each pod
	// working as intended (not a bug) and needs to be tolerated
	{
		Regexp:    regexp.MustCompile(`ns/openshift-controller-manager deployment/controller-manager - reason/ScalingReplicaSet \(combined from similar events\): Scaled (down|up) replica set controller-manager-[a-z0-9-]+ to [0-9]+`),
		TestSuite: StringPointer("openshift/build"),
	},
	//{ TODO this should only be skipped for single-node
	//	name:    "single=node-storage",
	//  BZ: https://bugzilla.redhat.com/show_bug.cgi?id=1990662
	//	message: "ns/openshift-cluster-csi-drivers pod/aws-ebs-csi-driver-controller-66469455cd-2thfv node/ip-10-0-161-38.us-east-2.compute.internal - reason/BackOff Back-off restarting failed container",
	//},
	{
		Regexp: regexp.MustCompile(`reason/ErrorReconcilingNode.*annotation not found for node.*macAddress annotation not found for node`),
		BZ:     "https://issues.redhat.com/browse/OCPBUGS-13400",
	},
}

// IsRepeatedEventOKFunc takes a monitorEvent as input and returns true if the repeated event is OK.
// This commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// Use this to handle cases where, "if X is true, then the repeated event is ok".
type IsRepeatedEventOKFunc func(monitorEvent monitorapi.Interval, kubeClientConfig *rest.Config, times int) (bool, error)

var AllowedRepeatedEventFns = []IsRepeatedEventOKFunc{
	isConsoleReadinessDuringInstallation,
	isConfigOperatorReadinessFailed,
	isConfigOperatorProbeErrorReadinessFailed,
	isConfigOperatorProbeErrorLivenessFailed,
	isOauthApiserverProbeErrorReadinessFailed,
	isOauthApiserverProbeErrorLivenessFailed,
	isOauthApiserverProbeErrorConnectionRefusedFailed,
	isErrorReconcilingNode,
	isFailedScheduling,
	isOperatorStatusChanged,
}

var AllowedSingleNodeRepeatedEventFns = []IsRepeatedEventOKFunc{
	isConnectionRefusedOnSingleNode,
}

func TopologyPointer(topology v1.TopologyMode) *v1.TopologyMode {
	return &topology
}

func PlatformPointer(platform v1.PlatformType) *v1.PlatformType {
	return &platform
}

func StringPointer(testSuite string) *string {
	return &testSuite
}

// isConsoleReadinessDuringInstallation returns true if the event is for console readiness and it happens during the
// initial installation of the cluster.
// we're looking for something like
// > ns/openshift-console pod/console-7c6f797fd9-5m94j node/ip-10-0-158-106.us-west-2.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.129.0.49:8443/health": dial tcp 10.129.0.49:8443: connect: connection refused
// with a firstTimestamp before the cluster completed the initial installation
func isConsoleReadinessDuringInstallation(monitorEvent monitorapi.Interval, kubeClientConfig *rest.Config, _ int) (bool, error) {
	if !strings.Contains(monitorEvent.Locator, "ns/openshift-console") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "pod/console-") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "Readiness probe") {
		return false, nil
	}
	if !strings.Contains(monitorEvent.Locator, "connect: connection refused") {
		return false, nil
	}

	regExp := regexp.MustCompile(ConsoleReadinessRegExpStr)
	// if the readiness probe failure for this pod happened AFTER the initial installation was complete,
	// then this probe failure is unexpected and should fail.
	return IsEventDuringInstallation(monitorEvent, kubeClientConfig, regExp)
}

// isConfigOperatorReadinessFailed returns true if the event matches a readinessFailed error that timed out
// in the openshift-config-operator.
// like this:
// ...ReadinessFailed Get \"https://10.130.0.16:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorReadinessFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ReadinessFailedMessageRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isConfigOperatorProbeErrorReadinessFailed returns true if the event matches a ProbeError Readiness Probe message
// in the openshift-config-operator.
// like this:
// reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorProbeErrorReadinessFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ProbeErrorReadinessMessageRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isConfigOperatorProbeErrorLivenessFailed returns true if the event matches a ProbeError Liveness Probe message
// in the openshift-config-operator.
// like this:
// ...reason/ProbeError Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
func isConfigOperatorProbeErrorLivenessFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ProbeErrorLivenessMessageRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-config-operator", regExp), nil
}

// isOauthApiserverProbeErrorReadinessFailed returns true if the event matches a ProbeError Readiness Probe message
// in the openshift-oauth-operator.
// like this:
// ...ns/openshift-oauth-apiserver pod/apiserver-65fd7ffc59-bt5sf node/q72hs3bx-ac890-4pxpm-master-2 - reason/ProbeError Readiness probe error: Get "https://10.129.0.8:8443/readyz": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
func isOauthApiserverProbeErrorReadinessFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ProbeErrorReadinessMessageRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// isOauthApiserverProbeErrorLivenessFailed returns true if the event matches a ProbeError Liveness Probe message
// in the openshift-oauth-operator.
// like this:
// ...reason/ProbeError Liveness probe error: Get "https://10.130.0.68:8443/healthz": net/http: request canceled (Client.Timeout exceeded while awaiting headers)
func isOauthApiserverProbeErrorLivenessFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ProbeErrorLivenessMessageRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// isOauthApiserverProbeErrorConnectionRefusedFailed returns true if the event matches a ProbeError Readiness Probe connection refused message
// in the openshift-oauth-operator.
// like this:
// ...ns/openshift-oauth-apiserver pod/apiserver-647fc6c7bf-s8b4h node/ip-10-0-150-209.us-west-1.compute.internal - reason/ProbeError Readiness probe error: Get "https://10.128.0.38:8443/readyz": dial tcp 10.128.0.38:8443: connect: connection refused
func isOauthApiserverProbeErrorConnectionRefusedFailed(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(ProbeErrorConnectionRefusedRegExpStr)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-oauth-apiserver", regExp), nil
}

// reason/ErrorReconcilingNode roles/worker [k8s.ovn.org/node-chassis-id annotation not found for node ci-op-nzi4gt1b-3efb3-ggmhb-worker-centralus2-jzx86, macAddress annotation not found for node "ci-op-nzi4gt1b-3efb3-ggmhb-worker-centralus2-jzx86" , k8s.ovn.org/l3-gateway-config annotation not found for node "ci-op-nzi4gt1b-3efb3-ggmhb-worker-centralus2-jzx86"]
func isErrorReconcilingNode(monitorEvent monitorapi.Interval, _ *rest.Config, count int) (bool, error) {
	regExp := regexp.MustCompile(ErrorReconcilingNode)
	return regExp.MatchString(monitorEvent.String()) && count < DuplicateSingleNodeEventThreshold, nil
}

// reason/FailedScheduling 0/6 nodes are available: 2 node(s) didn't match Pod's node affinity/selector, 2 node(s) didn't match pod anti-affinity rules, 2 node(s) were unschedulable. preemption: 0/6 nodes are available: 2 node(s) didn't match pod anti-affinity rules, 4 Preemption is not helpful for scheduling..
func isFailedScheduling(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(FailedScheduling)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-route-controller-manager", regExp), nil
}

// reason/OperatorStatusChanged Status for clusteroperator/etcd changed: Degraded message changed from "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy" to "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: No unhealthy members found"
func isOperatorStatusChanged(monitorEvent monitorapi.Interval, _ *rest.Config, _ int) (bool, error) {
	regExp := regexp.MustCompile(OperatorStatusChanged)
	return IsOperatorMatchRegexMessage(monitorEvent, "openshift-etcd", regExp), nil
}

// isConnectionRefusedOnSingleNode returns true if the event matched has a connection refused message for single node events and is with in threshold.
func isConnectionRefusedOnSingleNode(monitorEvent monitorapi.Interval, _ *rest.Config, count int) (bool, error) {
	regExp := regexp.MustCompile(SingleNodeErrorConnectionRefusedRegExpStr)
	return regExp.MatchString(monitorEvent.String()) && count < DuplicateSingleNodeEventThreshold, nil
}

// IsOperatorMatchRegexMessage returns true if this monitorEvent is for the operator identified by the operatorName
// and its message matches the given regex.
func IsOperatorMatchRegexMessage(monitorEvent monitorapi.Interval, operatorName string, regExp *regexp.Regexp) bool {
	locatorParts := monitorapi.LocatorParts(monitorEvent.Locator)
	if ns, ok := locatorParts["ns"]; ok {
		if ns != operatorName {
			return false
		}
	}
	if pod, ok := locatorParts["pod"]; ok {
		if !strings.HasPrefix(pod, operatorName) {
			return false
		}
	}
	if !regExp.MatchString(monitorEvent.Message) {
		return false
	}
	return true
}

// isEventDuringInstallation returns true if the monitorEvent represents a real event that happened after installation.
// regExp defines the pattern of the monitorEvent message. Named match is used in the pattern using `(?P<>)`. The names are placed inside <>. See example below
// `ns/(?P<NS>openshift-ovn-kubernetes) pod/(?P<POD>ovnkube-node-[a-z0-9-]+) node/(?P<NODE>[a-z0-9.-]+) - reason/(?P<REASON>Unhealthy) (?P<MSG>Readiness probe failed:.*$`
func IsEventDuringInstallation(monitorEvent monitorapi.Interval, kubeClientConfig *rest.Config, regExp *regexp.Regexp) (bool, error) {
	if kubeClientConfig == nil {
		// default to OK
		return true, nil
	}
	installCompletionTime := getInstallCompletionTime(kubeClientConfig)
	if installCompletionTime == nil {
		return true, nil
	}

	message := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)
	namespace, pod, _, reason, msg, err := getMatchedElementsFromMonitorEventMsg(regExp, message)
	if err != nil {
		return false, err
	}
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return true, nil
	}
	kubeEvents, err := kubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return true, nil
	}
	for _, event := range kubeEvents.Items {
		if event.Related == nil ||
			event.Related.Name != pod ||
			event.Reason != reason ||
			!strings.Contains(event.Message, msg) {
			continue
		}

		if event.FirstTimestamp.After(installCompletionTime.Time) {
			return false, nil
		}
	}
	return true, nil
}
func getInstallCompletionTime(kubeClientConfig *rest.Config) *metav1.Time {
	configClient, err := configclient.NewForConfig(kubeClientConfig)
	if err != nil {
		return nil
	}
	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(context.TODO(), "version", metav1.GetOptions{})
	if err != nil {
		return nil
	}
	if len(clusterVersion.Status.History) == 0 {
		return nil
	}
	return clusterVersion.Status.History[len(clusterVersion.Status.History)-1].CompletionTime
}

func getMatchedElementsFromMonitorEventMsg(regExp *regexp.Regexp, message string) (string, string, string, string, string, error) {
	var namespace, pod, node, reason, msg string
	if !regExp.MatchString(message) {
		return namespace, pod, node, reason, msg, errors.New("regex match error")
	}
	subMatches := regExp.FindStringSubmatch(message)
	subNames := regExp.SubexpNames()
	for i, name := range subNames {
		switch name {
		case "NS":
			namespace = subMatches[i]
		case "POD":
			pod = subMatches[i]
		case "NODE":
			node = subMatches[i]
		case "REASON":
			reason = subMatches[i]
		case "MSG":
			msg = subMatches[i]
		}
	}
	if len(namespace) == 0 ||
		len(pod) == 0 ||
		len(node) == 0 ||
		len(msg) == 0 {
		return namespace, pod, node, reason, msg, fmt.Errorf("regex match expects non-empty elements, got namespace: %s, pod: %s, node: %s, msg: %s", namespace, pod, node, msg)
	}
	return namespace, pod, node, reason, msg, nil
}
