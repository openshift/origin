package app

import (
	"net"
	"net/http"
	"strconv"

	schedmetrics "github.com/mesos/mesos-go/api/v1/cmd/example-scheduler/app/metrics"
	xmetrics "github.com/mesos/mesos-go/api/v1/lib/extras/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func initMetrics(cfg Config) *metricsAPI {
	schedmetrics.Register()
	metricsAddress := net.JoinHostPort(cfg.server.address, strconv.Itoa(cfg.metrics.port))
	http.Handle(cfg.metrics.path, prometheus.Handler())
	api := newMetricsAPI()
	go forever("api-server", cfg.jobRestartDelay, api.jobStartCount, func() error { return http.ListenAndServe(metricsAddress, nil) })
	return api
}

func newMetricAdder(m prometheus.Counter) xmetrics.Adder {
	return func(x float64, _ ...string) { m.Add(x) }
}

func newMetricCounter(m prometheus.Counter) xmetrics.Counter {
	return func(_ ...string) { m.Inc() }
}

func newMetricCounters(m *prometheus.CounterVec) xmetrics.Counter {
	return func(s ...string) { m.WithLabelValues(s...).Inc() }
}

func newMetricWatcher(m prometheus.Summary) xmetrics.Watcher {
	return func(x float64, _ ...string) { m.Observe(x) }
}

func newMetricWatchers(m *prometheus.SummaryVec) xmetrics.Watcher {
	return func(x float64, s ...string) { m.WithLabelValues(s...).Observe(x) }
}

type metricsAPI struct {
	eventErrorCount       xmetrics.Counter
	eventReceivedCount    xmetrics.Counter
	eventReceivedLatency  xmetrics.Watcher
	callCount             xmetrics.Counter
	callErrorCount        xmetrics.Counter
	callLatency           xmetrics.Watcher
	offersReceived        xmetrics.Adder
	offersDeclined        xmetrics.Adder
	tasksLaunched         xmetrics.Adder
	tasksFinished         xmetrics.Counter
	launchesPerOfferCycle xmetrics.Watcher
	offeredResources      xmetrics.Watcher
	jobStartCount         xmetrics.Counter
	artifactDownloads     xmetrics.Counter
}

func newMetricsAPI() *metricsAPI {
	return &metricsAPI{
		callCount:             newMetricCounters(schedmetrics.CallCount),
		callErrorCount:        newMetricCounters(schedmetrics.CallErrorCount),
		callLatency:           newMetricWatchers(schedmetrics.CallLatency),
		eventErrorCount:       newMetricCounters(schedmetrics.EventErrorCount),
		eventReceivedCount:    newMetricCounters(schedmetrics.EventReceivedCount),
		eventReceivedLatency:  newMetricWatchers(schedmetrics.EventReceivedLatency),
		offersReceived:        newMetricAdder(schedmetrics.OffersReceived),
		offersDeclined:        newMetricAdder(schedmetrics.OffersDeclined),
		tasksLaunched:         newMetricAdder(schedmetrics.TasksLaunched),
		tasksFinished:         newMetricCounter(schedmetrics.TasksFinished),
		launchesPerOfferCycle: newMetricWatcher(schedmetrics.TasksLaunchedPerOfferCycle),
		offeredResources:      newMetricWatchers(schedmetrics.OfferedResources),
		jobStartCount:         newMetricCounters(schedmetrics.JobStartCount),
		artifactDownloads:     newMetricCounter(schedmetrics.ArtifactDownloads),
	}
}
