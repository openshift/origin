package meta

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	"k8s.io/kubernetes/pkg/apis/extensions"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/validation/field"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	securityapi "github.com/openshift/origin/pkg/security/api"
)

// resourcesToCheck is a map of resources and corresponding kinds of things that
// we want handled in this plugin
var resourcesToCheck = map[unversioned.GroupResource]unversioned.GroupKind{
	kapi.Resource("pods"):                                       kapi.Kind("Pod"),
	kapi.Resource("podtemplates"):                               kapi.Kind("PodTemplate"),
	kapi.Resource("replicationcontrollers"):                     kapi.Kind("ReplicationController"),
	batch.Resource("jobs"):                                      batch.Kind("Job"),
	batch.Resource("jobtemplates"):                              batch.Kind("JobTemplate"),
	batch.Resource("scheduledjobs"):                             batch.Kind("ScheduledJob"),
	extensions.Resource("deployments"):                          extensions.Kind("Deployment"),
	extensions.Resource("replicasets"):                          extensions.Kind("ReplicaSet"),
	extensions.Resource("jobs"):                                 extensions.Kind("Job"),
	extensions.Resource("jobtemplates"):                         extensions.Kind("JobTemplate"),
	apps.Resource("petsets"):                                    apps.Kind("PetSet"),
	deployapi.Resource("deploymentconfigs"):                     deployapi.Kind("DeploymentConfig"),
	securityapi.Resource("podsecuritypolicysubjectreviews"):     securityapi.Kind("PodSecurityPolicySubjectReview"),
	securityapi.Resource("podsecuritypolicyselfsubjectreviews"): securityapi.Kind("PodSecurityPolicySelfSubjectReview"),
	securityapi.Resource("podsecuritypolicyreviews"):            securityapi.Kind("PodSecurityPolicyReview"),
}

// HasPodSpec returns true if the resource is known to have a pod spec.
func HasPodSpec(gr unversioned.GroupResource) (unversioned.GroupKind, bool) {
	gk, ok := resourcesToCheck[gr]
	return gk, ok
}

var errNoPodSpec = fmt.Errorf("No PodSpec available for this object")

// GetPodSpec returns a mutable pod spec out of the provided object, including a field path
// to the field in the object, or an error if the object does not contain a pod spec.
func GetPodSpec(obj runtime.Object) (*kapi.PodSpec, *field.Path, error) {
	switch r := obj.(type) {
	case *kapi.Pod:
		return &r.Spec, field.NewPath("spec"), nil
	case *kapi.PodTemplate:
		return &r.Template.Spec, field.NewPath("template", "spec"), nil
	case *kapi.ReplicationController:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensions.DaemonSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensions.Deployment:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensions.ReplicaSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batch.Job:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batch.ScheduledJob:
		return &r.Spec.JobTemplate.Spec.Template.Spec, field.NewPath("spec", "jobTemplate", "spec", "template", "spec"), nil
	case *batch.JobTemplate:
		return &r.Template.Spec.Template.Spec, field.NewPath("template", "spec", "template", "spec"), nil
	case *deployapi.DeploymentConfig:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *apps.PetSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicyReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	}
	return nil, nil, errNoPodSpec
}

// podSpecMutator implements the mutation interface over objects with a pod spec.
type podSpecMutator struct {
	spec *kapi.PodSpec
	path *field.Path
}

// Mutate applies fn to all containers and init containers. If fn changes the Kind to
// any value other than "DockerImage", an error is set on that field.
func (m *podSpecMutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i := range m.spec.InitContainers {
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: m.spec.InitContainers[i].Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, field.InternalError(m.path.Child("initContainers").Index(i).Child("image"), err))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, field.InternalError(m.path.Child("initContainers").Index(i).Child("image"), fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind)))
			continue
		}
		m.spec.InitContainers[i].Image = ref.Name
	}
	for i := range m.spec.Containers {
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: m.spec.Containers[i].Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, field.InternalError(m.path.Child("containers").Index(i).Child("image"), err))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, field.InternalError(m.path.Child("containers").Index(i).Child("image"), fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind)))
			continue
		}
		m.spec.Containers[i].Image = ref.Name
	}
	return errs
}
