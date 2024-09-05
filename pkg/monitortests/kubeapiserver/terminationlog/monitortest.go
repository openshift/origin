package terminationlog

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"
	"text/template"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func NewTerminationLogAnalyzer() monitortestframework.MonitorTest {
	return &monitorTest{}
}

type monitorTest struct {
	config          *rest.Config
	lateConnections *lateConnectionSummary
}

func (c *monitorTest) StartCollection(_ context.Context, config *rest.Config, _ monitorapi.RecorderWriter) error {
	c.config = config
	return nil
}

func (c *monitorTest) CollectData(ctx context.Context, _ string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	client, err := kubernetes.NewForConfig(c.config)
	if err != nil {
		return nil, nil, err
	}
	c.lateConnections = newLateConnectionsSummary()

	// process log entries with a list of handlers
	err = NewClusterTerminationLogs(beginning, end).Process(ctx, client,
		c.lateConnections.ProcessTerminationLogEntry,
	)

	var intervals []monitorapi.Interval

	// intervals from lateConnections
	for node, entries := range c.lateConnections.ByNode {
		for _, entry := range entries {
			intervals = append(intervals,
				monitorapi.NewInterval(monitorapi.SourceTerminationLog, monitorapi.Warning).
					Locator(monitorapi.NewLocator().NodeFromName(node)).
					Message(monitorapi.NewMessage().
						Reason(monitorapi.LateConnectionDuringShutdown).
						WithAnnotation(monitorapi.AnnotationRequestURI, entry.RequestURI).
						WithAnnotation(monitorapi.AnnotationSourceIP, entry.SourceIP).
						WithAnnotation(monitorapi.AnnotationUserAgent, entry.UserAgent).
						HumanMessagef("%s: %s", entry.Node, entry.Msg),
					).
					Build(entry.TS, entry.TS),
			)
		}
	}

	return intervals, nil, err
}

func (c *monitorTest) ConstructComputedIntervals(_ context.Context, intervals monitorapi.Intervals, _ monitorapi.ResourcesMap, _, _ time.Time) (monitorapi.Intervals, error) {
	var results []monitorapi.Interval

	lateConnectionIntervals := intervals.Filter(func(intv monitorapi.Interval) bool {
		return intv.Message.Reason == monitorapi.LateConnectionDuringShutdown
	})
	if len(lateConnectionIntervals) > 0 {
		byNode := map[string][]monitorapi.Interval{}
		for _, interval := range lateConnectionIntervals {
			byNode[interval.Locator.Keys[monitorapi.LocatorNodeKey]] = append(byNode[interval.Locator.Keys[monitorapi.LocatorNodeKey]], interval)
		}
		for _, nodeIntervals := range byNode {
			var fromIndex int
			for i := range nodeIntervals {
				switch {
				case i == 0:
					continue
				case nodeIntervals[i].To.Sub(nodeIntervals[i-1].To).Seconds() < 10:
					continue
				}
				results = append(results, newLateConnectionConstructedInterval(nodeIntervals[fromIndex:i]))
				fromIndex = i
			}
			results = append(results, newLateConnectionConstructedInterval(nodeIntervals[fromIndex:]))
		}
	}

	return results, nil
}

func newLateConnectionConstructedInterval(intervals []monitorapi.Interval) monitorapi.Interval {
	locator := intervals[0].Locator
	var sourceIPs, userAgents, requestURIs []string
	for _, interval := range intervals {
		sourceIPs = append(sourceIPs, interval.Message.Annotations[monitorapi.AnnotationSourceIP])
		userAgents = append(userAgents, interval.Message.Annotations[monitorapi.AnnotationUserAgent])
		requestURIs = append(requestURIs, interval.Message.Annotations[monitorapi.AnnotationRequestURI])
	}
	slices.Sort(sourceIPs)
	slices.Sort(userAgents)
	slices.Sort(requestURIs)
	return monitorapi.NewInterval(monitorapi.SourceTerminationLog, monitorapi.Error).
		Locator(locator).
		Message(monitorapi.NewMessage().
			Reason(monitorapi.LateConnectionDuringShutdown).
			WithAnnotation(monitorapi.AnnotationRequestURIs, strings.Join(slices.Compact(requestURIs), ", ")).
			WithAnnotation(monitorapi.AnnotationSourceIPs, strings.Join(slices.Compact(sourceIPs), ", ")).
			WithAnnotation(monitorapi.AnnotationUserAgents, strings.Join(slices.Compact(userAgents), ", ")).
			HumanMessagef("Connections are being routed to the kube-apiserver on node %q that are very late into the graceful termination process", locator.Keys[monitorapi.LocatorNodeKey]),
		).
		Display().
		Build(intervals[0].From, intervals[len(intervals)-1].To)
}

func (c *monitorTest) EvaluateTestsFromConstructedIntervals(_ context.Context, intervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	var results []*junitapi.JUnitTestCase

	// late connections test
	lateConnectionTest := &junitapi.JUnitTestCase{Name: "[sig-apimachinery] kube-apiserver should not see any new connections very late in the graceful termination process"}
	results = append(results, lateConnectionTest)
	lateConnectionIntervals := intervals.Filter(func(intv monitorapi.Interval) bool {
		return intv.Message.Reason == monitorapi.LateConnectionDuringShutdown && intv.Display
	})
	if len(lateConnectionIntervals) != 0 {
		output := make([]string, len(lateConnectionIntervals))
		for i, interval := range lateConnectionIntervals {
			output[i] = interval.Message.HumanMessage
		}
		slices.Sort(output)
		output = slices.Compact(output)
		lateConnectionTest.FailureOutput = &junitapi.FailureOutput{
			Message: "Connections are being routed to a kube-apiserver that was very late into the graceful termination process.",
			Output:  strings.Join(output, "\n"),
		}
	}

	return results, nil
}

//go:embed late_connections_summary.tmpl
var lateConnectionReportTemplate string

func (c *monitorTest) WriteContentToStorage(_ context.Context, storageDir, timeSuffix string, _ monitorapi.Intervals, _ monitorapi.ResourcesMap) error {

	// log late connections summary
	f, err := os.Create(path.Join(storageDir, "late_connections"+timeSuffix+".md"))
	if err != nil {
		return err
	}
	defer func() {
		err := f.Close()
		if err != nil {
			fmt.Printf("error closing late_connections%s.md: %v", timeSuffix, err)
		}
	}()

	tmpl := template.New("summary")
	tmpl, err = tmpl.Parse(lateConnectionReportTemplate)
	if err != nil {
		return err
	}
	return tmpl.Execute(f, c.lateConnections.ByNode)
}

func (c *monitorTest) Cleanup(ctx context.Context) error {
	// nothing to do
	return nil
}
