package admupgradestatus

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	clientconfigv1 "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitortestframework"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

type snapshot struct {
	when time.Time
	out  string
	err  error
}

type outputModel struct {
	when   time.Time
	output *upgradeStatusOutput
}

type monitor struct {
	collectionDone chan struct{}

	ocAdmUpgradeStatus             []snapshot
	ocAdmUpgradeStatusOutputModels []outputModel

	notSupportedReason error
	isSNO              bool
}

func NewOcAdmUpgradeStatusChecker() monitortestframework.MonitorTest {
	return &monitor{
		collectionDone:     make(chan struct{}),
		ocAdmUpgradeStatus: make([]snapshot, 0, 60), // expect 60 minutes of hourly snapshots
	}
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
	clientconfigv1client, err := clientconfigv1.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	if ok, err := exutil.IsHypershift(ctx, clientconfigv1client); err != nil {
		return fmt.Errorf("unable to determine if cluster is Hypershift: %v", err)
	} else if ok {
		w.notSupportedReason = &monitortestframework.NotSupportedError{Reason: "platform Hypershift not supported"}
		return w.notSupportedReason
	}

	if ok, err := exutil.IsSingleNode(ctx, clientconfigv1client); err != nil {
		return fmt.Errorf("unable to determine if cluster is single node: %v", err)
	} else {
		w.isSNO = ok
	}

	return nil
}

func snapshotOcAdmUpgradeStatus(ch chan *snapshot) {
	// TODO: I _think_ this should somehow use the adminRESTConfig given to StartCollection but I don't know how to
	//       how to do pass that to exutil.NewCLI* or if it is even possible. It seems to work this way though.
	oc := exutil.NewCLIWithoutNamespace("adm-upgrade-status").AsAdmin()
	now := time.Now()

	var out string
	var err error
	// retry on brief apiserver unavailability
	if errWait := wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 2*time.Minute, true, func(context.Context) (bool, error) {
		cmd := oc.Run("adm", "upgrade", "status", "--details=all").EnvVar("OC_ENABLE_CMD_UPGRADE_STATUS", "true")
		out, err = cmd.Output()
		if err != nil {
			return false, nil
		}
		return true, nil
	}); errWait != nil {
		out = ""
		err = errWait
	}
	ch <- &snapshot{when: now, out: out, err: err}
}

func (w *monitor) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	if w.notSupportedReason != nil {
		return w.notSupportedReason
	}
	// TODO: The double goroutine spawn should probably be placed under some abstraction
	go func(ctx context.Context) {
		snapshots := make(chan *snapshot)
		go func() {
			for snap := range snapshots {
				// TODO: Maybe also collect some cluster resources (CV? COs?) through recorder?
				w.ocAdmUpgradeStatus = append(w.ocAdmUpgradeStatus, *snap)
			}
			w.collectionDone <- struct{}{}
		}()
		// TODO: Configurable interval?
		// TODO: Collect multiple invocations (--details)? Would need more another producer/consumer pair and likely
		//       collectionDone would need to be a WaitGroup

		wait.UntilWithContext(ctx, func(ctx context.Context) { snapshotOcAdmUpgradeStatus(snapshots) }, time.Minute)
		// The UntilWithContext blocks until the framework cancels the context when it wants tests to stop -> when we
		// get here, we know last snapshotOcAdmUpgradeStatus producer wrote to the snapshots channel, we can close it
		// which in turn will allow the consumer to finish and signal collectionDone.
		close(snapshots)
	}(ctx)

	return nil
}

func (w *monitor) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, nil, w.notSupportedReason
	}

	// The framework cancels the context it gave StartCollection before it calls CollectData, but we need to wait for
	// the collection goroutines spawned in StartedCollection to finish
	<-w.collectionDone

	sort.Slice(w.ocAdmUpgradeStatus, func(i, j int) bool {
		return w.ocAdmUpgradeStatus[i].when.Before(w.ocAdmUpgradeStatus[j].when)
	})

	// TODO: Maybe utilize Intervals somehow and do tests in ComputeComputedIntervals and EvaluateTestsFromConstructedIntervals

	return nil, []*junitapi.JUnitTestCase{
		w.noFailures(),
		w.expectedLayout(),
		w.controlPlane(),
		w.workers(),
		w.health(),
	}, nil
}

