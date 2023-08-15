package pathologicaleventlibrary

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	e2e "k8s.io/kubernetes/test/e2e/framework"

	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
)

func TestDuplicatedEventForUpgrade(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	allowedPatterns := []*regexp.Regexp{}
	allowedPatterns = append(allowedPatterns, AllowedRepeatedEventPatterns...)
	allowedPatterns = append(allowedPatterns, AllowedUpgradeRepeatedEventPatterns...)

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedPatterns,
		allowedRepeatedEventFns:      AllowedRepeatedEventFns,
		knownRepeatedEventsBugs:      KnownEventsBugs,
	}

	if err := evaluator.getClusterInfo(kubeClientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	if evaluator.topology == v1.SingleReplicaTopologyMode {
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, AllowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, kubeClientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, kubeClientConfig)...)
	return tests
}

func TestDuplicatedEventForStableSystem(events monitorapi.Intervals, clientConfig *rest.Config) []*junitapi.JUnitTestCase {

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: AllowedRepeatedEventPatterns,
		allowedRepeatedEventFns:      AllowedRepeatedEventFns,
		knownRepeatedEventsBugs:      KnownEventsBugs,
	}

	operatorClient, err := operatorv1client.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	etcdAllowance, err := newDuplicatedEventsAllowedWhenEtcdRevisionChange(context.TODO(), operatorClient)
	if err != nil {
		panic(fmt.Errorf("unable to construct duplicated events allowance for etcd, err = %v", err))
	}
	evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, etcdAllowance.allowEtcdGuardReadinessProbeFailure)

	if err := evaluator.getClusterInfo(clientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	if evaluator.topology == v1.SingleReplicaTopologyMode {
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, AllowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, clientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, clientConfig)...)
	return tests
}

func combinedRegexp(arr ...*regexp.Regexp) *regexp.Regexp {
	s := ""
	for _, r := range arr {
		if s != "" {
			s += "|"
		}
		s += r.String()
	}
	return regexp.MustCompile(s)
}

type duplicateEventsEvaluator struct {
	allowedRepeatedEventPatterns []*regexp.Regexp
	allowedRepeatedEventFns      []IsRepeatedEventOKFunc

	// knownRepeatedEventsBugs are duplicates that are considered bugs and should flake, but not  fail a Test
	knownRepeatedEventsBugs []KnownProblem

	// platform contains the current platform of the cluster under Test.
	platform v1.PlatformType

	// topology contains the topology of the cluster under Test.
	topology v1.TopologyMode
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] Test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedCoreNamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	return d.testDuplicatedEvents(testName, false, events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)), kubeClientConfig, false)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] Test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedE2ENamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically in e2e namespaces"

	return d.testDuplicatedEvents(testName, true, events.Filter(monitorapi.IsInE2ENamespace), kubeClientConfig, true)
}

// appendToFirstLine appends add to the end of the first line of s
func appendToFirstLine(s string, add string) string {
	splits := strings.Split(s, "\n")
	splits[0] += add
	return strings.Join(splits, "\n")
}

func getJUnitName(testName string, namespace string) string {
	jUnitName := testName
	if namespace != "" {
		jUnitName = jUnitName + " for ns/" + namespace
	}
	return jUnitName
}

func getNamespacesForJUnits() sets.String {
	namespaces := platformidentification.KnownNamespaces.Clone()
	namespaces.Insert("")
	return namespaces
}

type eventResult struct {
	failures []string
	flakes   []string
}

func generateFailureOutput(failures []string, flakes []string) string {
	var output string
	if len(failures) > 0 {
		output = fmt.Sprintf("%d events happened too frequently\n\n%v", len(failures), strings.Join(failures, "\n"))
	}
	if len(flakes) > 0 {
		if output != "" {
			output += "\n\n"
		}
		output += fmt.Sprintf("%d events with known BZs\n\n%v", len(flakes), strings.Join(flakes, "\n"))
	}
	return output
}

