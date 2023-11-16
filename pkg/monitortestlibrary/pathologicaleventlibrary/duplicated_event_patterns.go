package pathologicaleventlibrary

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

const (
	ImagePullRedhatFlakeThreshold              = 5
	RequiredResourceMissingFlakeThreshold      = 10
	BackoffRestartingFlakeThreshold            = 10
	ErrorUpdatingEndpointSlicesFailedThreshold = -1 // flake only
	ErrorUpdatingEndpointSlicesFlakeThreshold  = 10

	DuplicateEventThreshold = 20
	PathologicalMark        = "pathological/true"
	InterestingMark         = "interesting/true"
)

// IsRepeatedEventOKFunc takes a monitorEvent as input and returns true if the repeated event is OK.
// This commonly happens for known bugs and for cases where events are repeated intentionally by tests.
// Use this to handle cases where, "if X is true, then the repeated event is ok".
type IsRepeatedEventOKFunc func(monitorEvent monitorapi.Interval, kubeClientConfig *rest.Config, times int) (bool, error)

type EventMatcher interface {
	Matches(i monitorapi.Interval, topology v1.TopologyMode) bool
}

// SimplePathologicalEventMatcher allows the definition of kube event intervals that can repeat more than the threshold we allow during a job run.
// All specified fields must match the interval for it to be allowed.
type SimplePathologicalEventMatcher struct {
	// Name is a unique CamelCase friendly name that briefly describes the allowed dupe events. It's used in
	// logging and unit tests to make sure we match on what we expect.
	Name string
	// LocatorKeyRegexes is a map of LocatorKey to regex that key must match.
	LocatorKeyRegexes map[monitorapi.LocatorKey]*regexp.Regexp
	// MessageReasonRegex checks the Reason on a structured interval Message.
	MessageReasonRegex *regexp.Regexp
	// MessageReasonRegex checks the HumanMessage on a structured interval Message.
	MessageHumanRegex *regexp.Regexp
	// Topology limits the exception to a specific topology. (e.g. single replica)
	Topology *v1.TopologyMode

	// RepeatThresholdOverride allows a matcher to allow more than our default number of repeats.
	// Less will not work as the matcher will not be invoked if we're over our threshold.
	RepeatThresholdOverride int

	// Jira is a link to a Jira (or legacy Bugzilla). If set it implies we consider this event a problem but there's
	// been a bug filed.
	Jira string
}

// Matches checks if the given locator/messagebuilder matches this allowed dupe event.
func (ade *SimplePathologicalEventMatcher) Matches(i monitorapi.Interval, topology v1.TopologyMode) bool {
	l := i.StructuredLocator
	msg := i.StructuredMessage
	for lk, r := range ade.LocatorKeyRegexes {
		if !r.MatchString(l.Keys[lk]) {
			logrus.WithField("allower", ade.Name).Debugf("key %s did not match", lk)
			return false
		}
	}
	if ade.MessageHumanRegex != nil && !ade.MessageHumanRegex.MatchString(msg.HumanMessage) {
		logrus.WithField("allower", ade.Name).Debugf("human message did not match")
		return false
	}
	if ade.MessageReasonRegex != nil && !ade.MessageReasonRegex.MatchString(string(msg.Reason)) {
		logrus.WithField("allower", ade.Name).Debugf("message reason did not match")
		return false
	}

	if ade.RepeatThresholdOverride != 0 {
		count := GetTimesAnEventHappened(msg)
		if count > ade.RepeatThresholdOverride {
			logrus.WithField("allower", ade.Name).Debugf("event repeated over threshold override: %d", ade.RepeatThresholdOverride)
			return false
		}
	}

	if ade.Topology != nil && *ade.Topology != topology {
		logrus.WithField("allower", ade.Name).Debugf("cluster did not match topology")
		return false
	}
	return true
}

type AllowedPathologicalEventRegistry struct {
	matchers map[string]EventMatcher
}