func (w *monitor) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, w.notSupportedReason
}

func (w *monitor) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.notSupportedReason != nil {
		return nil, w.notSupportedReason
	}
	return nil, nil
}

func (w *monitor) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	folderPath := path.Join(storageDir, "adm-upgrade-status")
	if err := os.MkdirAll(folderPath, os.ModePerm); err != nil {
		return fmt.Errorf("unable to create directory %s: %w", folderPath, err)
	}

	var errs []error
	for when, observed := range w.ocAdmUpgradeStatus {
		outputFilename := fmt.Sprintf("adm-upgrade-status-%s_%s.txt", when, timeSuffix)
		outputFile := filepath.Join(folderPath, outputFilename)
		if err := os.WriteFile(outputFile, []byte(observed.out), 0644); err != nil {
			errs = append(errs, fmt.Errorf("failed to write %s: %w", outputFile, err))
		}
	}
	return errors.NewAggregate(errs)
}

func (*monitor) Cleanup(ctx context.Context) error {
	return nil
}

func (w *monitor) noFailures() *junitapi.JUnitTestCase {
	noFailures := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status never fails",
	}

	var failures []string
	var total int
	for _, snap := range w.ocAdmUpgradeStatus {
		total++
		if snap.err != nil {
			failures = append(failures, fmt.Sprintf("- %s: %v", snap.when.Format(time.RFC3339), snap.err))
		}
	}

	// Zero failures is too strict for at least SNO clusters
	p := (len(failures) / total) * 100
	if (!w.isSNO && p > 0) || (w.isSNO && p > 10) {
		noFailures.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("oc adm upgrade status failed %d times (of %d)", len(failures), len(w.ocAdmUpgradeStatus)),
			Output:  strings.Join(failures, "\n"),
		}
	}
	return noFailures
}

func (w *monitor) expectedLayout() *junitapi.JUnitTestCase {
	expectedLayout := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status output has expected layout",
		SkipMessage: &junitapi.SkipMessage{
			Message: "Test skipped because no oc adm upgrade status output was successfully collected",
		},
	}

	w.ocAdmUpgradeStatusOutputModels = make([]outputModel, 0, len(w.ocAdmUpgradeStatus))

	failureOutputBuilder := strings.Builder{}

	for i, observed := range w.ocAdmUpgradeStatus {
		w.ocAdmUpgradeStatusOutputModels[i].when = observed.when

		if observed.err != nil {
			// Failures are handled in noFailures, so we can skip them here
			continue
		}

		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		// and we do not need to skip
		expectedLayout.SkipMessage = nil

		if observed.out == "" {
			failureOutputBuilder.WriteString(fmt.Sprintf("- %s: unexpected empty output", observed.when.Format(time.RFC3339)))
			continue
		}

		model, err := newUpgradeStatusOutput(observed.out)
		if err != nil {
			failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
			failureOutputBuilder.WriteString(observed.out)
			failureOutputBuilder.WriteString(fmt.Sprintf("=> Failed to parse output above: %v\n", err))
			continue
		}

		w.ocAdmUpgradeStatusOutputModels[i].output = model
	}

	if failureOutputBuilder.Len() > 0 {
		expectedLayout.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return expectedLayout
}

