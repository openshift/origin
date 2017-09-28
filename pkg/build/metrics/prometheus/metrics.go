package prometheus

import (
	"strings"

	"github.com/golang/glog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kselector "k8s.io/apimachinery/pkg/labels"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	internalversion "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
	"github.com/prometheus/client_golang/prometheus"
)

const (
	separator                 = "_"
	buildSubsystem            = "openshift_build"
	terminalBuildCount        = "terminal_phase_total"
	terminalBuildCountQuery   = buildSubsystem + separator + terminalBuildCount
	failedBuildCount          = "failed_phase_total"
	failedBuildCountQuery     = buildSubsystem + separator + failedBuildCount
	activeBuildCount          = "running_phase_start_time_seconds"
	activeBuildCountQuery     = buildSubsystem + separator + activeBuildCount
	newPendingBuildCount      = "new_pending_phase_creation_time_seconds"
	newPendingBuildCountQuery = buildSubsystem + separator + newPendingBuildCount
	errorBuildReason          = "BuildPodError"
)

var (
	terminalBuildCountDesc = prometheus.NewDesc(
		terminalBuildCountQuery,
		"Counts total successful/aborted builds by phase",
		[]string{"phase"},
		nil,
	)
	failedBuildCountDesc = prometheus.NewDesc(
		failedBuildCountQuery,
		"Counts total failed builds by reason",
		[]string{"reason"},
		nil,
	)
	activeBuildCountDesc = prometheus.NewDesc(
		activeBuildCountQuery,
		"Show the start time in unix epoch form of running builds by namespace and name",
		[]string{"namespace", "name"},
		nil,
	)
	newPendingBuildCountDesc = prometheus.NewDesc(
		newPendingBuildCountQuery,
		"Show the creation time in unix epoch form of new or pending builds by namespace, name, and phase",
		[]string{"namespace", "name", "phase"},
		nil,
	)
	bc             = buildCollector{}
	registered     = false
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
	var cancelledCount, completeCount int
	reasons := map[string]int{}
	for _, b := range result {
		cc, cp, r := bc.collectBuild(ch, b)
		for key, value := range r {
			reasons[key] = reasons[key] + value
		}
		cancelledCount = cancelledCount + cc
		completeCount = completeCount + cp
	}
	// explicitly note there are no failed builds
	if len(reasons) == 0 {
		addCountGauge(ch, failedBuildCountDesc, "", float64(0))
	}
	for reason, count := range reasons {
		addCountGauge(ch, failedBuildCountDesc, reason, float64(count))
	}
	addCountGauge(ch, terminalBuildCountDesc, cancelledPhase, float64(cancelledCount))
	addCountGauge(ch, terminalBuildCountDesc, completePhase, float64(completeCount))
}

func addCountGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, label string, v float64) {
	lv := []string{label}
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, v, lv...)
}

func addTimeGauge(ch chan<- prometheus.Metric, b *buildapi.Build, time *metav1.Time, desc *prometheus.Desc, phase string) {
	if time != nil {
		lv := []string{b.ObjectMeta.Namespace, b.ObjectMeta.Name}
		if len(phase) > 0 {
			lv = append(lv, strings.ToLower(phase))
		}
		ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, float64(time.Unix()), lv...)
	}
}

func (bc *buildCollector) collectBuild(ch chan<- prometheus.Metric, b *buildapi.Build) (cancelledCount, completeCount int, reasonsCount map[string]int) {

	reasonsCount = map[string]int{}
	switch b.Status.Phase {
	// remember, new and pending builds don't have a start time
	case buildapi.BuildPhaseNew:
	case buildapi.BuildPhasePending:
		addTimeGauge(ch, b, &b.CreationTimestamp, newPendingBuildCountDesc, string(b.Status.Phase))
	case buildapi.BuildPhaseRunning:
		addTimeGauge(ch, b, b.Status.StartTimestamp, activeBuildCountDesc, "")
	case buildapi.BuildPhaseFailed:
		// currently only failed builds have reasons
		reasonsCount[string(b.Status.Reason)] = 1
	case buildapi.BuildPhaseError:
		// it was decided to couple this one under failed, using the custom 'BuildPodError'
		reasonsCount[errorBuildReason] = 1
	case buildapi.BuildPhaseCancelled:
		cancelledCount++
	case buildapi.BuildPhaseComplete:
		completeCount++
	}
	return
}
