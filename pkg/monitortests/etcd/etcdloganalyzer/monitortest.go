package etcdloganalyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/monitortestlibrary/podaccess"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/informers"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
)

const leaderlessTimeout = 5 * time.Second

type etcdLogAnalyzer struct {
	adminRESTConfig *rest.Config

	stopCollection     context.CancelFunc
	finishedCollecting chan struct{}
	dualReplica        bool // true if running on DualReplica topology where etcd runs externally
	etcdRecorder       *etcdRecorder
}

func NewEtcdLogAnalyzer() monitortestframework.MonitorTest {
	return &etcdLogAnalyzer{
		finishedCollecting: make(chan struct{}),
	}
}

// isDualReplicaTopology checks if the cluster is running with DualReplica topology
// where etcd runs externally via Pacemaker/podman instead of as Kubernetes pods.
func isDualReplicaTopology(ctx context.Context, adminRESTConfig *rest.Config) bool {
	configClient, err := configv1client.NewForConfig(adminRESTConfig)
	if err != nil {
		framework.Logf("etcd-log-analyzer: failed to create config client: %v", err)
		return false
	}

	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
	if err != nil {
		framework.Logf("etcd-log-analyzer: failed to get infrastructure: %v", err)
		return false
	}

	return infrastructure.Status.ControlPlaneTopology == configv1.DualReplicaTopologyMode
}

func (w *etcdLogAnalyzer) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminRESTConfig = adminRESTConfig

	// Check if running on DualReplica topology where etcd runs externally via Pacemaker/podman.
	// In this case, there are no etcd pods to stream logs from, so skip the pod log streaming.
	if isDualReplicaTopology(ctx, adminRESTConfig) {
		framework.Logf("etcd-log-analyzer: DualReplica topology detected, etcd runs externally via podman - skipping pod log streaming")
		w.dualReplica = true
		close(w.finishedCollecting) // Signal that we're done (nothing to stream)
		return nil
	}

	w.etcdRecorder = newEtcdRecorder(recorder)
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
		w.etcdRecorder,
		namespaceScopedCoreInformers.Pods(),
	)

	go kubeInformers.Start(ctx.Done())
	go podStreamer.Run(ctx, w.finishedCollecting)

	return nil
}

func (w *etcdLogAnalyzer) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *etcdLogAnalyzer) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	if !w.dualReplica {
		w.stopCollection()
	}

	// wait until we're drained
	<-w.finishedCollecting

	// Flush any pending batched intervals
	if w.etcdRecorder != nil {
		w.etcdRecorder.Flush()
	}

	return nil, nil, nil
}

func (w *etcdLogAnalyzer) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}

	leader := ""
	term := ""
	var newInterval *monitorapi.IntervalBuilder
	startTime := time.Time{}

	interestingReasons := sets.NewString("LeaderFound", "LeaderElected", "LeaderLost", "LeaderMissing")

	podsToNode := podaccess.NonUniquePodToNode(startingIntervals)
	etcdMemberIDToPod := podaccess.NonUniqueEtcdMemberToPod(startingIntervals)

	for _, currInterval := range startingIntervals {
		reason := currInterval.Message.Reason
		if !interestingReasons.Has(string(reason)) {
			continue
		}

		annotations := currInterval.Message.Annotations
		newLeader := annotations[monitorapi.AnnotationEtcdLeader]
		newTerm := annotations[monitorapi.AnnotationEtcdTerm]

		leaderChanged := newLeader != leader
		termChanged := newTerm != term

		switch {
		case newInterval != nil && (leaderChanged || termChanged):
			interval := newInterval.Build(startTime, currInterval.To)
			ret = append(ret, interval)
			fallthrough

		case leaderChanged || termChanged:
			leaderPod := etcdMemberIDToPod[newLeader]
			leaderNode := podsToNode[leaderPod]

			newInterval = monitorapi.NewInterval(monitorapi.SourceEtcdLeadership, monitorapi.Warning).
				Locator(
					monitorapi.NewLocator().EtcdMemberFromNames(leaderNode, newLeader),
				).
				Message(
					monitorapi.NewMessage().
						Constructed(monitorapi.ConstructionOwnerEtcdLifecycle).
						WithAnnotation(monitorapi.AnnotationEtcdLeader, newLeader).
						WithAnnotation(monitorapi.AnnotationEtcdTerm, newTerm).
						HumanMessage(""),
				).Display()
			startTime = currInterval.From

		}

		leader = newLeader
		term = newTerm
	}

	// when we're finished, we must close the last
	if newInterval != nil {
		ret = append(ret, newInterval.Build(startTime, time.Time{}))
		newInterval = nil
	}

	return ret, nil
}