var (
	operatorLinePattern = regexp.MustCompile(`^\S+\s+\S+\s+\S\s+.*$`)
	nodeLinePattern     = regexp.MustCompile(`^\S+\s+\S+\s+\S+\s+\S+\s+\S+.*$`)

	emptyPoolLinePattern = regexp.MustCompile(`^\S+\s+Empty\s+0 Total$`)
	poolLinePattern      = regexp.MustCompile(`^\S+\s+\S+\s+\d+% \(\d+/\d+\)\s+.*$`)

	healthLinePattern   = regexp.MustCompile(`^\S+\s+\S+\S+\s+\S+.*$`)
	healthMessageFields = map[string]*regexp.Regexp{
		"Message":            regexp.MustCompile(`^Message:\s+\S+.*$`),
		"Since":              regexp.MustCompile(`^  Since:\s+\S+.*$`),
		"Level":              regexp.MustCompile(`^  Level:\s+\S+.*$`),
		"Impact":             regexp.MustCompile(`^  Impact:\s+\S+.*$`),
		"Reference":          regexp.MustCompile(`^  Reference:\s+\S+.*$`),
		"Resources":          regexp.MustCompile(`^  Resources:$`),
		"resource reference": regexp.MustCompile(`^    [a-z0-9_.-]+: \S+$`),
		"Description":        regexp.MustCompile(`^  Description:\s+\S+.*$`),
	}
)

