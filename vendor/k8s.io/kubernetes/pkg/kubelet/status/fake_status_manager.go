/*
Copyright 2021 The Kubernetes Authors.

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
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/status/state"
)

type fakeManager struct {
	state state.State
}

func (m *fakeManager) Start() {
	klog.InfoS("Start()")
	return
}

func (m *fakeManager) GetPodStatus(uid types.UID) (v1.PodStatus, bool) {
	klog.InfoS("GetPodStatus()")
	return v1.PodStatus{}, false
}

func (m *fakeManager) SetPodStatus(pod *v1.Pod, status v1.PodStatus) {
	klog.InfoS("SetPodStatus()")
	return
}

func (m *fakeManager) SetContainerReadiness(podUID types.UID, containerID kubecontainer.ContainerID, ready bool) {
	klog.InfoS("SetContainerReadiness()")
	return
}

func (m *fakeManager) SetContainerStartup(podUID types.UID, containerID kubecontainer.ContainerID, started bool) {
	klog.InfoS("SetContainerStartup()")
	return
}

func (m *fakeManager) TerminatePod(pod *v1.Pod) {
	klog.InfoS("TerminatePod()")
	return
}

func (m *fakeManager) RemoveOrphanedStatuses(podUIDs map[types.UID]bool) {
	klog.InfoS("RemoveOrphanedStatuses()")
	return
}

func (m *fakeManager) GetContainerResourceAllocation(podUID string, containerName string) (v1.ResourceList, bool) {
	klog.InfoS("GetContainerResourceAllocation()")
	return m.state.GetContainerResourceAllocation(podUID, containerName)
}

func (m *fakeManager) GetPodResizeStatus(podUID string) (v1.PodResizeStatus, bool) {
	klog.InfoS("GetPodResizeStatus()")
	return "", false
}

func (m *fakeManager) SetPodAllocation(pod *v1.Pod) error {
	klog.InfoS("SetPodAllocation()")
	for _, container := range pod.Spec.Containers {
		var alloc v1.ResourceList
		if container.Resources.Requests != nil {
			alloc = container.Resources.Requests.DeepCopy()
		}
		m.state.SetContainerResourceAllocation(string(pod.UID), container.Name, alloc)
	}
	return nil
}

func (m *fakeManager) SetPodResizeStatus(podUID types.UID, resizeStatus v1.PodResizeStatus) error {
	klog.InfoS("SetPodResizeStatus()")
	return nil
}

// NewFakeManager creates empty/fake memory manager
func NewFakeManager() Manager {
	return &fakeManager{
		state: state.NewStateMemory(),
	}
}
