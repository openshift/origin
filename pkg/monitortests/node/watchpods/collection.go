package watchpods

import (
	"context"
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/monitortestlibrary"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

func startPodMonitoring(ctx context.Context, recorderWriter monitorapi.RecorderWriter, client kubernetes.Interface) {
	podPendingFn := func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
		isCreate := oldPod == nil
		oldPodIsPending := oldPod != nil && oldPod.Status.Phase == "Pending"
		newPodIsPending := pod != nil && pod.Status.Phase == "Pending"

		switch {
		case !oldPodIsPending && newPodIsPending:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodPendingReason)).
					BuildNow(),
			}

		case !oldPodIsPending && !newPodIsPending:
			if isCreate { // if we're creating, then our first state is not-pending.
				return []monitorapi.Interval{
					monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().Reason(monitorapi.PodNotPendingReason)).
						BuildNow(),
				}
			}
			return nil

		case oldPodIsPending && newPodIsPending:
			return nil

		case oldPodIsPending && !newPodIsPending:
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodNotPendingReason)).
					BuildNow(),
			}
		}
		return nil
	}

	podScheduledFn := func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
		oldPodHasNode := oldPod != nil && len(oldPod.Spec.NodeName) > 0
		newPodHasNode := pod != nil && len(pod.Spec.NodeName) > 0
		if !oldPodHasNode && newPodHasNode {
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonScheduled).Node(pod.Spec.NodeName)).
					BuildNow(),
			}
		}
		return nil
	}

	containerStatusesReadinessFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Interval {
		isCreate := oldContainerStatuses == nil

		intervals := []monitorapi.Interval{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			newContainerReady := containerStatus.Ready
			oldContainerReady := false
			if oldContainerStatuses != nil {
				if oldContainerStatus := findContainerStatus(oldContainerStatuses, containerName, i); oldContainerStatus != nil {
					oldContainerReady = oldContainerStatus.Ready
				}
			}

			// always produce conditions during create
			if (isCreate && !newContainerReady) || (oldContainerReady && !newContainerReady) {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
					Message(monitorapi.NewMessage().Reason(monitorapi.ContainerReasonNotReady)).
					BuildNow())
			}
			if (isCreate && newContainerReady) || (!oldContainerReady && newContainerReady) {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
					Message(
						monitorapi.NewMessage().Reason(monitorapi.ContainerReasonReady),
					).BuildNow())
			}
		}

		return intervals
	}

	containerReadinessFn := func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
		isCreate := oldPod == nil

		intervals := []monitorapi.Interval{}
		if isCreate {
			intervals = append(intervals, containerStatusesReadinessFn(pod, pod.Status.ContainerStatuses, nil)...)
			intervals = append(intervals, containerStatusesReadinessFn(pod, pod.Status.InitContainerStatuses, nil)...)
		} else {
			intervals = append(intervals, containerStatusesReadinessFn(pod, pod.Status.ContainerStatuses, oldPod.Status.ContainerStatuses)...)
			intervals = append(intervals, containerStatusesReadinessFn(pod, pod.Status.InitContainerStatuses, oldPod.Status.InitContainerStatuses)...)
		}

		return intervals
	}

	containerStatusContainerWaitFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Interval {
		intervals := []monitorapi.Interval{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container is not waiting, then we don't need to compute an event for it.
			if containerStatus.State.Waiting == nil {
				continue
			}

			switch {
			// always produce messages if we have no previous container status
			// this happens on create and when a new container status appears as the kubelet starts working on it
			case oldContainerStatus == nil:
				intervals = append(intervals,
					intervalsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerWait, containerStatus.State.Waiting.Reason, " "+containerStatus.State.Waiting.Message)...)

			case oldContainerStatus.State.Waiting == nil || // if the container wasn't previously waiting OR
				containerStatus.State.Waiting.Reason != oldContainerStatus.State.Waiting.Reason: // the container was previously waiting for a different reason
				intervals = append(intervals,
					intervalsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerWait, containerStatus.State.Waiting.Reason, " "+containerStatus.State.Waiting.Message)...)
			}
		}

		return intervals
	}

	containerStatusContainerStartFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Interval {
		conditions := []monitorapi.Interval{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container is not running, then we don't need to compute an event for it.
			if containerStatus.State.Running == nil {
				continue
			}

			switch {
			// always produce messages if we have no previous container status
			// this happens on create and when a new container status appears as the kubelet starts working on it
			case oldContainerStatus == nil:
				conditions = append(conditions,
					intervalsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerStart, "", "")...)

			case oldContainerStatus.State.Running == nil: // the container was previously not running
				conditions = append(conditions,
					intervalsForTransitioningContainer(pod, containerStatus,
						monitorapi.ContainerReasonContainerStart, "", "")...)
			}
		}

		return conditions
	}

	containerStatusContainerExitFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Interval {
		intervals := []monitorapi.Interval{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			if oldContainerStatus != nil && oldContainerStatus.LastTerminationState.Terminated != nil && containerStatus.LastTerminationState.Terminated == nil {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
					Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
					Message(
						monitorapi.NewMessage().Reason(monitorapi.TerminationStateCleared).
							HumanMessage("lastState.terminated was cleared on a pod (bug https://bugzilla.redhat.com/show_bug.cgi?id=1933760 or similar)"),
					).BuildNow())
			}

			// if this container is not terminated, then we don't need to compute an event for it.
			lastTerminated := containerStatus.LastTerminationState.Terminated != nil
			currentTerminated := containerStatus.State.Terminated != nil
			if !lastTerminated && !currentTerminated {
				continue
			}

			switch {
			case oldContainerStatus == nil:
				// if we have no container status, then this is probably an initial list and we missed the initial start.  Don't emit the
				// the container exit because it will be a disconnected event that can confuse our container lifecycle ordering based
				// on the event stream

			case lastTerminated && oldContainerStatus.LastTerminationState.Terminated == nil:
				// if we are transitioning to a terminated state
				if containerStatus.LastTerminationState.Terminated.ExitCode != 0 {
					intervals = append(intervals,
						monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ContainerReasonContainerExit).
								WithAnnotation(monitorapi.AnnotationContainerExitCode, fmt.Sprintf("%d", containerStatus.LastTerminationState.Terminated.ExitCode)).
								Cause(containerStatus.LastTerminationState.Terminated.Reason).
								HumanMessage(containerStatus.LastTerminationState.Terminated.Message),
							).BuildNow(),
					)
				} else {
					intervals = append(intervals,
						monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ContainerReasonContainerExit).
								WithAnnotation(monitorapi.AnnotationContainerExitCode, "0").
								Cause(containerStatus.LastTerminationState.Terminated.Reason).
								HumanMessage(containerStatus.LastTerminationState.Terminated.Message)).
							BuildNow(),
					)
				}

			case currentTerminated && oldContainerStatus.State.Terminated == nil:
				// if we are transitioning to a terminated state
				if containerStatus.State.Terminated.ExitCode != 0 {
					intervals = append(intervals,
						monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ContainerReasonContainerExit).
								WithAnnotation(monitorapi.AnnotationContainerExitCode, fmt.Sprintf("%d", containerStatus.State.Terminated.ExitCode)).
								Cause(containerStatus.State.Terminated.Reason).
								HumanMessage(containerStatus.State.Terminated.Message),
							).
							BuildNow(),
					)
				} else {
					intervals = append(intervals,
						monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
							Message(monitorapi.NewMessage().
								Reason(monitorapi.ContainerReasonContainerExit).
								WithAnnotation(monitorapi.AnnotationContainerExitCode, "0").
								Cause(containerStatus.State.Terminated.Reason).
								HumanMessage(containerStatus.State.Terminated.Message),
							).
							BuildNow(),
					)
				}
			}

		}

		return intervals
	}

	containerStatusContainerRestartedFn := func(pod *corev1.Pod, containerStatuses, oldContainerStatuses []corev1.ContainerStatus) []monitorapi.Interval {
		intervals := []monitorapi.Interval{}
		for i := range containerStatuses {
			containerStatus := &containerStatuses[i]
			containerName := containerStatus.Name
			var oldContainerStatus *corev1.ContainerStatus
			if oldContainerStatuses != nil {
				oldContainerStatus = findContainerStatus(oldContainerStatuses, containerName, i)
			}

			// if this container has not been restarted, do not produce an event for it
			if containerStatus.RestartCount == 0 {
				continue
			}

			// if we have nothing to come the restart count against, do not produce an event for it.
			if oldContainerStatus == nil {
				continue
			}

			if containerStatus.RestartCount != oldContainerStatus.RestartCount {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().ContainerFromPod(pod, containerName)).
					Message(
						monitorapi.NewMessage().Reason(monitorapi.ContainerReasonRestarted),
					).BuildNow())
			}
		}

		return intervals
	}

	containerLifecycleStateFn := func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
		var oldContainerStatus []corev1.ContainerStatus
		var oldInitContainerStatus []corev1.ContainerStatus
		if oldPod != nil {
			oldContainerStatus = oldPod.Status.ContainerStatuses
			oldInitContainerStatus = oldPod.Status.InitContainerStatuses
		}

		conditions := []monitorapi.Interval{}
		conditions = append(conditions, containerStatusContainerWaitFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerStartFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerExitFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)
		conditions = append(conditions, containerStatusContainerRestartedFn(pod, pod.Status.ContainerStatuses, oldContainerStatus)...)

		conditions = append(conditions, containerStatusContainerWaitFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerStartFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerExitFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)
		conditions = append(conditions, containerStatusContainerRestartedFn(pod, pod.Status.InitContainerStatuses, oldInitContainerStatus)...)

		return conditions
	}

	podCreatedFns := []func(pod *corev1.Pod) []monitorapi.Interval{
		func(pod *corev1.Pod) []monitorapi.Interval {
			return []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonCreated)).
					BuildNow(),
			}
		},
		func(pod *corev1.Pod) []monitorapi.Interval {
			return podScheduledFn(pod, nil)
		},
		func(pod *corev1.Pod) []monitorapi.Interval {
			return containerLifecycleStateFn(pod, nil)
		},
		func(pod *corev1.Pod) []monitorapi.Interval {
			return containerReadinessFn(pod, nil)
		},
		func(pod *corev1.Pod) []monitorapi.Interval {
			return podPendingFn(pod, nil)
		},
	}

	podChangeFns := []func(pod, oldPod *corev1.Pod) []monitorapi.Interval{
		podPendingFn,
		// check if the pod was scheduled
		podScheduledFn,
		// check if container lifecycle state changed
		containerLifecycleStateFn,
		// check if readiness for containers changed
		containerReadinessFn,
		// check phase transitions
		func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
			new, old := pod.Status.Phase, oldPod.Status.Phase
			if new == old || len(old) == 0 {
				return nil
			}
			var intervals []monitorapi.Interval
			switch {
			case new == corev1.PodPending && old != corev1.PodUnknown:
				switch {
				case pod.DeletionTimestamp != nil:
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().HumanMessage("invariant violation (bug): pod should not transition %s->%s even when terminated")).
						BuildNow())
				case isMirrorPod(pod):
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().HumanMessage("invariant violation (bug): static pod should not transition %s->%s with same UID")).
						BuildNow())
				default:
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().HumanMessage("pod moved back to Pending")).
						BuildNow())
				}
			case new == corev1.PodUnknown:
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().HumanMessage("pod moved to the Unknown phase")).
					BuildNow())
			case new == corev1.PodFailed && old != corev1.PodFailed:
				switch pod.Status.Reason {
				case "Evicted":
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Warning).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonEvicted).HumanMessage(pod.Status.Message)).
						BuildNow())
				case "Preempting":
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonPreempted).HumanMessage(pod.Status.Message)).
						BuildNow())
				default:
					intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
						Locator(monitorapi.NewLocator().PodFromPod(pod)).
						Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonFailed).
							HumanMessagef("(%s): %s", pod.Status.Reason, pod.Status.Message)).
						BuildNow())
				}
				for _, s := range pod.Status.InitContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, s.Name)).
							Message(monitorapi.NewMessage().HumanMessagef("init container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message)).
							BuildNow())
					}
				}
				for _, s := range pod.Status.ContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
							Locator(monitorapi.NewLocator().ContainerFromPod(pod, s.Name)).
							Message(monitorapi.NewMessage().HumanMessagef("container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message)).
							BuildNow())
					}
				}
			}
			return intervals
		},
		// check for transitions to being deleted
		func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if pod.DeletionGracePeriodSeconds != nil && oldPod.DeletionGracePeriodSeconds == nil {
				switch {
				case len(pod.Spec.NodeName) == 0:
					// pods that have not been assigned to a node are deleted immediately
				case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
					// terminal pods are immediately deleted (do not undergo graceful deletion)
				default:
					if *pod.DeletionGracePeriodSeconds == 0 {
						intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
							Locator(monitorapi.NewLocator().PodFromPod(pod)).
							Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonForceDelete).
								WithAnnotation(monitorapi.AnnotationIsStaticPod, fmt.Sprintf("%t", isMirrorPod(pod)))).
							BuildNow())
					} else {
						intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
							Locator(monitorapi.NewLocator().PodFromPod(pod)).
							Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonGracefulDeleteStarted).
								WithAnnotation(monitorapi.AnnotationDuration, fmt.Sprintf("%ds", *pod.DeletionGracePeriodSeconds))).
							BuildNow())
					}
				}
			}
			if pod.DeletionGracePeriodSeconds == nil && oldPod.DeletionGracePeriodSeconds != nil {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().HumanMessage("invariant violation: pod was marked for deletion and then deletion grace period was cleared")).
					BuildNow())
			}
			return intervals
		},
		// check restarts, readiness drop outs, or other status changes
		func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
			var intervals []monitorapi.Interval

			// container status should never be removed since the kubelet should be
			// synthesizing status from the apiserver in order to determine what to
			// run after a reboot (this is likely to be the result of a pod going back
			// to pending status)
			if len(pod.Status.ContainerStatuses) < len(oldPod.Status.ContainerStatuses) {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().HumanMessage("invariant violation: container statuses were removed")).
					BuildNow())
			}
			if len(pod.Status.InitContainerStatuses) < len(oldPod.Status.InitContainerStatuses) {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().HumanMessage("invariant violation: init container statuses were removed")).
					BuildNow())
			}

			return intervals
		},
		// inform when a pod gets reassigned to a new node
		func(pod, oldPod *corev1.Pod) []monitorapi.Interval {
			var intervals []monitorapi.Interval
			if len(oldPod.Spec.NodeName) > 0 && pod.Spec.NodeName != oldPod.Spec.NodeName {
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Error).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().HumanMessagef(
						"invariant violation, pod once assigned to a node must stay on it. The pod previously scheduled to %s, has just been assigned to a new node %s", oldPod.Spec.NodeName, pod.Spec.NodeName)).
					BuildNow())
			}
			return intervals
		},
	}
	podDeleteFns := []func(pod *corev1.Pod) []monitorapi.Interval{
		// check for transitions to being deleted
		func(pod *corev1.Pod) []monitorapi.Interval {
			intervals := []monitorapi.Interval{
				monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonDeleted)).
					BuildNow(),
			}
			switch {
			case len(pod.Spec.NodeName) == 0:
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonDeletedBeforeScheduling)).
					BuildNow())
			case pod.Status.Phase == corev1.PodFailed, pod.Status.Phase == corev1.PodSucceeded:
				intervals = append(intervals, monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
					Locator(monitorapi.NewLocator().PodFromPod(pod)).
					Message(monitorapi.NewMessage().Reason(monitorapi.PodReasonDeletedAfterCompletion)).
					BuildNow())
			default:
			}
			return intervals
		},
	}

	listWatch := cache.NewListWatchFromClient(client.CoreV1().RESTClient(), "pods", "", fields.Everything())
	customStore := monitortestlibrary.NewMonitoringStore(
		"pods",
		toCreateFns(podCreatedFns),
		toUpdateFns(podChangeFns),
		toDeleteFns(podDeleteFns),
		recorderWriter,
		recorderWriter,
	)
	reflector := cache.NewReflector(listWatch, &corev1.Pod{}, customStore, 0)
	go reflector.Run(ctx.Done())

	// start controller to watch for shared pod IPs.
	sharedInformers := informers.NewSharedInformerFactory(client, 24*time.Hour)
	podIPController := NewSimultaneousPodIPController(recorderWriter, sharedInformers.Core().V1().Pods())
	go podIPController.Run(ctx)
	go sharedInformers.Start(ctx.Done())

}

