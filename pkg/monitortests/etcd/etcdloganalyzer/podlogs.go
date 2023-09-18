package etcdloganalyzer

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/sirupsen/logrus"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
)

// subStringLevel defines a sub-string we'll scan pod log lines for, and the level the resulting
// interval should have. (Info, Warning, Error)
type subStringLevel struct {
	subString string
	level     monitorapi.IntervalLevel
}

type podLogIntervalGenerator struct {
	namespace string
	// selector is a label selector for which pods to gather from. (i.e. app=etcd)
	selector  string
	container string
	// subStrings are matched against every log line to see what we should generate an interval for.
	subStrings []subStringLevel
	// lineParser is called to convert a log line to an EventInterval. Function is only called if
	// the line matches one of the substrings.
	lineParser func(locator, line string, intervalLevel monitorapi.IntervalLevel, logger logrus.FieldLogger) (*monitorapi.Interval, error)
}

func (g podLogIntervalGenerator) ScanLine(pod *kapiv1.Pod, line string, beginning, end time.Time, logger logrus.FieldLogger) (*monitorapi.Interval, error) {
	for _, subStr := range g.subStrings {
		if strings.Contains(line, subStr.subString) {
			locator := monitorapi.LocatePodContainer(pod, g.container)
			// Add a src/podLog to the locator for filtering:
			locator = fmt.Sprintf("%s src/podLog", locator)
			interval, err := g.lineParser(locator, line, subStr.level, logger)
			if err != nil {
				return nil, err
			}
			// If we're outside our beginning/end times, we're throwing this interval away.
			if interval.From.Before(beginning) || interval.From.After(end) ||
				interval.To.Before(beginning) || interval.To.After(end) {
				return nil, nil
			}
			return interval, err
		}
	}
	return nil, nil
}

type etcdLogLine struct {
	Level     string    `json:"level"`
	Timestamp time.Time `json:"ts"`
	Msg       string    `json:"msg"`
}

// etcdLogParser handles etcd logs which are already nicely json formatted such as:
//
// {"level":"info","ts":"2023-03-03T18:09:01.471Z","caller":"mvcc/index.go:214","msg":"compact tree index","revision":738215}
func etcdLogParser(locator, line string, level monitorapi.IntervalLevel, logger logrus.FieldLogger) (*monitorapi.Interval, error) {
	parsedLine := etcdLogLine{}
	err := json.Unmarshal([]byte(line), &parsedLine)
	if err != nil {
		logger.WithError(err).Errorf("error parsing matched log line: %s", line)
		return nil, err
	}
	return &monitorapi.Interval{
		Condition: monitorapi.Condition{
			Level:   level,
			Locator: locator,
			Message: parsedLine.Msg,
		},
		From: parsedLine.Timestamp,
		To:   parsedLine.Timestamp.Add(1 * time.Second),
	}, nil
}

func buildLogGatherers() []podLogIntervalGenerator {
	return []podLogIntervalGenerator{
		{
			namespace: "openshift-etcd",
			selector:  "app=etcd",
			container: "etcd",
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
			lineParser: etcdLogParser,
		},
	}
}

// intervalsFromPodLogs fetches pod logs for a hardcoded set of namespace, label selector, and container. Each line is
// then checked for a match of certain substrings and if found, passed to a function to parse the line to a
// monitorapi.EventInterval, which will then be included in the main list of intervals.
// Beginning and end times are specified so we only build intervals for the phase of testing we're interested in.
// A single cluster in an upgrade job will have separate intervals for the upgrade phase and the conformance testing
// phase.
func intervalsFromPodLogs(kubeClient kubernetes.Interface, beginning, end time.Time) (monitorapi.Intervals, error) {

	intervals := monitorapi.Intervals{}
	gatherers := buildLogGatherers()

	for _, g := range gatherers {
		podClient := kubeClient.CoreV1().Pods(g.namespace)
		selector, _ := labels.Parse(g.selector)
		pods, err := podClient.List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			logrus.WithError(err).Errorf("unable to list pods in %s namespace", g.namespace)
			return nil, err
		}
		for _, pod := range pods.Items {
			logger := logrus.WithField("pod", pod.Name)
			logger.Infof("fetching logs between %s and %s", beginning.Format(time.RFC3339), end.Format(time.RFC3339))
			reader, err := podClient.GetLogs(pod.Name, &kapiv1.PodLogOptions{Container: g.container}).Stream(context.Background())
			if err != nil {

				// If there's trouble getting logs, the intervals will be missing.  During troubleshooting,
				// error information (including the pod name) will be in this message.
				logger.WithError(err).Error("error reading pod logs")
				continue
			}
			scan := bufio.NewScanner(reader)
			for scan.Scan() {
				line := scan.Text()
				interval, err := g.ScanLine(&pod, line, beginning, end, logger)
				if err != nil {
					logrus.WithError(err).Errorf("error scanning log line: %s", line)
					continue
				}
				if interval != nil {
					logger.Infof("added interval: %+v", *interval)
					intervals = append(intervals, *interval)

				}

			}
			logger.Info("log file completed")
		}
	}
	return intervals, nil
}
