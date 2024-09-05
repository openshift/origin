package auditloganalyzer

import (
	"context"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type auditLogAnalyzer struct {
	adminRESTConfig *rest.Config

	summarizer *summarizer
}

func NewAuditLogAnalyzer() monitortestframework.MonitorTest {
	return &auditLogAnalyzer{
		summarizer: NewAuditLogSummarizer(),
	}
}

func (w *auditLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig
	return nil
}

func (w *auditLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return nil, nil, err
	}

	auditLogHandlers := []AuditEventHandler{
		w.summarizer,
	}
	err = GetKubeAuditLogSummary(ctx, kubeClient, &beginning, &end, auditLogHandlers)

	return nil, nil, err
}

func (*auditLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*auditLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *auditLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	if currErr := WriteAuditLogSummary(storageDir, timeSuffix, w.summarizer.auditLogSummary); currErr != nil {
		return currErr
	}
	return nil
}

func (*auditLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
