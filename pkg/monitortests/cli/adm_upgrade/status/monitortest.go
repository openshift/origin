package admupgradestatus

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
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
		ocAdmUpgradeStatus: make([]snapshot, 0, 60), // expect at least 60 snaphots in a job, one per minute
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
		// TODO: Collect multiple invocations (without --details)? Would need more another producer/consumer pair and likely
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
	for _, snap := range w.ocAdmUpgradeStatus {
		outputFilename := fmt.Sprintf("adm-upgrade-status-%s_%s.txt", snap.when, timeSuffix)
		outputFile := filepath.Join(folderPath, outputFilename)
		if err := os.WriteFile(outputFile, []byte(snap.out), 0644); err != nil {
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
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
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

	w.ocAdmUpgradeStatusOutputModels = make([]outputModel, len(w.ocAdmUpgradeStatus))

	failureOutputBuilder := strings.Builder{}

	for i, observed := range w.ocAdmUpgradeStatus {
		w.ocAdmUpgradeStatusOutputModels[i] = outputModel{
			when: observed.when,
		}

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