func (r *AllowedPathologicalEventRegistry) AddPathologicalEventMatcher(name string, eventMatcher EventMatcher) error {
	if _, ok := r.matchers[name]; ok {
		return fmt.Errorf("%q is already registered", name)
	}
	if name == "" {
		return fmt.Errorf("must specify a name for pathological event matchers")
	}
	r.matchers[name] = eventMatcher
	return nil
}

func (r *AllowedPathologicalEventRegistry) AddPathologicalEventMatcherOrDie(name string, eventMatcher EventMatcher) {
	err := r.AddPathologicalEventMatcher(name, eventMatcher)
	if err != nil {
		panic(err)
	}
}

// MatchesAny checks if the given event locator and message match any registered matcher.
// Returns true if so, the matcher name, and the matcher itself.
func (r *AllowedPathologicalEventRegistry) MatchesAny(
	i monitorapi.Interval,
	topology v1.TopologyMode) (bool, string, EventMatcher) {
	l := i.StructuredLocator
	msg := i.StructuredMessage
	for k, m := range r.matchers {
		allowed := m.Matches(i, topology)
		if allowed {
			logrus.WithField("message", msg).WithField("locator", l).Infof("duplicated event allowed by %s", k)
			return allowed, k, m
		}
	}
	return false, "", nil
}

func (r *AllowedPathologicalEventRegistry) GetMatcherByName(name string) (EventMatcher, error) {

	matcher, ok := r.matchers[name]
	if !ok {
		return nil, fmt.Errorf("no pathological event matcher registered with name: %s", name)
	}
	return matcher, nil
}

