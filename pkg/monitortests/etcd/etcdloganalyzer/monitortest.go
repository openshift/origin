package etcdloganalyzer

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/util/sets"

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
				)
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
			{"is starting a new election", monitorapi.Info},
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

	for _, substring := range g.subStrings {
		if !strings.Contains(parsedLine.Msg, substring.subString) {
			continue
		}

		g.recorder.AddIntervals(
			monitorapi.NewInterval(monitorapi.SourceEtcdLog, monitorapi.Warning).
				Locator(logLine.Locator).
				Message(
					monitorapi.NewMessage().
						HumanMessage(parsedLine.Msg),
				).
				Build(parsedLine.Timestamp, parsedLine.Timestamp.Add(1*time.Second)))
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
