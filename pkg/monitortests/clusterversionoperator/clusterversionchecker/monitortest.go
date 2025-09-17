package clusterversionchecker

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
)

type monitor struct {
	notSupportedReason error
	summary            map[string]int
}

func NewClusterVersionChecker() monitortestframework.MonitorTest {
	return &monitor{summary: map[string]int{}}
}

func (w *monitor) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "platform MicroShift not supported"}
		return w.notSupportedReason
	}

	nodes, err := kubeClient.CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		return fmt.Errorf("unable to list nodes: %v", err)
	}

	if s := len(nodes.Items); s > 250 {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: fmt.Sprintf("cluster with %d worker nodes (over 250) not supported", s)}
		return w.notSupportedReason
	}

	return nil
}

func (w *monitor) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return w.notSupportedReason
}

func (w *monitor) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, w.notSupportedReason
}

func (w *monitor) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *monitor) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	return w.noFailingUnknownCondition(finalIntervals), nil
}

func (w *monitor) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	outputFile := filepath.Join(storageDir, "cluster-version-checker.json")
	data, err := json.Marshal(w.summary)
	if err != nil {
		return fmt.Errorf("unable to marshal summary: %w", err)
	}
	if err := os.WriteFile(outputFile, data, 0644); err != nil {
		return fmt.Errorf("unable to write summary to %q: %w", outputFile, err)
	}
	return nil
}

func (*monitor) Cleanup(ctx context.Context) error {
	return nil
}

func (w *monitor) noFailingUnknownCondition(intervals monitorapi.Intervals) []*junitapi.JUnitTestCase {
	noFailures := &junitapi.JUnitTestCase{
		Name: "[bz-Cluster Version Operator] cluster version should not report Failing=Unknown during a normal cluster upgrade",
	}

	var failures []string
	violations := sets.New[string]()

	for _, interval := range intervals {
		cvName, ok := interval.Locator.Keys[monitorapi.LocatorClusterVersionKey]
		if !ok || cvName != "version" {
			continue
		}

		c := monitorapi.GetOperatorConditionStatus(interval)
		if c == nil {
			continue
		}
		key := fmt.Sprintf("%s-%s-%s", string(c.Type), string(c.Status), c.Reason)
		if _, ok := w.summary[key]; ok {
			w.summary[key]++
		} else {
			w.summary[key] = 1
		}
		// https://github.com/openshift/cluster-version-operator/blob/28a376a13ad1daec926f6174ac37ada2bd32c071/pkg/cvo/status.go#L332-L333
		if c.Type == "Failing" && c.Status == configv1.ConditionUnknown && c.Reason == "SlowClusterOperator" {
			// This is too hacky, but we do not have API to expose the CO names that took long to upgrade
			coNames, err := parseClusterOperatorNames(c.Message)
			if err != nil {
				failures = append(failures, fmt.Sprintf("failed to parse cluster operator names from message %q: %v", c.Message, err))
				continue
			}
			violations = violations.Union(coNames)
		}
	}

	if len(failures) > 0 {
		noFailures.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("Checking cluster version failed %d times (of %d intervals)", len(failures), len(intervals)),
			Output:  strings.Join(failures, "\n"),
		}
	}

	ret := []*junitapi.JUnitTestCase{noFailures}

	except := func(coName string) string {
		return ""
	}

	for _, coName := range sets.List(violations) {
		if reason := except(coName); reason == "" {
			bzComponent := platformidentification.GetBugzillaComponentForOperator(coName)
			if bzComponent == "Unknown" {
				bzComponent = coName
			}

			m := 30
			if coName == "machine-config" {
				m = 3 * m
			}
			output := fmt.Sprintf("Cluster Operator %s too over %d minutes to complete version change", coName, m)
			ret = append(ret, &junitapi.JUnitTestCase{
				Name: fmt.Sprintf("[bz-%v] clusteroperator/%v must complete version change under 20 minutes", bzComponent, coName),
				FailureOutput: &junitapi.FailureOutput{
					Output:  output,
					Message: output,
				},
			})
		}
	}

	return ret
}

func parseClusterOperatorNames(message string) (sets.Set[string], error) {
	// we have to modify the keyword here accordingly if CVO changes the message
	if !strings.Contains(message, "which is longer than expected") {
		return nil, fmt.Errorf("failed to parse cluster operator names from %q", message)
	}
	i := strings.Index(message, "waiting on ")
	if i == -1 {
		return nil, fmt.Errorf("failed to parse cluster operator names from %q", message)
	}
	ret := sets.Set[string]{}
	if strings.Contains(message, "machine-config over 90 minutes") {
		ret.Insert("machine-config")
	}
	part := message[i+len("waiting on "):]
	if j := strings.Index(part, " over 30 minutes"); j > -1 {
		coNames := strings.Split(part[:j], ",")
		for _, coName := range coNames {
			ret.Insert(strings.TrimSpace(coName))
		}
	}
	return ret, nil
}
