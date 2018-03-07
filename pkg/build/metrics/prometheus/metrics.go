package prometheus

import (
	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kselector "k8s.io/apimachinery/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	internalversion "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator        = "_"
	buildSubsystem   = "openshift_build"
	buildCount       = "total"
	buildCountQuery  = buildSubsystem + separator + buildCount
	activeBuild      = "active_time_seconds"
	activeBuildQuery = buildSubsystem + separator + activeBuild
)

var (
	buildCountDesc = prometheus.NewDesc(
		buildCountQuery,
		"Counts builds by phase, reason, and strategy",
		[]string{"phase", "reason", "strategy"},
		nil,
	)
	activeBuildDesc = prometheus.NewDesc(
		activeBuildQuery,
		"Shows the last transition time in unix epoch for running builds by namespace, name, phase, reason, and strategy",
		[]string{"namespace", "name", "phase", "reason", "strategy"},
		nil,
	)
	bc             = buildCollector{}
	registered     = false
	cancelledPhase = string(buildapi.BuildPhaseCancelled)
	completePhase  = string(buildapi.BuildPhaseComplete)
	failedPhase    = string(buildapi.BuildPhaseFailed)
	errorPhase     = string(buildapi.BuildPhaseError)
	newPhase       = string(buildapi.BuildPhaseNew)
	pendingPhase   = string(buildapi.BuildPhasePending)
	runningPhase   = string(buildapi.BuildPhaseRunning)
)

type buildCollector struct {
	lister internalversion.BuildLister
}

// InitializeMetricsCollector calls into prometheus to register the buildCollector struct as a Collector in prometheus
// for the terminal and active build metrics; note, in comparing with how kube-state-metrics integrates with prometheus,
// kube-state-metrics leverages the prometheus.Registerer function, but it does not exist in the version of prometheus
// vendored into origin as of this writing
func IntializeMetricsCollector(buildLister internalversion.BuildLister) {
	bc.lister = buildLister
	// unit tests unearthed multiple (sequential, not in parallel) registrations with prometheus via multiple calls to new build controller
	if !registered {
		prometheus.MustRegister(&bc)
		registered = true
	}
	glog.V(4).Info("build metrics registered with prometheus")
}

// Describe implements the prometheus.Collector interface.
func (bc *buildCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- buildCountDesc
	ch <- activeBuildDesc
}

type collectKey struct {
	phase    string
	reason   string
	strategy string
}

// Collect implements the prometheus.Collector interface.
func (bc *buildCollector) Collect(ch chan<- prometheus.Metric) {
	result, err := bc.lister.List(kselector.Everything())

	if err != nil {
		glog.V(4).Infof("Collect err %v", err)
		return
	}

	// collectBuild will return counts for the build's phase/reason tuple,
	// and counts for these tuples be added to the total amount posted to prometheus
	counts := map[collectKey]int{}
	for _, b := range result {
		k := bc.collectBuild(ch, b)
		counts[k] = counts[k] + 1
	}

	for key, count := range counts {
		addCountGauge(ch, buildCountDesc, key.phase, key.reason, key.strategy, float64(count))
	}
}

func addCountGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, phase, reason, strategy string, v float64) {
	lv := []string{phase, reason, strategy}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
}

func addTimeGauge(ch chan<- prometheus.Metric, b *buildapi.Build, time *metav1.Time, desc *prometheus.Desc, phase string, reason string, strategy string) {
	if time != nil {
		lv := []string{b.ObjectMeta.Namespace, b.ObjectMeta.Name, phase, reason, strategy}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(time.Unix()), lv...)
	}
}

func (bc *buildCollector) collectBuild(ch chan<- prometheus.Metric, b *buildapi.Build) (key collectKey) {

	r := string(b.Status.Reason)
	s := buildapi.StrategyType(b.Spec.Strategy)
	key = collectKey{reason: r, strategy: s}
	switch b.Status.Phase {
	// remember, new and pending builds don't have a start time
	case buildapi.BuildPhaseNew:
		key.phase = newPhase
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, newPhase, r, s)
	case buildapi.BuildPhasePending:
		key.phase = pendingPhase
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, pendingPhase, r, s)
	case buildapi.BuildPhaseRunning:
		key.phase = runningPhase
		addTimeGauge(ch, b, b.Status.StartTimestamp, activeBuildDesc, runningPhase, r, s)
	case buildapi.BuildPhaseFailed:
		key.phase = failedPhase
	case buildapi.BuildPhaseError:
		key.phase = errorPhase
	case buildapi.BuildPhaseCancelled:
		key.phase = cancelledPhase
	case buildapi.BuildPhaseComplete:
		key.phase = completePhase
	}
	return key
}
