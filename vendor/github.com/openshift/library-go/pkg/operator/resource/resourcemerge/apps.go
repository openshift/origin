package resourcemerge

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1 "github.com/openshift/api/operator/v1"
)

func GenerationFor(generations []operatorsv1.GenerationStatus, resource schema.GroupResource, namespace, name string) *operatorsv1.GenerationStatus {
	for i := range generations {
		curr := &generations[i]
		if curr.Namespace == namespace &&
			curr.Name == name &&
			curr.Group == resource.Group &&
			curr.Resource == resource.Resource {

			return curr
		}
	}

	return nil
}

func SetGeneration(generations *[]operatorsv1.GenerationStatus, newGeneration operatorsv1.GenerationStatus) {
	if generations == nil {
		generations = &[]operatorsv1.GenerationStatus{}
	}

	existingGeneration := GenerationFor(*generations, schema.GroupResource{Group: newGeneration.Group, Resource: newGeneration.Resource}, newGeneration.Namespace, newGeneration.Name)
	if existingGeneration == nil {
		*generations = append(*generations, newGeneration)
		return
	}

	existingGeneration.LastGeneration = newGeneration.LastGeneration
	existingGeneration.Hash = newGeneration.Hash
}

func ExpectedDeploymentGeneration(required *appsv1.Deployment, previousGenerations []operatorsv1.GenerationStatus) int64 {
	generation := GenerationFor(previousGenerations, schema.GroupResource{Group: "apps", Resource: "deployments"}, required.Namespace, required.Name)
	if generation != nil {
		return generation.LastGeneration
	}
	return -1
}

func SetDeploymentGeneration(generations *[]operatorsv1.GenerationStatus, actual *appsv1.Deployment) {
	if actual == nil {
		return
	}
	SetGeneration(generations, operatorsv1.GenerationStatus{
		Group:          "apps",
		Resource:       "deployments",
		Namespace:      actual.Namespace,
		Name:           actual.Name,
		LastGeneration: actual.ObjectMeta.Generation,
	})
}

func ExpectedDaemonSetGeneration(required *appsv1.DaemonSet, previousGenerations []operatorsv1.GenerationStatus) int64 {
	generation := GenerationFor(previousGenerations, schema.GroupResource{Group: "apps", Resource: "daemonsets"}, required.Namespace, required.Name)
	if generation != nil {
		return generation.LastGeneration
	}
	return -1
}

func SetDaemonSetGeneration(generations *[]operatorsv1.GenerationStatus, actual *appsv1.DaemonSet) {
	if actual == nil {
		return
	}
	SetGeneration(generations, operatorsv1.GenerationStatus{
		Group:          "apps",
		Resource:       "daemonsets",
		Namespace:      actual.Namespace,
		Name:           actual.Name,
		LastGeneration: actual.ObjectMeta.Generation,
	})
}