// AllowedPathologicalEvents is the list of all allowed duplicate events on all jobs. Upgrade has an additional
// list which is combined with this one.
func NewUniversalPathologicalEventMatchers(kubeConfig *rest.Config, finalIntervals monitorapi.Intervals) *AllowedPathologicalEventRegistry {
	registry := &AllowedPathologicalEventRegistry{matchers: map[string]EventMatcher{}}

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should not deadlock when a pod's predecessor fails [Suite:openshift/conformance/parallel] [Suite:k8s]
	// PauseNewPods intentionally causes readiness probe to fail.
	registry.AddPathologicalEventMatcherOrDie("UnhealthyE2EStatefulSet", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-statefulset-[0-9]+`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`ss-[0-9]`),
			monitorapi.LocatorNodeKey:      regexp.MustCompile(`[a-z0-9.-]+`),
		},
		MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed: `),
	})

	// Kubectl Port forwarding ***
	// The same pod name is used many times for all these tests with a tight readiness check to make the tests fast.
	// This results in hundreds of events while the pod isn't ready.
	/*

		This is duplicated with KubeletUnhealthyReadinessProbeFailed, I am keeping
		commented out as a historical artifact in case the blanked Unhealthy readiness probe
		matcher is removed some day and this specific case starts firing again.

		registry.AddPathologicalEventMatcherOrDie("UnhealthyE2EPortForwarding", &SimplePathologicalEventMatcher{
			LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-port-forwarding-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`^pfpod$`),
			},
			MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
			MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed: `),
		})

	*/

	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	registry.AddPathologicalEventMatcherOrDie("UnhealthyStatefulSetPod", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-statefulset-[0-9]+`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`ss2-[0-9]`),
		},
		MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed: HTTP probe failed with statuscode: 404`),
	})

	/*

		Historical artifact, covered by KubeletUnhealthyReadinessProbeFailed

		// [sig-node] Probing container ***
		// these tests intentionally cause repeated probe failures to ensure good handling
		registry.AddPathologicalEventMatcherOrDie("E2EContainerProbeFailedOrWarning", &SimplePathologicalEventMatcher{
			LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-container-probe-[0-9]+`),
			},
			MessageHumanRegex: regexp.MustCompile(`probe (failed|warning):`),
		})
	*/

	/*

		Historical artifact, covered by FailedScheduling

		// TestAllowedSCCViaRBAC and TestPodUpdateSCCEnforcement
		// The pod is shaped to intentionally not be scheduled.  Looks like an artifact of the old integration testing.
		registry.AddPathologicalEventMatcherOrDie("E2ESCCFailedScheduling", &SimplePathologicalEventMatcher{
			LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-test-scc-[a-z0-9]+`),
			},
			MessageReasonRegex: regexp.MustCompile(`FailedScheduling`),
		})
	*/

	// Security Context ** should not run with an explicit root user ID
	// Security Context ** should not run without a specified user ID
	// This container should never run
	registry.AddPathologicalEventMatcherOrDie("E2ESecurityContextBreaksNonRootPolicy", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-security-context-test-[0-9]+`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`.*-root-uid`),
		},
		MessageReasonRegex: regexp.MustCompile(`^Failed$`),
		MessageHumanRegex:  regexp.MustCompile(`Error: container's runAsUser breaks non-root policy.*`),
	})

	// PersistentVolumes-local tests should not run the pod when there is a volume node
	// affinity and node selector conflicts.
	/*

		Blanked allowed later by FailedScheduling matcher. Keeping for historical artifact.

		registry.AddPathologicalEventMatcherOrDie("E2EPersistentVolumesFailedScheduling", &SimplePathologicalEventMatcher{
			LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-persistent-local-volumes-test-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`pod-[a-z0-9.-]+`),
			},
			MessageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
		})
	*/

	// various DeploymentConfig tests trigger this by cancelling multiple rollouts
	registry.AddPathologicalEventMatcherOrDie("E2EDeploymentConfigCancellation", &SimplePathologicalEventMatcher{
		MessageReasonRegex: regexp.MustCompile(`^DeploymentAwaitingCancellation$`),
		MessageHumanRegex:  regexp.MustCompile(`Deployment of version [0-9]+ awaiting cancellation of older running deployments`),
	})

	// If image pulls in e2e namespaces fail catastrophically we'd expect them to lead to test failures
	// We are deliberately not ignoring image pull failures for core component namespaces
	registry.AddPathologicalEventMatcherOrDie("E2EImagePullBackOff", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^e2e-.*`),
		},
		MessageReasonRegex: regexp.MustCompile(`^BackOff$`),
		MessageHumanRegex:  regexp.MustCompile(`Back-off pulling image`),
	})

	// Several allowances were related to Loki, I think we can generally ignore any repeating event
	// from the Loki NS, this should not fail tests.
	registry.AddPathologicalEventMatcherOrDie("LokiPromtailReadinessProbe", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-e2e-loki$`),
		},
	})

	// kube apiserver, controller-manager and scheduler guard pod probes can fail due to operands getting rolled out
	// multiple times during the bootstrapping phase of a cluster installation

	registry.AddPathologicalEventMatcherOrDie("KubeControlPlaneGuardReadinessProbeError", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-kube-*`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^kube.*guard.*`),
		},
		MessageReasonRegex: regexp.MustCompile(`^ProbeError$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe error`),
	})

	// this is the less specific even sent by the kubelet when a probe was executed successfully but returned false
	// we ignore this event because openshift has a patch in patch_prober that sends a more specific event about
	// readiness failures in openshift-* namespaces.  We will catch the more specific ProbeError events.
	registry.AddPathologicalEventMatcherOrDie("KubeletUnhealthyReadinessProbeFailed", &SimplePathologicalEventMatcher{
		MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed`),
	})

	/*

			This looks duplicated with AllowBackOffRestartingFailedContainer
		Kept for historical purposes

		// should not start app containers if init containers fail on a RestartAlways pod
		// the init container intentionally fails to start
		registry.AddPathologicalEventMatcherOrDie("E2EInitContainerRestartBackoff", &SimplePathologicalEventMatcher{
			LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-init-container-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`pod-init-[a-z0-9.-]+`),
			},
			MessageReasonRegex: regexp.MustCompile(`^BackOff$`),
			MessageHumanRegex:  regexp.MustCompile(`Back-off restarting failed container`),
		})
	*/

	// If you see this error, it means enough was working to get this event which implies enough retries happened to allow initial openshift
	// installation to succeed. Hence, we can ignore it.
	registry.AddPathologicalEventMatcherOrDie("FailedCreateEC2InsufficientInstanceCapacity", &SimplePathologicalEventMatcher{
		MessageReasonRegex: regexp.MustCompile(`^FailedCreate$`),
		MessageHumanRegex:  regexp.MustCompile(`error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient .* capacity in the Availability Zone you requested`),
	})

	// This was originally filed as a bug in 2021, closed as fixed, but the events continue repeating in 2023.
	// They only occur in the namespace for a specific horizontal pod autoscaling test. Ignoring permanently,
	// as they have been for the past two years.
	// https://bugzilla.redhat.com/show_bug.cgi?id=1993985
	registry.AddPathologicalEventMatcherOrDie("E2EHorizontalPodAutoscalingFailedToGetCPUUtilization", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`horizontalpodautoscaler`),
		},
		MessageHumanRegex: regexp.MustCompile(`failed to get cpu utilization: unable to get metrics for resource cpu: no metrics returned from resource metrics API`),
	})

	// Formerly bug: https://bugzilla.redhat.com/show_bug.cgi?id=2075204
	// Left stale and closed automatically. Assuming we can live with it now.
	registry.AddPathologicalEventMatcherOrDie("EtcdGuardProbeErrorConnectionRefused", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`etcd-guard.*`),
		},
		MessageReasonRegex: regexp.MustCompile(`^ProbeError$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe error: .* connect: connection refused`),
	})

	registry.AddPathologicalEventMatcherOrDie("OpenShiftAPICheckFailed", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(``),
			monitorapi.LocatorPodKey:       regexp.MustCompile(``),
		},
		MessageReasonRegex: regexp.MustCompile(`^OpenShiftAPICheckFailed$`),
		MessageHumanRegex:  regexp.MustCompile(`user.openshift.io.v1.*503`),
		// TODO: Jira long closed as stale, and this problem occurs well outside single node now.
		// A new bug should probably be filed.
		Jira: "https://bugzilla.redhat.com/show_bug.cgi?id=2017435",
	})

	registry.AddPathologicalEventMatcherOrDie("StaleConditionChallengeReset", &SimplePathologicalEventMatcher{
		MessageHumanRegex: regexp.MustCompile(`message changed from "\x{FEFF}`),
	})

	// This was originally intended to be limited to only during the openshift/build test suite, however it was
	// never hooked up and was just ignored everywhere. We do not have the capability to detect if
	// events were within specific test suites yet. Leaving them as an always allow for now.
	registry.AddPathologicalEventMatcherOrDie("ScaledReplicaSet", &SimplePathologicalEventMatcher{
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`(openshift-controller-manager|openshift-route-controller-manager)`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`(controller-manager|route-controller-manager)`),
		},
		MessageReasonRegex: regexp.MustCompile(`^ScalingReplicaSet$`),
		MessageHumanRegex:  regexp.MustCompile(`\(combined from similar events\): Scaled (down|up) replica set.*controller-manager-[a-z0-9-]+ to [0-9]+`),
	})

	registry.AddPathologicalEventMatcherOrDie(
		AllowBackOffRestartingFailedContainer.Name, AllowBackOffRestartingFailedContainer)

	registry.AddPathologicalEventMatcherOrDie(AllowOVNReadiness.Name, AllowOVNReadiness)

	registry.AddPathologicalEventMatcherOrDie(AllowImagePullFromRedHatRegistry.Name, AllowImagePullFromRedHatRegistry)

	registry.AddPathologicalEventMatcherOrDie(EtcdRequiredResourcesMissing.Name, EtcdRequiredResourcesMissing)

	registry.AddPathologicalEventMatcherOrDie(EtcdClusterOperatorStatusChanged.Name, EtcdClusterOperatorStatusChanged)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorTimeoutAwaitingHeaders.Name, ProbeErrorTimeoutAwaitingHeaders)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorLiveness.Name, ProbeErrorLiveness)
	registry.AddPathologicalEventMatcherOrDie(ReadinessFailed.Name, ReadinessFailed)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorConnectionRefused.Name, ProbeErrorConnectionRefused)
	registry.AddPathologicalEventMatcherOrDie(NodeHasNoDiskPressure.Name, NodeHasNoDiskPressure)
	registry.AddPathologicalEventMatcherOrDie(NodeHasSufficientMemory.Name, NodeHasSufficientMemory)
	registry.AddPathologicalEventMatcherOrDie(NodeHasSufficientPID.Name, NodeHasSufficientPID)
	registry.AddPathologicalEventMatcherOrDie(FailedScheduling.Name, FailedScheduling)
	registry.AddPathologicalEventMatcherOrDie(ErrorUpdatingEndpointSlices.Name, ErrorUpdatingEndpointSlices)
	registry.AddPathologicalEventMatcherOrDie(MarketplaceStartupProbeFailure.Name, MarketplaceStartupProbeFailure)

	// Inject the dynamic allowance for etcd readiness probe failures based on the number of
	// etcd revisions the cluster went through.
	etcdMatcher, err := newDuplicatedEventsAllowedWhenEtcdRevisionChange(context.TODO(), kubeConfig)
	if err != nil {
		logrus.WithError(err).Warning("unable to initialize dynamic etcd matcher based on revisions, skipping")
	} else {
		registry.AddPathologicalEventMatcherOrDie(etcdMatcher.Name, etcdMatcher)
	}

	return registry
}

