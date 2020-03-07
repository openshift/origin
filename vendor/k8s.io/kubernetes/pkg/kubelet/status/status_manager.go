/*
Copyright 2014 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package status

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"

	clientset "k8s.io/client-go/kubernetes"

	v1 "k8s.io/api/core/v1"
	apiequality "k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog"
	podutil "k8s.io/kubernetes/pkg/api/v1/pod"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/metrics"
	kubepod "k8s.io/kubernetes/pkg/kubelet/pod"
	kubetypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	statusutil "k8s.io/kubernetes/pkg/util/pod"
)

// A wrapper around v1.PodStatus that includes a version to enforce that stale pod statuses are
// not sent to the API server.
type versionedPodStatus struct {
	// Monotonically increasing version number (per pod).
	version uint64
	// Priority assigned to the pod status change, higher is more important. Set to zero after
	// the pod is synced.
	priority int
	// Name of pod.
	podName string
	// Namespace of pod.
	podNamespace string
	// The the time at which the most recent change was detected. Set to zero after writing to
	// the API server.
	at time.Time

	status v1.PodStatus
}

// Compare returns true if the two pod statuses are comparable, -1 if
// a is newer, 1 if b is newer, and 0 if the two are at the same revision.
func podVersionCompare(a, b *v1.Pod) (int, bool) {
	// Two pods with different UIDs, names, or namespaces are not comparable
	if a.UID != b.UID || a.Name != b.Name || a.Namespace != b.Namespace {
		return 0, false
	}
	// If the server implementation supports monotonic resource versions, compare
	// using those values, otherwise return false.
	aRV, err := strconv.ParseUint(a.ResourceVersion, 10, 64)
	if err != nil {
		return 0, false
	}
	bRV, err := strconv.ParseUint(b.ResourceVersion, 10, 64)
	if err != nil {
		return 0, false
	}
	if aRV > bRV {
		return -1, true
	}
	if aRV < bRV {
		return 1, true
	}
	return 0, true
}

type podStatusSyncRequest struct {
	// uid is the Kubelet UID for the pod (excludes mirror-pods)
	uid    types.UID
	status versionedPodStatus
}

// highestPrioritySyncRequests sorts a slice of sync requests with highest priority first.
// If priority is identical, oldest status date is used (zero date is sorted after set dates).
type highestPrioritySyncRequests []podStatusSyncRequest

func (r highestPrioritySyncRequests) Len() int      { return len(r) }
func (r highestPrioritySyncRequests) Swap(i, j int) { r[i], r[j] = r[j], r[i] }
func (r highestPrioritySyncRequests) Less(i, j int) bool {
	a, b := r[i], r[j]
	if a.status.priority > b.status.priority {
		return true
	}
	if a.status.priority < b.status.priority {
		return false
	}
	aSet, bSet := !a.status.at.IsZero(), !b.status.at.IsZero()
	if aSet && !bSet {
		return true
	}
	if !aSet && bSet {
		return false
	}
	return a.status.at.Sub(b.status.at) <= 0
}

// Updates pod statuses in apiserver. Writes only when new status has changed.
// All methods are thread-safe.
type manager struct {
	kubeClient clientset.Interface
	podManager kubepod.Manager

	podStatusChannel chan struct{}

	podStatusesLock sync.RWMutex
	// Map from pod UID to sync status of the corresponding pod.
	podStatuses       map[types.UID]versionedPodStatus
	recentPodWrites   map[types.UID]*v1.Pod
	podStatusQueue    map[types.UID]struct{}
	hasReportedStatus bool

	// Map from (mirror) pod UID to latest status version successfully sent to the API server.
	// apiStatusVersions must only be accessed from the sync thread.
	apiStatusVersions map[kubetypes.MirrorPodUID]uint64

	podDeletionSafety PodDeletionSafetyProvider
}

// PodStatusProvider knows how to provide status for a pod. It's intended to be used by other components
// that need to introspect status.
type PodStatusProvider interface {
	// GetPodStatus returns the cached status for the provided pod UID, as well as whether it
	// was a cache hit.
	GetPodStatus(uid types.UID) (v1.PodStatus, bool)
}

// PodDeletionSafetyProvider provides guarantees that a pod can be safely deleted.
type PodDeletionSafetyProvider interface {
	// A function which returns true if the pod can safely be deleted
	PodResourcesAreReclaimed(pod *v1.Pod, status v1.PodStatus) bool
}

// Manager is the Source of truth for kubelet pod status, and should be kept up-to-date with
// the latest v1.PodStatus. It also syncs updates back to the API server.
type Manager interface {
	PodStatusProvider

	// Start the API server status sync loop.
	Start()

	// SetPodStatus caches updates the cached status for the given pod, and triggers a status update.
	SetPodStatus(pod *v1.Pod, status v1.PodStatus)

	// SetContainerReadiness updates the cached container status with the given readiness, and
	// triggers a status update.
	SetContainerReadiness(podUID types.UID, containerID kubecontainer.ContainerID, ready bool)

	// SetContainerStartup updates the cached container status with the given startup, and
	// triggers a status update.
	SetContainerStartup(podUID types.UID, containerID kubecontainer.ContainerID, started bool)

	// TerminatePod resets the container status for the provided pod to terminated and triggers
	// a status update.
	TerminatePod(pod *v1.Pod)

	// RemoveOrphanedStatuses scans the status cache and removes any entries for pods not included in
	// the provided podUIDs.
	RemoveOrphanedStatuses(podUIDs map[types.UID]bool)
}

const syncPeriod = 10 * time.Second

// NewManager returns a functional Manager.
func NewManager(kubeClient clientset.Interface, podManager kubepod.Manager, podDeletionSafety PodDeletionSafetyProvider) Manager {
	return &manager{
		kubeClient:        kubeClient,
		podManager:        podManager,
		podStatuses:       make(map[types.UID]versionedPodStatus),
		recentPodWrites:   make(map[types.UID]*v1.Pod),
		podStatusQueue:    make(map[types.UID]struct{}),
		podStatusChannel:  make(chan struct{}, 1),
		apiStatusVersions: make(map[kubetypes.MirrorPodUID]uint64),
		podDeletionSafety: podDeletionSafety,
	}
}

// isPodStatusByKubeletEqual returns true if the given pod statuses are equal when non-kubelet-owned
// pod conditions are excluded.
// This method normalizes the status before comparing so as to make sure that meaningless
// changes will be ignored.
func isPodStatusByKubeletEqual(oldStatus, status *v1.PodStatus) bool {
	oldCopy := oldStatus.DeepCopy()
	for _, c := range status.Conditions {
		if kubetypes.PodConditionByKubelet(c.Type) {
			_, oc := podutil.GetPodCondition(oldCopy, c.Type)
			if oc == nil || oc.Status != c.Status || oc.Message != c.Message || oc.Reason != c.Reason {
				return false
			}
		}
	}
	oldCopy.Conditions = status.Conditions
	return apiequality.Semantic.DeepEqual(oldCopy, status)
}

func (m *manager) Start() {
	// Don't start the status manager if we don't have a client. This will happen
	// on the master, where the kubelet is responsible for bootstrapping the pods
	// of the master components.
	if m.kubeClient == nil {
		klog.Infof("Kubernetes client is nil, not starting status manager.")
		return
	}

	klog.Info("Starting to sync pod status with apiserver")
	//lint:ignore SA1015 Ticker can link since this is only called once and doesn't handle termination.
	syncTicker := time.Tick(syncPeriod)
	// syncPod and syncBatch share the same go routine to avoid sync races.
	go wait.Forever(func() {
		for {
			select {
			case <-m.podStatusChannel:
				m.syncBatch(false)
			case <-syncTicker:
				m.syncBatch(true)
			}
		}
	}, 0)
}

func (m *manager) GetPodStatus(uid types.UID) (v1.PodStatus, bool) {
	m.podStatusesLock.RLock()
	defer m.podStatusesLock.RUnlock()
	status, ok := m.podStatuses[types.UID(m.podManager.TranslatePodUID(uid))]
	return status.status, ok
}

func (m *manager) SetPodStatus(pod *v1.Pod, status v1.PodStatus) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()

	for _, c := range pod.Status.Conditions {
		if !kubetypes.PodConditionByKubelet(c.Type) {
			klog.Errorf("Kubelet is trying to update pod condition %q for pod %q. "+
				"But it is not owned by kubelet.", string(c.Type), format.Pod(pod))
		}
	}
	// Make sure we're caching a deep copy.
	status = *status.DeepCopy()

	for _, c := range pod.Status.ContainerStatuses {
		if c.Started == nil {
			if c, _, ok := findContainerStatus(&status, c.Name); ok && c.Started != nil {
				klog.V(3).Infof("DEBUG: SetPodStatus is setting %q container=%s to started=%t from pod nil", format.Pod(pod), c.Name, *c.Started)
			}
		}
	}

	// Force a status update if deletion timestamp is set. This is necessary
	// because if the pod is in the non-running state, the pod worker still
	// needs to be able to trigger an update and/or deletion.
	m.updateStatusInternal(pod, status, pod.DeletionTimestamp != nil)
}

func (m *manager) SetContainerReadiness(podUID types.UID, containerID kubecontainer.ContainerID, ready bool) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()

	pod, ok := m.podManager.GetPodByUID(podUID)
	if !ok {
		klog.V(4).Infof("Pod %q has been deleted, no need to update readiness", string(podUID))
		return
	}

	oldStatus, found := m.podStatuses[pod.UID]
	if !found {
		klog.Warningf("Container readiness changed before pod has synced: %q - %q",
			format.Pod(pod), containerID.String())
		return
	}

	// Find the container to update.
	containerStatus, _, ok := findContainerStatus(&oldStatus.status, containerID.String())
	if !ok {
		klog.Warningf("Container readiness changed for unknown container: %q - %q",
			format.Pod(pod), containerID.String())
		return
	}

	if containerStatus.Ready == ready {
		klog.V(4).Infof("Container readiness unchanged (%v): %q - %q", ready,
			format.Pod(pod), containerID.String())
		return
	}

	// Make sure we're not updating the cached version.
	status := *oldStatus.status.DeepCopy()
	containerStatus, _, _ = findContainerStatus(&status, containerID.String())
	containerStatus.Ready = ready

	// updateConditionFunc updates the corresponding type of condition
	updateConditionFunc := func(conditionType v1.PodConditionType, condition v1.PodCondition) {
		conditionIndex := -1
		for i, condition := range status.Conditions {
			if condition.Type == conditionType {
				conditionIndex = i
				break
			}
		}
		if conditionIndex != -1 {
			status.Conditions[conditionIndex] = condition
		} else {
			klog.Warningf("PodStatus missing %s type condition: %+v", conditionType, status)
			status.Conditions = append(status.Conditions, condition)
		}
	}
	updateConditionFunc(v1.PodReady, GeneratePodReadyCondition(&pod.Spec, status.Conditions, status.ContainerStatuses, status.Phase))
	updateConditionFunc(v1.ContainersReady, GenerateContainersReadyCondition(&pod.Spec, status.ContainerStatuses, status.Phase))

	for _, c := range pod.Status.ContainerStatuses {
		if c.Started == nil {
			if c, _, ok := findContainerStatus(&status, c.Name); ok && c.Started != nil {
				klog.V(3).Infof("DEBUG: SetContainerReadiness is setting %q container=%s to started=%t from pod nil", format.Pod(pod), c.Name, *c.Started)
			}
		}
	}

	m.updateStatusInternal(pod, status, false)
}

func (m *manager) SetContainerStartup(podUID types.UID, containerID kubecontainer.ContainerID, started bool) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()

	pod, ok := m.podManager.GetPodByUID(podUID)
	if !ok {
		klog.V(4).Infof("Pod %q has been deleted, no need to update startup", string(podUID))
		return
	}

	oldStatus, found := m.podStatuses[pod.UID]
	if !found {
		klog.Warningf("Container startup changed before pod has synced: %q - %q",
			format.Pod(pod), containerID.String())
		return
	}

	// Find the container to update.
	containerStatus, _, ok := findContainerStatus(&oldStatus.status, containerID.String())
	if !ok {
		klog.Warningf("Container startup changed for unknown container: %q - %q",
			format.Pod(pod), containerID.String())
		return
	}

	if containerStatus.Started != nil && *containerStatus.Started == started {
		klog.V(4).Infof("Container startup unchanged (%v): %q - %q", started,
			format.Pod(pod), containerID.String())
		return
	}

	if c, _, ok := findContainerStatus(&pod.Status, containerID.String()); ok && c.Started == nil {
		klog.V(3).Infof("DEBUG: SetContainerStartup is setting %q container=%s to started=%t from pod nil", format.Pod(pod), c.Name, started)
	}

	// Make sure we're not updating the cached version.
	status := *oldStatus.status.DeepCopy()
	containerStatus, _, _ = findContainerStatus(&status, containerID.String())
	containerStatus.Started = &started

	m.updateStatusInternal(pod, status, false)
}

func findContainerStatus(status *v1.PodStatus, containerID string) (containerStatus *v1.ContainerStatus, init bool, ok bool) {
	// Find the container to update.
	for i, c := range status.ContainerStatuses {
		if c.ContainerID == containerID {
			return &status.ContainerStatuses[i], false, true
		}
	}

	for i, c := range status.InitContainerStatuses {
		if c.ContainerID == containerID {
			return &status.InitContainerStatuses[i], true, true
		}
	}

	return nil, false, false

}

func (m *manager) TerminatePod(pod *v1.Pod) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()

	// ensure that all containers have a terminated state - because we do not know whether the container
	// was successful, always report an error
	oldStatus := &pod.Status
	if cachedStatus, ok := m.podStatuses[pod.UID]; ok {
		oldStatus = &cachedStatus.status
	}
	status := *oldStatus.DeepCopy()
	for i := range status.ContainerStatuses {
		if status.ContainerStatuses[i].State.Terminated != nil || status.ContainerStatuses[i].State.Waiting != nil {
			continue
		}
		status.ContainerStatuses[i].State = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				Reason:   "ContainerStatusUnknown",
				Message:  "The container could not be located when the pod was terminated",
				ExitCode: 137,
			},
		}
	}
	for i := range status.InitContainerStatuses {
		if status.InitContainerStatuses[i].State.Terminated != nil || status.InitContainerStatuses[i].State.Waiting != nil {
			continue
		}
		status.InitContainerStatuses[i].State = v1.ContainerState{
			Terminated: &v1.ContainerStateTerminated{
				Reason:   "ContainerStatusUnknown",
				Message:  "The container could not be located when the pod was terminated",
				ExitCode: 137,
			},
		}
	}

	m.updateStatusInternal(pod, status, true)
}

// checkContainerStateTransition ensures that no container is trying to transition
// from a terminated to non-terminated state, which is illegal and indicates a
// logical error in the kubelet.
func checkContainerStateTransition(oldStatuses, newStatuses []v1.ContainerStatus, restartPolicy v1.RestartPolicy) error {
	// If we should always restart, containers are allowed to leave the terminated state
	if restartPolicy == v1.RestartPolicyAlways {
		return nil
	}
	for _, oldStatus := range oldStatuses {
		// Skip any container that wasn't terminated
		if oldStatus.State.Terminated == nil {
			continue
		}
		// Skip any container that failed but is allowed to restart
		if oldStatus.State.Terminated.ExitCode != 0 && restartPolicy == v1.RestartPolicyOnFailure {
			continue
		}
		for _, newStatus := range newStatuses {
			if oldStatus.Name == newStatus.Name && newStatus.State.Terminated == nil {
				return fmt.Errorf("terminated container %v attempted illegal transition to non-terminated state", newStatus.Name)
			}
		}
	}
	return nil
}

// calculatePriority returns the relative priority of this pod status change. Higher priority changes should be
// processed more quickly.
func calculatePriority(oldStatus, newStatus *v1.PodStatus) int {
	oldTerminalPhase := oldStatus.Phase == v1.PodSucceeded || oldStatus.Phase == v1.PodFailed
	newTerminalPhase := newStatus.Phase == v1.PodSucceeded || newStatus.Phase == v1.PodFailed
	if newTerminalPhase && !oldTerminalPhase {
		return 100
	}

	_, oldReady := podutil.GetPodCondition(oldStatus, v1.PodReady)
	_, newReady := podutil.GetPodCondition(newStatus, v1.PodReady)
	isOldReady := oldReady != nil && oldReady.Status == v1.ConditionTrue
	isNewReady := newReady != nil && newReady.Status == v1.ConditionTrue
	if isOldReady != isNewReady {
		return 100
	}

	return 0
}

// updateStatusInternal updates the internal status cache, and queues an update to the api server if
// necessary. Returns whether an update was triggered.
// This method IS NOT THREAD SAFE and must be called from a locked function.
func (m *manager) updateStatusInternal(pod *v1.Pod, status v1.PodStatus, forceUpdate bool) bool {
	var oldStatus v1.PodStatus
	cachedStatus, isCached := m.podStatuses[pod.UID]
	if isCached {
		oldStatus = cachedStatus.status
	} else if mirrorPod, ok := m.podManager.GetMirrorPodByPod(pod); ok {
		oldStatus = mirrorPod.Status
	} else {
		oldStatus = pod.Status
	}

	// Check for illegal state transition in containers
	if err := checkContainerStateTransition(oldStatus.ContainerStatuses, status.ContainerStatuses, pod.Spec.RestartPolicy); err != nil {
		klog.Errorf("Status update on pod %v/%v aborted: %v", pod.Namespace, pod.Name, err)
		return false
	}
	if err := checkContainerStateTransition(oldStatus.InitContainerStatuses, status.InitContainerStatuses, pod.Spec.RestartPolicy); err != nil {
		klog.Errorf("Status update on pod %v/%v aborted: %v", pod.Namespace, pod.Name, err)
		return false
	}

	// Set ContainersReadyCondition.LastTransitionTime.
	updateLastTransitionTime(&status, &oldStatus, v1.ContainersReady)

	// Set ReadyCondition.LastTransitionTime.
	updateLastTransitionTime(&status, &oldStatus, v1.PodReady)

	// Set InitializedCondition.LastTransitionTime.
	updateLastTransitionTime(&status, &oldStatus, v1.PodInitialized)

	// Set PodScheduledCondition.LastTransitionTime.
	updateLastTransitionTime(&status, &oldStatus, v1.PodScheduled)

	// ensure that the start time does not change across updates.
	if oldStatus.StartTime != nil && !oldStatus.StartTime.IsZero() {
		status.StartTime = oldStatus.StartTime
	} else if status.StartTime.IsZero() {
		// if the status has no start time, we need to set an initial time
		now := metav1.Now()
		status.StartTime = &now
	}

	normalizeStatus(pod, &status)
	// The intent here is to prevent concurrent updates to a pod's status from
	// clobbering each other so the phase of a pod progresses monotonically.
	if isCached && isPodStatusByKubeletEqual(&cachedStatus.status, &status) && !forceUpdate {
		klog.V(3).Infof("Ignoring same status for pod %q rv=%s, status: %s", format.Pod(pod), pod.ResourceVersion, diff.ObjectReflectDiff(&cachedStatus.status, &status))
		return false // No new status.
	}

	priority := calculatePriority(&cachedStatus.status, &status)
	klog.V(3).Infof("Adding status to uid=%s at priority=%d (len=%d)", pod.UID, priority, len(m.podStatusQueue))

	newStatus := versionedPodStatus{
		status:       status,
		version:      cachedStatus.version + 1,
		podName:      pod.Name,
		podNamespace: pod.Namespace,
		priority:     priority,
	}
	if cachedStatus.priority > newStatus.priority {
		newStatus.priority = cachedStatus.priority
	}

	// only track status time after the first API server sync has succeeded
	var now time.Time
	if m.hasReportedStatus {
		now = time.Now()
	}
	if cachedStatus.at.IsZero() {
		newStatus.at = now
	} else {
		newStatus.at = cachedStatus.at
	}

	m.podStatuses[pod.UID] = newStatus
	m.podStatusQueue[pod.UID] = struct{}{}

	select {
	case m.podStatusChannel <- struct{}{}:
	default:
	}
	return true
}

// updateLastTransitionTime updates the LastTransitionTime of a pod condition.
func updateLastTransitionTime(status, oldStatus *v1.PodStatus, conditionType v1.PodConditionType) {
	_, condition := podutil.GetPodCondition(status, conditionType)
	if condition == nil {
		return
	}
	// Need to set LastTransitionTime.
	lastTransitionTime := metav1.Now()
	_, oldCondition := podutil.GetPodCondition(oldStatus, conditionType)
	if oldCondition != nil && condition.Status == oldCondition.Status {
		lastTransitionTime = oldCondition.LastTransitionTime
	}
	condition.LastTransitionTime = lastTransitionTime
}

// deletePodStatus simply removes the given pod from the status cache.
func (m *manager) deletePodStatus(uid types.UID) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()
	delete(m.podStatuses, uid)
	delete(m.recentPodWrites, uid)
}

// TODO(filipg): It'd be cleaner if we can do this without signal from user.
func (m *manager) RemoveOrphanedStatuses(podUIDs map[types.UID]bool) {
	m.podStatusesLock.Lock()
	defer m.podStatusesLock.Unlock()
	for key := range m.podStatuses {
		if _, ok := podUIDs[key]; !ok {
			klog.V(5).Infof("Removing %q from status map.", key)
			delete(m.podStatuses, key)
			delete(m.recentPodWrites, key)
		}
	}
}

// syncBatch syncs pods statuses with the apiserver.
func (m *manager) syncBatch(clean bool) {
	var update, reconcile, total int
	start := time.Now()
	defer func() {
		klog.V(3).Infof("syncBatch complete clean=%t duration=%s total=%d updated=%d reconciled=%d", clean, time.Now().Sub(start), total, update, reconcile)
	}()

	// calculate the statuses that should be updated (under the lock)
	var updatedStatuses []podStatusSyncRequest
	func() {
		if !clean {
			m.podStatusesLock.RLock()
			defer m.podStatusesLock.RUnlock()

			updatedStatuses = make([]podStatusSyncRequest, 0, len(m.podStatusQueue))
			for uid := range m.podStatusQueue {
				status, ok := m.podStatuses[uid]
				if !ok {
					continue
				}
				updatedStatuses = append(updatedStatuses, podStatusSyncRequest{uid, status})
			}
			for k := range m.podStatusQueue {
				delete(m.podStatusQueue, k)
			}
			return
		}

		_, mirrorToPod := m.podManager.GetUIDTranslations()
		m.podStatusesLock.RLock()
		defer m.podStatusesLock.RUnlock()

		updatedStatuses = make([]podStatusSyncRequest, 0, len(m.podStatuses))

		// Clean up orphaned versions.
		for uid := range m.apiStatusVersions {
			_, hasPod := m.podStatuses[types.UID(uid)]
			_, hasMirror := mirrorToPod[uid]
			if !hasPod && !hasMirror {
				delete(m.apiStatusVersions, uid)
				klog.V(3).Infof("syncBatch purge status version for uid=%s", uid)
			}
		}

		for uid, status := range m.podStatuses {
			updatedStatuses = append(updatedStatuses, podStatusSyncRequest{uid, status})
		}
	}()

	// process all pods in priority order
	sort.Sort(highestPrioritySyncRequests(updatedStatuses))
	for _, toUpdate := range updatedStatuses {
		uid, status := toUpdate.uid, toUpdate.status
		pod, ok := m.podForAPIServer(uid)
		if !ok {
			continue
		}

		total++

		var reason string
		switch {
		case m.needsUpdate(pod, status):
			// The pod status has either been updated internally, or the pod can be deleted
			reason = "Update"
			update++
		case m.needsReconcile(pod, status):
			// The pod status on the pod appears to be out of sync with the expected status
			reason = "Reconcile"
			reconcile++
		default:
			continue
		}

		klog.V(3).Infof("syncBatch will syncPod clean=%t uid=%q priority=%d reason=%s", clean, uid, status.priority, reason)
		m.syncPod(uid, pod, status)
	}
}

// syncPod syncs the given status with the API server. The caller must not hold the lock.
func (m *manager) syncPod(uid types.UID, pod *v1.Pod, status versionedPodStatus) {
	podDesc := format.PodDesc(status.podName, status.podNamespace, uid)

	// if this is a mirror pod, check that it resolves to the current static pod
	translatedUID := m.podManager.TranslatePodUID(pod.UID)
	if len(translatedUID) > 0 && translatedUID != kubetypes.ResolvedPodUID(uid) {
		klog.V(2).Infof("Pod %q was deleted and then recreated, skipping status update; old UID %q, new UID %q", podDesc, uid, translatedUID)
		m.deletePodStatus(uid)
		return
	}

	// Generate a patch from the last recorded status to the new state. Note that if another client performs
	// an update on the pod, the generated patch may stomp their values as that is the behavior that patch
	// allows.
	oldStatus := pod.Status.DeepCopy()
	newPod, patchBytes, unchanged, err := statusutil.PatchPodStatus(m.kubeClient, pod.Namespace, pod.Name, pod.UID, *oldStatus, mergePodStatus(*oldStatus, status.status))
	if err != nil {
		klog.Warningf("Patch status for pod %q failed: %v", podDesc, err)
		return
	}
	klog.V(3).Infof("Patch status for pod %q with %q", podDesc, patchBytes)

	// measure how long the status update took to propagate from generation to update on the server
	var duration time.Duration
	if status.at.IsZero() {
		klog.V(3).Infof("Pod %q had no status time set", format.Pod(pod))
	} else {
		duration = time.Now().Sub(status.at).Truncate(time.Millisecond)
		metrics.PodStatusSyncDuration.WithLabelValues(strconv.Itoa(status.priority)).Observe(duration.Seconds())

		// clear the pod status time and priority
		func() {
			m.podStatusesLock.Lock()
			defer m.podStatusesLock.Unlock()
			if current, ok := m.podStatuses[uid]; ok && current.version == status.version {
				current.at = time.Time{}
				current.priority = 0
				m.podStatuses[uid] = current
			}
		}()
	}

	// update our caches with the result of the write
	m.apiStatusVersions[kubetypes.MirrorPodUID(pod.UID)] = status.version
	if unchanged {
		klog.V(3).Infof("Status for pod %q is up-to-date after %s: (%d)", podDesc, duration, status.version)
	} else {
		klog.V(3).Infof("Status for pod %q updated successfully after %s: (%d, %+v)", podDesc, duration, status.version, status.status)
		pod = newPod

		func() {
			m.podStatusesLock.Lock()
			defer m.podStatusesLock.Unlock()
			m.recentPodWrites[uid] = pod
			m.hasReportedStatus = true
		}()
	}

	// if we can control deleting the pod, do so here
	if m.canBeDeleted(pod, status.status) {
		m.finalizePod(uid, pod)
	}
}

func (m *manager) finalizePod(uid types.UID, pod *v1.Pod) {
	podDesc := format.PodDesc(pod.Name, pod.Namespace, uid)
	deleteOptions := metav1.NewDeleteOptions(0)
	// Use the pod UID as the precondition for deletion to prevent deleting a newly created pod with the same name and namespace.
	deleteOptions.Preconditions = metav1.NewUIDPreconditions(string(pod.UID))
	if err := m.kubeClient.CoreV1().Pods(pod.Namespace).Delete(pod.Name, deleteOptions); err != nil {
		if !errors.IsNotFound(err) {
			klog.Warningf("Failed to delete pod %q: %v", podDesc, err)
		}
		return
	}
	klog.V(3).Infof("Pod %q fully terminated and removed from etcd", podDesc)
	m.deletePodStatus(uid)
}

// podForAPIServer retrieves the recent pod or mirror pod for a given Kubelet
// internal UID. This method must only be accessed by the sync thread.
func (m *manager) podForAPIServer(uid types.UID) (*v1.Pod, bool) {
	// The pod could be a static pod, so we should translate first.
	pod, ok := m.podManager.GetPodByUID(uid)
	if !ok {
		klog.V(4).Infof("Pod %q has been deleted, no need to reconcile", string(uid))
		return nil, false
	}
	// If the pod is a static pod, we should check its mirror pod, because only status in mirror pod is meaningful to us.
	if kubetypes.IsStaticPod(pod) {
		mirrorPod, ok := m.podManager.GetMirrorPodByPod(pod)
		if !ok {
			klog.V(4).Infof("Static pod %q has no corresponding mirror pod, no need to reconcile", format.Pod(pod))
			return nil, false
		}
		pod = mirrorPod
	}

	// TODO: move into a function
	// see if our recent cached write is newer and use it, otherwise purge it
	func() {
		m.podStatusesLock.Lock()
		defer m.podStatusesLock.Unlock()
		lastPod, ok := m.recentPodWrites[uid]
		if !ok {
			return
		}
		rel, comparable := podVersionCompare(pod, lastPod)
		if !comparable {
			return
		}
		if rel <= 0 {
			delete(m.recentPodWrites, uid)
			return
		}
		klog.V(3).Infof("Pod %q has a newer cached version rv=%s,%s", format.Pod(pod), pod.ResourceVersion, lastPod.ResourceVersion)
		pod = lastPod
	}()

	return pod, true
}

// needsUpdate returns whether the status is stale for the given pod UID.
// This method is not thread safe, and must only be accessed by the sync thread.
func (m *manager) needsUpdate(pod *v1.Pod, status versionedPodStatus) bool {
	latestVersion, ok := m.apiStatusVersions[kubetypes.MirrorPodUID(pod.UID)]
	if !ok || latestVersion < status.version {
		return true
	}
	return m.canBeDeleted(pod, status.status)
}

func (m *manager) canBeDeleted(pod *v1.Pod, status v1.PodStatus) bool {
	if pod.DeletionTimestamp == nil || kubetypes.IsMirrorPod(pod) {
		return false
	}
	return m.podDeletionSafety.PodResourcesAreReclaimed(pod, status)
}

// needsReconcile compares the given status with the status in the pod manager (which
// in fact comes from apiserver), returns whether the status needs to be reconciled with
// the apiserver. Now when pod status is inconsistent between apiserver and kubelet,
// kubelet should forcibly send an update to reconcile the inconsistence, because kubelet
// should be the source of truth of pod status.
func (m *manager) needsReconcile(pod *v1.Pod, status versionedPodStatus) bool {
	podStatus := pod.Status.DeepCopy()
	normalizeStatus(pod, podStatus)

	if isPodStatusByKubeletEqual(podStatus, &status.status) {
		// If the status from the source is the same with the cached status,
		// reconcile is not needed. Just return.
		return false
	}

	klog.V(3).Infof("Cached status for pod %q rv=%s is inconsistent with generated status %d, a reconciliation should be triggered:\n%s\nnon-normalized\n%s",
		format.Pod(pod), pod.ResourceVersion, status.version,
		diff.ObjectDiff(podStatus, &status.status),
		diff.ObjectDiff(&pod.Status, &status.status),
	)

	return true
}

// normalizeStatus normalizes nanosecond precision timestamps in podStatus
// down to second precision (*RFC339NANO* -> *RFC3339*). This must be done
// before comparing podStatus to the status returned by apiserver because
// apiserver does not support RFC339NANO.
// Related issue #15262/PR #15263 to move apiserver to RFC339NANO is closed.
func normalizeStatus(pod *v1.Pod, status *v1.PodStatus) *v1.PodStatus {
	bytesPerStatus := kubecontainer.MaxPodTerminationMessageLogLength
	if containers := len(pod.Spec.Containers) + len(pod.Spec.InitContainers); containers > 0 {
		bytesPerStatus = bytesPerStatus / containers
	}
	normalizeTimeStamp := func(t *metav1.Time) {
		*t = t.Rfc3339Copy()
	}
	normalizeContainerState := func(c *v1.ContainerState) {
		if c.Running != nil {
			normalizeTimeStamp(&c.Running.StartedAt)
		}
		if c.Terminated != nil {
			normalizeTimeStamp(&c.Terminated.StartedAt)
			normalizeTimeStamp(&c.Terminated.FinishedAt)
			if len(c.Terminated.Message) > bytesPerStatus {
				c.Terminated.Message = c.Terminated.Message[:bytesPerStatus]
			}
		}
	}

	if status.StartTime != nil {
		normalizeTimeStamp(status.StartTime)
	}
	for i := range status.Conditions {
		condition := &status.Conditions[i]
		normalizeTimeStamp(&condition.LastProbeTime)
		normalizeTimeStamp(&condition.LastTransitionTime)
	}

	// update container statuses
	for i := range status.ContainerStatuses {
		cstatus := &status.ContainerStatuses[i]
		normalizeContainerState(&cstatus.State)
		normalizeContainerState(&cstatus.LastTerminationState)
	}
	// Sort the container statuses, so that the order won't affect the result of comparison
	sort.Sort(kubetypes.SortedContainerStatuses(status.ContainerStatuses))

	// update init container statuses
	for i := range status.InitContainerStatuses {
		cstatus := &status.InitContainerStatuses[i]
		normalizeContainerState(&cstatus.State)
		normalizeContainerState(&cstatus.LastTerminationState)
	}
	// Sort the container statuses, so that the order won't affect the result of comparison
	kubetypes.SortInitContainerStatuses(pod, status.InitContainerStatuses)
	return status
}

// mergePodStatus merges oldPodStatus and newPodStatus where pod conditions
// not owned by kubelet is preserved from oldPodStatus
func mergePodStatus(oldPodStatus, newPodStatus v1.PodStatus) v1.PodStatus {
	podConditions := []v1.PodCondition{}
	for _, c := range oldPodStatus.Conditions {
		if !kubetypes.PodConditionByKubelet(c.Type) {
			podConditions = append(podConditions, c)
		}
	}

	for _, c := range newPodStatus.Conditions {
		if kubetypes.PodConditionByKubelet(c.Type) {
			podConditions = append(podConditions, c)
		}
	}
	newPodStatus.Conditions = podConditions
	return newPodStatus
}

// NeedToReconcilePodReadiness returns if the pod "Ready" condition need to be reconcile
func NeedToReconcilePodReadiness(pod *v1.Pod) bool {
	if len(pod.Spec.ReadinessGates) == 0 {
		return false
	}
	podReadyCondition := GeneratePodReadyCondition(&pod.Spec, pod.Status.Conditions, pod.Status.ContainerStatuses, pod.Status.Phase)
	i, curCondition := podutil.GetPodConditionFromList(pod.Status.Conditions, v1.PodReady)
	// Only reconcile if "Ready" condition is present and Status or Message is not expected
	if i >= 0 && (curCondition.Status != podReadyCondition.Status || curCondition.Message != podReadyCondition.Message) {
		return true
	}
	return false
}