func (w *etcdLogAnalyzer) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	etcdIntervals := monitorapi.Intervals{}
	for _, interval := range finalIntervals {
		value, ok := interval.Message.Annotations[monitorapi.AnnotationConstructed]
		if !ok {
			continue
		}
		if value != monitorapi.ConstructionOwnerEtcdLifecycle {
			continue
		}
		if len(interval.Message.Reason) != 0 {
			continue
		}
		if interval.Locator.HasKey("node") {
			if len(interval.Locator.Keys["node"]) == 0 {
				continue
			}
		}
		if value, ok := interval.Message.Annotations[monitorapi.AnnotationEtcdLeader]; ok && len(value) == 0 {
			continue
		}
		etcdIntervals = append(etcdIntervals, interval)
	}
	sort.Sort(etcdIntervals)

	junitTest := &junitTest{
		name:     fmt.Sprintf("[sig-etcd] cluster should not be without a leader for too long"),
		computed: etcdIntervals,
	}

	framework.Logf("monitor[%s]: found %d intervals of interest", "EtcdLogAnalyzer", len(junitTest.computed))
	if len(junitTest.computed) == 0 {
		// no constructed/computed interval observed, mark the test as skipped.
		// TODO: we should fail it, since we should always observe
		// intervals where etcd has a leader.
		return junitTest.Skip(), nil
	}

	return junitTest.Result(), nil
}

func (w *etcdLogAnalyzer) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return nil
}

func (*etcdLogAnalyzer) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

// batchKey uniquely identifies a batch of similar etcd log intervals
type batchKey struct {
	locator      string
	humanMessage string
	minuteBucket int64 // Unix timestamp truncated to minute
}

// batchEntry holds the data for a batch of intervals
type batchEntry struct {
	locator    monitorapi.Locator
	level      monitorapi.IntervalLevel
	message    string
	fromMinute time.Time
	count      int
}

type etcdRecorder struct {
	recorder monitorapi.RecorderWriter
	// TODO this limits our ability to have custom messages, we probably want something better
	subStrings []subStringLevel

	// batches tracks ongoing batches of high-volume log messages
	batchMu sync.Mutex
	batches map[batchKey]*batchEntry
}

func newEtcdRecorder(recorder monitorapi.RecorderWriter) *etcdRecorder {
	return &etcdRecorder{
		recorder: recorder,
		subStrings: []subStringLevel{
			{"slow fdatasync", monitorapi.Warning},
			{"dropped internal Raft message since sending buffer is full", monitorapi.Warning},
			{"waiting for ReadIndex response took too long, retrying", monitorapi.Warning},
			{"apply request took too long", monitorapi.Warning},
			{"is starting a new election", monitorapi.Info},
		},
		batches: make(map[batchKey]*batchEntry),
	}
}

// Flush emits all pending batched intervals to the recorder.
// This should be called when log collection is complete.
func (g *etcdRecorder) Flush() {
	g.batchMu.Lock()
	defer g.batchMu.Unlock()

	for _, batch := range g.batches {
		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourceEtcdLog, batch.level).
				Locator(batch.locator).
				Message(
					monitorapi.NewMessage().
						WithAnnotation(monitorapi.AnnotationCount, fmt.Sprintf("%d", batch.count)).
						HumanMessage(batch.message),
				).
				Display().
				Build(batch.fromMinute, batch.fromMinute.Add(time.Minute)),
		)
	}

	// Clear the batches
	g.batches = make(map[batchKey]*batchEntry)
}

