package statefulsetsrecreation

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/platformidentification"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	exutil "github.com/openshift/origin/test/extended/util"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/rest"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	"k8s.io/client-go/kubernetes"

	"github.com/sirupsen/logrus"
)

const (
	openshiftMonitoringNs = "openshift-monitoring"
	testName              = "[sig-instrumentation] Monitoring statefulsets are not recreated after upgrade"
)

var statefulsetsToCheck = []string{"prometheus-k8s", "alertmanager-main"}

type statefulsetsChecker struct {
	statefulsetsUID    map[string]string
	kubeClient         kubernetes.Interface
	notSupportedReason error
}

// NewStatefulsetsChecker makes sure that some statefulsets are not recreated
// after the upgrade to avoid downtime.
func NewStatefulsetsChecker() monitortestframework.MonitorTest {
	return &statefulsetsChecker{
		statefulsetsUID: make(map[string]string),
	}
}

func (sc *statefulsetsChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (sc *statefulsetsChecker) StartCollection(
	ctx context.Context,
	adminRESTConfig *rest.Config,
	recorder monitorapi.RecorderWriter,
) error {

	var err error
	sc.kubeClient, err = kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	isMicroShift, err := exutil.IsMicroShiftCluster(sc.kubeClient)
	if err != nil {
		return fmt.Errorf("unable to determine if cluster is MicroShift: %v", err)
	}
	if isMicroShift {
		sc.notSupportedReason = &monitortestframework.NotSupportedError{
			Reason: "platform MicroShift not supported",
		}
		return sc.notSupportedReason
	}
	sc.statefulsetsUID, err = sc.getStatefulsetsUID(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (sc *statefulsetsChecker) CollectData(
	ctx context.Context,
	storageDir string,
	beginning, end time.Time,
) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if sc.notSupportedReason != nil {
		return nil, nil, sc.notSupportedReason
	}
	return nil, nil, nil
}

func (sc *statefulsetsChecker) ConstructComputedIntervals(
	ctx context.Context,
	startingIntervals monitorapi.Intervals,
	recordedResources monitorapi.ResourcesMap,
	beginning, end time.Time,
) (monitorapi.Intervals, error) {
	return nil, nil
}

func (sc *statefulsetsChecker) EvaluateTestsFromConstructedIntervals(
	ctx context.Context,
	finalIntervals monitorapi.Intervals,
) ([]*junitapi.JUnitTestCase, error) {
	if !platformidentification.DidUpgradeHappenDuringCollection(
		finalIntervals,
		time.Time{},
		time.Time{},
	) {
		return nil, nil
	}

	currentStatefulsetsUID, err := sc.getStatefulsetsUID(ctx)
	if err != nil {
		return nil, fmt.Errorf("couldn't get the monitoring statefulsets UID: %v", err)
	}
	if len(currentStatefulsetsUID) != len(sc.statefulsetsUID) {
		return nil, fmt.Errorf(
			"got %d monitoring statefulsets' UID before but %d now, the mismatch means the test is broken",
			len(sc.statefulsetsUID),
			len(currentStatefulsetsUID),
		)
	}
	for sts, initialUID := range sc.statefulsetsUID {
		if finalUID, ok := currentStatefulsetsUID[sts]; !ok || finalUID != initialUID {
			return []*junitapi.JUnitTestCase{
				{
					Name: testName,
					SystemOut: fmt.Sprintf(
						"Some monitoring statefulsets UID changed or were not collected, initial UID: %+v, current UID: %+v",
						sc.statefulsetsUID,
						currentStatefulsetsUID,
					),
					FailureOutput: &junitapi.FailureOutput{
						Output: "Monitoring statefulsets UID changed, this will result in downtime.",
					},
				},
			}, nil
		}
	}
	return []*junitapi.JUnitTestCase{{Name: testName}}, nil
}

func (sc *statefulsetsChecker) WriteContentToStorage(
	ctx context.Context,
	storageDir, timeSuffix string,
	finalIntervals monitorapi.Intervals,
	finalResourceState monitorapi.ResourcesMap,
) error {
	return nil
}

func (sc *statefulsetsChecker) Cleanup(ctx context.Context) error {
	return nil
}

func (sc *statefulsetsChecker) getStatefulsetsUID(ctx context.Context) (map[string]string, error) {
	statefulsetsUID := make(map[string]string)
	var failures []error
	for _, statefulsetName := range statefulsetsToCheck {
		var lastErr error
		err := wait.PollUntilContextTimeout(
			ctx,
			time.Second,
			1*time.Minute,
			true,
			func(context.Context) (bool, error) {
				statefulset, err := sc.kubeClient.AppsV1().
					StatefulSets(openshiftMonitoringNs).
					Get(ctx, statefulsetName, metav1.GetOptions{})
				if err != nil {
					lastErr = err
					return false, nil
				}
				statefulsetsUID[statefulsetName] = string(statefulset.UID)
				return true, nil
			},
		)
		if err != nil {
			if lastErr != nil {
				err = fmt.Errorf("%w: lastError: %w", err, lastErr)
			}
			logrus.Infof(
				"Error occured while getting statefulset %s/%s: %v",
				openshiftMonitoringNs,
				statefulsetName,
				err,
			)
			failures = append(failures, err)
		}
	}
	return statefulsetsUID, utilerrors.NewAggregate(failures)
}
