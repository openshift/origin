package watchclusteroperators

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"

	configv1 "github.com/openshift/api/config/v1"
	configclientset "github.com/openshift/client-go/config/clientset/versioned"
)

func startClusterOperatorMonitoring(ctx context.Context, m monitorapi.RecorderWriter, client configclientset.Interface) {
	coInformer := cache.NewSharedIndexInformer(
		newErrorRecordingListWatcher(m, &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return client.ConfigV1().ClusterOperators().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return client.ConfigV1().ClusterOperators().Watch(ctx, options)
			},
		}),
		&configv1.ClusterOperator{},
		time.Hour,
		nil,
	)

	coChangeFns := []func(co, oldCO *configv1.ClusterOperator) []monitorapi.Interval{
		func(co, oldCO *configv1.ClusterOperator) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			intervalTime := time.Now()
			for i := range co.Status.Conditions {
				c := &co.Status.Conditions[i]
				previousCondition := findOperatorStatusCondition(oldCO.Status.Conditions, c.Type)
				// If we don't have a previous state, then we should always mark the starting state with an event.
				// We recently had a PR that caused the kube-apiserver operator be permanently degraded and it didn't show up.
				if previousCondition == nil || c.Status != previousCondition.Status {

					msg := monitorapi.NewMessage().
						WithAnnotations(
							map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCondition: string(c.Type),
								monitorapi.AnnotationStatus:    string(c.Status),
							}).
						HumanMessagef("%s", c.Message)

					if len(c.Reason) > 0 {
						msg = msg.Reason(monitorapi.IntervalReason(c.Reason))
					}

					level := monitorapi.Warning
					if c.Type == configv1.OperatorDegraded && c.Status == configv1.ConditionTrue {
						level = monitorapi.Error
					}
					if c.Type == configv1.OperatorAvailable && c.Status == configv1.ConditionFalse {
						level = monitorapi.Error
					}
					if c.Type == configv1.OperatorProgressing && c.Status == configv1.ConditionTrue {
						level = monitorapi.Warning
					}
					if c.Type == configv1.ClusterStatusConditionType("Failing") && c.Status == configv1.ConditionTrue {
						level = monitorapi.Error
					}
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, level).
						Locator(monitorapi.NewLocator().ClusterOperator(co.Name)).
						Message(msg).Build(intervalTime, intervalTime))
				}
			}
			if changes := findOperatorVersionChange(oldCO.Status.Versions, co.Status.Versions); len(changes) > 0 {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().ClusterOperator(co.Name)).
					Message(monitorapi.NewMessage().HumanMessagef("versions: %v", strings.Join(changes, ", "))).
					Build(intervalTime, intervalTime))
			}
			return intervals
		},
	}

	startTime := time.Now().UTC().Add(-time.Minute)
	coInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				co, ok := obj.(*configv1.ClusterOperator)
				if !ok {
					return
				}
				// filter out old pods so our monitor doesn't send a big chunk
				// of co creations
				if co.CreationTimestamp.Time.Before(startTime) {
					return
				}
				m.AddIntervals(monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().ClusterOperator(co.Name)).
					Message(monitorapi.NewMessage().HumanMessage("created")).BuildNow())
			},
			DeleteFunc: func(obj interface{}) {
				co, ok := obj.(*configv1.ClusterOperator)
				if !ok {
					return
				}
				m.AddIntervals(monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ClusterOperator(co.Name)).
					Message(monitorapi.NewMessage().HumanMessage("deleted")).BuildNow())
			},
			UpdateFunc: func(old, obj interface{}) {
				co, ok := obj.(*configv1.ClusterOperator)
				if !ok {
					return
				}
				oldCO, ok := old.(*configv1.ClusterOperator)
				if !ok {
					return
				}
				if co.UID != oldCO.UID {
					return
				}
				for _, fn := range coChangeFns {
					m.AddIntervals(fn(co, oldCO)...)
				}
			},
		},
	)

	go coInformer.Run(ctx.Done())

	cvInformer := cache.NewSharedIndexInformer(
		newErrorRecordingListWatcher(m, &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				options.FieldSelector = "metadata.name=version"
				return client.ConfigV1().ClusterVersions().List(ctx, options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				options.FieldSelector = "metadata.name=version"
				return client.ConfigV1().ClusterVersions().Watch(ctx, options)
			},
		}),
		&configv1.ClusterVersion{},
		time.Hour,
		nil,
	)

	cvChangeFns := []func(cv, oldCV *configv1.ClusterVersion) []monitorapi.Interval{
		func(cv, oldCV *configv1.ClusterVersion) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if len(cv.Status.History) == 0 {
				return nil
			}
			if len(oldCV.Status.History) == 0 {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ClusterVersion(cv)).
					Message(monitorapi.NewMessage().HumanMessagef("cluster converging to %s", versionOrImage(cv.Status.History[0]))).BuildNow())
				return intervals
			}
			cvNew, cvOld := cv.Status.History[0], oldCV.Status.History[0]
			switch {
			case cvNew.State == configv1.CompletedUpdate && cvOld.State != cvNew.State:
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ClusterVersion(cv)).
					Message(monitorapi.NewMessage().HumanMessagef("cluster reached %s", versionOrImage(cvNew))).BuildNow())
			case cvNew.State == configv1.PartialUpdate && cvOld.State == cvNew.State && cvOld.Image != cvNew.Image:
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ClusterVersion(cv)).
					Message(monitorapi.NewMessage().HumanMessagef("cluster upgrading to %s without completing %s", versionOrImage(cvNew), versionOrImage(cvOld))).BuildNow())
			}
			return intervals
		},
		func(cv, oldCV *configv1.ClusterVersion) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			for i := range cv.Status.Conditions {
				s := &cv.Status.Conditions[i]
				previous := findOperatorStatusCondition(oldCV.Status.Conditions, s.Type)
				if previous == nil || s.Status != previous.Status {
					level := monitorapi.Warning
					if s.Type == configv1.OperatorDegraded && s.Status == configv1.ConditionTrue {
						level = monitorapi.Error
					}
					if s.Type == configv1.ClusterStatusConditionType("Failing") && s.Status == configv1.ConditionTrue {
						level = monitorapi.Error
					}
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, level).
						Locator(monitorapi.NewLocator().ClusterVersion(cv)).
						Message(monitorapi.NewMessage().
							WithAnnotations(map[monitorapi.AnnotationKey]string{
								monitorapi.AnnotationCondition: string(s.Type),
								monitorapi.AnnotationStatus:    string(s.Status),
								monitorapi.AnnotationReason:    s.Reason,
							}).
							HumanMessage(monitorapi.GetOperatorConditionHumanMessage(s, "changed to "))).BuildNow())
				}
			}
			return intervals
		},
	}

	cvInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				cv, ok := obj.(*configv1.ClusterVersion)
				if !ok {
					return
				}
				// filter out old pods so our monitor doesn't send a big chunk
				// of co creations
				if cv.CreationTimestamp.Time.Before(startTime) {
					return
				}
				m.AddIntervals(monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().ClusterVersion(cv)).
					Message(monitorapi.NewMessage().HumanMessage("created")).BuildNow())
			},
			DeleteFunc: func(obj interface{}) {
				cv, ok := obj.(*configv1.ClusterVersion)
				if !ok {
					return
				}
				m.AddIntervals(monitorapi.NewInterval(monitorapi.SourceClusterOperatorMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ClusterVersion(cv)).
					Message(monitorapi.NewMessage().HumanMessage("deleted")).BuildNow())
			},
			UpdateFunc: func(old, obj interface{}) {
				cv, ok := obj.(*configv1.ClusterVersion)
				if !ok {
					return
				}
				oldCV, ok := old.(*configv1.ClusterVersion)
				if !ok {
					return
				}
				if cv.UID != oldCV.UID {
					return
				}
				for _, fn := range cvChangeFns {
					m.AddIntervals(fn(cv, oldCV)...)
				}
			},
		},
	)

	go cvInformer.Run(ctx.Done())
}

func versionOrImage(h configv1.UpdateHistory) string {
	if len(h.Version) == 0 {
		return h.Image
	}
	return h.Version
}

func findOperatorVersionChange(old, new []configv1.OperandVersion) []string {
	var changed []string
	for i := 0; i < len(new); i++ {
		for j := 0; j < len(old); j++ {
			p := (j + i) % len(old)
			if old[p].Name != new[i].Name {
				continue
			}
			if old[p].Version == new[i].Version {
				break
			}
			changed = append(changed, fmt.Sprintf("%s %s -> %s", new[i].Name, old[p].Version, new[i].Version))
			break
		}
	}
	return changed
}

func findOperatorStatusCondition(conditions []configv1.ClusterOperatorStatusCondition, conditionType configv1.ClusterStatusConditionType) *configv1.ClusterOperatorStatusCondition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}