func toCreateFns(podCreateFns []func(pod *corev1.Pod) []monitorapi.Interval) []monitortestlibrary.ObjCreateFunc {
	ret := []monitortestlibrary.ObjCreateFunc{}

	for i := range podCreateFns {
		fn := podCreateFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(*corev1.Pod))
		})
	}

	return ret
}

func toDeleteFns(podDeleteFns []func(pod *corev1.Pod) []monitorapi.Interval) []monitortestlibrary.ObjDeleteFunc {
	ret := []monitortestlibrary.ObjDeleteFunc{}

	for i := range podDeleteFns {
		fn := podDeleteFns[i]
		ret = append(ret, func(obj interface{}) []monitorapi.Interval {
			return fn(obj.(*corev1.Pod))
		})
	}

	return ret
}

func toUpdateFns(podUpdateFns []func(pod, oldPod *corev1.Pod) []monitorapi.Interval) []monitortestlibrary.ObjUpdateFunc {
	ret := []monitortestlibrary.ObjUpdateFunc{}

	for i := range podUpdateFns {
		fn := podUpdateFns[i]
		ret = append(ret, func(obj, oldObj interface{}) []monitorapi.Interval {
			if oldObj == nil {
				return fn(obj.(*corev1.Pod), nil)
			}
			return fn(obj.(*corev1.Pod), oldObj.(*corev1.Pod))
		})
	}

	return ret
}

func lastContainerTimeFromStatus(current *corev1.ContainerStatus) time.Time {
	if state := current.LastTerminationState.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	if state := current.State.Terminated; state != nil && !state.FinishedAt.Time.IsZero() {
		return state.FinishedAt.Time
	}
	return time.Time{}
}

func intervalsForTransitioningContainer(pod *corev1.Pod, current *corev1.ContainerStatus, reason monitorapi.IntervalReason, cause, message string) []monitorapi.Interval {
	return []monitorapi.Interval{
		monitorapi.NewInterval(monitorapi.SourcePodMonitor, monitorapi.Info).
			Locator(monitorapi.NewLocator().ContainerFromPod(pod, current.Name)).
			Message(monitorapi.NewMessage().Reason(reason).Cause(cause).HumanMessage(message)).
			BuildNow(),
	}
}

func isMirrorPod(pod *corev1.Pod) bool {
	return len(pod.Annotations["kubernetes.io/config.mirror"]) > 0
}

func findContainerStatus(status []corev1.ContainerStatus, name string, position int) *corev1.ContainerStatus {
	if position < len(status) {
		if status[position].Name == name {
			return &status[position]
		}
	}
	for i := range status {
		if status[i].Name == name {
			return &status[i]
		}
	}
	return nil
}
