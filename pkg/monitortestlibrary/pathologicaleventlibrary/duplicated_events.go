package pathologicaleventlibrary

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/sirupsen/logrus"

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

func TestDuplicatedEventForUpgrade(
	events monitorapi.Intervals,
	kubeClientConfig *rest.Config,
	clusterStabilityDuringTest *monitortestframework.ClusterStabilityDuringTest,
) []*junitapi.JUnitTestCase {
	registry := NewUpgradePathologicalEventMatchers(kubeClientConfig, events, clusterStabilityDuringTest)

	evaluator := duplicateEventsEvaluator{
		registry: registry,
	}

	platform, topology, err := GetClusterInfraInfo(kubeClientConfig)
	if err != nil {
		logrus.WithError(err).Error("could not fetch cluster infra info")
	} else {
		// These could be coming out "" in theory
		evaluator.platform = platform
		evaluator.topology = topology
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, kubeClientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, kubeClientConfig)...)
	return tests
}

func TestDuplicatedEventForStableSystem(
	events monitorapi.Intervals,
	clientConfig *rest.Config,
	clusterStabilityDuringTest *monitortestframework.ClusterStabilityDuringTest,
) []*junitapi.JUnitTestCase {
	registry := NewUniversalPathologicalEventMatchers(clientConfig, events, clusterStabilityDuringTest)

	evaluator := duplicateEventsEvaluator{
		registry: registry,
	}

	platform, topology, err := GetClusterInfraInfo(clientConfig)
	if err != nil {
		logrus.WithError(err).Error("could not fetch cluster infra info")
	} else {
		// These could be coming out "" in theory
		evaluator.platform = platform
		evaluator.topology = topology
	}

	tests := []*junitapi.JUnitTestCase{}
	tests = append(tests, evaluator.testDuplicatedCoreNamespaceEvents(events, clientConfig)...)
	tests = append(tests, evaluator.testDuplicatedE2ENamespaceEvents(events, clientConfig)...)
	return tests
}

type duplicateEventsEvaluator struct {
	registry *AllowedPathologicalEventRegistry

	// platform contains the current platform of the cluster under Test.
	platform v1.PlatformType

	// topology contains the topology of the cluster under Test.
	topology v1.TopologyMode
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] Test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d *duplicateEventsEvaluator) testDuplicatedCoreNamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
	const testName = "[sig-arch] events should not repeat pathologically"

	return d.testDuplicatedEvents(testName, false, events.Filter(monitorapi.Not(monitorapi.IsInE2ENamespace)), kubeClientConfig, false)
}

// we want to identify events based on the monitor because it is (currently) our only spot that tracks events over time
// for every run. this means we see events that are created during updates and in e2e tests themselves.  A [late] Test
// is easier to author, but less complete in its view.
// I hate regexes, so I only do this because I really have to.
func (d *duplicateEventsEvaluator) testDuplicatedE2ENamespaceEvents(events monitorapi.Intervals, kubeClientConfig *rest.Config) []*junitapi.JUnitTestCase {
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
func (d *duplicateEventsEvaluator) testDuplicatedEvents(testName string, flakeOnly bool, events monitorapi.Intervals, kubeClientConfig *rest.Config, isE2E bool) []*junitapi.JUnitTestCase {

	// displayToCount maps a static display message to the matching repeating interval we saw with the highest count
	displayToCount := map[string]monitorapi.Interval{}

	for _, event := range events {

		times := GetTimesAnEventHappened(event.Message)
		if times > DuplicateEventThreshold {

			// Check if we have an allowance for this event. This code used to just check if it had an interesting flag,
			// implying it matches some pattern, but that happens even for upgrade patterns occurring in non-upgrade jobs,
			// so we were ignoring patterns that were meant to be allowed only in upgrade jobs in all jobs. The list of
			// allowed patterns passed to this object wasn't even used.
			if allowed, _ := d.registry.AllowedByAny(event, d.topology); allowed {
				continue
			}

			// key used in a map to identify the common interval that is repeating and we may
			// encounter multiple times.
			eventDisplayMessage := fmt.Sprintf("%s - reason/%s %s", event.Locator.OldLocator(),
				event.Message.Reason, event.Message.HumanMessage)

			if _, ok := displayToCount[eventDisplayMessage]; !ok {
				displayToCount[eventDisplayMessage] = event
			}
			if times > GetTimesAnEventHappened(displayToCount[eventDisplayMessage].Message) {
				// Update to the latest interval we saw with the higher count, so from/to are more accurate
				displayToCount[eventDisplayMessage] = event
			}
		}
	}

	nsResults := map[string]*eventResult{}
	for intervalDisplayMsg, interval := range displayToCount {
		namespace := interval.Locator.Keys[monitorapi.LocatorNamespaceKey]
		intervalMsgWithTime := intervalDisplayMsg + " (" + interval.From.Format("15:04:05Z") + ")"
		msg := fmt.Sprintf("event happened %d times, something is wrong: %v",
			GetTimesAnEventHappened(interval.Message), intervalMsgWithTime)

		// We only create junit for known namespaces
		if !platformidentification.KnownNamespaces.Has(namespace) {
			namespace = ""
		}

		if _, ok := nsResults[namespace]; !ok {
			tmp := &eventResult{}
			nsResults[namespace] = tmp
		}
		if flakeOnly {
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

func GetTimesAnEventHappened(msg monitorapi.Message) int {
	countStr, ok := msg.Annotations[monitorapi.AnnotationCount]
	if !ok {
		return 1
	}
	times, err := strconv.ParseInt(countStr, 10, 0)
	if err != nil { // not an int somehow
		logrus.Warnf("interval had a non-integer count? %+v", msg)
		return 0
	}
	return int(times)
}

func GetClusterInfraInfo(c *rest.Config) (platform v1.PlatformType, topology v1.TopologyMode, err error) {
	if c == nil {
		return
	}

	oc, err := configclient.NewForConfig(c)
	if err != nil {
		return "", "", err
	}
	infra, err := oc.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	if infra.Status.PlatformStatus != nil && infra.Status.PlatformStatus.Type != "" {
		platform = infra.Status.PlatformStatus.Type
	}

	if infra.Status.ControlPlaneTopology != "" {
		topology = infra.Status.ControlPlaneTopology
	}

	return platform, topology, nil
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

// BuildTestDupeKubeEvent is a test utility to make the process of creating these specific intervals a little
// more brief.
func BuildTestDupeKubeEvent(namespace, pod, reason, msg string, count int) monitorapi.Interval {
	l := monitorapi.NewLocator().PodFromNames(namespace, pod, "")

	i := monitorapi.NewInterval(monitorapi.SourceKubeEvent, monitorapi.Info).
		Locator(l).
		Message(
			monitorapi.NewMessage().
				Reason(monitorapi.IntervalReason(reason)).
				HumanMessage(msg).
				WithAnnotation(monitorapi.AnnotationCount, fmt.Sprintf("%d", count))).
		BuildNow()

	return i
}
