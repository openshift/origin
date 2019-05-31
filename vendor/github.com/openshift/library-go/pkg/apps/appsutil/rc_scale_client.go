package appsutil

import (
	"fmt"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	scaleclient "k8s.io/client-go/scale"
	"k8s.io/client-go/scale/scheme/autoscalingv1"
)

// rcMapper pins preferred version to v1 and scale kind to autoscaling/v1 Scale
// this avoids putting complete server discovery (including extension APIs) in the critical path for deployments
type rcMapper struct{}

func (rcMapper) ResourceFor(gvr schema.GroupVersionResource) (schema.GroupVersionResource, error) {
	if gvr.Group == "" && gvr.Resource == "replicationcontrollers" {
		return schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"}, nil
	}
	return schema.GroupVersionResource{}, fmt.Errorf("unknown replication controller resource: %#v", gvr)
}

func (rcMapper) ScaleForResource(gvr schema.GroupVersionResource) (schema.GroupVersionKind, error) {
	rcGvr := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "replicationcontrollers"}
	if gvr == rcGvr {
		return autoscalingv1.SchemeGroupVersion.WithKind("Scale"), nil
	}
	return schema.GroupVersionKind{}, fmt.Errorf("unknown replication controller resource: %#v", gvr)
}

func NewReplicationControllerScaleClient(client kubernetes.Interface) scaleclient.ScalesGetter {
	return scaleclient.New(client.CoreV1().RESTClient(), rcMapper{}, dynamic.LegacyAPIPathResolverFunc, rcMapper{})
}
