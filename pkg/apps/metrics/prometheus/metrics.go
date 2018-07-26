package prometheus

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/prometheus/client_golang/prometheus"

	"k8s.io/apimachinery/pkg/labels"
	kcorelisters "k8s.io/client-go/listers/core/v1"

	"github.com/openshift/origin/pkg/apps/util"
)

const (
	completeRolloutCount         = "complete_rollouts_total"
	activeRolloutDurationSeconds = "active_rollouts_duration_seconds"
	lastFailedRolloutTime        = "last_failed_rollout_time"

	availablePhase = "available"
	failedPhase    = "failed"
	cancelledPhase = "cancelled"
)

var (
	nameToQuery = func(name string) string {
		return strings.Join([]string{"openshift_apps_deploymentconfigs", name}, "_")
	}

	completeRolloutCountDesc = prometheus.NewDesc(
		nameToQuery(completeRolloutCount),
		"Counts total complete rollouts",
		[]string{"phase"}, nil,
	)

	lastFailedRolloutTimeDesc = prometheus.NewDesc(
		nameToQuery(lastFailedRolloutTime),
		"Tracks the time of last failure rollout per deployment config",
		[]string{"namespace", "name", "latest_version"}, nil,
	)

	activeRolloutDurationSecondsDesc = prometheus.NewDesc(
		nameToQuery(activeRolloutDurationSeconds),
		"Tracks the active rollout duration in seconds",
		[]string{"namespace", "name", "phase", "latest_version"}, nil,
	)

	apps       = appsCollector{}
	registered = false
)

type appsCollector struct {
	lister kcorelisters.ReplicationControllerLister
	nowFn  func() time.Time
}

func InitializeMetricsCollector(rcLister kcorelisters.ReplicationControllerLister) {
	apps.lister = rcLister
	apps.nowFn = func() time.Time { return time.Now() }
	if !registered {
		prometheus.MustRegister(&apps)
		registered = true
	}
	glog.V(4).Info("apps metrics registered with prometheus")
}

func (c *appsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- completeRolloutCountDesc
	ch <- activeRolloutDurationSecondsDesc
}

type failedRollout struct {
	timestamp     float64
	latestVersion int64
}

// Collect implements the prometheus.Collector interface.
func (c *appsCollector) Collect(ch chan<- prometheus.Metric) {
	result, err := c.lister.List(labels.Everything())
	if err != nil {
		glog.V(4).Infof("Collecting metrics for apps failed: %v", err)
		return
	}

	var available, failed, cancelled float64

	latestFailedRollouts := map[string]failedRollout{}

	for _, d := range result {
		dcName := util.DeploymentConfigNameFor(d)
		if len(dcName) == 0 {
			continue
		}
		latestVersion := util.DeploymentVersionFor(d)
		key := d.Namespace + "/" + dcName

		if util.IsTerminatedDeployment(d) {
			if util.IsDeploymentCancelled(d) {
				cancelled++
				continue
			}
			if util.IsFailedDeployment(d) {
				failed++
				// Track the latest failed rollout per deployment config
				// continue only when this is the latest version (add if below)
				if r, exists := latestFailedRollouts[key]; exists && latestVersion <= r.latestVersion {
					continue
				}
				latestFailedRollouts[key] = failedRollout{
					timestamp:     float64(d.CreationTimestamp.Unix()),
					latestVersion: latestVersion,
				}
				continue
			}
			if util.IsCompleteDeployment(d) {
				// If a completed rollout is found AFTER we recorded a failed rollout,
				// do not record the lastFailedRollout as the latest rollout is not
				// failed.
				if r, hasFailedRollout := latestFailedRollouts[key]; hasFailedRollout && r.latestVersion < latestVersion {
					delete(latestFailedRollouts, key)
				}
				available++
				continue
			}
		}

		// TODO: Figure out under what circumstances the phase is not set.
		phase := strings.ToLower(string(util.DeploymentStatusFor(d)))
		if len(phase) == 0 {
			phase = "unknown"
		}

		// Record duration in seconds for active rollouts
		// TODO: possible time skew?
		durationSeconds := c.nowFn().Unix() - d.CreationTimestamp.Unix()
		ch <- prometheus.MustNewConstMetric(
			activeRolloutDurationSecondsDesc,
			prometheus.CounterValue,
			float64(durationSeconds),
			[]string{
				d.Namespace,
				dcName,
				phase,
				fmt.Sprintf("%d", latestVersion),
			}...)
	}

	// Record latest failed rollouts
	for dc, r := range latestFailedRollouts {
		parts := strings.Split(dc, "/")
		ch <- prometheus.MustNewConstMetric(
			lastFailedRolloutTimeDesc,
			prometheus.GaugeValue,
			r.timestamp,
			[]string{
				parts[0],
				parts[1],
				fmt.Sprintf("%d", r.latestVersion),
			}...)
	}

	ch <- prometheus.MustNewConstMetric(completeRolloutCountDesc, prometheus.GaugeValue, available, []string{availablePhase}...)
	ch <- prometheus.MustNewConstMetric(completeRolloutCountDesc, prometheus.GaugeValue, failed, []string{failedPhase}...)
	ch <- prometheus.MustNewConstMetric(completeRolloutCountDesc, prometheus.GaugeValue, cancelled, []string{cancelledPhase}...)
}