func generateJUnitTestCasesCoreNamespaces(testName string, nsResults map[string]*eventResult) []*junitapi.JUnitTestCase {
	var tests []*junitapi.JUnitTestCase
	namespaces := getNamespacesForJUnits()
	for namespace := range namespaces {
		jUnitName := getJUnitName(testName, namespace)
		if result, ok := nsResults[namespace]; ok {
			output := generateFailureOutput(result.failures, result.flakes)
			tests = append(tests, &junitapi.JUnitTestCase{
				Name: jUnitName,
				FailureOutput: &junitapi.FailureOutput{
					Output: output,
				},
			})
			// Add a success for flakes
			if len(result.failures) == 0 && len(result.flakes) > 0 {
				tests = append(tests, &junitapi.JUnitTestCase{Name: jUnitName})
			}
		} else {
			tests = append(tests, &junitapi.JUnitTestCase{Name: jUnitName})
		}
	}
	return tests
}

func generateJUnitTestCasesE2ENamespaces(testName string, nsResults map[string]*eventResult) []*junitapi.JUnitTestCase {
	var tests []*junitapi.JUnitTestCase
	if result, ok := nsResults[""]; ok {
		if len(result.failures) > 0 || len(result.flakes) > 0 {
			output := generateFailureOutput(result.failures, result.flakes)
			tests = append(tests, &junitapi.JUnitTestCase{
				Name: testName,
				FailureOutput: &junitapi.FailureOutput{
					Output: output,
				},
			})
		}
		if len(result.failures) == 0 {
			// Add success for flake
			tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
		}
	}
	if len(tests) == 0 {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] Test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedEvents(testName string, flakeOnly bool, events monitorapi.Intervals, kubeClientConfig *rest.Config, isE2E bool) []*junitapi.JUnitTestCase {

	type pathologicalEvents struct {
		count        int    // max number of times the message occurred
		eventMessage string // holds original message so you can compare with d.knownRepeatedEventsBugs
	}

	displayToCount := map[string]*pathologicalEvents{}
	for _, event := range events {
		eventDisplayMessage, times := GetTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > DuplicateEventThreshold {

			// If we marked this message earlier in recordAddOrUpdateEvent as interesting/true, we know it matched one of
			// the existing patterns or one of the AllowedRepeatedEventFns functions returned true.
			if strings.Contains(eventDisplayMessage, InterestingMark) {
				continue
			}

			eventMessageString := eventDisplayMessage + " From: " + event.From.Format("15:04:05Z") + " To: " + event.To.Format("15:04:05Z")
			if _, ok := displayToCount[eventMessageString]; !ok {
				tmp := &pathologicalEvents{
					count:        times,
					eventMessage: eventDisplayMessage,
				}
				displayToCount[eventMessageString] = tmp
			}
			if times > displayToCount[eventMessageString].count {
				displayToCount[eventMessageString].count = times
			}
		}
	}

	nsResults := map[string]*eventResult{}
	for msgWithTime, pathoItem := range displayToCount {
		namespace := monitorapi.NamespaceFromLocator(msgWithTime)
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", pathoItem.count, msgWithTime)
		flake := false
		for _, kp := range d.knownRepeatedEventsBugs {
			if kp.Regexp != nil && kp.Regexp.MatchString(pathoItem.eventMessage) {
				// Check if this exception only applies to our specific platform
				if kp.Platform != nil && *kp.Platform != d.platform {
					continue
				}

				// Check if this exception only applies to a specific topology
				if kp.Topology != nil && *kp.Topology != d.topology {
					continue
				}

				msg += " - " + kp.BZ
				flake = true
			}
		}

		// We only creates junit for known namespaces
		if !platformidentification.KnownNamespaces.Has(namespace) {
			namespace = ""
		}

		if _, ok := nsResults[namespace]; !ok {
			tmp := &eventResult{}
			nsResults[namespace] = tmp
		}
		if flake || flakeOnly {
			nsResults[namespace].flakes = append(nsResults[namespace].flakes, appendToFirstLine(msg, " result=allow "))
		} else {
			nsResults[namespace].failures = append(nsResults[namespace].failures, appendToFirstLine(msg, " result=reject "))
		}
	}

	var tests []*junitapi.JUnitTestCase
	if isE2E {
		tests = generateJUnitTestCasesE2ENamespaces(testName, nsResults)
	} else {
		tests = generateJUnitTestCasesCoreNamespaces(testName, nsResults)
	}
	return tests
}

func GetTimesAnEventHappened(message string) (string, int) {
	matches := EventCountExtractor.FindAllStringSubmatch(message, -1)
	if len(matches) != 1 { // not present or weird
		return "", 0
	}
	if len(matches[0]) < 2 { // no capture
		return "", 0
	}
	times, err := strconv.ParseInt(matches[0][2], 10, 0)
	if err != nil { // not an int somehow
		return "", 0
	}
	return matches[0][1], int(times)
}

func (d *duplicateEventsEvaluator) getClusterInfo(c *rest.Config) (err error) {
	if c == nil {
		return
	}

	oc, err := configclient.NewForConfig(c)
	if err != nil {
		return err
	}
	infra, err := oc.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return err
	}

	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type != "" {
		d.platform = infra.Status.PlatformStatus.Type
	}

	if infra.Status.ControlPlaneTopology != "" {
		d.topology = infra.Status.ControlPlaneTopology
	}

	return nil
}

