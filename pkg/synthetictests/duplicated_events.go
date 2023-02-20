package synthetictests

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	v1 "github.com/openshift/api/config/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	operatorv1client "github.com/openshift/client-go/operator/clientset/versioned/typed/operator/v1"
	"github.com/openshift/origin/pkg/duplicateevents"
	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// Used to collect more Event Intervals for dupilcated messages that occurred more than
// some threshold.
var SyntheticTestIntervals *monitor.Monitor

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
	allowedRepeatedEventFns      []duplicateevents.IsRepeatedEventOKFunc

	// knownRepeatedEventsBugs are duplicates that are considered bugs and should flake, but not  fail a test
	knownRepeatedEventsBugs []duplicateevents.KnownProblem

	// platform contains the current platform of the cluster under test.
	platform v1.PlatformType

	// topology contains the topology of the cluster under test.
	topology v1.TopologyMode

	// testSuite contains the name of the test suite invoked.
	testSuite string
}

func testDuplicatedEventForUpgrade(events monitorapi.Intervals, kubeClientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {
	allowedPatterns := []*regexp.Regexp{}
	allowedPatterns = append(allowedPatterns, duplicateevents.AllowedRepeatedEventPatterns...)
	allowedPatterns = append(allowedPatterns, duplicateevents.AllowedUpgradeRepeatedEventPatterns...)

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: allowedPatterns,
		allowedRepeatedEventFns:      duplicateevents.AllowedRepeatedEventFns,
		knownRepeatedEventsBugs:      duplicateevents.KnownEventsBugs,
		testSuite:                    testSuite,
	}

	if err := evaluator.getClusterInfo(kubeClientConfig); err != nil {
		e2e.Logf("could not fetch cluster info: %w", err)
	}

	if evaluator.topology == v1.SingleReplicaTopologyMode {
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, duplicateevents.AllowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, kubeClientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, kubeClientConfig)...)
	return tests
}

func testDuplicatedEventForStableSystem(events monitorapi.Intervals, clientConfig *rest.Config, testSuite string) []*junitapi.JUnitTestCase {

	evaluator := duplicateEventsEvaluator{
		allowedRepeatedEventPatterns: duplicateevents.AllowedRepeatedEventPatterns,
		allowedRepeatedEventFns:      duplicateevents.AllowedRepeatedEventFns,
		knownRepeatedEventsBugs:      duplicateevents.KnownEventsBugs,
		testSuite:                    testSuite,
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
		evaluator.allowedRepeatedEventFns = append(evaluator.allowedRepeatedEventFns, duplicateevents.AllowedSingleNodeRepeatedEventFns...)
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, clientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, clientConfig)...)
	return tests
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedCoreNamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	return d.testDuplicatedEvents(testName, false, events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)), kubeClientConfig)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedE2ENamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically in e2e namespaces"

	return d.testDuplicatedEvents(testName, true, events.Filter(monitorapi.IsInE2ENamespace), kubeClientConfig)
}

// appendToFirstLine appends add to the end of the first line of s
func appendToFirstLine(s string, add string) string {
	splits := strings.Split(s, "\n")
	splits[0] += add
	return strings.Join(splits, "\n")
}