// Some broken out matchers are re-used in a test for that specific event, keeping them as package vars
// for compile time protection.

var AllowOVNReadiness = &SimplePathologicalEventMatcher{
	Name: "OVNReadinessProbeFailed",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-ovn-kubernetes`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`ovnkube-node-`),
	},
	MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
	MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed:`),
}

// Separated out in testBackoffPullingRegistryRedhatImage
var AllowImagePullFromRedHatRegistry = &SimplePathologicalEventMatcher{
	Name:              "AllowImagePullBackOffFromRedHatRegistry",
	MessageHumanRegex: regexp.MustCompile(`Back-off pulling image .*registry.redhat.io`),
}

// Separated out in testBackoffStartingFailedContainer
var AllowBackOffRestartingFailedContainer = &SimplePathologicalEventMatcher{
	Name:               "AllowBackOffRestartingFailedContainer",
	MessageReasonRegex: regexp.MustCompile(`^BackOff$`),
	MessageHumanRegex:  regexp.MustCompile(`Back-off restarting failed container`),
}

// Separated out in testRequiredInstallerResourcesMissing
var EtcdRequiredResourcesMissing = &SimplePathologicalEventMatcher{
	Name:               "EtcdRequiredResourcesMissing",
	MessageReasonRegex: regexp.MustCompile(`^RequiredInstallerResourcesMissing$`),
	MessageHumanRegex:  regexp.MustCompile(`secrets: etcd-all-certs-[0-9]+`),
}

