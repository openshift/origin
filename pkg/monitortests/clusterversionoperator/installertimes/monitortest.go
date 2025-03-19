package installertimes

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"time"

	"github.com/openshift/origin/pkg/dataloader"

	apierrors "k8s.io/apimachinery/pkg/api/errors"

	"k8s.io/client-go/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	configv1client "github.com/openshift/client-go/config/clientset/versioned"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type installerTimeAnalyzer struct {
	approximateInstallStartTime metav1.Time
	approximateBootstrapEndTime metav1.Time
	approximateInstallEndTime   metav1.Time
}

func NewInstallTimeAnalyzer() monitortestframework.MonitorTest {
	return &installerTimeAnalyzer{}
}

func (w *installerTimeAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	configClient, err := configv1client.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}

	clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(clusterVersion.Status.History) == 0 {
		return nil
	}
	if clusterVersion.Status.History[len(clusterVersion.Status.History)-1].CompletionTime == nil {
		return nil
	}

	bootstrapConfigMap, err := kubeClient.CoreV1().ConfigMaps("kube-system").Get(ctx, "bootstrap", metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	w.approximateInstallStartTime = clusterVersion.CreationTimestamp
	w.approximateBootstrapEndTime = bootstrapConfigMap.CreationTimestamp
	w.approximateInstallEndTime = *clusterVersion.Status.History[len(clusterVersion.Status.History)-1].CompletionTime

	return nil
}

func (w *installerTimeAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (w *installerTimeAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	if w.approximateInstallEndTime.IsZero() {
		return nil, nil
	}

	computedIntervals := monitorapi.Intervals{}
	computedIntervals = append(computedIntervals,
		monitorapi.NewInterval(monitorapi.ClusterInstallOverall, monitorapi.Info).
			Locator(monitorapi.NewLocator().LocateCluster()).
			Message(monitorapi.NewMessage().
				Constructed("install-time-analyzer").
				Reason(monitorapi.InstallStartedReason),
			).
			Display().
			Build(w.approximateInstallStartTime.Time, w.approximateInstallEndTime.Time),
	)
	computedIntervals = append(computedIntervals,
		monitorapi.NewInterval(monitorapi.ClusterInstallBootstrap, monitorapi.Info).
			Locator(monitorapi.NewLocator().LocateCluster()).
			Message(monitorapi.NewMessage().
				Constructed("install-time-analyzer").
				Reason(monitorapi.InstallStartedReason),
			).
			Display().
			Build(w.approximateInstallStartTime.Time, w.approximateBootstrapEndTime.Time),
	)

	return computedIntervals, nil
}

func (*installerTimeAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *installerTimeAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if err := w.writeInstallerTimes(storageDir, timeSuffix); err != nil {
		return err
	}

	return nil
}

func (*installerTimeAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func (w *installerTimeAnalyzer) writeInstallerTimes(artifactDir, timeSuffix string) error {
	if w.approximateInstallEndTime.IsZero() {
		return nil
	}

	installSeconds := w.approximateInstallEndTime.Time.Sub(w.approximateInstallStartTime.Time).Seconds()
	boostrapSeconds := w.approximateBootstrapEndTime.Time.Sub(w.approximateInstallStartTime.Time).Seconds()

	dataFile := dataloader.DataFile{
		TableName: "important_durations",
		Schema:    map[string]dataloader.DataType{"Name": dataloader.DataTypeString, "DurationSeconds": dataloader.DataTypeInteger},
		Rows: []map[string]string{
			{"Name": "TotalInstallDuration", "DurationSeconds": strconv.FormatInt(int64(installSeconds), 10)},
			{"Name": "ClusterBootstrapDuration", "DurationSeconds": strconv.FormatInt(int64(boostrapSeconds), 10)},
		},
	}

	fileName := filepath.Join(artifactDir, fmt.Sprintf("installer_durations%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	err := dataloader.WriteDataFile(fileName, dataFile)
	if err != nil {
		return err
	}
	return nil
}