func (g *etcdRecorder) HandleLogLine(logLine podaccess.LogLineContent) {
	line := logLine.Line
	parsedLine := etcdLogLine{}
	err := json.Unmarshal([]byte(line), &parsedLine)
	if err != nil {
		// not all lines are json, only look at those that are.
		return
	}

	for _, substring := range g.subStrings {
		if !strings.Contains(parsedLine.Msg, substring.subString) {
			continue
		}

		// Batch these high-volume intervals instead of recording each one individually.
		// They will be flushed as aggregated intervals when collection ends.
		minuteBucket := parsedLine.Timestamp.Truncate(time.Minute)
		key := batchKey{
			locator:      logLine.Locator.OldLocator(),
			humanMessage: parsedLine.Msg,
			minuteBucket: minuteBucket.Unix(),
		}

		g.batchMu.Lock()
		if existing, ok := g.batches[key]; ok {
			existing.count++
		} else {
			g.batches[key] = &batchEntry{
				locator:    logLine.Locator,
				level:      substring.level,
				message:    parsedLine.Msg,
				fromMinute: minuteBucket,
				count:      1,
			}
		}
		g.batchMu.Unlock()
	}

	var etcdSource monitorapi.IntervalSource = monitorapi.SourceEtcdLeadership
	messages := []*monitorapi.MessageBuilder{}
	switch {
	case strings.Contains(parsedLine.Msg, "restarting local member"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LocalMemberRestart"). // this message provides a mapping from pod ID to member ID
				WithAnnotation(monitorapi.AnnotationEtcdLocalMember, parsedLine.LocalMemberID).
				HumanMessage(parsedLine.Msg),
		}
		etcdSource = monitorapi.SourceEtcdLog

	case strings.Contains(parsedLine.Msg, "elected leader"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LeaderFound"). // this message can be produced when etcd starts up
				WithAnnotation(monitorapi.AnnotationEtcdLeader, currentLeaderFromMessage(parsedLine.Msg)).
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
		}

	case strings.Contains(parsedLine.Msg, "became leader"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LeaderElected"). // this message is produce when a leader is chosen
				WithAnnotation(monitorapi.AnnotationEtcdLeader, currentLeaderFromMessage(parsedLine.Msg)).
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
		}

	case strings.Contains(parsedLine.Msg, "lost leader"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LeaderLost").
				WithAnnotation(monitorapi.AnnotationPreviousEtcdLeader, prevLeaderFromMessage(parsedLine.Msg)).
				WithAnnotation(monitorapi.AnnotationEtcdLeader, "").
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
		}

	case strings.Contains(parsedLine.Msg, "no leader"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LeaderMissing").
				WithAnnotation(monitorapi.AnnotationEtcdLeader, "").
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
		}

	case strings.Contains(parsedLine.Msg, "changed leader"):
		messages = []*monitorapi.MessageBuilder{
			monitorapi.NewMessage().
				Reason("LeaderLost").
				WithAnnotation(monitorapi.AnnotationPreviousEtcdLeader, prevLeaderFromMessage(parsedLine.Msg)).
				WithAnnotation(monitorapi.AnnotationEtcdLeader, "").
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
			monitorapi.NewMessage().
				Reason("LeaderFound").
				WithAnnotation(monitorapi.AnnotationEtcdLeader, currentLeaderFromMessage(parsedLine.Msg)).
				WithAnnotation(monitorapi.AnnotationEtcdTerm, electionTermFromMessage(parsedLine.Msg)).
				HumanMessage(parsedLine.Msg),
		}

	}

	for _, message := range messages {
		g.recorder.AddIntervals(
			monitorapi.NewInterval(etcdSource, monitorapi.Warning).
				Locator(logLine.Locator).
				Message(message).
				Display().
				Build(logLine.Instant, logLine.Instant.Add(time.Second)),
		)
	}

}

