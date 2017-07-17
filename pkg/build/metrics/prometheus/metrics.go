package prometheus

import (
	kselector "k8s.io/apimachinery/pkg/labels"
	"strings"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	internalversion "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator               = "_"
	buildSubsystem          = "openshift_build"
	terminalBuildCount      = "terminal_phase_total"
	terminalBuildCountQuery = buildSubsystem + separator + terminalBuildCount
	activeBuildCount        = "running_phase_start_time_seconds"
	activeBuildCountQuery   = buildSubsystem + separator + activeBuildCount
)

var (
	// decided not to have a separate counter for failed builds, which have reasons,
	// vs. the other "finished" builds phases, where the reason is not set
	terminalBuildCountDesc = prometheus.NewDesc(
		buildSubsystem+separator+terminalBuildCount,
		"Counts total teriminal builds by phase",
		[]string{"phase"},
		nil,
	)
	activeBuildCountDesc = prometheus.NewDesc(
		buildSubsystem+separator+activeBuildCount,
		"Show the start time in unix epoch form of running builds by namespace, name, and phase",
		[]string{"namespace", "name", "phase"},
		nil,
	)
	bc             = buildCollector{}
	registered     = false
	failedPhase    = strings.ToLower(string(buildapi.BuildPhaseFailed))
	errorPhase     = strings.ToLower(string(buildapi.BuildPhaseError))
	cancelledPhase = strings.ToLower(string(buildapi.BuildPhaseCancelled))
	completePhase  = strings.ToLower(string(buildapi.BuildPhaseComplete))
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
	ch <- terminalBuildCountDesc
	ch <- activeBuildCountDesc
}

// Collect implements the prometheus.Collector interface.
func (bc *buildCollector) Collect(ch chan<- prometheus.Metric) {
	result, err := bc.lister.List(kselector.Everything())

	if err != nil {
		glog.V(4).Infof("Collect err %v", err)
		return
	}

	// since we do not collect terminal build metrics on a per build basis, collectBuild will return counts
	// to be added to the total amount posted to prometheus
	var failed, error, cancelled, complete int
	for _, b := range result {
		f, e, cc, cp := bc.collectBuild(ch, b)
		failed = failed + f
		error = error + e
		cancelled = cancelled + cc
		complete = complete + cp
	}
	addCountGauge(ch, terminalBuildCountDesc, failedPhase, float64(failed))
	addCountGauge(ch, terminalBuildCountDesc, errorPhase, float64(error))
	addCountGauge(ch, terminalBuildCountDesc, cancelledPhase, float64(cancelled))
	addCountGauge(ch, terminalBuildCountDesc, completePhase, float64(complete))
}

func addCountGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, phase string, v float64) {
	lv := []string{phase}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
}

func addTimeGauge(ch chan<- prometheus.Metric, b *buildapi.Build, desc *prometheus.Desc) {
	if b.Status.StartTimestamp != nil {
		lv := []string{b.ObjectMeta.Namespace, b.ObjectMeta.Name, strings.ToLower(string(b.Status.Phase))}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(b.Status.StartTimestamp.Unix()), lv...)
	}
}

func (bc *buildCollector) collectBuild(ch chan<- prometheus.Metric, b *buildapi.Build) (failed, error, cancelled, complete int) {

	switch b.Status.Phase {
	// remember, new and pending builds don't have a start time
	case buildapi.BuildPhaseRunning:
		addTimeGauge(ch, b, activeBuildCountDesc)
	case buildapi.BuildPhaseFailed:
		failed++
	case buildapi.BuildPhaseError:
		error++
	case buildapi.BuildPhaseCancelled:
		cancelled++
	case buildapi.BuildPhaseComplete:
		complete++
	}
	return
}