// originalMessageFromMessage takes a message from an EventInterval that had a duplicate message
// (i.e., ends with "(n times)") and removes all substrings added to it when it was processed by
// the RecordAddOrUpdateEvent function.
func originalMessageFromMessage(messageString string) string {
	partsRe := []*regexp.Regexp{
		regexp.MustCompile(`node/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`(namespace|ns)/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`pod/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`container/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`clusteroperator/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`e2e-testa/[a-zA-Z0-9-]+ `),
		regexp.MustCompile(`reason/[a-zA-Z0-9]+ `),
		regexp.MustCompile(`alert/[a-zA-Z0-9]+ `),
		regexp.MustCompile(`roles/[a-zA-Z0-9,-]+ `),
		regexp.MustCompile(fmt.Sprintf("%s ", duplicateevents.PathologicalMark)),
		regexp.MustCompile(fmt.Sprintf("%s ", duplicateevents.InterestingMark)),
		regexp.MustCompile(` \([1-9][0-9]+ times\)`),
	}

	originalMessage := messageString
	for _, partsRegexp := range partsRe {
		originalMessage = partsRegexp.ReplaceAllString(originalMessage, "")
	}
	return originalMessage
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d duplicateEventsEvaluator) testDuplicatedEvents(testName string, flakeOnly bool, events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {

	type pathologicalEvents struct {
		count                       int                      // max number of times the message occurred
		originalEventDisplayMessage string                   // holds original eventDisplayMessage so you can compare with d.knownRepeatedEventsBugs
		pathologicalEvent           monitorapi.EventInterval // holds the first event with pathological/true and times > 20 so we can search previous instances of this event
	}

	var failures []string
	displayToCount := map[string]*pathologicalEvents{}
	fmt.Println("testDuplicatedEvents: ", len(events))
	for _, event := range events {
		eventDisplayMessage, times := getTimesAnEventHappened(fmt.Sprintf("%s - %s", event.Locator, event.Message))
		if times > duplicateevents.DuplicateEventThreshold {

			// If we marked this message earlier in RecordAddOrUpdateEvent as interesting/true, we know it matched one of
			// the existing patterns or one of the AllowedRepeatedEventFns functions returned true.
			if strings.Contains(eventDisplayMessage, duplicateevents.InterestingMark) {
				continue
			}

			eventMessageString := eventDisplayMessage + " From: " + event.From.Format("15:04:05Z") + " To: " + event.To.Format("15:04:05Z")
			fmt.Println("Processing eventMessageString: ", eventMessageString)
			if _, ok := displayToCount[eventMessageString]; !ok {
				tmp := &pathologicalEvents{
					count:                       times,
					originalEventDisplayMessage: eventDisplayMessage,
					pathologicalEvent:           event,
				}
				displayToCount[eventMessageString] = tmp
			}
			if times > displayToCount[eventMessageString].count {
				displayToCount[eventMessageString].count = times
			}
		}
	}

	fmt.Println("Processing displayToCount: ", len(displayToCount))
	var flakes []string
	for msgWithTime, pathoItem := range displayToCount {
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v", pathoItem.count, msgWithTime)
		flake := false
		for _, kp := range d.knownRepeatedEventsBugs {
			if kp.Regexp != nil && kp.Regexp.MatchString(pathoItem.originalEventDisplayMessage) {
				// Check if this exception only applies to our specific platform
				if kp.Platform != nil && *kp.Platform != d.platform {
					continue
				}

				// Check if this exception only applies to a specific topology
				if kp.Topology != nil && *kp.Topology != d.topology {
					continue
				}

				// Check if this exception only applies to a specific test suite
				if kp.TestSuite != nil && *kp.TestSuite != d.testSuite {
					continue
				}

				msg += " - " + kp.BZ
				flake = true
			}
		}

		if flake || flakeOnly {
			flakes = append(flakes, appendToFirstLine(msg, " result=allow "))
			fmt.Println("Found patho flake")
		} else {
			fmt.Println("Found patho failure")
			failures = append(failures, appendToFirstLine(msg, " result=reject "))
		}

		pathoEvent := pathoItem.pathologicalEvent
		fmt.Println("Checking pathoEvent.Message: ", pathoEvent.Message)

		if strings.Contains(pathoEvent.Message, duplicateevents.PathologicalMark) && !strings.Contains(pathoEvent.Message, duplicateevents.InterestingMark) {

			fmt.Println("Processing event with pathologicalTrue but no interesting mark")

			// This event occurred more than threshold but is not recognized (i.e., unknown).
			// So we need to mark all other instances of this event so we can chart when this
			// event started happening.

			clientSet, clientErr := kubernetes.NewForConfig(kubeClientConfig)
			if clientErr != nil {
				fmt.Printf("Unexpected error getting AdminConfig for event-raw-events: %v\n", clientErr)
				fmt.Println("Checking for previous pathological events will be skipped")
			} else {
				// Narrow the search by namespace.  For some events (e.g., node related), there
				// will be no namespace and so we end up searching all events.
				pathoEventNamespace := monitorapi.NamespaceFromLocator(pathoEvent.Locator)
				coreEvents, coreEventsErr := clientSet.CoreV1().Events(pathoEventNamespace).List(context.TODO(), metav1.ListOptions{})
				if coreEventsErr != nil {
					fmt.Printf("Unexpected error getting events from %s: %v\n", pathoEventNamespace, coreEventsErr)
					fmt.Println("Going back through events will be skipped")
				} else {
					fmt.Printf("Going back through events for %s with length %d\n", pathoEventNamespace, len(coreEvents.Items))
					matchCount := 0
					for i, coreEvent := range coreEvents.Items {
						coreEventObjName := "initially empty match"
						if len(coreEvent.InvolvedObject.Name) > 0 {
							coreEventObjName = coreEvent.InvolvedObject.Name
						}

						originalPathoMessage := originalMessageFromMessage(pathoEvent.Message)

						// Use the same algorithm used in RecordAddOrUpdateEvent to get the From/To time.
						coreEventFrom := coreEvent.LastTimestamp.Time
						if coreEventFrom.IsZero() {
							coreEventFrom = coreEvent.EventTime.Time
						}
						if coreEventFrom.IsZero() {
							coreEventFrom = coreEvent.CreationTimestamp.Time
						}
						coreEventTo := coreEventFrom.Add(1 * time.Second)

						fmt.Println("Counter: ", i)
						fmt.Printf("pathoEvent.Locator: %s\n", pathoEvent.Locator)
						fmt.Printf("coreEvent.objectName: %s\n", coreEventObjName)
						fmt.Println()
						fmt.Printf("coreEvent.Count: %d; pathoItem.count: %d\n", coreEvent.Count, pathoItem.count)
						fmt.Printf("pathoEvent.Reason: %s ; coreEvent.Reason: %s\n", monitorapi.ReasonFrom(pathoEvent.Message), coreEvent.Reason)
						fmt.Println()
						fmt.Printf("pathoEvent originalMessage: %s\n", originalPathoMessage)
						fmt.Printf("coreEvent.Message: %s\n", coreEvent.Message)
						fmt.Println()
						fmt.Printf("coreEvent.Firsttimestamp: %s ; pathoEvent.from: %s\n", coreEvent.FirstTimestamp.Time.Format("15:04:05"), pathoEvent.From.Format("15:04:05"))
						if monitorapi.ReasonFrom(pathoEvent.Message) == coreEvent.Reason &&
							coreEvent.Message == originalPathoMessage &&
							strings.Contains(pathoEvent.Locator, coreEventObjName) &&
							!(coreEventFrom == pathoEvent.From && coreEventTo == pathoEvent.To) {

							// This event is an occurrence of the unknown event that occurred more than
							// threshold times. Process it the same way we processed the original message
							// so that it can be charted.
							matchCount++
							fmt.Printf("Found a times match at: %d ; total: %d\n", i, matchCount)

							significantlyBeforeNow := pathoEvent.From.UTC().Add(-15 * time.Minute)

							// We pass in a different monitor so we can record EventIntervals that happened during
							// this synthetic test.  We pass threshold as 0 so we can mark this EventInterval as
							// red (Unknown)
							monitor.RecordAddOrUpdateEvent(context.TODO(), SyntheticTestIntervals, clientSet, monitor.ReMatchFirstQuote, significantlyBeforeNow, &coreEvent, 0)
						}
					}
				}
			}
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	fmt.Printf("testDuplicatedEvents, failures: %d ; flakes: %d\n", len(failures), len(flakes))
	if len(failures) > 0 || len(flakes) > 0 {
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
		tests = append(tests, &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Output: output,
			},
		})
	}

	if len(tests) == 0 || len(failures) == 0 {
		// Add a successful result to mark the test as flaky if there are no
		// unknown problems.
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}

func getTimesAnEventHappened(message string) (string, int) {
	matches := duplicateevents.EventCountExtractor.FindAllStringSubmatch(message, -1)
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
func (a *etcdRevisionChangeAllowance) allowEtcdGuardReadinessProbeFailure(monitorEvent monitorapi.EventInterval, _ *rest.Config, times int) (bool, error) {
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
