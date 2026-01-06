package pathologicaleventlibrary

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/sirupsen/logrus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
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

type EventMatcher interface {
	// Name returns a unique name (enforced by registry) for this matcher.
	Name() string

	// Matches returns true if the given interval looks relevant to this matcher, but does not
	// indicate the interval should be allowed to repeat pathologically. For that use Allows.
	Matches(i monitorapi.Interval) bool

	// Allows returns true if the given interval should be allowed to repeat as many times
	// as it did. It performs the Matches, check, and layers in additional logic from runtime.
	Allows(i monitorapi.Interval, topology v1.TopologyMode) bool
}

// SimplePathologicalEventMatcher allows the definition of kube event intervals that can repeat more than the threshold we allow during a job run.
// All specified fields must match the interval for it to be allowed.
type SimplePathologicalEventMatcher struct {
	// Name is a unique CamelCase friendly name that briefly describes the allowed dupe events. It's used in
	// logging and unit tests to make sure we match on what we expect.
	name string
	// locatorKeyRegexes is a map of LocatorKey to regex that key must match.
	locatorKeyRegexes map[monitorapi.LocatorKey]*regexp.Regexp
	// messageReasonRegex checks the Reason on a structured interval Message.
	messageReasonRegex *regexp.Regexp
	// messageReasonRegex checks the HumanMessage on a structured interval Message.
	messageHumanRegex *regexp.Regexp

	// jira is a link to a jira (or legacy Bugzilla). If set it implies we consider this event a problem but there's
	// been a bug filed.
	jira string

	// repeatThresholdOverride allows a matcher to allow more than our default number of repeats.
	// Less will not work as the matcher will not be invoked if we're over our threshold.
	// This is only considered in the context of Allows, not Matches.
	repeatThresholdOverride int

	// neverAllow is for matchers we may use to get things flagged as "interesting" and thus charted, but
	// we don't want to allow to repeat pathologically.
	neverAllow bool

	// topology limits the exception to a specific topology. (e.g. single replica)
	// This is only considered in the context of Allows, not Matches.
	topology *v1.TopologyMode
}

func (ade *SimplePathologicalEventMatcher) Name() string {
	return ade.name
}

func (ade *SimplePathologicalEventMatcher) Matches(i monitorapi.Interval) bool {
	l := i.Locator
	msg := i.Message
	log := logrus.WithField("allower", ade.Name())
	for lk, r := range ade.locatorKeyRegexes {
		if !r.MatchString(l.Keys[lk]) {
			log.Debugf("%s: key %s did not match", ade.Name(), lk)
			return false
		}
	}
	if ade.messageHumanRegex != nil && !ade.messageHumanRegex.MatchString(msg.HumanMessage) {
		log.Debugf("%s: human message did not match", ade.Name())
		return false
	}
	if ade.messageReasonRegex != nil && !ade.messageReasonRegex.MatchString(string(msg.Reason)) {
		log.Debugf("%s: message reason did not match", ade.Name())
		return false
	}

	return true
}

// Allows checks if the given locator/messagebuilder matches this allowed dupe event, and if the
// interval should be allowed to repeat pathologically.
func (ade *SimplePathologicalEventMatcher) Allows(i monitorapi.Interval, topology v1.TopologyMode) bool {

	if ade.neverAllow {
		return false
	}

	msg := i.Message
	if !ade.Matches(i) {
		return false
	}

	if ade.repeatThresholdOverride != 0 {
		count := GetTimesAnEventHappened(msg)
		if count > ade.repeatThresholdOverride {
			logrus.WithField("allower", ade.Name).Debugf("event repeated over threshold override: %d", ade.repeatThresholdOverride)
			return false
		}
	}

	if ade.topology != nil && *ade.topology != topology {
		logrus.WithField("allower", ade.Name).Debugf("cluster did not match topology")
		return false
	}
	return true
}

type AllowedPathologicalEventRegistry struct {
	matchers map[string]EventMatcher
}

func (r *AllowedPathologicalEventRegistry) AddPathologicalEventMatcher(eventMatcher EventMatcher) error {
	if _, ok := r.matchers[eventMatcher.Name()]; ok {
		return fmt.Errorf("%q is already registered", eventMatcher.Name())
	}
	if eventMatcher.Name() == "" {
		return fmt.Errorf("must specify a name for pathological event matchers")
	}
	r.matchers[eventMatcher.Name()] = eventMatcher
	return nil
}

func (r *AllowedPathologicalEventRegistry) AddPathologicalEventMatcherOrDie(eventMatcher EventMatcher) {
	err := r.AddPathologicalEventMatcher(eventMatcher)
	if err != nil {
		panic(err)
	}
}

// MatchesAny checks if the given event locator and message match any registered matcher.
// Returns true if so, the matcher name, and the matcher itself.
// It does NOT check if the interval should be allowed.
func (r *AllowedPathologicalEventRegistry) MatchesAny(i monitorapi.Interval) (bool, EventMatcher) {
	l := i.Locator
	msg := i.Message
	for k, m := range r.matchers {
		allowed := m.Matches(i)
		if allowed {
			logrus.WithField("message", msg).WithField("locator", l).Infof("event interval matches %s", k)
			return allowed, m
		}
	}
	return false, nil
}

