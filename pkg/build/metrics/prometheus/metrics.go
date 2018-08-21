package prometheus

import (
	"github.com/golang/glog"

	"github.com/prometheus/client_golang/prometheus"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kselector "k8s.io/apimachinery/pkg/labels"

	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
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
	cancelledPhase = string(buildv1.BuildPhaseCancelled)
	completePhase  = string(buildv1.BuildPhaseComplete)
	failedPhase    = string(buildv1.BuildPhaseFailed)
	errorPhase     = string(buildv1.BuildPhaseError)
	newPhase       = string(buildv1.BuildPhaseNew)
	pendingPhase   = string(buildv1.BuildPhasePending)
	runningPhase   = string(buildv1.BuildPhaseRunning)
)

type buildCollector struct {
	lister buildlister.BuildLister
}

// InitializeMetricsCollector calls into prometheus to register the buildCollector struct as a Collector in prometheus
// for the terminal and active build metrics; note, in comparing with how kube-state-metrics integrates with prometheus,
// kube-state-metrics leverages the prometheus.Registerer function, but it does not exist in the version of prometheus
// vendored into origin as of this writing
func IntializeMetricsCollector(buildLister buildlister.BuildLister) {
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

func addTimeGauge(ch chan<- prometheus.Metric, b *buildv1.Build, time *metav1.Time, desc *prometheus.Desc, phase string, reason string, strategy string) {
	if time != nil {
		lv := []string{b.ObjectMeta.Namespace, b.ObjectMeta.Name, phase, reason, strategy}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(time.Unix()), lv...)
	}
}

func (bc *buildCollector) collectBuild(ch chan<- prometheus.Metric, b *buildv1.Build) (key collectKey) {

	r := string(b.Status.Reason)
	s := buildapihelpers.StrategyType(b.Spec.Strategy)
	key = collectKey{reason: r, strategy: s}
	switch b.Status.Phase {
	// remember, new and pending builds don't have a start time
	case buildv1.BuildPhaseNew:
		key.phase = newPhase
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, newPhase, r, s)
	case buildv1.BuildPhasePending:
		key.phase = pendingPhase
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, pendingPhase, r, s)
	case buildv1.BuildPhaseRunning:
		key.phase = runningPhase
		addTimeGauge(ch, b, b.Status.StartTimestamp, activeBuildDesc, runningPhase, r, s)
	case buildv1.BuildPhaseFailed:
		key.phase = failedPhase
	case buildv1.BuildPhaseError:
		key.phase = errorPhase
	case buildv1.BuildPhaseCancelled:
		key.phase = cancelledPhase
	case buildv1.BuildPhaseComplete:
		key.phase = completePhase
	}
	return key
}
