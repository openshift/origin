package kasloganalyzer

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type kasLogAnalyzer struct {
	stopCollection     context.CancelFunc
	finishedCollecting chan struct{}

	evaluator evaluator
}

func NewKASLogAnalyzer() monitortestframework.MonitorTest {
	return &kasLogAnalyzer{
		finishedCollecting: make(chan struct{}),
		evaluator:          newEvaluator(),
	}
}

func (w *kasLogAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	kubeClient, err := kubernetes.NewForConfig(adminRESTConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 0)
	namespaceScopedCoreInformers := coreinformers.New(kubeInformers, "openshift-kube-apiserver", nil)

	ctx, w.stopCollection = context.WithCancel(ctx)
	podStreamer := podaccess.NewPodsStreamer(
		kubeClient,
		labels.NewSelector(),
		"openshift-kube-apiserver",
		"kube-apiserver",
		w.evaluator,
		namespaceScopedCoreInformers.Pods(),
	)

	go kubeInformers.Start(ctx.Done())
	go podStreamer.Run(ctx, w.finishedCollecting)

	return nil
}

func (w *kasLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *kasLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	w.stopCollection()

	// wait until we're drained
	<-w.finishedCollecting

	return nil, w.evaluator.Reports(), nil
}

func (w *kasLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *kasLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *kasLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*kasLogAnalyzer) Cleanup(ctx context.Context) error {
	return nil
}

type evaluator struct {
	evaluations []evaluation
}

func (e evaluator) Reports() []*junitapi.JUnitTestCase {
	out := []*junitapi.JUnitTestCase{}

	for _, eval := range e.evaluations {
		out = append(out, eval.Report())
	}

	return out
}

func newEvaluator() evaluator {
	return evaluator{
		evaluations: defaultEvaluations,
	}
}

func (e evaluator) HandleLogLine(logLine podaccess.LogLineContent) {
	for _, evaluation := range e.evaluations {
		if evaluation.regex.MatchString(logLine.Line) {
			evaluation.count++
		}
	}
}

type evaluation struct {
	name      string
	threshold int
	count     int
	regex     *regexp.Regexp
}

var defaultEvaluations = []evaluation{
	{
		name:      "[Jira:\"kube-apiserver\"] should not excessively log informer reflector unhandled errors",
		regex:     regexp.MustCompile(`reflector\.go.+\"Failed to watch\".+err=\"failed to list.+\".+logger=\"UnhandledError\"`),
		threshold: 0,
	},
}

func (e evaluation) Report() *junitapi.JUnitTestCase {
	out := &junitapi.JUnitTestCase{
		Name: e.name,
	}

	if e.count > e.threshold {
		out.FailureOutput = &junitapi.FailureOutput{
			Message: fmt.Sprintf("kube-apiserver logged %d informer-related unhandled errors. Should not log more than %d", e.count, e.threshold),
		}
	}

	return out
}