// AllowedByAny checks if the given event locator and message match any registered matcher, and if
// the repeating event should be allowed.
// Returns true if so, the matcher name, and the matcher itself.
func (r *AllowedPathologicalEventRegistry) AllowedByAny(
	i monitorapi.Interval,
	topology v1.TopologyMode) (bool, EventMatcher) {
	l := i.Locator
	msg := i.Message
	for k, m := range r.matchers {
		allowed := m.Allows(i, topology)
		if allowed {
			logrus.WithField("message", msg).WithField("locator", l).Infof("duplicated event allowed by %s", k)
			return allowed, m
		}
	}
	return false, nil
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
	// [sig-apps] StatefulSet Basic StatefulSet functionality [StatefulSetBasic] should perform rolling updates and roll backs of template modifications [Conformance] [Suite:openshift/conformance/parallel/minimal] [Suite:k8s]
	// breakPodHTTPProbe intentionally causes readiness probe to fail.
	/*

					This is duplicated with KubeletUnhealthyReadinessProbeFailed, I am keeping
					commented out as a historical artifact in case the blanked Unhealthy readiness probe
					matcher is removed some day and this specific case starts firing again.

		registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
			name: "E2EStatefulSetReadinessProbeFailed",
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-statefulset-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`ss2-[0-9]`),
				monitorapi.LocatorNodeKey:      regexp.MustCompile(`[a-z0-9.-]+`),
			},
			messageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
			messageHumanRegex:  regexp.MustCompile(`Readiness probe failed: `),
		})
	*/

	// Kubectl Port forwarding ***
	// The same pod name is used many times for all these tests with a tight readiness check to make the tests fast.
	// This results in hundreds of events while the pod isn't ready.
	/*

		This is duplicated with KubeletUnhealthyReadinessProbeFailed, I am keeping
		commented out as a historical artifact in case the blanked Unhealthy readiness probe
		matcher is removed some day and this specific case starts firing again.

		registry.AddPathologicalEventMatcherOrDie("UnhealthyE2EPortForwarding", &SimplePathologicalEventMatcher{
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-port-forwarding-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`^pfpod$`),
			},
			messageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
			messageHumanRegex:  regexp.MustCompile(`Readiness probe failed: `),
		})

	*/

	/*

		Historical artifact, covered by KubeletUnhealthyReadinessProbeFailed

		// [sig-node] Probing container ***
		// these tests intentionally cause repeated probe failures to ensure good handling
		registry.AddPathologicalEventMatcherOrDie("E2EContainerProbeFailedOrWarning", &SimplePathologicalEventMatcher{
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-container-probe-[0-9]+`),
			},
			messageHumanRegex: regexp.MustCompile(`probe (failed|warning):`),
		})
	*/

	/*

		Historical artifact, covered by FailedScheduling

		// TestAllowedSCCViaRBAC and TestPodUpdateSCCEnforcement
		// The pod is shaped to intentionally not be scheduled.  Looks like an artifact of the old integration testing.
		registry.AddPathologicalEventMatcherOrDie("E2ESCCFailedScheduling", &SimplePathologicalEventMatcher{
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-test-scc-[a-z0-9]+`),
			},
			messageReasonRegex: regexp.MustCompile(`FailedScheduling`),
		})
	*/

	// Security Context ** should not run with an explicit root user ID
	// Security Context ** should not run without a specified user ID
	// This container should never run
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "E2ESecurityContextBreaksNonRootPolicy",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-security-context-test-[0-9]+`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`.*-root-uid`),
		},
		messageReasonRegex: regexp.MustCompile(`^Failed$`),
		messageHumanRegex:  regexp.MustCompile(`Error: container's runAsUser breaks non-root policy.*`),
	})

	// PersistentVolumes-local tests should not run the pod when there is a volume node
	// affinity and node selector conflicts.
	/*

		Blanked allowed later by FailedScheduling matcher. Keeping for historical artifact.

		registry.AddPathologicalEventMatcherOrDie("E2EPersistentVolumesFailedScheduling", &SimplePathologicalEventMatcher{
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-persistent-local-volumes-test-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`pod-[a-z0-9.-]+`),
			},
			messageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
		})
	*/

	// various DeploymentConfig tests trigger this by cancelling multiple rollouts
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:               "DeploymentAwaitingCancellation",
		messageReasonRegex: regexp.MustCompile(`^DeploymentAwaitingCancellation$`),
		messageHumanRegex:  regexp.MustCompile(`Deployment of version [0-9]+ awaiting cancellation of older running deployments`),
	})

	// If image pulls in e2e namespaces fail catastrophically we'd expect them to lead to test failures
	// We are deliberately not ignoring image pull failures for core component namespaces
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "E2EImagePullBackOff",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^e2e-.*`),
		},
		messageReasonRegex: regexp.MustCompile(`^BackOff$`),
		messageHumanRegex:  regexp.MustCompile(`Back-off pulling image`),
	})

	// Several allowances were related to Loki, I think we can generally ignore any repeating event
	// from the Loki NS, this should not fail tests.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "E2ELoki",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-e2e-loki$`),
		},
	})

	// kube apiserver, controller-manager and scheduler guard pod probes can fail due to operands getting rolled out
	// multiple times during the bootstrapping phase of a cluster installation

	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "KubeAPIReadinessProbeError",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-kube-*`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`kube.*guard.*`),
		},
		messageReasonRegex: regexp.MustCompile(`^ProbeError$`),
		messageHumanRegex:  regexp.MustCompile(`Readiness probe error`),
	})

	// this is the less specific even sent by the kubelet when a probe was executed successfully but returned false
	// we ignore this event because openshift has a patch in patch_prober that sends a more specific event about
	// readiness failures in openshift-* namespaces.  We will catch the more specific ProbeError events.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:               "KubeletUnhealthyReadinessProbeFailed",
		messageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		messageHumanRegex:  regexp.MustCompile(`Readiness probe failed`),
	})

	// Managed services osd-cluster-ready will fail until the OSD operators are ready, this triggers pathological events
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "OSDClusterReadyRestart",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-monitoring`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`.*osd-cluster-ready.*`),
		},
		messageReasonRegex: regexp.MustCompile(`^BackOff$`),
		messageHumanRegex:  regexp.MustCompile(`Back-off restarting failed container.*osd-cluster-ready.*`),
	})

	/*

			This looks duplicated with AllowBackOffRestartingFailedContainer
		Kept for historical purposes

		// should not start app containers if init containers fail on a RestartAlways pod
		// the init container intentionally fails to start
		registry.AddPathologicalEventMatcherOrDie("E2EInitContainerRestartBackoff", &SimplePathologicalEventMatcher{
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`e2e-init-container-[0-9]+`),
				monitorapi.LocatorPodKey:       regexp.MustCompile(`pod-init-[a-z0-9.-]+`),
			},
			messageReasonRegex: regexp.MustCompile(`^BackOff$`),
			messageHumanRegex:  regexp.MustCompile(`Back-off restarting failed container`),
		})
	*/

	// If you see this error, it means enough was working to get this event which implies enough retries happened to allow initial openshift
	// installation to succeed. Hence, we can ignore it.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:               "AWSFailedCreateInsufficientInstanceCapacity",
		messageReasonRegex: regexp.MustCompile(`^FailedCreate$`),
		messageHumanRegex:  regexp.MustCompile(`error creating EC2 instance: InsufficientInstanceCapacity: We currently do not have sufficient .* capacity in the Availability Zone you requested`),
	})

	// This was originally filed as a bug in 2021, closed as fixed, but the events continue repeating in 2023.
	// They only occur in the namespace for a specific horizontal pod autoscaling test. Ignoring permanently,
	// as they have been for the past two years.
	// https://bugzilla.redhat.com/show_bug.cgi?id=1993985
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "PodAutoscalerFailedToGetCPUUtilization",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`horizontalpodautoscaler`),
		},
		messageHumanRegex: regexp.MustCompile(`failed to get cpu utilization: unable to get metrics for resource cpu: no metrics returned from resource metrics API`),
	})

	// Formerly bug: https://bugzilla.redhat.com/show_bug.cgi?id=2075204
	// Left stale and closed automatically. Assuming we can live with it now.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "EtcdReadinessProbeError",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`etcd-guard.*`),
		},
		messageReasonRegex: regexp.MustCompile(`^ProbeError$`),
		messageHumanRegex:  regexp.MustCompile(`Readiness probe error: .* connect: connection refused`),
	})

	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "OpenShiftAPICheckFailed",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(``),
			monitorapi.LocatorPodKey:       regexp.MustCompile(``),
		},
		messageReasonRegex: regexp.MustCompile(`^OpenShiftAPICheckFailed$`),
		messageHumanRegex:  regexp.MustCompile(`user.openshift.io.v1.*503`),
		// TODO: Jira long closed as stale, and this problem occurs well outside single node now.
		// A new bug should probably be filed.
		jira: "https://bugzilla.redhat.com/show_bug.cgi?id=2017435",
	})

	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:              "MessageChangedFromFEFF",
		messageHumanRegex: regexp.MustCompile(`message changed from "\\ufeff`),
	})

	// This was originally intended to be limited to only during the openshift/build test suite, however it was
	// never hooked up and was just ignored everywhere. We do not have the capability to detect if
	// events were within specific test suites yet. Leaving them as an always allow for now.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "ScalingReplicaSet",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`(openshift-controller-manager|openshift-route-controller-manager)`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`(controller-manager|route-controller-manager)`),
		},
		messageReasonRegex: regexp.MustCompile(`^ScalingReplicaSet$`),
		messageHumanRegex:  regexp.MustCompile(`\(combined from similar events\): Scaled (down|up) replica set.*controller-manager-[a-z0-9-]+ from [0-9]+ to [0-9]+`),
	})

	// Match pod sandbox errors as "interesting" so they get charted, but we do not ever allow them to repeat
	// pathologically.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:              "PodSandbox",
		messageHumanRegex: regexp.MustCompile(`pod sandbox`),
		neverAllow:        true,
	})

	// Ignore repeated TopologyAwareHintsDisabled events from the endpoint
	// slice controller's topology cache.  The controller emits these events
	// if a service (such as the cluster DNS service) enables topology-aware
	// hints but certain "safeguards" (such as having sufficient service
	// endpoints and sufficient nodes with allocatable CPU in each zone) are
	// not met, in which case the default non-topology-aware behavior is
	// used.  This means that the service remains reachable even if these
	// events are emitted.  Note also that ovn-kubernetes does not implement
	// topology-aware hints; the feature is enabled only for the benefit of
	// third-party CNI plugins, such as Calico.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:               "TopologyAwareHintsDisabled",
		messageReasonRegex: regexp.MustCompile(`^TopologyAwareHintsDisabled$`),
		jira:               "https://issues.redhat.com/browse/OCPBUGS-69400",
	})

	registry.AddPathologicalEventMatcherOrDie(AllowBackOffRestartingFailedContainer)

	registry.AddPathologicalEventMatcherOrDie(AllowOVNReadiness)

	registry.AddPathologicalEventMatcherOrDie(AllowImagePullFromRedHatRegistry)

	registry.AddPathologicalEventMatcherOrDie(EtcdRequiredResourcesMissing)

	registry.AddPathologicalEventMatcherOrDie(EtcdClusterOperatorStatusChanged)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorTimeoutAwaitingHeaders)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorLiveness)
	registry.AddPathologicalEventMatcherOrDie(ReadinessFailed)
	registry.AddPathologicalEventMatcherOrDie(ProbeErrorConnectionRefused)
	registry.AddPathologicalEventMatcherOrDie(NodeHasNoDiskPressure)
	registry.AddPathologicalEventMatcherOrDie(NodeHasSufficientMemory)
	registry.AddPathologicalEventMatcherOrDie(NodeHasSufficientPID)
	registry.AddPathologicalEventMatcherOrDie(FailedScheduling)
	registry.AddPathologicalEventMatcherOrDie(ErrorUpdatingEndpointSlices)
	registry.AddPathologicalEventMatcherOrDie(MarketplaceStartupProbeFailure)
	registry.AddPathologicalEventMatcherOrDie(CertificateRotation)
	registry.AddPathologicalEventMatcherOrDie(KubeAPIServerAvoids500s)

	// Inject the dynamic allowance for etcd readiness probe failures based on the number of
	// etcd revisions the cluster went through.
	etcdMatcher, err := newDuplicatedEventsAllowedWhenEtcdRevisionChange(context.TODO(), kubeConfig)
	if err != nil {
		logrus.WithError(err).Warning("unable to initialize dynamic etcd matcher based on revisions, skipping")
	} else {
		registry.AddPathologicalEventMatcherOrDie(etcdMatcher)
	}

	topologyAwareMatcher := newTopologyAwareHintsDisabledDuringTaintTestsPathologicalEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(topologyAwareMatcher)

	singleNodeConnectionRefusedMatcher := newSingleNodeConnectionRefusedEventMatcher(finalIntervals)
	singleNodeKubeAPIServerProgressingMatcher := newSingleNodeKubeAPIProgressingEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(singleNodeConnectionRefusedMatcher)
	registry.AddPathologicalEventMatcherOrDie(singleNodeKubeAPIServerProgressingMatcher)

	vsphereConfigurationTestsRollOutTooOftenMatcher := newVsphereConfigurationTestsRollOutTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(vsphereConfigurationTestsRollOutTooOftenMatcher)

	newDeferringOperatorNodeUpdateTooOftenEventMatcher := newDeferringOperatorNodeUpdateTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newDeferringOperatorNodeUpdateTooOftenEventMatcher)

	newAnnotationChangeTooOftenEventMatcher := newAnnotationChangeTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newAnnotationChangeTooOftenEventMatcher)

	newSetDesiredConfigTooOftenEventMatcher := newSetDesiredConfigTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newSetDesiredConfigTooOftenEventMatcher)

	newCrioReloadedTooOftenEventMatcher := newCrioReloadedTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newCrioReloadedTooOftenEventMatcher)

	twoNodeEtcdEndpointsMatcher := newTwoNodeEtcdEndpointsConfigMissingEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(twoNodeEtcdEndpointsMatcher)

	newConfigDriftMonitorStoppedTooOftenEventMatcher := newConfigDriftMonitorStoppedTooOftenEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newConfigDriftMonitorStoppedTooOftenEventMatcher)

	newAddSigtermProtectionEventMatcher := newAddSigtermProtectionEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newAddSigtermProtectionEventMatcher)

	newRemoveSigtermProtectionEventMatcher := newRemoveSigtermProtectionEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(newRemoveSigtermProtectionEventMatcher)

	return registry
}