var (
	// "raft.node: 38360899e3c7337e elected leader d8a2c1adbed17efe at term 6"
	electedLeaderRegex = regexp.MustCompile("elected leader (?P<CURR_LEADER>[a-z0-9.-]+) at term (?P<TERM>[0-9]+)")

	// "38360899e3c7337e became leader at term 8"
	becameLeaderRegex = regexp.MustCompile("(?P<CURR_LEADER>[a-z0-9.-]+) became leader at term (?P<TERM>[0-9]+)")

	// r.logger.Infof("raft.node: %x changed leader from %x to %x at term %d", r.id, lead, r.lead, r.Term)
	changedLeaderRegex = regexp.MustCompile(" changed leader from (?P<PREV_LEADER>[a-z0-9.-]+) to (?P<CURR_LEADER>[a-z0-9.-]+) at term (?P<TERM>[0-9]+)")

	// "raft.node: 38360899e3c7337e lost leader eaa12e18c7611129 at term 6"
	lostLeaderRegex = regexp.MustCompile("lost leader (?P<PREV_LEADER>[a-z0-9.-]+) at term (?P<TERM>[0-9]+)")

	// "38360899e3c7337e no leader at term 6; dropping index reading msg"
	noLeaderRegex = regexp.MustCompile("no leader at term (?P<TERM>[0-9]+)")

	leaderMessages = []*regexp.Regexp{
		electedLeaderRegex,
		becameLeaderRegex,
		changedLeaderRegex,
		lostLeaderRegex,
		noLeaderRegex,
	}
)

func currentLeaderFromMessage(msg string) string {
	return searchForKey(msg, "CURR_LEADER")
}

func prevLeaderFromMessage(msg string) string {
	return searchForKey(msg, "PREV_LEADER")
}

func electionTermFromMessage(msg string) string {
	return searchForKey(msg, "TERM")
}

func searchForKey(msg, key string) string {
	for _, leaderMessageRegexp := range leaderMessages {
		matches := leaderMessageRegexp.MatchString(msg)
		if !matches {
			continue
		}

		subMatches := leaderMessageRegexp.FindStringSubmatch(msg)
		subNames := leaderMessageRegexp.SubexpNames()
		for i, name := range subNames {
			switch name {
			case key:
				return subMatches[i]
			}
		}
	}
	return ""
}

type junitTest struct {
	name     string
	computed monitorapi.Intervals
}

func (jut *junitTest) Result() []*junitapi.JUnitTestCase {
	passed := &junitapi.JUnitTestCase{
		Name:      jut.name,
		SystemOut: "",
	}

	type leaderless struct {
		duration   time.Duration
		prev, next *monitorapi.Interval
	}

	exceeded := make([]leaderless, 0)
	for i := 1; i < len(jut.computed); i++ {
		prev, next := &jut.computed[i-1], &jut.computed[i]
		if duration := next.From.Sub(prev.To); duration > leaderlessTimeout {
			exceeded = append(exceeded, leaderless{
				duration: duration,
				prev:     prev,
				next:     next,
			})
		}
	}

	if len(exceeded) == 0 {
		// passed
		return []*junitapi.JUnitTestCase{passed}
	}

	// flake
	failed := &junitapi.JUnitTestCase{
		Name:          jut.name,
		SystemOut:     fmt.Sprintf("etcd cluster leader loss exceeded threshold %d times", len(exceeded)),
		FailureOutput: &junitapi.FailureOutput{},
	}
	for _, l := range exceeded {
		failed.FailureOutput.Output = fmt.Sprintf("%s\netcd cluster did not have a leader for %s\n%s\n%s",
			failed.FailureOutput.Output, l.duration, l.prev.String(), l.next.String())
	}

	// TODO: for now, we flake the test, Once we know it's fully
	// passing then we can remove the flake test case.
	return []*junitapi.JUnitTestCase{failed, passed}
}

func (jut *junitTest) Skip() []*junitapi.JUnitTestCase {
	skipped := &junitapi.JUnitTestCase{
		Name: jut.name,
		SkipMessage: &junitapi.SkipMessage{
			Message: "No intervals of interest seen",
		},
	}
	return []*junitapi.JUnitTestCase{skipped}
}