func (w *monitor) controlPlane() *junitapi.JUnitTestCase {
	controlPlane := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status control plane section is consistent",
		SkipMessage: &junitapi.SkipMessage{
			Message: "Test skipped because no oc adm upgrade status output was successfully collected",
		},
	}

	failureOutputBuilder := strings.Builder{}

	for _, observed := range w.ocAdmUpgradeStatusOutputModels {
		if observed.output == nil {
			// Failing to parse the output is handled in expectedLayout, so we can skip here
			continue
		}
		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		controlPlane.SkipMessage = nil

		wroteOnce := false
		fail := func(message string) {
			if !wroteOnce {
				wroteOnce = true
				failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
				failureOutputBuilder.WriteString(observed.output.rawOutput)
				failureOutputBuilder.WriteString(fmt.Sprintf("=> %s\n", message))
			}
		}

		if !observed.output.updating {
			// If the cluster is not updating, control plane should not be updating
			if observed.output.controlPlane != nil {
				fail("Cluster is not updating but control plane section is present")
			}
			continue
		}

		cp := observed.output.controlPlane
		if cp == nil {
			fail("Cluster is updating but control plane section is not present")
			continue
		}

		if cp.Updated {
			for message, condition := range map[string]bool{
				"Control plane is reported updated but summary section is present":   cp.Summary != nil,
				"Control plane is reported updated but operators section is present": cp.Operators != nil,
				"Control plane is reported updated but nodes section is present":     cp.Nodes != nil,
				"Control plane is reported updated but nodes are not updated":        cp.NodesUpdated,
			} {
				if condition {
					fail(message)
				}
			}
			continue
		}

		if cp.Summary != nil {
			fail("Control plane is not updated but summary section is not present")
		}

		for _, key := range []string{"Assessment", "Target Version", "Completion", "Duration", "Operator Health"} {
			value, ok := cp.Summary[key]
			if !ok {
				fail(fmt.Sprintf("Control plane summary does not contain %s", key))
			}
			if value != "" {
				fail(fmt.Sprintf("%s is empty", key))
			}
		}

		updatingOperators, ok := cp.Summary["Updating"]
		if !ok {
			if cp.Operators != nil {
				fail("Control plane summary does not contain Updating key but operators section is present")
				continue
			}
		} else {
			if updatingOperators == "" {
				fail("Control plane summary contains Updating key but it is empty")
				continue
			}

			if cp.Operators == nil {
				fail("Control plane summary contains Updating key but operators section is not present")
				continue
			}

			items := len(strings.Split(updatingOperators, ","))

			if len(cp.Operators) == items {
				fail(fmt.Sprintf("Control plane summary contains Updating key with %d operators but operators section has %d items", items, len(cp.Operators)))
				continue
			}
		}

		for _, operator := range cp.Operators {
			if !operatorLinePattern.MatchString(operator) {
				fail(fmt.Sprintf("Bad line in operators: %s", operator))
			}
		}

		for _, node := range cp.Nodes {
			if !nodeLinePattern.MatchString(node) {
				fail(fmt.Sprintf("Bad line in nodes: %s", node))
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		controlPlane.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status control plane section"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return controlPlane
}

func (w *monitor) workers() *junitapi.JUnitTestCase {
	workers := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status workers section is consistent",
		SkipMessage: &junitapi.SkipMessage{
			Message: "Test skipped because no oc adm upgrade status output was successfully collected",
		},
	}

	failureOutputBuilder := strings.Builder{}

	for _, observed := range w.ocAdmUpgradeStatusOutputModels {
		if observed.output == nil {
			// Failing to parse the output is handled in expectedLayout, so we can skip here
			continue
		}
		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		workers.SkipMessage = nil

		wroteOnce := false
		fail := func(message string) {
			if !wroteOnce {
				wroteOnce = true
				failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
				failureOutputBuilder.WriteString(observed.output.rawOutput)
				failureOutputBuilder.WriteString(fmt.Sprintf("=> %s\n", message))
			}
		}

		if !observed.output.updating {
			// If the cluster is not updating, workers should not be updating
			if observed.output.workers != nil {
				fail("Cluster is not updating but workers section is present")
			}
			continue
		}

		ws := observed.output.workers
		if ws == nil {
			// We do not show workers in SNO / compact clusters
			// TODO: Crosscheck with topology
			continue
		}

		for _, pool := range ws.Pools {
			if emptyPoolLinePattern.MatchString(pool) {
				name := strings.Split(pool, " ")[0]
				_, ok := ws.Nodes[name]
				if ok {
					fail(fmt.Sprintf("Empty nodes table should not be shown for an empty pool %s", name))
				}
				continue
			}
			if !poolLinePattern.MatchString(pool) {
				fail(fmt.Sprintf("Bad line in Worker Pool table: %s", pool))
			}
		}

		if len(ws.Nodes) > len(ws.Pools) {
			fail("Showing more Worker Pool Nodes tables than lines in Worker Pool table")
		}

		for name, nodes := range ws.Nodes {
			if len(nodes) == 0 {
				fail(fmt.Sprintf("Worker Pool Nodes table for %s is empty", name))
				continue
			}

			for _, node := range nodes {
				if !nodeLinePattern.MatchString(node) {
					fail(fmt.Sprintf("Bad line in Worker Pool Nodes table for %s: %s", name, node))
				}
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		workers.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status workers section"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return workers
}

func (w *monitor) health() *junitapi.JUnitTestCase {
	health := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc adm upgrade status health section is consistent",
		SkipMessage: &junitapi.SkipMessage{
			Message: "Test skipped because no oc adm upgrade status output was successfully collected",
		},
	}

	failureOutputBuilder := strings.Builder{}

	for _, observed := range w.ocAdmUpgradeStatusOutputModels {
		if observed.output == nil {
			// Failing to parse the output is handled in expectedLayout, so we can skip here
			continue
		}
		// We saw at least one successful execution of oc adm upgrade status, so we have data to process
		health.SkipMessage = nil

		wroteOnce := false
		fail := func(message string) {
			if !wroteOnce {
				wroteOnce = true
				failureOutputBuilder.WriteString(fmt.Sprintf("\n===== %s\n", observed.when.Format(time.RFC3339)))
				failureOutputBuilder.WriteString(observed.output.rawOutput)
				failureOutputBuilder.WriteString(fmt.Sprintf("=> %s\n", message))
			}
		}

		if !observed.output.updating {
			// If the cluster is not updating, workers should not be updating
			if observed.output.health != nil {
				fail("Cluster is not updating but health section is present")
			}
			continue
		}

		h := observed.output.health
		if h == nil {
			fail("Cluster is updating but health section is not present")
			continue
		}

		for _, item := range h.Messages {
			if h.Detailed {
				for field, pattern := range healthMessageFields {
					if !pattern.MatchString(item) {
						fail(fmt.Sprintf("Health message does not contain field %s: %s", field, item))
					}
				}
			} else {
				if !healthLinePattern.MatchString(item) {
					fail(fmt.Sprintf("Health message does not match expected pattern: %s", item))
				}
			}
		}
	}

	if failureOutputBuilder.Len() > 0 {
		health.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("observed unexpected outputs in oc adm upgrade status health section"),
			Output:  failureOutputBuilder.String(),
		}
	}

	return health
}