// reason/OperatorStatusChanged Status for clusteroperator/etcd changed: Degraded message changed from "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy" to "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: No unhealthy members found"
var EtcdClusterOperatorStatusChanged = &SimplePathologicalEventMatcher{
	Name: "EtcdClusterOperatorStatusChanged",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`^openshift-etcd`),
	},
	MessageReasonRegex: regexp.MustCompile(`^OperatorStatusChanged$`),
	MessageHumanRegex:  regexp.MustCompile(`Status for clusteroperator/etcd changed.*No unhealthy members found`),
}

// ProbeErrorTimeoutAwaitingHeaders matches events in specific namespaces such as:
// reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorTimeoutAwaitingHeaders = &SimplePathologicalEventMatcher{
	Name: "ProbeErrorTimeoutAwaitingHeaders",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	MessageReasonRegex: regexp.MustCompile(`^ProbeError$`),
	MessageHumanRegex:  regexp.MustCompile(`Readiness probe error.*Client.Timeout exceeded while awaiting headers`),
}

// ProbeErrorConnectionRefused matches events in specific namespaces.
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorConnectionRefused = &SimplePathologicalEventMatcher{
	Name: "ProbeErrorConnectionRefused",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	MessageReasonRegex: regexp.MustCompile(`^ProbeError$`),
	MessageHumanRegex:  regexp.MustCompile(`Readiness probe error.*connection refused`),
}

