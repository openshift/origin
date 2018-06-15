package prometheus

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kcorelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

var (
	timeNow          = metav1.Now()
	defaultTimeNowFn = func() time.Time { return timeNow.Time }
)

func mockRC(name string, version int, annotations map[string]string, generation int64, creationTime metav1.Time) *kapiv1.ReplicationController {
	r := &kapiv1.ReplicationController{}
	annotations[appsapi.DeploymentConfigAnnotation] = name
	r.SetName(name + fmt.Sprintf("-%d", version))
	r.SetNamespace("test")
	r.SetCreationTimestamp(creationTime)
	r.SetAnnotations(annotations)
	return r
}

func TestCollect(t *testing.T) {
	tests := []struct {
		name  string
		count int
		rcs   []*kapiv1.ReplicationController
		// expected values
		available     float64
		failed        float64
		cancelled     float64
		timestamp     float64
		latestVersion string
	}{
		{
			name:      "no deployments",
			count:     3,
			available: 0,
			failed:    0,
			cancelled: 0,
			rcs:       []*kapiv1.ReplicationController{},
		},
		{
			name:      "single successful deployment",
			count:     3,
			available: 1,
			failed:    0,
			cancelled: 0,
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation: string(appsapi.DeploymentStatusComplete),
				}, 0, timeNow),
			},
		},
		{
			name:          "single cancelled deployment",
			count:         3,
			available:     0,
			failed:        0,
			cancelled:     1,
			latestVersion: "1",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentCancelledAnnotation: appsapi.DeploymentCancelledAnnotationValue,
					appsapi.DeploymentStatusAnnotation:    string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation:   "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "single failed deployment",
			count:         4,
			available:     0,
			failed:        1,
			cancelled:     0,
			latestVersion: "1",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "multiple failed deployment",
			count:         4,
			available:     0,
			failed:        4,
			cancelled:     0,
			latestVersion: "4",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
				mockRC("foo", 4, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "4",
				}, 0, timeNow),
			},
		},
		{
			name:          "single failed deployment within successful deployments",
			count:         3,
			available:     2,
			failed:        1,
			cancelled:     0,
			latestVersion: "2",
			timestamp:     float64(timeNow.Unix()),
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusComplete),
					appsapi.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusFailed),
					appsapi.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusComplete),
					appsapi.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
			},
		},
		{
			name:          "single active deployment",
			count:         4,
			available:     0,
			failed:        0,
			cancelled:     0,
			latestVersion: "1",
			// the timestamp is duration in this case, which is 0 as the creation time
			// and current time are the same.
			timestamp: 0,
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusRunning),
					appsapi.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
			},
		},
		{
			name:          "single active deployment with history",
			count:         4,
			available:     2,
			failed:        0,
			cancelled:     0,
			latestVersion: "3",
			// the timestamp is duration in this case, which is 0 as the creation time
			// and current time are the same.
			timestamp: 0,
			rcs: []*kapiv1.ReplicationController{
				mockRC("foo", 1, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusComplete),
					appsapi.DeploymentVersionAnnotation: "1",
				}, 0, timeNow),
				mockRC("foo", 2, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusComplete),
					appsapi.DeploymentVersionAnnotation: "2",
				}, 0, timeNow),
				mockRC("foo", 3, map[string]string{
					appsapi.DeploymentStatusAnnotation:  string(appsapi.DeploymentStatusRunning),
					appsapi.DeploymentVersionAnnotation: "3",
				}, 0, timeNow),
			},
		},
	}

	for _, c := range tests {
		rcCache := cache.NewIndexer(cache.MetaNamespaceKeyFunc, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc})
		fakeCollector := appsCollector{
			lister: kcorelisters.NewReplicationControllerLister(rcCache),
			nowFn:  defaultTimeNowFn,
		}

		for _, rc := range c.rcs {
			if err := rcCache.Add(rc); err != nil {
				t.Fatalf("unable to add rc %s: %v", rc.Name, err)
			}
		}

		collectedMetrics := []prometheus.Metric{}
		collectionChan := make(chan prometheus.Metric)
		stopChan := make(chan struct{})
		go func() {
			defer close(collectionChan)
			fakeCollector.Collect(collectionChan)
			<-stopChan
		}()

		for {
			select {
			case m := <-collectionChan:
				collectedMetrics = append(collectedMetrics, m)
			case <-time.After(time.Second * 5):
				t.Fatalf("[%s] timeout receiving expected results (got %d, want %d)", c.name, len(collectedMetrics), c.count)
			}
			if len(collectedMetrics) == c.count {
				close(stopChan)
				break
			}
		}

		if len(collectedMetrics) == 0 {
			continue
		}

		for _, m := range collectedMetrics {
			var out dto.Metric
			m.Write(&out)

			// last_failed_rollout_time
			if strings.Contains(m.Desc().String(), nameToQuery(lastFailedRolloutTime)) {
				gaugeValue := out.GetGauge().GetValue()
				labels := out.GetLabel()
				if gaugeValue != c.timestamp {
					t.Errorf("[%s][last_failed_rollout_time] creation timestamp %f does not match expected timestamp: %f", c.name, gaugeValue, c.timestamp)
				}
				for _, l := range labels {
					if l.GetName() == "latest_version" && l.GetValue() != c.latestVersion {
						t.Errorf("[%s][last_failed_rollout_time] latest_version %q does not match expected version %q", c.name, l.GetValue(), c.latestVersion)
					}
				}
				continue
			}

			// active_rollouts_duration_seconds
			if strings.Contains(m.Desc().String(), nameToQuery(activeRolloutDurationSeconds)) {
				gaugeValue := out.GetGauge().GetValue()
				labels := out.GetLabel()
				if gaugeValue != c.timestamp {
					t.Errorf("[%s][active_rollouts_duration_seconds] creation timestamp %f does not match expected timestamp: %f", c.name, gaugeValue, c.timestamp)
				}
				for _, l := range labels {
					if l.GetName() == "latest_version" && l.GetValue() != c.latestVersion {
						t.Errorf("[%s][active_rollouts_duration_seconds] latest_version %q does not match expected version %q", c.name, l.GetValue(), c.latestVersion)
					}
				}
				continue
			}

			// complete_rollouts_total
			if strings.Contains(m.Desc().String(), nameToQuery(completeRolloutCount)) {
				gaugeValue := out.GetGauge().GetValue()
				switch out.GetLabel()[0].GetValue() {
				case availablePhase:
					if c.available != gaugeValue {
						t.Errorf("[%s][complete_rollouts_total] expected available %f, got %f", c.name, c.available, gaugeValue)
					}
				case failedPhase:
					if c.failed != gaugeValue {
						t.Errorf("[%s][complete_rollouts_total] expected failed %f, got %f", c.name, c.failed, gaugeValue)
					}
				case cancelledPhase:
					if c.cancelled != gaugeValue {
						t.Errorf("[%s][]complete_rollouts_total expected cancelled %f, got %f", c.name, c.cancelled, gaugeValue)
					}
				}
				continue
			}

			t.Errorf("[%s] unexpected metric recorded: %s", c.name, m.Desc().String())
		}
	}
}
