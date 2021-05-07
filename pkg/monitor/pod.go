package monitor

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startPodMonitoring(ctx context.Context, m Recorder, client kubernetes.Interface) {
	podInformer := cache.NewSharedIndexInformer(
		NewErrorRecordingListWatcher(m, &cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				items, err := client.CoreV1().Pods("").List(ctx, options)
				if err == nil {
					last := 0
					for i := range items.Items {
						item := &items.Items[i]
						// if !filterToSystemNamespaces(item) {
						// 	continue
						// }
						if i != last {
							items.Items[last] = *item
							last++
						}
					}
					items.Items = items.Items[:last]
				}
				return items, err
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				w, err := client.CoreV1().Pods("").Watch(ctx, options)
				if err == nil {
					w = watch.Filter(w, func(in watch.Event) (watch.Event, bool) {
						// return in, filterToSystemNamespaces(in.Object)
						return in, true
					})
				}
				return w, err
			},
		}),
		&corev1.Pod{},
		time.Hour,
		nil,
	)

	m.AddSampler(func(now time.Time) []*monitorapi.Condition {
		var conditions []*monitorapi.Condition
		for _, obj := range podInformer.GetStore().List() {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}
			if pod.Status.Phase == "Pending" {
				if now.Sub(pod.CreationTimestamp.Time) > time.Minute {
					conditions = append(conditions, &monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePod(pod),
						Message: "pod has been pending longer than a minute",
					})
				}
			}
		}
		return conditions
	})

	podChangeFns := []func(pod, oldPod *corev1.Pod) []monitorapi.Condition{
		// check phase transitions
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			new, old := pod.Status.Phase, oldPod.Status.Phase
			if new == old || len(old) == 0 {
				return nil
			}
			var conditions []monitorapi.Condition
			switch {
			case new == corev1.PodPending && old != corev1.PodUnknown:
				switch {
				case pod.DeletionTimestamp != nil:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): pod should not transition %s->%s even when terminated", old, new),
					})
				case isMirrorPod(pod):
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): static pod should not transition %s->%s with same UID", old, new),
					})
				default:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("pod moved back to Pending"),
					})
				}
			case new == corev1.PodUnknown:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Warning,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("pod moved to the Unknown phase"),
				})
			case new == corev1.PodFailed && old != corev1.PodFailed:
				switch pod.Status.Reason {
				case "Evicted":
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Evicted %s", pod.Status.Message),
					})
				case "Preempting":
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Preempted %s", pod.Status.Message),
					})
				default:
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Failed (%s): %s", pod.Status.Reason, pod.Status.Message),
					})
				}
				for _, s := range pod.Status.InitContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("init container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
				for _, s := range pod.Status.ContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
			}
			return conditions
		},
		// check for transitions to being deleted
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			if pod.DeletionGracePeriodSeconds != nil && oldPod.DeletionGracePeriodSeconds == nil {
				switch {
				case len(pod.Spec.NodeName) == 0:
					// pods that have not been assigned to a node are deleted immediately
				case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
					// terminal pods are immediately deleted (do not undergo graceful deletion)
				default:
					if *pod.DeletionGracePeriodSeconds == 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: locatePod(pod),
							Message: fmt.Sprintf("reason/ForceDelete mirrored/%t", isMirrorPod(pod)),
						})
					} else {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: locatePod(pod),
							Message: fmt.Sprintf("reason/GracefulDelete duration/%ds", *pod.DeletionGracePeriodSeconds),
						})
					}
				}
			}
			if pod.DeletionGracePeriodSeconds == nil && oldPod.DeletionGracePeriodSeconds != nil {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: locatePod(pod),
					Message: "invariant violation: pod was marked for deletion and then deletion grace period was cleared",
				})
			}
			return conditions
		},
		// check restarts, readiness drop outs, or other status changes
		func(pod, oldPod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition

			// container status should never be removed since the kubelet should be
			// synthesizing status from the apiserver in order to determine what to
			// run after a reboot (this is likely to be the result of a pod going back
			// to pending status)
			if len(pod.Status.ContainerStatuses) < len(oldPod.Status.ContainerStatuses) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: locatePod(pod),
					Message: "invariant violation: container statuses were removed",
				})
			}
			if len(pod.Status.InitContainerStatuses) < len(oldPod.Status.InitContainerStatuses) {
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Error,
					Locator: locatePod(pod),
					Message: "invariant violation: init container statuses were removed",
				})
			}

			for i := range pod.Status.ContainerStatuses {
				s := &pod.Status.ContainerStatuses[i]
				previous := findContainerStatus(oldPod.Status.ContainerStatuses, s.Name, i)
				if previous == nil {
					continue
				}
				if previous.LastTerminationState.Terminated != nil && s.LastTerminationState.Terminated == nil {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locatePodContainer(pod, s.Name),
						Message: fmt.Sprintf("reason/TerminationStateCleared lastState.terminated was cleared on a pod (bug https://bugzilla.redhat.com/show_bug.cgi?id=1933760 or similar)"),
					})
				}
				if pod.DeletionTimestamp == nil {
					if t := s.State.Waiting; t != nil && (previous.State.Waiting == nil || t.Reason != previous.State.Waiting.Reason) {
						conditions = append(conditions, conditionsForTransitioningContainer(pod, s, previous, false, "ContainerWait", t.Reason, " "+t.Message, time.Time{}, lastContainerTimeFromStatus(s, previous))...)
					}
					if s.State.Running != nil && previous.State.Running == nil {
						conditions = append(conditions, conditionsForTransitioningContainer(pod, s, previous, false, "ContainerStart", "", "", s.State.Running.StartedAt.Time, lastContainerTimeFromStatus(s, previous))...)
					}
				}
				if t := s.State.Terminated; t != nil && previous.State.Terminated == nil {
					if t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("reason/ContainerExit code/%d cause/%s %s", t.ExitCode, t.Reason, t.Message),
						})
					} else {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("reason/ContainerExit code/0 cause/%s %s", t.Reason, t.Message),
						})
					}
				}
				if s.RestartCount != previous.RestartCount && s.RestartCount != 0 {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/Restarted",
					})
				}
				if s.State.Terminated == nil && previous.Ready && !s.Ready {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/NotReady",
					})
				}
				if !previous.Ready && s.Ready {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Info,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/Ready",
					})
				}
			}

			for i := range pod.Status.InitContainerStatuses {
				s := &pod.Status.InitContainerStatuses[i]
				previous := findContainerStatus(oldPod.Status.InitContainerStatuses, s.Name, i)
				if previous == nil {
					continue
				}
				if previous.LastTerminationState.Terminated != nil && s.LastTerminationState.Terminated == nil {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Error,
						Locator: locatePodContainer(pod, s.Name),
						Message: fmt.Sprintf("reason/TerminationStateCleared lastState.terminated was cleared on a pod (bug https://bugzilla.redhat.com/show_bug.cgi?id=1933760 or similar)"),
					})
				}
				if pod.DeletionTimestamp == nil {
					if t := s.State.Waiting; t != nil && (previous.State.Waiting == nil || t.Reason != previous.State.Waiting.Reason) {
						conditions = append(conditions, conditionsForTransitioningContainer(pod, s, previous, true, "ContainerWait", t.Reason, " "+t.Message, time.Time{}, lastContainerTimeFromStatus(s, previous))...)
					}
					if s.State.Running != nil && previous.State.Running == nil {
						conditions = append(conditions, conditionsForTransitioningContainer(pod, s, previous, true, "ContainerStart", "", "", s.State.Running.StartedAt.Time, lastContainerTimeFromStatus(s, previous))...)
					}
				}
				if t := s.State.Terminated; t != nil && previous.State.Terminated == nil {
					if t.ExitCode != 0 {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("reason/ContainerExit code/%d cause/%s %s", t.ExitCode, t.Reason, t.Message),
						})
					} else {
						conditions = append(conditions, monitorapi.Condition{
							Level:   monitorapi.Info,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("reason/ContainerExit code/0 cause/%s %s", t.Reason, t.Message),
						})
					}
				}
				if s.RestartCount != previous.RestartCount {
					conditions = append(conditions, monitorapi.Condition{
						Level:   monitorapi.Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/Restarted",
					})
				}
			}
			return conditions
		},
	}
	podDeleteFns := []func(pod *corev1.Pod) []monitorapi.Condition{
		// check for transitions to being deleted
		func(pod *corev1.Pod) []monitorapi.Condition {
			var conditions []monitorapi.Condition
			switch {
			case len(pod.Spec.NodeName) == 0:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locatePod(pod),
					Message: "reason/DeletedBeforeScheduling",
				})
			case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locatePod(pod),
					Message: "reason/DeletedAfterCompletion",
				})
			default:
				conditions = append(conditions, monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locatePod(pod),
					Message: "reason/Deleted",
				})
			}
			return conditions
		},
	}

	startTime := time.Now().UTC().Add(-time.Minute)
	podInformer.AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				// filter out old pods so our monitor doesn't send a big chunk
				// of pod creations
				if pod.CreationTimestamp.Time.Before(startTime) {
					return
				}
				m.Record(monitorapi.Condition{
					Level:   monitorapi.Info,
					Locator: locatePod(pod),
					Message: "reason/Created",
				})
			},
			DeleteFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				for _, fn := range podDeleteFns {
					m.Record(fn(pod)...)
				}
			},
			UpdateFunc: func(old, obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				oldPod, ok := old.(*corev1.Pod)
				if !ok {
					return
				}
				if pod.UID != oldPod.UID {
					return
				}
				for _, fn := range podChangeFns {
					m.Record(fn(pod, oldPod)...)
				}
			},
		},
	)

	go podInformer.Run(ctx.Done())
}