// NewUpgradePathologicalEventMatchers creates the registry for allowed events during upgrade.
// Contains everything in the universal set as well.
func NewUpgradePathologicalEventMatchers(kubeConfig *rest.Config, finalIntervals monitorapi.Intervals) *AllowedPathologicalEventRegistry {

	// Start with the main list of matchers:
	registry := NewUniversalPathologicalEventMatchers(kubeConfig, finalIntervals)

	// Now add in the matchers we only want to apply during upgrade:

	// Operators that use library-go can report about multiple versions during upgrades.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "OperatorMultipleVersions",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`(openshift-etcd-operator|openshift-kube-apiserver-operator|openshift-kube-controller-manager-operator|openshift-kube-scheduler-operator)`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`(etcd-operator|kube-apiserver-operator|kube-controller-manager-operator|openshift-kube-scheduler-operator)`),
		},
		messageReasonRegex: regexp.MustCompile(`^MultipleVersions$`),
		messageHumanRegex:  regexp.MustCompile(`multiple versions found, probably in transition`),
	})

	// etcd-quorum-guard can fail during upgrades.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "EtcdQuorumGuardReadinessProbe",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^etcd-quorum-guard.*`),
		},
		messageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
		messageHumanRegex:  regexp.MustCompile(`Readiness probe failed:`),
	})

	// etcd can have unhealthy members during an upgrade
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "EtcdUnhealthyMembers",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey:  regexp.MustCompile(`openshift-etcd-operator`),
			monitorapi.LocatorDeploymentKey: regexp.MustCompile(`etcd-operator`),
		},
		messageReasonRegex: regexp.MustCompile(`^UnhealthyEtcdMember$`),
		messageHumanRegex:  regexp.MustCompile(`unhealthy members`),
	})

	// Ignore NetworkNotReady repeat events.
	// This was originally linked to bugzilla: https://bugzilla.redhat.com/show_bug.cgi?id=1986370
	// The bug has been closed as NOTABUG.
	// We used to allow this for three namespaces (openshift-multus, openshift-e2e-loki, and openshift-network-diagnostics),
	// however a quick search of the intervals in bigquery shows this happening a ton in lots of namespaces,
	// and killing jobs when it does. Given the bug status, I am ignoring these events, whenever they occur, in
	// all upgrade jobs for now. - dgoodwin
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name:               "NetworkNotReady",
		messageReasonRegex: regexp.MustCompile(`^NetworkNotReady$`),
		messageHumanRegex:  regexp.MustCompile(`network is not ready: container runtime network not ready: NetworkReady=false reason:NetworkPluginNotReady message:Network plugin returns error: No CNI configuration file.*Has your network provider started\?`),
	})

	// Allow FailedScheduling repeat events during node upgrades:
	m := newFailedSchedulingDuringNodeUpdatePathologicalEventMatcher(finalIntervals)
	registry.AddPathologicalEventMatcherOrDie(m)

	// Prometheus pods may have readiness probe errors during upgrades.
	registry.AddPathologicalEventMatcherOrDie(&SimplePathologicalEventMatcher{
		name: "PrometheusReadinessProbeErrors",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-monitoring$`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^prometheus-k8s-[0,1]$`),
		},
		messageReasonRegex: regexp.MustCompile(`^` + string(monitorapi.UnhealthyReason) + `$`),
		messageHumanRegex:  regexp.MustCompile("Readiness probe errored"),
		jira:               "https://issues.redhat.com/browse/OCPBUGS-62703",
		/*
			05:50:32	openshift-monitoring	kubelet	prometheus-k8s-1
			Unhealthy
			Readiness probe errored: rpc error: code = NotFound desc = container is not created or running: checking if PID of 58577e7deb7b8ae87b8029b9988fa268613748d0743ce989748f27e52b199ef5 is running failed: container process not found

			05:53:52 (x25)	openshift-monitoring	kubelet	prometheus-k8s-0
			Unhealthy
			Readiness probe errored: rpc error: code = Unknown desc = command error: cannot register an exec PID: container is stopping, stdout: , stderr: , exit code -1

			11:44:16 (x56)	openshift-monitoring	kubelet	prometheus-k8s-0
			Unhealthy
			Readiness probe errored and resulted in unknown state: rpc error: code = Unknown desc = command error: cannot register an exec PID: container is stopping, stdout: , stderr: , exit code -1

			Readiness probes run during the lifecycle of the container, including termination.
			Prometheus pods may take some time to stop, and thus result in more kubelet pings than permitted by default (20).
			With a termination grace period of 600s, these pods may lead to probe errors (e.g. the web service is stopped but the process is still running), which is expected during upgrades.

			To address this, set the threshold to 100 (approximately 600 (termination period) / 5 (probe interval)), to allow for a high number of readiness probe errors during the upgrade, but not so high that we would miss a real problem.
			The job below hit ~60 readiness errors during the upgrade:
			https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-ci-4.20-upgrade-from-stable-4.19-e2e-aws-ovn-upgrade/1977094149035266048, which makes sense to ignore,
			However, the job below hit readiness errors 774 times during the upgrade:
			https://prow.ci.openshift.org/view/gs/test-platform-results/logs/periodic-ci-openshift-release-master-ci-4.19-upgrade-from-stable-4.18-e2e-metal-ovn-single-node-rt-upgrade-test/1975691393640697856, which should be caught.
		*/
		repeatThresholdOverride: 100,
	})

	return registry
}

