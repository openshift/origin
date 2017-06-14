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

package api

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/apis/extensions/v1beta1"
)

type ResourceType string

const (
	Pods                   ResourceType = "pods"
	PersistentVolumes      ResourceType = "persistentvolumes"
	ReplicationControllers ResourceType = "replicationcontrollers"
	Nodes                  ResourceType = "nodes"
	Services               ResourceType = "services"
	PersistentVolumeClaims ResourceType = "persistentvolumeclaims"
	ReplicaSets            ResourceType = "replicasets"
)

func (r ResourceType) String() string {
	return string(r)
}

func (r ResourceType) ObjectType() runtime.Object {
	switch r {
	case "pods":
		return &v1.Pod{}
	case "persistentvolumes":
		return &v1.PersistentVolume{}
	case "replicationcontrollers":
		return &v1.ReplicationController{}
	case "nodes":
		return &v1.Node{}
	case "services":
		return &v1.Service{}
	case "persistentvolumeclaims":
		return &v1.PersistentVolumeClaim{}
	case "replicasets":
		return &v1beta1.ReplicaSet{}
	}
	return nil
}

func StringToResourceType(resource string) (ResourceType, error) {
	switch resource {
	case "pods":
		return Pods, nil
	case "persistentvolumes":
		return PersistentVolumes, nil
	case "replicationcontrollers":
		return ReplicationControllers, nil
	case "nodes":
		return Nodes, nil
	case "services":
		return Services, nil
	case "persistentvolumeclaims":
		return PersistentVolumeClaims, nil
	case "replicasets":
		return ReplicaSets, nil
	default:
		return "", fmt.Errorf("Resource type %v not recognized", resource)
	}
}
