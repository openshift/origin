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
		"Counts builds by phase and reason",
		[]string{"phase", "reason"},
		nil,
	)
	activeBuildDesc = prometheus.NewDesc(
		activeBuildQuery,
		"Shows the last transition time in unix epoch for running builds by namespace, name, and phase",
		[]string{"namespace", "name", "phase"},
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

// Collect implements the prometheus.Collector interface.
func (bc *buildCollector) Collect(ch chan<- prometheus.Metric) {
	result, err := bc.lister.List(kselector.Everything())

	if err != nil {
		glog.V(4).Infof("Collect err %v", err)
		return
	}

	// since we do not collect terminal build metrics on a per build basis, collectBuild will return counts
	// to be added to the total amount posted to prometheus
	var cancelledCount, completeCount, errorCount, newCount, pendingCount, runningCount int
	reasons := map[string]int{}
	for _, b := range result {
		cc, cp, ec, nc, pc, rc, r := bc.collectBuild(ch, b)
		for key, value := range r {
			reasons[key] = reasons[key] + value
		}
		cancelledCount = cancelledCount + cc
		completeCount = completeCount + cp
		errorCount = errorCount + ec
		newCount = newCount + nc
		pendingCount = pendingCount + pc
		runningCount = runningCount + rc
	}
	// explicitly note there are no failed builds
	if len(reasons) == 0 {
		addCountGauge(ch, buildCountDesc, failedPhase, "", float64(0))
	}
	for reason, count := range reasons {
		addCountGauge(ch, buildCountDesc, failedPhase, reason, float64(count))
	}
	addCountGauge(ch, buildCountDesc, cancelledPhase, "", float64(cancelledCount))
	addCountGauge(ch, buildCountDesc, completePhase, "", float64(completeCount))
	addCountGauge(ch, buildCountDesc, errorPhase, "", float64(errorCount))
	addCountGauge(ch, buildCountDesc, newPhase, "", float64(newCount))
	addCountGauge(ch, buildCountDesc, pendingPhase, "", float64(pendingCount))
	addCountGauge(ch, buildCountDesc, runningPhase, "", float64(runningCount))
}

func addCountGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, phase, reason string, v float64) {
	lv := []string{phase, reason}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
}

func addTimeGauge(ch chan<- prometheus.Metric, b *buildapi.Build, time *metav1.Time, desc *prometheus.Desc, phase string) {
	if time != nil {
		lv := []string{b.ObjectMeta.Namespace, b.ObjectMeta.Name, phase}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(time.Unix()), lv...)
	}
}

func (bc *buildCollector) collectBuild(ch chan<- prometheus.Metric, b *buildapi.Build) (cancelledCount, completeCount, errorCount, newCount, pendingCount, runningCount int, reasonsCount map[string]int) {

	reasonsCount = map[string]int{}
	switch b.Status.Phase {
	// remember, new and pending builds don't have a start time
	case buildapi.BuildPhaseNew:
		newCount++
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, newPhase)
	case buildapi.BuildPhasePending:
		pendingCount++
		addTimeGauge(ch, b, &b.CreationTimestamp, activeBuildDesc, pendingPhase)
	case buildapi.BuildPhaseRunning:
		runningCount++
		addTimeGauge(ch, b, b.Status.StartTimestamp, activeBuildDesc, runningPhase)
	case buildapi.BuildPhaseFailed:
		// currently only failed builds have reasons
		reasonsCount[string(b.Status.Reason)] = 1
	case buildapi.BuildPhaseError:
		errorCount++
	case buildapi.BuildPhaseCancelled:
		cancelledCount++
	case buildapi.BuildPhaseComplete:
		completeCount++
	}
	return
}
