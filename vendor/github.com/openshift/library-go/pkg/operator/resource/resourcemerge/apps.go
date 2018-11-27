package resourcemerge

import (
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1 "github.com/openshift/api/operator/v1"
	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
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

func ExpectedDeploymentGenerationV1alpha1(required *appsv1.Deployment, previousAvailability *operatorsv1alpha1.VersionAvailability) int64 {
	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Namespace == required.Namespace &&
				curr.Name == required.Name &&
				curr.Group == "apps" &&
				curr.Resource == "deployments" {

				generation = curr.LastGeneration
			}
		}
	}

	return generation
}

func ApplyDeploymentGenerationAvailabilityV1alpha1(versionAvailability operatorsv1alpha1.VersionAvailability, actual *appsv1.Deployment, errors ...error) operatorsv1alpha1.VersionAvailability {
	newAvailability := versionAvailability.DeepCopy()
	if actual != nil {
		newAvailability.UpdatedReplicas = actual.Status.UpdatedReplicas
		newAvailability.ReadyReplicas = actual.Status.ReadyReplicas
		newAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "deployments",
				Namespace: actual.Namespace, Name: actual.Name,
				LastGeneration: actual.ObjectMeta.Generation,
			},
		}
	}
	v1alpha1helpers.SetErrors(newAvailability, errors...)

	return *newAvailability
}

func ExpectedDaemonSetGenerationV1alpha1(required *appsv1.DaemonSet, previousAvailability *operatorsv1alpha1.VersionAvailability) int64 {
	generation := int64(-1)
	if previousAvailability != nil {
		for _, curr := range previousAvailability.Generations {
			if curr.Namespace == required.Namespace &&
				curr.Name == required.Name &&
				curr.Group == "apps" &&
				curr.Resource == "daemonsets" {

				generation = curr.LastGeneration
			}
		}
	}

	return generation
}

func ApplyDaemonSetGenerationAvailabilityV1alpha1(versionAvailability operatorsv1alpha1.VersionAvailability, actual *appsv1.DaemonSet, errors ...error) operatorsv1alpha1.VersionAvailability {
	newAvailability := versionAvailability.DeepCopy()
	if actual != nil {
		newAvailability.UpdatedReplicas = actual.Status.UpdatedNumberScheduled
		newAvailability.ReadyReplicas = actual.Status.NumberReady
		newAvailability.Generations = []operatorsv1alpha1.GenerationHistory{
			{
				Group: "apps", Resource: "daemonsets",
				Namespace: actual.Namespace, Name: actual.Name,
				LastGeneration: actual.ObjectMeta.Generation,
			},
		}
	}
	v1alpha1helpers.SetErrors(newAvailability, errors...)

	return *newAvailability
}
