package dnsdisruptionstats

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/openshift/origin/pkg/dataloader"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestframework"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	"github.com/sirupsen/logrus"
	"k8s.io/client-go/rest"
)

type dnsDisruptionStats struct{}

func NewDNSDisruptionStats() monitortestframework.MonitorTest {
	return &dnsDisruptionStats{}
}

func (w *dnsDisruptionStats) PrepareCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *dnsDisruptionStats) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	return nil
}

func (w *dnsDisruptionStats) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (w *dnsDisruptionStats) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

func (w *dnsDisruptionStats) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	return nil, nil
}

func (w *dnsDisruptionStats) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	logger := logrus.WithField("MonitorTest", "DNSDisruptionStats")

	dnsIntervals := finalIntervals.Filter(func(eventInterval monitorapi.Interval) bool {
		return eventInterval.Message.Reason == monitorapi.DisruptionSamplerOutageBeganEventReason
	})

	var totalDurationSeconds int
	for _, interval := range dnsIntervals {
		totalDurationSeconds += int(interval.To.Sub(interval.From).Seconds())
	}

	logger.Infof("DNS disruption: %d intervals, %d total seconds", len(dnsIntervals), totalDurationSeconds)

	dataFile := dataloader.DataFile{
		TableName: "dns_disruption_stats",
		Schema: map[string]dataloader.DataType{
			"IntervalCount":        dataloader.DataTypeInteger,
			"TotalDurationSeconds": dataloader.DataTypeInteger,
		},
		Rows: []map[string]string{
			{
				"IntervalCount":        fmt.Sprintf("%d", len(dnsIntervals)),
				"TotalDurationSeconds": fmt.Sprintf("%d", totalDurationSeconds),
			},
		},
	}

	fileName := filepath.Join(storageDir, fmt.Sprintf("dns-disruption-stats%s-%s", timeSuffix, dataloader.AutoDataLoaderSuffix))
	if err := dataloader.WriteDataFile(fileName, dataFile); err != nil {
		logger.WithError(err).Warnf("unable to write data file: %s", fileName)
		return fmt.Errorf("failed to write dns disruption stats: %w", err)
	}

	return nil
}

func (w *dnsDisruptionStats) Cleanup(ctx context.Context) error {
	return nil
}
