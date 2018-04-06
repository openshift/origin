package prometheus

import (
	"sync"

	"github.com/golang/glog"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator          = "_"
	metricController   = "openshift_imagestreamcontroller"
	metricCount        = "count"
	metricSuccessCount = metricController + separator + "success" + separator + metricCount
	metricErrorCount   = metricController + separator + "error" + separator + metricCount

	labelScheduled = "scheduled"
	labelRegistry  = "registry"
	labelReason    = "reason"
)

// ImportErrorInfo contains dimensions of metricErrorCount
type ImportErrorInfo struct {
	Registry string
	Reason   string
}

// ImportSuccessCounts maps registry hostname (with port) to the count of successful imports. It serves as a
// container of counters for the success_count metric.
type ImportSuccessCounts map[string]uint64

// ImportErrorCounts serves as a container of counters for the error_count metric.
type ImportErrorCounts map[ImportErrorInfo]uint64

// QueuedImageStreamFetcher is a callback passed to the importStatusCollector that is supposed to be invoked
// by image import controller with the current state of counters.
type QueuedImageStreamFetcher func() (ImportSuccessCounts, ImportErrorCounts, error)

var (
	successCountDesc = prometheus.NewDesc(
		metricSuccessCount,
		"Counts successful image stream imports - both scheduled and not scheduled - per image registry",
		[]string{labelScheduled, labelRegistry},
		nil,
	)
	errorCountDesc = prometheus.NewDesc(
		metricErrorCount,
		"Counts number of failed image stream imports - both scheduled and not scheduled"+
			" - per image registry and failure reason",
		[]string{labelScheduled, labelRegistry, labelReason},
		nil,
	)

	isc                 = importStatusCollector{}
	registerLock        = sync.Mutex{}
	collectorRegistered = false
)

type importStatusCollector struct {
	cbCollectISCounts        QueuedImageStreamFetcher
	cbCollectScheduledCounts QueuedImageStreamFetcher
}

// InitializeImportCollector is supposed to be called by image import controllers when they are prepared to
// serve requests. Once all the controllers register their callbacks, the collector registers the metrics with
// the prometheus.
func InitializeImportCollector(
	scheduled bool,
	cbCollectISCounts QueuedImageStreamFetcher,
) {
	registerLock.Lock()
	defer registerLock.Unlock()

	if scheduled {
		isc.cbCollectScheduledCounts = cbCollectISCounts
	} else {
		isc.cbCollectISCounts = cbCollectISCounts
	}

	if collectorRegistered {
		return
	}

	if isc.cbCollectISCounts != nil && isc.cbCollectScheduledCounts != nil {
		prometheus.MustRegister(&isc)
		collectorRegistered = true
		glog.V(4).Info("Image import controller metrics registered with prometherus")
	}
}

func (isc *importStatusCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- successCountDesc
	ch <- errorCountDesc
}

func (isc *importStatusCollector) Collect(ch chan<- prometheus.Metric) {
	successCounts, errorCounts, err := isc.cbCollectISCounts()
	if err != nil {
		glog.Errorf("Failed to collect image import metrics: %v", err)
		ch <- prometheus.NewInvalidMetric(successCountDesc, err)
	} else {
		pushSuccessCounts("false", successCounts, ch)
		pushErrorCounts("false", errorCounts, ch)
	}

	successCounts, errorCounts, err = isc.cbCollectScheduledCounts()
	if err != nil {
		glog.Errorf("Failed to collect scheduled image import metrics: %v", err)
		ch <- prometheus.NewInvalidMetric(errorCountDesc, err)
		return
	}

	pushSuccessCounts("true", successCounts, ch)
	pushErrorCounts("true", errorCounts, ch)
}

func pushSuccessCounts(scheduled string, counts ImportSuccessCounts, ch chan<- prometheus.Metric) {
	for registry, count := range counts {
		ch <- prometheus.MustNewConstMetric(
			successCountDesc,
			prometheus.CounterValue,
			float64(count),
			scheduled,
			registry)
	}
}

func pushErrorCounts(scheduled string, counts ImportErrorCounts, ch chan<- prometheus.Metric) {
	for info, count := range counts {
		ch <- prometheus.MustNewConstMetric(
			errorCountDesc,
			prometheus.CounterValue,
			float64(count),
			scheduled,
			info.Registry,
			info.Reason)
	}
}