// Some broken out matchers are re-used in a test for that specific event, keeping them as package vars
// for compile time protection.

var AllowOVNReadiness = &SimplePathologicalEventMatcher{
	name: "OVNReadinessProbeFailed",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-ovn-kubernetes`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`ovnkube-node-`),
	},
	messageReasonRegex: regexp.MustCompile(`^Unhealthy$`),
	messageHumanRegex:  regexp.MustCompile(`Readiness probe failed:`),
}

// Separated out in testBackoffPullingRegistryRedhatImage
var AllowImagePullFromRedHatRegistry = &SimplePathologicalEventMatcher{
	name:              "AllowImagePullBackOffFromRedHatRegistry",
	messageHumanRegex: regexp.MustCompile(`Back-off pulling image .*registry.redhat.io`),
}

// Separated out in testBackoffStartingFailedContainer
var AllowBackOffRestartingFailedContainer = &SimplePathologicalEventMatcher{
	name:               "AllowBackOffRestartingFailedContainer",
	messageReasonRegex: regexp.MustCompile(`^BackOff$`),
	messageHumanRegex:  regexp.MustCompile(`Back-off restarting failed container`),
}

// Separated out in testRequiredInstallerResourcesMissing
var EtcdRequiredResourcesMissing = &SimplePathologicalEventMatcher{
	name:               "EtcdRequiredResourcesMissing",
	messageReasonRegex: regexp.MustCompile(`^RequiredInstallerResourcesMissing$`),
}

// reason/OperatorStatusChanged Status for clusteroperator/etcd changed: Degraded message changed from "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: 2 of 3 members are available, ip-10-0-217-93.us-west-1.compute.internal is unhealthy" to "NodeControllerDegraded: All master nodes are ready\nEtcdMembersDegraded: No unhealthy members found"
var EtcdClusterOperatorStatusChanged = &SimplePathologicalEventMatcher{
	name: "EtcdClusterOperatorStatusChanged",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-etcd`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`^openshift-etcd`),
	},
	messageReasonRegex: regexp.MustCompile(`^OperatorStatusChanged$`),
	messageHumanRegex:  regexp.MustCompile(`Status for clusteroperator/etcd changed.*No unhealthy members found`),
}

