package monitor

import (
	"context"
	"fmt"
	"time"

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
				items, err := client.CoreV1().Pods("").List(options)
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
				w, err := client.CoreV1().Pods("").Watch(options)
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

	m.AddSampler(func(now time.Time) []*Condition {
		var conditions []*Condition
		for _, obj := range podInformer.GetStore().List() {
			pod, ok := obj.(*corev1.Pod)
			if !ok {
				continue
			}
			if pod.Status.Phase == "Pending" {
				if now.Sub(pod.CreationTimestamp.Time) > time.Minute {
					conditions = append(conditions, &Condition{
						Level:   Warning,
						Locator: locatePod(pod),
						Message: "pod has been pending longer than a minute",
					})
				}
			}
		}
		return conditions
	})

	podChangeFns := []func(pod, oldPod *corev1.Pod) []Condition{
		// check phase transitions
		func(pod, oldPod *corev1.Pod) []Condition {
			new, old := pod.Status.Phase, oldPod.Status.Phase
			if new == old || len(old) == 0 {
				return nil
			}
			var conditions []Condition
			switch {
			case new == corev1.PodPending && old != corev1.PodUnknown:
				switch {
				case pod.DeletionTimestamp != nil:
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): pod should not transition %s->%s even when terminated", old, new),
					})
				case len(pod.Annotations["kubernetes.io/config.mirror"]) > 0:
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("invariant violation (bug): static pod should not transition %s->%s with same UID", old, new),
					})
				default:
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("pod moved back to Pending"),
					})
				}
			case new == corev1.PodUnknown:
				conditions = append(conditions, Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("pod moved to the Unknown phase"),
				})
			case new == corev1.PodFailed && old != corev1.PodFailed:
				switch pod.Status.Reason {
				case "Evicted":
					conditions = append(conditions, Condition{
						Level:   Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Evicted %s", pod.Status.Message),
					})
				case "Preempting":
					conditions = append(conditions, Condition{
						Level:   Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Preempted %s", pod.Status.Message),
					})
				default:
					conditions = append(conditions, Condition{
						Level:   Error,
						Locator: locatePod(pod),
						Message: fmt.Sprintf("reason/Failed (%s): %s", pod.Status.Reason, pod.Status.Message),
					})
				}
				for _, s := range pod.Status.InitContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, Condition{
							Level:   Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
				for _, s := range pod.Status.ContainerStatuses {
					if t := s.State.Terminated; t != nil && t.ExitCode != 0 {
						conditions = append(conditions, Condition{
							Level:   Error,
							Locator: locatePodContainer(pod, s.Name),
							Message: fmt.Sprintf("init container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
						})
					}
				}
			}
			return conditions
		},
		// check for transitions to being deleted
		func(pod, oldPod *corev1.Pod) []Condition {
			var conditions []Condition
			if pod.DeletionGracePeriodSeconds != nil && oldPod.DeletionGracePeriodSeconds == nil {
				conditions = append(conditions, Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: fmt.Sprintf("reason/GracefulDelete in %ds", *pod.DeletionGracePeriodSeconds),
				})
			}
			if pod.DeletionGracePeriodSeconds == nil && oldPod.DeletionGracePeriodSeconds != nil {
				conditions = append(conditions, Condition{
					Level:   Error,
					Locator: locatePod(pod),
					Message: "invariant violation: pod was marked for deletion and then deletion grace period was cleared",
				})
			}
			return conditions
		},
		// check restarts, readiness drop outs, or other status changes
		func(pod, oldPod *corev1.Pod) []Condition {
			var conditions []Condition
			for i := range pod.Status.ContainerStatuses {
				s := &pod.Status.ContainerStatuses[i]
				previous := findContainerStatus(oldPod.Status.ContainerStatuses, s.Name, i)
				if previous == nil {
					continue
				}
				if t := s.State.Terminated; t != nil && previous.State.Terminated == nil && t.ExitCode != 0 {
					conditions = append(conditions, Condition{
						Level:   Error,
						Locator: locatePodContainer(pod, s.Name),
						Message: fmt.Sprintf("container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
					})
				}
				if s.RestartCount != previous.RestartCount && s.RestartCount != 0 {
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/Restarted",
					})
				}
				if s.State.Terminated == nil && previous.Ready && !s.Ready {
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "reason/NotReady",
					})
				}
				if !previous.Ready && s.Ready {
					conditions = append(conditions, Condition{
						Level:   Info,
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
				if t := s.State.Terminated; t != nil && previous.State.Terminated == nil && t.ExitCode != 0 {
					conditions = append(conditions, Condition{
						Level:   Error,
						Locator: locatePodContainer(pod, s.Name),
						Message: fmt.Sprintf("init container exited with code %d (%s): %s", t.ExitCode, t.Reason, t.Message),
					})
				}
				if s.RestartCount != previous.RestartCount {
					conditions = append(conditions, Condition{
						Level:   Warning,
						Locator: locatePodContainer(pod, s.Name),
						Message: "init container restarted",
					})
				}
			}
			return conditions
		},
	}

	startTime := time.Now().Add(-time.Minute)
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
				m.Record(Condition{
					Level:   Info,
					Locator: locatePod(pod),
					Message: "reason/Created",
				})
			},
			DeleteFunc: func(obj interface{}) {
				pod, ok := obj.(*corev1.Pod)
				if !ok {
					return
				}
				m.Record(Condition{
					Level:   Warning,
					Locator: locatePod(pod),
					Message: "reason/Deleted",
				})
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
