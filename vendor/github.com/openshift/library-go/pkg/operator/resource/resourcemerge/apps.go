package resourcemerge

import (
	appsv1 "k8s.io/api/apps/v1"

	operatorsv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	"github.com/openshift/library-go/pkg/operator/v1alpha1helpers"
)

func ExpectedDeploymentGeneration(required *appsv1.Deployment, previousAvailability *operatorsv1alpha1.VersionAvailability) int64 {
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

func ApplyDeploymentGenerationAvailability(versionAvailability operatorsv1alpha1.VersionAvailability, actual *appsv1.Deployment, errors ...error) operatorsv1alpha1.VersionAvailability {
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

func ExpectedDaemonSetGeneration(required *appsv1.DaemonSet, previousAvailability *operatorsv1alpha1.VersionAvailability) int64 {
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

func ApplyDaemonSetGenerationAvailability(versionAvailability operatorsv1alpha1.VersionAvailability, actual *appsv1.DaemonSet, errors ...error) operatorsv1alpha1.VersionAvailability {
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