// ProbeErrorTimeoutAwaitingHeaders matches events in specific namespaces such as:
// reason/ProbeError Readiness probe error: Get "https://10.130.0.15:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorTimeoutAwaitingHeaders = &SimplePathologicalEventMatcher{
	name: "ProbeErrorTimeoutAwaitingHeaders",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	messageReasonRegex: regexp.MustCompile(`^ProbeError$`),
	messageHumanRegex:  regexp.MustCompile(`Readiness probe error.*Client.Timeout exceeded while awaiting headers`),
}

// ProbeErrorConnectionRefused matches events in specific namespaces.
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorConnectionRefused = &SimplePathologicalEventMatcher{
	name: "ProbeErrorConnectionRefused",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	messageReasonRegex: regexp.MustCompile(`^ProbeError$`),
	messageHumanRegex:  regexp.MustCompile(`Readiness probe error.*connection refused`),
}

// ProbeErrorLiveness matches events in specific namespaces such as:
// Liveness probe error: Get "https://10.128.0.21:8443/healthz": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ProbeErrorLiveness = &SimplePathologicalEventMatcher{
	name: "ProbeErrorLiveness",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	messageReasonRegex: regexp.MustCompile(`^(ProbeError|Unhealthy)$`),
	messageHumanRegex:  regexp.MustCompile(`Liveness probe error.*Client.Timeout exceeded while awaiting headers`),
}

// ReadinessFailed matches events in specific namespaces such as:
// ...ReadinessFailed Get \"https://10.130.0.16:8443/healthz\": net/http: request canceled while waiting for connection (Client.Timeout exceeded while awaiting headers)
//
// These namespaces have their own tests where you'll see this matcher re-used with additional checks on the namespace.
var ReadinessFailed = &SimplePathologicalEventMatcher{
	name: "ReadinessFailed",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
		//monitorapi.LocatorPodKey:       regexp.MustCompile(`(openshift-config-operator|openshift-oauth-apiserver)`),
	},
	messageReasonRegex: regexp.MustCompile(`^ReadinessFailed$`),
	messageHumanRegex:  regexp.MustCompile(`Get.*healthz.*net/http.*request canceled while waiting for connection.*Client.Timeout exceeded`),
}

// Separated out in testNodeHasNoDiskPressure
var NodeHasNoDiskPressure = &SimplePathologicalEventMatcher{
	name:               "NodeHasNoDiskPressure",
	messageReasonRegex: regexp.MustCompile(`^NodeHasNoDiskPressure$`),
	messageHumanRegex:  regexp.MustCompile(`status is now: NodeHasNoDiskPressure`),
}

// Separated out in testNodeHasSufficientMemory
var NodeHasSufficientMemory = &SimplePathologicalEventMatcher{
	name:               "NodeHasSufficientMemory",
	messageReasonRegex: regexp.MustCompile(`^NodeHasSufficientMemory$`),
	messageHumanRegex:  regexp.MustCompile(`status is now: NodeHasSufficientMemory`),
}

// Separated out in testNodeHasSufficientPID
var NodeHasSufficientPID = &SimplePathologicalEventMatcher{
	name:               "NodeHasSufficientPID",
	messageReasonRegex: regexp.MustCompile(`^NodeHasSufficientPID$`),
	messageHumanRegex:  regexp.MustCompile(`status is now: NodeHasSufficientPID`),
}

// reason/FailedScheduling 0/6 nodes are available: 2 node(s) didn't match Pod's node affinity/selector, 2 node(s) didn't match pod anti-affinity rules, 2 node(s) were unschedulable. preemption: 0/6 nodes are available: 2 node(s) didn't match pod anti-affinity rules, 4 Preemption is not helpful for scheduling..
var FailedScheduling = &SimplePathologicalEventMatcher{
	name:               "FailedScheduling",
	messageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
	messageHumanRegex:  regexp.MustCompile(`nodes are available.*didn't match Pod's node affinity/selector`),
}

// Separated out in testErrorUpdatingEndpointSlices
var ErrorUpdatingEndpointSlices = &SimplePathologicalEventMatcher{
	name:               "ErrorUpdatingEndpointSlices",
	messageReasonRegex: regexp.MustCompile(`^FailedToUpdateEndpointSlices$`),
	messageHumanRegex:  regexp.MustCompile(`Error updating Endpoint Slices`),
}

// Separated out in testMarketplaceStartupProbeFailure
var MarketplaceStartupProbeFailure = &SimplePathologicalEventMatcher{
	name: "MarketplaceStartupProbeFailure",
	locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
		monitorapi.LocatorNamespaceKey: regexp.MustCompile(`openshift-marketplace`),
		monitorapi.LocatorPodKey:       regexp.MustCompile(`(community-operators|redhat-operators)-[a-z0-9-]+`),
	},
	messageHumanRegex: regexp.MustCompile(`Startup probe failed`),
}

// Separated out in auditloganalyzer
var KubeAPIServerAvoids500s = &SimplePathologicalEventMatcher{
	name:               "KubeAPIServerAvoids500s",
	messageReasonRegex: regexp.MustCompile(fmt.Sprintf("^%s$", monitorapi.ReasonKubeAPIServer500s)),
}