func containerHasPreviousState(c *corev1.ContainerStatus) bool {
	return c.LastTerminationState != corev1.ContainerState{}
}

func podContainerPhaseStartTime(pod *corev1.Pod, init bool) time.Time {
	var t time.Time
	if !init {
		if c := findPodCondition(pod.Status.Conditions, corev1.PodInitialized); c != nil && c.Status == corev1.ConditionTrue {
			t = c.LastTransitionTime.Time
		}
	}
	if t.IsZero() {
		if c := findPodCondition(pod.Status.Conditions, corev1.PodScheduled); c != nil && c.Status == corev1.ConditionTrue {
			t = c.LastTransitionTime.Time
		}
	}
	if t.IsZero() {
		t = pod.CreationTimestamp.Time
	}
	return t
}

func findPodCondition(conditions []corev1.PodCondition, name corev1.PodConditionType) *corev1.PodCondition {
	for i, c := range conditions {
		if name == c.Type {
			return &conditions[i]
		}
	}
	return nil
}

func lastContainerTimeFromStatus(current, previous *corev1.ContainerStatus) time.Time {
	if state := current.LastTerminationState.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	if state := current.State.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	return time.Time{}
}

func conditionsForTransitioningContainer(pod *corev1.Pod, current, previous *corev1.ContainerStatus, init bool, reason, cause, message string, currentTime time.Time, lastContainerTime time.Time) []monitorapi.Condition {
	var conditions []monitorapi.Condition
	switch cause {
	case "PodInitializing" /*, "ContainerCreating"*/ :
	default:
		// on first container start, use either pod initialized or pod scheduled time
		if lastContainerTime.IsZero() && current.LastTerminationState.Terminated == nil {
			lastContainerTime = podContainerPhaseStartTime(pod, init)
		}
		if len(cause) > 0 {
			cause = fmt.Sprintf(" cause/%s", cause)
		}
		if currentTime.IsZero() {
			currentTime = time.Now().UTC()
		}
		switch seconds := currentTime.Sub(lastContainerTime).Seconds(); {
		case lastContainerTime.IsZero():
			data, _ := json.Marshal(pod)
			conditions = append(conditions, monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: locatePodContainer(pod, current.Name),
				Message: fmt.Sprintf("reason/%s%s unable to calculate container transition time in pod: %s", reason, cause, string(data)),
			})
		case seconds > 60 && previous != nil && previous.LastTerminationState.Terminated != nil && current.LastTerminationState.Terminated == nil:
			currentData, _ := json.Marshal(current)
			previousData, _ := json.Marshal(previous)
			conditions = append(conditions, monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: locatePodContainer(pod, current.Name),
				Message: fmt.Sprintf("reason/%s%s duration/%.2fs very long transition, possible container status clear from Kubelet: %s -> %s", reason, cause, seconds, string(previousData), string(currentData)),
			})
		default:
			conditions = append(conditions, monitorapi.Condition{
				Level:   monitorapi.Info,
				Locator: locatePodContainer(pod, current.Name),
				Message: fmt.Sprintf("reason/%s%s duration/%.2fs%s", reason, cause, seconds, message),
			})
		}
	}
	return conditions
}

func isMirrorPod(pod *corev1.Pod) bool {
	return len(pod.Annotations["kubernetes.io/config.mirror"]) > 0
}
