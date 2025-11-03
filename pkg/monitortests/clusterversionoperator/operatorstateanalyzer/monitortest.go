package operatorstateanalyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"k8s.io/client-go/rest"
)

type operatorStateChecker struct {
}

type OperatorStateMetrics struct {
	OperatorName                    string
	ProgressingCount                int
	TotalProgressingSeconds         float64
	MaxIndividualProgressingSeconds float64
	DegradedCount                   int
	TotalDegradedSeconds            float64
	MaxIndividualDegradedSeconds    float64
}

func NewAnalyzer() monitortestframework.MonitorTest {
	return &operatorStateChecker{}
}

func (w *operatorStateChecker) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorStateChecker) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *operatorStateChecker) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*operatorStateChecker) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	ret := monitorapi.Intervals{}
	ret = append(ret, intervalsFromEvents_OperatorAvailable(startingIntervals, nil, beginning, end)...)
	ret = append(ret, intervalsFromEvents_OperatorProgressing(startingIntervals, nil, beginning, end)...)
	ret = append(ret, intervalsFromEvents_OperatorDegraded(startingIntervals, nil, beginning, end)...)

	return ret, nil
}

func (*operatorStateChecker) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (*operatorStateChecker) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	metrics := calculateOperatorStateMetrics(finalIntervals)
	if len(metrics) > 0 {
		rows := generateRowsFromMetrics(metrics)
		dataFile := dataloader.DataFile{
			TableName: "operator_state_metrics",
			Schema: map[string]dataloader.DataType{
				"Operator":                     dataloader.DataTypeString,
				"State":                        dataloader.DataTypeString,
				"Count":                        dataloader.DataTypeInteger,
				"TotalSeconds":                 dataloader.DataTypeFloat64,
				"MaxIndividualDurationSeconds": dataloader.DataTypeFloat64,
			},
			Rows: rows,
		}
		fileName := filepath.Join(storageDir, fmt.Sprintf("operator-state-metrics%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
		if err := dataloader.WriteDataFile(fileName, dataFile); err != nil {
			return fmt.Errorf("failed to write operator state metrics: %w", err)
		}
		logrus.Infof("Write operator state metrics to %s successfully.", fileName)
	}

	return nil
}

// calculateOperatorStateMetrics processes raw intervals and aggregates them into a metrics summary map.
func calculateOperatorStateMetrics(finalIntervals monitorapi.Intervals) map[string]*OperatorStateMetrics {
	metrics := make(map[string]*OperatorStateMetrics)

	for _, interval := range finalIntervals {
		if interval.Source != monitorapi.SourceOperatorState {
			continue
		}
		if interval.Locator.Type != monitorapi.LocatorTypeClusterOperator {
			continue
		}
		operatorName := interval.Locator.Keys[monitorapi.LocatorClusterOperatorKey]
		if _, ok := metrics[operatorName]; !ok {
			metrics[operatorName] = &OperatorStateMetrics{OperatorName: operatorName}
		}

		duration := interval.To.Sub(interval.From).Seconds()
		condition := interval.Message.Annotations[monitorapi.AnnotationCondition]

		switch condition {
		case "Progressing":
			metrics[operatorName].ProgressingCount++
			metrics[operatorName].TotalProgressingSeconds += duration
			if duration > metrics[operatorName].MaxIndividualProgressingSeconds {
				metrics[operatorName].MaxIndividualProgressingSeconds = duration
			}
		case "Degraded":
			metrics[operatorName].DegradedCount++
			metrics[operatorName].TotalDegradedSeconds += duration
			if duration > metrics[operatorName].MaxIndividualDegradedSeconds {
				metrics[operatorName].MaxIndividualDegradedSeconds = duration
			}
		}
	}
	return metrics
}

// generateRowsFromMetrics converts the aggregated metrics map into a slice of rows for the dataloader.
func generateRowsFromMetrics(metrics map[string]*OperatorStateMetrics) []map[string]string {
	rows := []map[string]string{}

	// Sort operator names for consistent output order in tests
	operatorNames := make([]string, 0, len(metrics))
	for name := range metrics {
		operatorNames = append(operatorNames, name)
	}
	sort.Strings(operatorNames)

	for _, operatorName := range operatorNames {
		metric := metrics[operatorName]
		if metric.ProgressingCount > 0 {
			rows = append(rows, map[string]string{
				"Operator":                     operatorName,
				"State":                        "Progressing",
				"Count":                        fmt.Sprintf("%d", metric.ProgressingCount),
				"TotalSeconds":                 fmt.Sprintf("%f", metric.TotalProgressingSeconds),
				"MaxIndividualDurationSeconds": fmt.Sprintf("%f", metric.MaxIndividualProgressingSeconds),
			})
		}
		if metric.DegradedCount > 0 {
			rows = append(rows, map[string]string{
				"Operator":                     operatorName,
				"State":                        "Degraded",
				"Count":                        fmt.Sprintf("%d", metric.DegradedCount),
				"TotalSeconds":                 fmt.Sprintf("%f", metric.TotalDegradedSeconds),
				"MaxIndividualDurationSeconds": fmt.Sprintf("%f", metric.MaxIndividualDegradedSeconds),
			})
		}
	}
	return rows
}

func (*operatorStateChecker) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}