var CertificateRotation = &SimplePathologicalEventMatcher{
	name:               "CertificateRotation",
	messageReasonRegex: regexp.MustCompile(`^(CABundleUpdateRequired|SignerUpdateRequired|TargetUpdateRequired|CertificateUpdated|CertificateRemoved|CertificateUpdateFailed|CSRCreated|CSRApproved|CertificateRotationStarted|ClientCertificateCreated|NoValidCertificateFound)$`),
}

func newTwoNodeEtcdEndpointsConfigMissingEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	// Use topology-based detection for DualReplica (two-node fencing) clusters
	dualReplicaTopology := v1.DualReplicaTopologyMode
	return &SimplePathologicalEventMatcher{
		name: "EtcdEndpointsConfigMissingDuringTwoNodeTests",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-kube-apiserver-operator$`),
		},
		messageReasonRegex: regexp.MustCompile(`^ConfigMissing$`),
		messageHumanRegex:  regexp.MustCompile(`apiServerArguments\.etcd-servers has less than two live etcd endpoints`),
		topology:           &dualReplicaTopology,
	}
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

	namespace := monitorEvent.Locator.Keys[monitorapi.LocatorNamespaceKey]
	pod := monitorEvent.Locator.Keys[monitorapi.LocatorNamespaceKey]
	reason := monitorEvent.Message.Reason
	msg := monitorEvent.Message.HumanMessage
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

func IsDuringAPIServerProgressingOnSNO(topology string, events monitorapi.Intervals) monitorapi.EventIntervalMatchesFunc {
	if topology != "single" {
		return func(eventInterval monitorapi.Interval) bool { return false }
	}
	ocpKubeAPIServerProgressingInterval := events.Filter(func(interval monitorapi.Interval) bool {
		isNodeInstaller := interval.Message.Reason == monitorapi.NodeInstallerReason
		isOperatorSource := interval.Source == monitorapi.SourceOperatorState
		isKubeAPI := interval.Locator.Keys[monitorapi.LocatorClusterOperatorKey] == "kube-apiserver"

		isKubeAPIInstaller := isNodeInstaller && isOperatorSource && isKubeAPI
		isKubeAPIInstallProgressing := isKubeAPIInstaller && interval.Message.Annotations[monitorapi.AnnotationCondition] == "Progressing"

		return isKubeAPIInstallProgressing
	})

	return func(i monitorapi.Interval) bool {
		for _, progressingInterval := range ocpKubeAPIServerProgressingInterval {
			// Before and After are not inclusive, we buffer 1 second for that.
			if progressingInterval.From.Before(i.From.Add(time.Second)) && progressingInterval.To.After(i.To.Add(-1*time.Second)) {
				return true
			}
		}
		return false
	}
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
	matcher := &SimplePathologicalEventMatcher{
		name: "EtcdReadinessProbeFailuresPerRevisionChange",
		locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
			monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-etcd$`),
			monitorapi.LocatorPodKey:       regexp.MustCompile(`^etcd-guard-`),
		},
		messageReasonRegex: regexp.MustCompile(`^(Unhealthy|ProbeError)$`),
		messageHumanRegex:  regexp.MustCompile(`Readiness probe`),
	}
	if clientConfig == nil {
		// We were not given a kubeconfig likely because the caller is looking for runtime
		// checks for interesting events, and not actually evaluating which repeated pathologically yet.
		// In this case, return a matcher capable of flagging Etcd readiness probe errors
		// as interesting, but not allowing them to repeat pathologically.
		matcher.neverAllow = true
		return matcher, nil
	}
	operatorClient, err := operatorv1client.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	currentRevision, err := getBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
		return nil, err
	}
	repeatThresholdOverride := currentRevision * (60 / 5)
	logrus.WithFields(logrus.Fields{
		"etcdRevision":   currentRevision,
		"allowedRepeats": repeatThresholdOverride,
	}).Info("created toleration for etcd readiness probes per revision")
	matcher.repeatThresholdOverride = repeatThresholdOverride

	return matcher, nil
}

func newFailedSchedulingDuringNodeUpdatePathologicalEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	if finalIntervals == nil || len(finalIntervals) == 0 {
		// We were not given final intervals likely because the caller is looking for runtime
		// checks for interesting events, and not actually evaluating which repeated pathologically yet.
		// In this case, return a matcher capable of flagging FailedScheduling
		// as interesting, but not allowing them to repeat pathologically.
		return &SimplePathologicalEventMatcher{
			name:               "FailedSchedulingDuringNodeUpdate",
			messageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
			neverAllow:         true,
		}
	}

	// Filter out a list of NodeUpdate events, we use these to ignore some other potential pathological events that are
	// expected during NodeUpdate.
	nodeUpdateIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceNodeState &&
			eventInterval.Locator.Type == monitorapi.LocatorTypeNode &&
			eventInterval.Message.Annotations[monitorapi.AnnotationConstructed] == monitorapi.ConstructionOwnerNodeLifecycle &&
			eventInterval.Message.Annotations[monitorapi.AnnotationPhase] == "Update" &&
			strings.Contains(eventInterval.Message.Annotations[monitorapi.AnnotationRoles], "master")
	})
	logrus.Infof("found %d NodeUpdate intervals", len(nodeUpdateIntervals))
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "FailedSchedulingDuringNodeUpdate",
			messageReasonRegex: regexp.MustCompile(`^FailedScheduling$`),
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

func (ade *OverlapOtherIntervalsPathologicalEventMatcher) Name() string {
	return ade.delegate.Name()
}

func (ade *OverlapOtherIntervalsPathologicalEventMatcher) Matches(i monitorapi.Interval) bool {
	return ade.delegate.Matches(i)
}

func (ade *OverlapOtherIntervalsPathologicalEventMatcher) Allows(i monitorapi.Interval, topology v1.TopologyMode) bool {

	// Check the delegate matcher first, if it matches, proceed to additional checks
	if !ade.delegate.Allows(i, topology) {
		return false
	}

	// Match the pathological event if it overlaps with any of the given set of intervals.
	for _, nui := range ade.allowIfWithinIntervals {
		if nui.From.Before(i.From) && nui.To.After(i.To) {
			logrus.Infof("%s was found to overlap with %s, ignoring pathological event as they fall within range of specified intervals", i, nui)
			return true
		}
	}
	return false
}