// ProbeErrorLiveness matches events in specific namespaces such as:
// Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorLiveness = &SimplePathologicalEventMatcher{
	Name: "ProbeErrorLiveness",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	MessageReasonRegex: regexp.MustCompile(`^(ProbeError|Unhealthy)$`),
	MessageHumanRegex:  regexp.MustCompile(`Liveness probe error.*Client.Timeout exceeded while awaiting headers`),
}

// ReadinessFailed matches events in specific namespaces such as:
// ...ReadinessFailed Get \"https://10.130.0.16:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ReadinessFailed = &SimplePathologicalEventMatcher{
	Name: "ReadinessFailed",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
		//monitorapi.LocatorPodKey:       regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	MessageReasonRegex: regexp.MustCompile(`^ReadinessFailed$`),
	MessageHumanRegex:  regexp.MustCompile(`Get.*healthz.*net/http.*request canceled while waiting for connection.*Client.Timeout exceeded`),
}

// Separated out in testNodeHasNoDiskPressure
var NodeHasNoDiskPressure = &SimplePathologicalEventMatcher{
	Name:               "NodeHasNoDiskPressure",
	MessageReasonRegex: regexp.MustCompile(`^NodeHasNoDiskPressure$`),
	MessageHumanRegex:  regexp.MustCompile(`status is now: NodeHasNoDiskPressure`),
}

// Separated out in testNodeHasSufficientMemory
var NodeHasSufficientMemory = &SimplePathologicalEventMatcher{
	Name:               "NodeHasSufficientMemory",
	MessageReasonRegex: regexp.MustCompile(`^NodeHasSufficientMemory$`),
	MessageHumanRegex:  regexp.MustCompile(`status is now: NodeHasSufficientMemory`),
}

// Separated out in testNodeHasSufficientPID
var NodeHasSufficientPID = &SimplePathologicalEventMatcher{
	Name:               "NodeHasSufficientPID",
	MessageReasonRegex: regexp.MustCompile(`^NodeHasSufficientPID$`),
	MessageHumanRegex:  regexp.MustCompile(`status is now: NodeHasSufficientPID`),
}

// reason/FailedScheduling 0/6 nodes are available: 2 node(s) didn't match Pod's node affinity/selector, 2 node(s) didn't match pod anti-affinity rules, 2 node(s) were unschedulable. preemption: 0/6 nodes are available: 2 node(s) didn't match pod anti-affinity rules, 4 Preemption is not helpful for scheduling..
var FailedScheduling = &SimplePathologicalEventMatcher{
	Name:               "FailedScheduling",
	MessageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
	MessageHumanRegex:  regexp.MustCompile(`nodes are available.*didn't match Pod's node affinity/selector`),
}

// Separated out in testErrorUpdatingEndpointSlices
var ErrorUpdatingEndpointSlices = &SimplePathologicalEventMatcher{
	Name:               "ErrorUpdatingEndpointSlices",
	MessageReasonRegex: regexp.MustCompile(`^FailedToUpdateEndpointSlices$`),
	MessageHumanRegex:  regexp.MustCompile(`Error updating Endpoint Slices`),
}