type etcdRevisionChangeAllowance struct {
	allowedGuardProbeFailurePattern        *regexp.Regexp
	maxAllowedGuardProbeFailurePerRevision int

	currentRevision int
}

func newDuplicatedEventsAllowedWhenEtcdRevisionChange(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (*etcdRevisionChangeAllowance, error) {
	currentRevision, err := getBiggestRevisionForEtcdOperator(ctx, operatorClient)
	if err != nil {
		return nil, err
	}
	return &etcdRevisionChangeAllowance{
		allowedGuardProbeFailurePattern:        regexp.MustCompile(`ns/openshift-etcd pod/etcd-guard-.* node/[a-z0-9.-]+ - reason/(Unhealthy|ProbeError) Readiness probe.*`),
		maxAllowedGuardProbeFailurePerRevision: 60 / 5, // 60s for starting a new pod, divided by the probe interval
		currentRevision:                        currentRevision,
	}, nil
}

// allowEtcdGuardReadinessProbeFailure tolerates events that match allowedGuardProbeFailurePattern unless we receive more than a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision
func (a *etcdRevisionChangeAllowance) allowEtcdGuardReadinessProbeFailure(monitorEvent monitorapi.Interval, _ *rest.Config, times int) (bool, error) {
	eventMessage := fmt.Sprintf("%s - %s", monitorEvent.Locator, monitorEvent.Message)

	// allow for a.maxAllowedGuardProbeFailurePerRevision * a.currentRevision failed readiness probe from the etcd-guard pods
	// since the guards are static and the etcd pods come and go during a rollout
	// which causes allowedGuardProbeFailurePattern to fire
	if a.allowedGuardProbeFailurePattern.MatchString(eventMessage) && a.maxAllowedGuardProbeFailurePerRevision*a.currentRevision > times {
		return true, nil
	}
	return false, nil
}

// getBiggestRevisionForEtcdOperator calculates the biggest revision among replicas of the most recently successful deployment
func getBiggestRevisionForEtcdOperator(ctx context.Context, operatorClient operatorv1client.OperatorV1Interface) (int, error) {
	etcd, err := operatorClient.Etcds().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		// instead of panicking when there no etcd operator (e.g. microshift), just estimate the biggest revision to be 0
		if apierrors.IsNotFound(err) {
			return 0, nil
		} else {
			return 0, err
		}

	}
	biggestRevision := 0
	for _, nodeStatus := range etcd.Status.NodeStatuses {
		if int(nodeStatus.CurrentRevision) > biggestRevision {
			biggestRevision = int(nodeStatus.CurrentRevision)
		}
	}
	return biggestRevision, nil
}
