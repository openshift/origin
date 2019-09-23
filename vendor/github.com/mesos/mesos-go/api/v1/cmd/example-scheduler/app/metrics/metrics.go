package metrics

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

const (
	Subsystem = "example_scheduler"
)

// TODO(jdef) time in between offers

var (
	CallErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "call_error_count",
		Help:      "The number of errors for outgoing calls.",
	}, []string{"type"})
	CallCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "call_count",
		Help:      "The number of outgoing calls.",
	}, []string{"type"})
	CallLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Subsystem: Subsystem,
		Name:      "call_latency",
		Help:      "Time to execute various calls, by type.",
	}, []string{"type"})
	EventErrorCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "event_error_count",
		Help:      "The number of event processing errors.",
	}, []string{"type"})
	EventReceivedCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "event_received_count",
		Help:      "The number of events received.",
	}, []string{"type"})
	EventReceivedLatency = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Subsystem: Subsystem,
		Name:      "event_received_latency",
		Help:      "Time to process various events, by type.",
	}, []string{"type"})
	OffersReceived = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "offers_received",
		Help:      "The number of individual offers received.",
	})
	OffersDeclined = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "offers_declined",
		Help:      "The number of offers declined.",
	})
	TasksFinished = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "tasks_finished",
		Help:      "The number of tasks finished.",
	})
	TasksLaunched = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "tasks_launched",
		Help:      "The number of tasks launched.",
	})
	JobStartCount = prometheus.NewCounterVec(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "job_start_count",
		Help:      "The number of internal background jobs started.",
	}, []string{"job"})
	OfferedResources = prometheus.NewSummaryVec(prometheus.SummaryOpts{
		Subsystem: Subsystem,
		Name:      "offered_resources",
		Help:      "Scalar resources offered by type.",
	}, []string{"type"})
	TasksLaunchedPerOfferCycle = prometheus.NewSummary(prometheus.SummaryOpts{
		Subsystem: Subsystem,
		Name:      "tasks_launched_per_cycle",
		Help:      "Number of tasks launched per-offers cycle (event).",
	})
	ArtifactDownloads = prometheus.NewCounter(prometheus.CounterOpts{
		Subsystem: Subsystem,
		Name:      "artifact_downloads",
		Help:      "The number of artifacts served by the built-in http server.",
	})
)

var registerMetrics sync.Once

func Register() {
	registerMetrics.Do(func() {
		prometheus.MustRegister(CallErrorCount)
		prometheus.MustRegister(CallCount)
		prometheus.MustRegister(CallLatency)
		prometheus.MustRegister(EventErrorCount)
		prometheus.MustRegister(EventReceivedCount)
		prometheus.MustRegister(EventReceivedLatency)
		prometheus.MustRegister(OffersReceived)
		prometheus.MustRegister(OffersDeclined)
		prometheus.MustRegister(JobStartCount)
		prometheus.MustRegister(TasksFinished)
		prometheus.MustRegister(TasksLaunched)
		prometheus.MustRegister(OfferedResources)
		prometheus.MustRegister(TasksLaunchedPerOfferCycle)
		prometheus.MustRegister(ArtifactDownloads)
	})
}