// Separated out in testMarketplaceStartupProbeFailure
var MarketplaceStartupProbeFailure = &SimplePathologicalEventMatcher{
	Name: "MarketplaceStartupProbeFailure",
	LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-marketplace`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`(community-operators|redhat-operators)-[a-z0-9-]+`),
	},
	MessageHumanRegex: regexp.MustCompile(`Startup probe failed`),
}

func NewUpgradePathologicalEventMatchers(kubeConfig *rest.Config, finalIntervals monitorapi.Intervals) *AllowedPathologicalEventRegistry {

	// Start with the main list of matchers:
	registry := NewUniversalPathologicalEventMatchers(kubeConfig, finalIntervals)

	// Now add in the matchers we only want to apply during upgrade:

	// Operators that use library-go can report about multiple versions during upgrades.
	registry.AddPathologicalEventMatcherOrDie("OperatorMultipleVersions", &SimplePathologicalEventMatcher{
		Name: "OperatorMultipleVersions",
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`(openshift-etcd-operator|openshift-kube-apiserver-operator|openshift-kube-controller-manager-operator|openshift-kube-scheduler-operator)`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`(etcd-operator|kube-apiserver-operator|kube-controller-manager-operator|openshift-kube-scheduler-operator)`),
		},
		MessageReasonRegex: regexp.MustCompile(`^MultipleVersions$`),
		MessageHumanRegex:  regexp.MustCompile(`multiple versions found, probably in transition`),
	})

	// etcd-quorum-guard can fail during upgrades.
	registry.AddPathologicalEventMatcherOrDie("EtcdQuorumGuardReadinessProbe", &SimplePathologicalEventMatcher{
		Name: "EtcdQuorumGuardReadinessProbe",
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^etcd-quorum-guard.*`),
		},
		MessageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		MessageHumanRegex:  regexp.MustCompile(`Readiness probe failed:`),
	})

	// etcd can have unhealthy members during an upgrade
	registry.AddPathologicalEventMatcherOrDie("EtcdUnhealthyMembers", &SimplePathologicalEventMatcher{
		Name: "EtcdUnhealthyMembers",
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`openshift-etcd-operator`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`etcd-operator`),
		},
		MessageReasonRegex: regexp.MustCompile(`^UnhealthyEtcdMember$`),
		MessageHumanRegex:  regexp.MustCompile(`unhealthy members`),
	})

	// Ignore NetworkNotReady repeat events.
	// This was originally linked to bugzilla: https://bugzilla.redhat.com/show_bug.cgi?id=1986370
	// The bug has been closed as NOTABUG.
	// We used to allow this for three namespaces (openshift-multus, openshift-e2e-loki, and openshift-network-diagnostics),
	// however a quick search of the intervals in bigquery shows this happening a ton in lots of namespaces,
	// and killing jobs when it does. Given the bug status, I am ignoring these events, whenever they occur, in
	// all upgrade jobs for now. - dgoodwin
	registry.AddPathologicalEventMatcherOrDie("NetworkNotReady", &SimplePathologicalEventMatcher{
		Name:               "NetworkNotReady",
		MessageReasonRegex: regexp.MustCompile(`^NetworkNotReady$`),
		MessageHumanRegex:  regexp.MustCompile(`network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file.*Has your network provider started\?`),
	})

	// Allow FailedScheduling repeat events during node upgrades:
	m := newFailedSchedulingDuringNodeUpdatePathologicalEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie("FailedSchedulingDuringNodeUpdate", m)

	return registry
}

