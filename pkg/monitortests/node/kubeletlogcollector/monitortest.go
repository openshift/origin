package kubeletlogcollector

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/monitortestlibrary/utility"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type kubeletLogCollector struct {
	adminRESTConfig *rest.Config
	startedAt       time.Time
	reducedTopology bool
}

func NewKubeletLogCollector() monitortestframework.MonitorTest {
	return &kubeletLogCollector{}
}

func (w *kubeletLogCollector) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

// StartCollection resolves the control plane topology up front, while the
// cluster is still healthy — evaluation runs after whatever disruption the suite
// caused, which is the worst moment to ask the apiserver a question.
func (w *kubeletLogCollector) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	w.startedAt = time.Now()

	reducedTopology, _, err := platformidentification.ResolveReducedTopology(ctx, adminRESTConfig)
	if err != nil {
		logrus.Warningf("kubelet-log-collector: couldn't determine control plane topology, treating it as reduced: %s", utility.ErrorSummary(err))
	}
	w.reducedTopology = reducedTopology

	return nil
}

func (w *kubeletLogCollector) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}
	// MicroShift does not have a proper journal for the node logs api.
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return nil, nil, err
	}
	if isMicroShift {
		return nil, nil, nil
	}

	intervals, err := intervalsFromNodeLogs(ctx, kubeClient, beginning, end)
	return intervals, nil, err
}

func (*kubeletLogCollector) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals produces the node log junits, flaking
// rather than failing the tests in reducedTopologyFlakedTests when the cluster
// cannot keep a quorum through a node reboot.
func (w *kubeletLogCollector) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	junits := []*junitapi.JUnitTestCase{}
	junits = append(junits, nodeFailedLeaseErrorsInRapidSuccession(w.startedAt, finalIntervals)...)
	junits = append(junits, nodeFailedLeaseErrorsBackOff(w.startedAt, finalIntervals)...)
	junits = append(junits, testNoSystemdCoreDumps(finalIntervals)...)
	junits = append(junits, nodeKubeletAndCrioPanicsInvariant(w.startedAt, finalIntervals)...)
	if w.reducedTopology {
		junits = ensureFlakeOnReducedTopology(junits, reducedTopologyFlakedTests)
	}
	return junits, nil
}

// reducedTopologyFlakedTests names the tests whose failures are expected on a
// control plane that loses quorum when a single node reboots.
var reducedTopologyFlakedTests = map[string]bool{
	"[sig-node] kubelet-log-collector detects node failed to lease events in rapid succession": true,
}

// ensureFlakeOnReducedTopology converts hard failures to flakes for tests expected
// to fail during disruptive recovery on DualReplica/SingleReplica topologies.
func ensureFlakeOnReducedTopology(junits []*junitapi.JUnitTestCase, flakedTests map[string]bool) []*junitapi.JUnitTestCase {
	failed := map[string]bool{}
	passed := map[string]bool{}
	for _, j := range junits {
		if j.FailureOutput != nil {
			failed[j.Name] = true
		} else {
			passed[j.Name] = true
		}
	}
	for name := range flakedTests {
		if failed[name] && !passed[name] {
			junits = append(junits, &junitapi.JUnitTestCase{Name: name})
		}
	}
	return junits
}

func (*kubeletLogCollector) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*kubeletLogCollector) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func nodeFailedLeaseErrorsInRapidSuccession(startedAt time.Time, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet-log-collector detects node failed to lease events in rapid succession"
	var failures []string

	intervalsToFailOn := findLeaseIntervalsImportant(finalIntervals)
	for _, event := range intervalsToFailOn {
		if event.From.After(startedAt) {
			failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("kubelet-log-collector reports %d node failed to lease events.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
		return tests
	} else {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
		return tests
	}
}

func nodeFailedLeaseErrorsBackOff(startedAt time.Time, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] kubelet-log-collector detected lease failures in backoff"
	var failures []string
	intervalsToFlake := findLeaseBackOffs(finalIntervals)
	for _, event := range intervalsToFlake {
		if event.From.After(startedAt) {
			failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}
	// failures during a run always fail the test suite
	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("kubelet-log-collector reports %d lease back off events.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	}

	// Mark as a flake and monitor in 4.18.
	tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	return tests
}

func testNoSystemdCoreDumps(events monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[Jira:\"Test Framework\"] should not find any systemd-coredump logs in system journal"
	success := &junitapi.JUnitTestCase{Name: testName}

	var failures []string
	processCount := make(map[string]int)

	for _, event := range events {
		if event.Source != monitorapi.SourceSystemdCoreDumpLog {
			continue
		}
		if strings.Contains(event.Message.HumanMessage, "dumped core") {
			processName := "unknown"
			if event.Message.Annotations != nil {
				if proc, exists := event.Message.Annotations["process"]; exists {
					processName = proc
				}
			}

			processCount[processName]++
			msg := fmt.Sprintf("%v - Process: %s - %v", event.Locator.OldLocator(), processName, event.Message.OldMessage())
			failures = append(failures, msg)
		}
	}

	if len(failures) == 0 {
		return []*junitapi.JUnitTestCase{success}
	}

	// Create summary of process failures
	processSummary := make([]string, 0, len(processCount))
	for process, count := range processCount {
		processSummary = append(processSummary, fmt.Sprintf("%s: %d occurrences", process, count))
	}

	failure := &junitapi.JUnitTestCase{
		Name:      testName,
		SystemOut: strings.Join(failures, "\n"),
		FailureOutput: &junitapi.FailureOutput{
			Output: fmt.Sprintf("Found %d core dumps from %d different processes. Process breakdown:\n%s\n\nDetailed events:\n%v",
				len(failures), len(processCount), strings.Join(processSummary, "\n"), strings.Join(failures, "\n")),
		},
	}

	return []*junitapi.JUnitTestCase{failure}
}

func nodeKubeletAndCrioPanicsInvariant(startedAt time.Time, finalIntervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	const testName = "[sig-node] node-system-log-collector detects kubelet or CRI-O panics"
	var failures []string

	intervalsToFailOn := findKubeletAndCrioPanics(finalIntervals)
	for _, event := range intervalsToFailOn {
		if event.From.After(startedAt) {
			failures = append(failures, fmt.Sprintf("%s %v - %v", event.From.Format(time.RFC3339), event.Locator.OldLocator(), event.Message.OldMessage()))
		}
	}

	var tests []*junitapi.JUnitTestCase
	if len(failures) > 0 {
		tests = append(tests, &junitapi.JUnitTestCase{
			Name:      testName,
			SystemOut: strings.Join(failures, "\n"),
			FailureOutput: &junitapi.FailureOutput{
				Output: fmt.Sprintf("kubelet-log-collector reports %d kubelet or CRI-O panics.\n\n%v", len(failures), strings.Join(failures, "\n")),
			},
		})
	} else {
		tests = append(tests, &junitapi.JUnitTestCase{Name: testName})
	}
	return tests
}
