package etcdloganalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	coreinformers "k8s.io/client-go/informers/core/v1"

	"k8s.io/client-go/informers"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type etcdLogAnalyzer struct {
	adminRESTConfig *rest.Config

	stopCollection     context.CancelFunc
	finishedCollecting chan struct{}
}

func NewEtcdLogAnalyzer() monitortestframework.MonitorTest {
	return &etcdLogAnalyzer{
		finishedCollecting: make(chan struct{}),
	}
}

func (w *etcdLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	logToIntervalConverter := newEtcdRecorder(recorder)
	w.adminRESTConfig = adminRESTConfig
	kubeClient, err := kubernetes.NewForConfig(w.adminRESTConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 0)
	namespaceScopedCoreInformers := coreinformers.New(kubeInformers, "openshift-etcd", nil)

	// stream all pods that appear or disappear from this label
	etcdLabel, err := labels.NewRequirement("app", selection.Equals, []string{"etcd"})
	if err != nil {
		return err
	}
	ctx, w.stopCollection = context.WithCancel(ctx)
	podStreamer := podaccess.NewPodsStreamer(
		kubeClient,
		labels.NewSelector().Add(*etcdLabel),
		"openshift-etcd",
		"etcd",
		logToIntervalConverter,
		namespaceScopedCoreInformers.Pods(),
	)

	go kubeInformers.Start(ctx.Done())
	go podStreamer.Run(ctx, w.finishedCollecting)

	return nil
}

func (w *etcdLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	w.stopCollection()

	// wait until we're drained
	<-w.finishedCollecting

	return nil, nil, nil
}

func (*etcdLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (*etcdLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *etcdLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*etcdLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

type etcdRecorder struct {
	recorder monitorapi.RecorderWriter
	// TODO this limits our ability to have custom messages, we probably want something better
	subStrings []subStringLevel
}

func newEtcdRecorder(recorder monitorapi.RecorderWriter) etcdRecorder {
	return etcdRecorder{
		recorder: recorder,
		subStrings: []subStringLevel{
			{"slow fdatasync", monitorapi.Warning},
			{"dropped internal Raft message since sending buffer is full", monitorapi.Warning},
			{"waiting for ReadIndex response took too long, retrying", monitorapi.Warning},
			{"apply request took too long", monitorapi.Warning},
			{"elected leader", monitorapi.Info},
			{"lost leader", monitorapi.Info},
			{"is starting a new election", monitorapi.Info},
			{"became leader", monitorapi.Info},
		},
	}
}

func (g etcdRecorder) HandleLogLine(logLine podaccess.LogLineContent) {
	line := logLine.Line
	parsedLine := etcdLogLine{}
	err := json.Unmarshal([]byte(line), &parsedLine)
	if err != nil {
		// not all lines are json, only look at those that are.
		return
	}

	locator := logLine.Locator.OldLocator()
	// Add a src/podLog to the locator for filtering:
	// TODO this seems like a bad kind of marker since all sorts of different things originate here and we care about what it means, not where its from
	locator = fmt.Sprintf("%s src/podLog", locator)

	for _, substring := range g.subStrings {
		if !strings.Contains(parsedLine.Msg, substring.subString) {
			continue
		}

		// TODO need the src/podLog locator to make the display work
		//g.recorder.AddIntervals(
		//	monitorapi.NewInterval(monitorapi.SourcePodLog, monitorapi.Warning).
		//		Locator(logLine.Locator).
		//		// TODO details in the message
		//		Message(monitorapi.NewMessage().HumanMessage(parsedLine.Msg)).
		//		Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		//)
		g.recorder.AddIntervals(monitorapi.Interval{
			Condition: monitorapi.Condition{
				Level:   monitorapi.Warning,
				Locator: locator,
				Message: parsedLine.Msg,
			},
			From: parsedLine.Timestamp,
			To:   parsedLine.Timestamp.Add(1 * time.Second),
		})
	}
}