// IsEventAfterInstallation returns true if the monitorEvent represents an event that happened after installation.
func IsEventAfterInstallation(monitorEvent monitorapi.Interval, kubeClientConfig *rest.Config) (bool, error) {
	if kubeClientConfig == nil {
		// default to OK
		return true, nil
	}
	installCompletionTime := getInstallCompletionTime(kubeClientConfig)
	if installCompletionTime == nil {
		return true, nil
	}

	namespace := monitorEvent.StructuredLocator.Keys[monitorapi.LocatorNamespaceKey]
	pod := monitorEvent.StructuredLocator.Keys[monitorapi.LocatorNamespaceKey]
	reason := monitorEvent.StructuredMessage.Reason
	msg := monitorEvent.StructuredMessage.HumanMessage
	kubeClient, err := kubernetes.NewForConfig(kubeClientConfig)
	if err != nil {
		return true, nil
	}

	// TODO: listing all kube events when we already have intervals for them seems drastic.
	// It appears to be so we could get FirstTimestamp, but we could perhaps store that in a message
	// annotation when we receive events.
	kubeEvents, err := kubeClient.CoreV1().Events(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return true, nil
	}
	for _, event := range kubeEvents.Items {
		if event.Related == nil ||
			event.Related.Name != pod ||
			event.Reason != string(reason) ||
			!strings.Contains(event.Message, msg) {
			continue
		}

		// TODO: if an event happened 21 times, and only the first was during install, we're going
		// to return true here when we shouldn't... to do this properly we'd need to separate out
		// a count of how many occurred *after* install, and that's going to be quite difficult.
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

// newDuplicatedEventsAllowedWhenEtcdRevisionChange tolerates etcd readiness probe failures unless we receive more
// than the allowance per revisions of etcd.
func newDuplicatedEventsAllowedWhenEtcdRevisionChange(ctx context.Context, clientConfig *rest.Config) (*SimplePathologicalEventMatcher, error) {
	if clientConfig == nil {
		return nil, fmt.Errorf("no kubeconfig provided, cannot lookup number of etcd revisions")
	}
	operatorClient, err := operatorv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	currentRevision, err := getBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
	}
	repeatThresholdOverride := currentRevision * (60 / 5)
	logrus.WithFields(logrus.Fields{
		"etcdRevision":   currentRevision,
		"allowedRepeats": repeatThresholdOverride,
	}).Info("created toleration for etcd readiness probes per revision")

	return &SimplePathologicalEventMatcher{
		Name: "EtcdReadinessProbeFailuresPerRevisionChange",
		LocatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-etcd$`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^etcd-guard-`),
		},
		MessageReasonRegex:      regexp.MustCompile(`^(Unhealthy|ProbeError)$`),
		MessageHumanRegex:       regexp.MustCompile(`Readiness probe`),
		RepeatThresholdOverride: repeatThresholdOverride, // 60s for starting a new pod, divided by the probe interval
	}, nil
}

func newFailedSchedulingDuringNodeUpdatePathologicalEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	// Filter out a list of NodeUpdate events, we use these to ignore some other potential pathological events that are
	// expected during NodeUpdate.
	nodeUpdateIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceNodeState &&
			eventInterval.StructuredLocator.Type == monitorapi.LocatorTypeNode &&
			eventInterval.StructuredMessage.Annotations[monitorapi.AnnotationConstructed] == monitorapi.ConstructionOwnerNodeLifecycle &&
			eventInterval.StructuredMessage.Annotations[monitorapi.AnnotationPhase] == "Update" &&
			strings.Contains(eventInterval.StructuredMessage.Annotations[monitorapi.AnnotationRoles], "master")
	})
	logrus.Infof("found %d NodeUpdate intervals", len(nodeUpdateIntervals))
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			Name:               "FailedSchedulingDuringNodeUpdate",
			MessageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
		},
		allowIfWithinIntervals: nodeUpdateIntervals,
	}
}

// OverlapOtherIntervalsPathologicalEventMatcher is an implementation containing a regular
// matcher, plus additional logic that will allow the event only if it is contained
// within another set of intervals provided. (i.e. used to allow FailedScheduling pathological
// events if they are contained within NodeUpdate intervals)
type OverlapOtherIntervalsPathologicalEventMatcher struct {
	// delegate is a normal event matcher.
	delegate *SimplePathologicalEventMatcher
	// allowIfWithinIntervals is the list of intervals that the incoming pathological event will
	// match if it contained within one of these.
	allowIfWithinIntervals monitorapi.Intervals
}

func (ade *OverlapOtherIntervalsPathologicalEventMatcher) Matches(i monitorapi.Interval, topology v1.TopologyMode) bool {

	// Check the delegate matcher first, if it matches, proceed to additional checks
	if !ade.delegate.Matches(i, topology) {
		return false
	}

	// Match the pathological event if it overlaps with any of the given set of intervals.
	for _, nui := range ade.allowIfWithinIntervals {
		if nui.From.Before(i.From) && nui.To.After(i.To) {
			logrus.Infof("%s was found to overlap with %s, ignoring pathological event as we expect these during master updates", i, nui)
			return true
		}
	}
	return false
}