func newTopologyAwareHintsDisabledDuringTaintTestsPathologicalEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {

	if finalIntervals == nil || len(finalIntervals) == 0 {
		// We were not given final intervals likely because the caller is looking for runtime
		// checks for interesting events, and not actually evaluating which repeated pathologically yet.
		// In this case, return a matcher capable of flagging TopologyAwareHintsDisabled
		// as interesting, but not allowing them to repeat pathologically.
		return &SimplePathologicalEventMatcher{
			name:               "TopologyAwareHintsDisabledDuringTaintManagerTests",
			messageReasonRegex: regexp.MustCompile(`^TopologyAwareHintsDisabled$`),
			neverAllow:         true,
		}
	}

	taintManagerTestIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "NoExecuteTaintManager")
	})

	// The original mechanism doesn't properly count the compensation factor the number of events that happened in the window that were batched
	// into a single, potentially later event count update by the recorder.
	// The following sequence of events is an example:
	// 1. t0 topology fail event counter - 16
	// 2. t1 NoTaintManager test start
	// 3. t2 NoTaintManager test end
	// 4. t3 topology fail event counter - 21
	//
	// For now we are going to allow this event in any job where NoExecuteTaintManager runs.
	// This will give critical green signal for 4.15 component readiness for now.
	matcher := &SimplePathologicalEventMatcher{
		name:               "TopologyAwareHintsDisabledDuringTaintManagerTests",
		messageReasonRegex: regexp.MustCompile(`^TopologyAwareHintsDisabled$`),
	}
	if len(taintManagerTestIntervals) > 0 {
		matcher.repeatThresholdOverride = 10000
	} else {
		matcher.neverAllow = true
	}

	return matcher
}

// Repeating events about pod creation are expected for snapshot options tests in vsphere csi driver.
// The tests change clusterCSIDriver object and have to rollout new pods to load new configuration.
func newVsphereConfigurationTestsRollOutTooOftenEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	configurationTestIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "snapshot options in clusterCSIDriver")
	})
	for i := range configurationTestIntervals {
		configurationTestIntervals[i].To = configurationTestIntervals[i].To.Add(time.Minute * 10)
		configurationTestIntervals[i].From = configurationTestIntervals[i].From.Add(time.Minute * -10)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name: "VsphereConfigurationTestsRollOutTooOften",
			locatorKeyRegexes: map[monitorapi.LocatorKey]*regexp.Regexp{
				monitorapi.LocatorNamespaceKey: regexp.MustCompile(`^openshift-cluster-csi-drivers$`),
			},
			messageReasonRegex: regexp.MustCompile(`(.*Create.*|.*Delete.*|.*Update.*)`),
			messageHumanRegex:  regexp.MustCompile(`(.*Create.*|.*Delete.*|.*Update.*)`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-42610",
		},
		allowIfWithinIntervals: configurationTestIntervals,
	}
}

// Ignore connection refused events during OCP APIServer or OAuth APIServer being down
func newSingleNodeConnectionRefusedEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	const (
		ocpAPINamespace      = "openshift-apiserver"
		ocpOAuthAPINamespace = "openshift-oauth-apiserver"
		defaultNamespace     = "default"

		bufferTime     = time.Second * 45
		bufferSourceID = "GeneratedSNOBufferInterval"
	)
	snoTopology := v1.SingleReplicaTopologyMode

	// Intervals are collected as they come to the monitorapi and the `from` and `to` is recorded at that point,
	// this works fine for most runs however for single node the events might be sent at irregular intervals.
	// This makes it hard to determine if connection refused errors are false positives,
	// here we collect intervals we know are acceptable for connection refused errors to occur for single node.
	bufferInterval := []monitorapi.Interval{}

	ocpAPISeverTargetDownIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {

		// If we find a graceful shutdown event, we create a buffer interval after shutdown to account
		// for the API Server coming back up, as well as a 5 second before `from` buffer to account for a
		// situation where the `event.from` falls exactly on the `interval.from` thus causing time.Before() logic to return false.
		if eventInterval.Source == monitorapi.APIServerGracefulShutdown && eventInterval.Message.Reason == monitorapi.GracefulAPIServerShutdown {
			temp := eventInterval
			temp.Locator = monitorapi.Locator{Type: bufferSourceID, Keys: temp.Locator.Keys}
			temp.Source = bufferSourceID
			temp.From = eventInterval.From.Add(time.Second * -5)
			temp.To = eventInterval.To.Add(bufferTime)
			bufferInterval = append(bufferInterval, temp)
		}

		isTargetDownAlert := eventInterval.Source == monitorapi.SourceAlert && eventInterval.Locator.Keys[monitorapi.LocatorAlertKey] == "TargetDown"
		identifiedSkipInterval := false

		switch eventInterval.Locator.Keys[monitorapi.LocatorNamespaceKey] {
		case ocpAPINamespace, ocpOAuthAPINamespace:
			identifiedSkipInterval = true
		case defaultNamespace:
			identifiedSkipInterval = strings.Contains(eventInterval.Message.HumanMessage, "apiserver")
		}

		return isTargetDownAlert && identifiedSkipInterval
	})
	ocpAPISeverTargetDownIntervals = append(ocpAPISeverTargetDownIntervals, bufferInterval...)
	sort.SliceStable(ocpAPISeverTargetDownIntervals, func(i, j int) bool {
		return ocpAPISeverTargetDownIntervals[i].To.Before(ocpAPISeverTargetDownIntervals[j].To)
	})
	if len(ocpAPISeverTargetDownIntervals) > 0 {
		logrus.Infof("found %d OCP APIServer TargetDown intervals", len(ocpAPISeverTargetDownIntervals))
	}
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:              "ConnectionErrorDuringSingleNodeAPIServerTargetDown",
			messageHumanRegex: regexp.MustCompile(`dial tcp .* connect: connection refused`),
			topology:          &snoTopology,
		},
		allowIfWithinIntervals: ocpAPISeverTargetDownIntervals,
	}
}

