/*
Copyright 2017 The Kubernetes Authors.

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

package strategy

import (
	"fmt"
	"reflect"
	goruntime "runtime"
	"testing"

	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apitesting "k8s.io/kubernetes/pkg/api/testing"
	"k8s.io/kubernetes/pkg/version"

	"github.com/kubernetes-incubator/cluster-capacity/pkg/framework/store"
)

func getTestNode(nodeName string) *v1.Node {
	return &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: nodeName},
		Spec:       v1.NodeSpec{},
		Status: v1.NodeStatus{
			Conditions: []v1.NodeCondition{
				{
					Type:               v1.NodeOutOfDisk,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientDisk",
					Message:            fmt.Sprintf("kubelet has sufficient disk space available"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeMemoryPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasSufficientMemory",
					Message:            fmt.Sprintf("kubelet has sufficient memory available"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeDiskPressure,
					Status:             v1.ConditionFalse,
					Reason:             "KubeletHasNoDiskPressure",
					Message:            fmt.Sprintf("kubelet has no disk pressure"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
				{
					Type:               v1.NodeReady,
					Status:             v1.ConditionTrue,
					Reason:             "KubeletReady",
					Message:            fmt.Sprintf("kubelet is posting ready status"),
					LastHeartbeatTime:  metav1.Time{},
					LastTransitionTime: metav1.Time{},
				},
			},
			NodeInfo: v1.NodeSystemInfo{
				MachineID:               "123",
				SystemUUID:              "abc",
				BootID:                  "1b3",
				KernelVersion:           "3.16.0-0.bpo.4-amd64",
				OSImage:                 "Debian GNU/Linux 7 (wheezy)",
				OperatingSystem:         goruntime.GOOS,
				Architecture:            goruntime.GOARCH,
				ContainerRuntimeVersion: "test://1.5.0",
				KubeletVersion:          version.Get().String(),
				KubeProxyVersion:        version.Get().String(),
			},
			Capacity: v1.ResourceList{
				v1.ResourceCPU:       *resource.NewMilliQuantity(2000, resource.DecimalSI),
				v1.ResourceMemory:    *resource.NewQuantity(10E9, resource.BinarySI),
				v1.ResourcePods:      *resource.NewQuantity(0, resource.DecimalSI),
				v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
			},
			Allocatable: v1.ResourceList{
				v1.ResourceCPU:       *resource.NewMilliQuantity(300, resource.DecimalSI),
				v1.ResourceMemory:    *resource.NewQuantity(20E6, resource.BinarySI),
				v1.ResourcePods:      *resource.NewQuantity(0, resource.DecimalSI),
				v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
			},
			Addresses: []v1.NodeAddress{
				{Type: v1.NodeExternalIP, Address: "127.0.0.1"},
				{Type: v1.NodeInternalIP, Address: "127.0.0.1"},
			},
			Images: []v1.ContainerImage{},
		},
	}
}

var testStrategyNode string = "node1"

func newScheduledPod() *v1.Pod {
	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "schedulerPod", Namespace: "test", ResourceVersion: "10"},
		Spec:       apitesting.V1DeepEqualSafePodSpec(),
	}

	// set pod's resource consumption
	pod.Spec.Containers = []v1.Container{
		{
			Resources: v1.ResourceRequirements{
				Limits: v1.ResourceList{
					v1.ResourceCPU:       *resource.NewMilliQuantity(400, resource.DecimalSI),
					v1.ResourceMemory:    *resource.NewQuantity(10E6, resource.BinarySI),
					v1.ResourcePods:      *resource.NewQuantity(0, resource.DecimalSI),
					v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
				},
				Requests: v1.ResourceList{
					v1.ResourceCPU:       *resource.NewMilliQuantity(400, resource.DecimalSI),
					v1.ResourceMemory:    *resource.NewQuantity(10E6, resource.BinarySI),
					v1.ResourcePods:      *resource.NewQuantity(0, resource.DecimalSI),
					v1.ResourceNvidiaGPU: *resource.NewQuantity(0, resource.DecimalSI),
				},
			},
		},
	}

	// schedule the pod on the node
	pod.Spec.NodeName = testStrategyNode

	return pod
}

func TestAddPodStrategy(t *testing.T) {
	// 1. create resource storage and fill it with a fake node
	resourceStore := store.NewResourceStore()
	resourceStore.Add("nodes", getTestNode(testStrategyNode))

	predictiveStrategy := NewPredictiveStrategy(resourceStore)

	// 2. create fake pod with some consumed resources assigned to the fake fake
	scheduledPod := newScheduledPod()

	// 3. run the strategy to retrieve the node from the resource store recomputing the node's allocatable
	err := predictiveStrategy.Add(scheduledPod)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	// 4. check both the update node and the pod is stored back into the resource store
	foundPod, exists, err := resourceStore.Get("pods", metav1.Object(scheduledPod))
	if err != nil {
		t.Errorf("Unexpected error when retrieving scheduled pod: %v", err)
	}
	if !exists {
		t.Errorf("Unable to find scheduled pod: %v", err)
	}

	actualPod := foundPod.(*v1.Pod)
	if !reflect.DeepEqual(scheduledPod, actualPod) {
		t.Errorf("Unexpected object: expected: %#v\n actual: %#v", scheduledPod, actualPod)
	}

	node := &v1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: testStrategyNode},
	}

	foundNode, exists, err := resourceStore.Get("nodes", metav1.Object(node))
	if err != nil {
		t.Errorf("Unexpected error when retrieving scheduled node: %v", err)
	}
	if !exists {
		t.Errorf("Unable to find scheduled node: %v", err)
	}

	actualNode := foundNode.(*v1.Node)
	if reflect.DeepEqual(node, actualNode) {
		t.Errorf("Expected %q node to be modified", testStrategyNode)
	}
}
