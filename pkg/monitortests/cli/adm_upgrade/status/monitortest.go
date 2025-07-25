package admupgradestatus

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type snapshot struct {
	when time.Time
	out  string
	err  error
}
type monitor struct {
	collectionDone     chan struct{}
	ocAdmUpgradeStatus map[time.Time]*snapshot
}

func NewOcAdmUpgradeStatusChecker() monitortestframework.MonitorTest {
	return &monitor{
		collectionDone:     make(chan struct{}),
		ocAdmUpgradeStatus: map[time.Time]*snapshot{},
	}
}

func (w *monitor) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func snapshotOcAdmUpgradeStatus(ch chan *snapshot) {
	// TODO: I _think_ this should somehow use the adminRESTConfig given to StartCollection but I don't know how to
	//       how to do pass that to exutil.NewCLI* or if it is even possible. It seems to work this way though.
	oc := exutil.NewCLIWithoutNamespace("adm-upgrade-status").AsAdmin()
	now := time.Now()
	// TODO: Consider retrying on brief apiserver unavailability
	cmd := oc.Run("adm", "upgrade", "status").EnvVar("OC_ENABLE_CMD_UPGRADE_STATUS", "true")
	out, err := cmd.Output()
	ch <- &snapshot{when: now, out: out, err: err}
}

func (w *monitor) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	// TODO: The double goroutine spawn should probably be placed under some abstraction
	go func(ctx context.Context) {
		snapshots := make(chan *snapshot)
		go func() {
			for snap := range snapshots {
				// TODO: Maybe also collect some cluster resources (CV? COs?) through recorder?
				w.ocAdmUpgradeStatus[snap.when] = snap
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
	// The framework cancels the context it gave StartCollection before it calls CollectData, but we need to wait for
	// the collection goroutines spawned in StartedCollection to finish
	<-w.collectionDone

	noFailures := &junitapi.JUnitTestCase{
		Name: "[sig-cli][OCPFeatureGate:UpgradeStatus] oc amd upgrade status never fails",
	}

	var failures []string
	for when, observed := range w.ocAdmUpgradeStatus {
		if observed.err != nil {
			failures = append(failures, fmt.Sprintf("- %s: %v", when.Format(time.RFC3339), observed.err))
		}
	}

	// TODO: Zero failures is too strict for at least SNO clusters
	if len(failures) > 0 {
		noFailures.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("oc adm upgrade status failed %d times (of %d)", len(failures), len(w.ocAdmUpgradeStatus)),
			Output:  strings.Join(failures, "\n"),
		}
	}

	// TODO: Maybe utilize Intervals somehow and do tests in ComputeComputedIntervals and EvaluateTestsFromConstructedIntervals

	return nil, []*junitapi.JUnitTestCase{noFailures}, nil
}

func (*monitor) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*monitor) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *monitor) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	var errs []error
	for when, observed := range w.ocAdmUpgradeStatus {
		// TODO: Maybe make a directory for these files
		outputFilename := fmt.Sprintf("adm-upgrade-status-%s_%s.txt", when, timeSuffix)
		outputFile := filepath.Join(storageDir, outputFilename)
		if err := os.WriteFile(outputFile, []byte(observed.out), 0644); err != nil {
			errs = append(errs, fmt.Errorf("failed to write %s: %w", outputFile, err))
		}
	}
	return errors.NewAggregate(errs)
}

func (*monitor) Cleanup(ctx context.Context) error {
	return nil
}