// We ignore pathological errors that happen during the kube-apiserver progressing interval on SNO. The primary errors
// that occur during this time are the kube-apiserver-operator waiting for etcd/installer to stabilize and if we're unlucky
// an operator might call out for leader and since the KAS is down, it'll trigger a restart since it can't get leader.
func newSingleNodeKubeAPIProgressingEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	snoTopology := v1.SingleReplicaTopologyMode

	ocpKubeAPIServerProgressingInterval := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {

		isNodeInstaller := eventInterval.Message.Reason == monitorapi.NodeInstallerReason
		isOperatorSource := eventInterval.Source == monitorapi.SourceOperatorState
		isKubeAPI := eventInterval.Locator.Keys[monitorapi.LocatorClusterOperatorKey] == "kube-apiserver"

		isKubeAPIInstaller := isNodeInstaller && isOperatorSource && isKubeAPI
		isKubeAPIInstallProgressing := isKubeAPIInstaller && eventInterval.Message.Annotations[monitorapi.AnnotationCondition] == "Progressing"

		return isKubeAPIInstallProgressing
	})

	// We buffer 1 second since Before and After are not inclusive for time comparisons.
	for i := range ocpKubeAPIServerProgressingInterval {
		ocpKubeAPIServerProgressingInterval[i].From = ocpKubeAPIServerProgressingInterval[i].From.Add(time.Second * -1)
		ocpKubeAPIServerProgressingInterval[i].To = ocpKubeAPIServerProgressingInterval[i].To.Add(time.Second * 1)
	}

	if len(ocpKubeAPIServerProgressingInterval) > 0 {
		logrus.Infof("found %d OCP Kube APIServer Progressing intervals", len(ocpKubeAPIServerProgressingInterval))
	}
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:              "KubeAPIServerProgressingDuringSingleNodeUpgrade",
			messageHumanRegex: regexp.MustCompile(`^(clusteroperator/kube-apiserver version .* changed from |Back-off restarting failed container|.*Client\.Timeout exceeded while awaiting headers)`),
			topology:          &snoTopology,
		},
		allowIfWithinIntervals: ocpKubeAPIServerProgressingInterval,
	}
}

func newDeferringOperatorNodeUpdateTooOftenEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	DeferringOperatorNodeUpdateIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "imagepolicy signature validation")
	})
	for i := range DeferringOperatorNodeUpdateIntervals {
		DeferringOperatorNodeUpdateIntervals[i].To = DeferringOperatorNodeUpdateIntervals[i].To.Add(time.Minute * 2)
		DeferringOperatorNodeUpdateIntervals[i].From = DeferringOperatorNodeUpdateIntervals[i].From.Add(time.Minute * -2)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "DeferringOperatorNodeUpdateTooOften",
			messageReasonRegex: regexp.MustCompile(`^DeferringOperatorNodeUpdate$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-52260",
		},
		allowIfWithinIntervals: DeferringOperatorNodeUpdateIntervals,
	}
}

func newAnnotationChangeTooOftenEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	AnnotationChangeIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "imagepolicy signature validation")
	})
	for i := range AnnotationChangeIntervals {
		AnnotationChangeIntervals[i].To = AnnotationChangeIntervals[i].To.Add(time.Minute * 10)
		AnnotationChangeIntervals[i].From = AnnotationChangeIntervals[i].From.Add(time.Minute * -10)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "AnnotationChangeTooOften",
			messageReasonRegex: regexp.MustCompile(`^AnnotationChange$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-58376",
		},
		allowIfWithinIntervals: AnnotationChangeIntervals,
	}
}

func newSetDesiredConfigTooOftenEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	SetDesiredConfigIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "imagepolicy signature validation")
	})
	for i := range SetDesiredConfigIntervals {
		SetDesiredConfigIntervals[i].To = SetDesiredConfigIntervals[i].To.Add(time.Minute * 10)
		SetDesiredConfigIntervals[i].From = SetDesiredConfigIntervals[i].From.Add(time.Minute * -10)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "SetDesiredConfigTooOften",
			messageReasonRegex: regexp.MustCompile(`^SetDesiredConfig$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-58376",
		},
		allowIfWithinIntervals: SetDesiredConfigIntervals,
	}
}

func newCrioReloadedTooOftenEventMatcher(finalInternals monitorapi.Intervals) EventMatcher {
	crioReloadedIntervals := finalInternals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "imagepolicy signature validation")
	})
	for i := range crioReloadedIntervals {
		crioReloadedIntervals[i].To = crioReloadedIntervals[i].To.Add(time.Minute * 10)
		crioReloadedIntervals[i].From = crioReloadedIntervals[i].From.Add(time.Minute * -10)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "CrioReloadedTooOften",
			messageReasonRegex: regexp.MustCompile(`^ServiceReload$`),
			messageHumanRegex:  regexp.MustCompile(`Service crio.service was reloaded.`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-52260",
		},
		allowIfWithinIntervals: crioReloadedIntervals,
	}
}

func newConfigDriftMonitorStoppedTooOftenEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	configDriftMonitorStoppedIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "SigstoreImageVerification")
	})
	for i := range configDriftMonitorStoppedIntervals {
		configDriftMonitorStoppedIntervals[i].To = configDriftMonitorStoppedIntervals[i].To.Add(time.Second * 30)
		configDriftMonitorStoppedIntervals[i].From = configDriftMonitorStoppedIntervals[i].From.Add(time.Second * -30)
	}

	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "ConfigDriftMonitorStoppedTooOften",
			messageReasonRegex: regexp.MustCompile(`^ConfigDriftMonitorStopped$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-63307",
		},
		allowIfWithinIntervals: configDriftMonitorStoppedIntervals,
	}
}

func newAddSigtermProtectionEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	AddSigtermProtectionIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "SigstoreImageVerification")
	})
	for i := range AddSigtermProtectionIntervals {
		AddSigtermProtectionIntervals[i].To = AddSigtermProtectionIntervals[i].To.Add(time.Second * 30)
		AddSigtermProtectionIntervals[i].From = AddSigtermProtectionIntervals[i].From.Add(time.Second * -30)
	}
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "AddSigtermProtection",
			messageReasonRegex: regexp.MustCompile(`^AddSigtermProtection$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-63307",
		},
		allowIfWithinIntervals: AddSigtermProtectionIntervals,
	}
}

func newRemoveSigtermProtectionEventMatcher(finalIntervals monitorapi.Intervals) EventMatcher {
	RemoveSigtermProtectionIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Source == monitorapi.SourceE2ETest &&
			strings.Contains(eventInterval.Locator.Keys[monitorapi.LocatorE2ETestKey], "SigstoreImageVerification")
	})
	for i := range RemoveSigtermProtectionIntervals {
		RemoveSigtermProtectionIntervals[i].To = RemoveSigtermProtectionIntervals[i].To.Add(time.Second * 30)
		RemoveSigtermProtectionIntervals[i].From = RemoveSigtermProtectionIntervals[i].From.Add(time.Second * -30)
	}
	return &OverlapOtherIntervalsPathologicalEventMatcher{
		delegate: &SimplePathologicalEventMatcher{
			name:               "RemoveSigtermProtection",
			messageReasonRegex: regexp.MustCompile(`^RemoveSigtermProtection$`),
			jira:               "https://issues.redhat.com/browse/OCPBUGS-63307",
		},
		allowIfWithinIntervals: RemoveSigtermProtectionIntervals,
	}
}
