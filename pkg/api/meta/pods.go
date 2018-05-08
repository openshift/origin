package meta

import (
	"fmt"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	batchv1 "k8s.io/api/batch/v1"
	batchv2alpha1 "k8s.io/api/batch/v2alpha1"
	kapiv1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/apps"
	"k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/apis/extensions"

	appsapiv1 "github.com/openshift/api/apps/v1"
	securityapiv1 "github.com/openshift/api/security/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	securityapi "github.com/openshift/origin/pkg/security/apis/security"
)

type ContainerMutator interface {
	GetName() string
	GetImage() string
	SetImage(image string)
}

type PodSpecReferenceMutator interface {
	GetContainerByIndex(init bool, i int) (ContainerMutator, bool)
	GetContainerByName(name string) (ContainerMutator, bool)
	Path() *field.Path
}

// GetPodSpecReferenceMutator returns a mutator for the provided object, or an error if no
// such mutator is defined.
func GetPodSpecReferenceMutator(obj runtime.Object) (PodSpecReferenceMutator, error) {
	if spec, path, err := GetPodSpec(obj); err == nil {
		return &podSpecMutator{spec: spec, path: path}, nil
	}
	if spec, path, err := GetPodSpecV1(obj); err == nil {
		return &podSpecV1Mutator{spec: spec, path: path}, nil
	}
	return nil, errNoImageMutator
}

// resourcesToCheck is a map of resources and corresponding kinds of things that
// we want handled in this plugin
var resourcesToCheck = map[schema.GroupResource]schema.GroupKind{
	kapi.Resource("pods"):                   kapi.Kind("Pod"),
	kapi.Resource("podtemplates"):           kapi.Kind("PodTemplate"),
	kapi.Resource("replicationcontrollers"): kapi.Kind("ReplicationController"),
	batch.Resource("jobs"):                  batch.Kind("Job"),
	batch.Resource("jobtemplates"):          batch.Kind("JobTemplate"),

	batch.Resource("cronjobs"):         batch.Kind("CronJob"),
	extensions.Resource("deployments"): extensions.Kind("Deployment"),
	extensions.Resource("replicasets"): extensions.Kind("ReplicaSet"),
	apps.Resource("statefulsets"):      apps.Kind("StatefulSet"),

	{Group: "", Resource: "deploymentconfigs"}:                   {Group: "", Kind: "DeploymentConfig"},
	{Group: "", Resource: "podsecuritypolicysubjectreviews"}:     {Group: "", Kind: "PodSecurityPolicySubjectReview"},
	{Group: "", Resource: "podsecuritypolicyselfsubjectreviews"}: {Group: "", Kind: "PodSecurityPolicySelfSubjectReview"},
	{Group: "", Resource: "podsecuritypolicyreviews"}:            {Group: "", Kind: "PodSecurityPolicyReview"},

	appsapi.Resource("deploymentconfigs"):                       appsapi.Kind("DeploymentConfig"),
	securityapi.Resource("podsecuritypolicysubjectreviews"):     securityapi.Kind("PodSecurityPolicySubjectReview"),
	securityapi.Resource("podsecuritypolicyselfsubjectreviews"): securityapi.Kind("PodSecurityPolicySelfSubjectReview"),
	securityapi.Resource("podsecuritypolicyreviews"):            securityapi.Kind("PodSecurityPolicyReview"),
}

// HasPodSpec returns true if the resource is known to have a pod spec.
func HasPodSpec(gr schema.GroupResource) (schema.GroupKind, bool) {
	gk, ok := resourcesToCheck[gr]
	return gk, ok
}

var errNoPodSpec = fmt.Errorf("No PodSpec available for this object")

// GetPodSpec returns a mutable pod spec out of the provided object, including a field path
// to the field in the object, or an error if the object does not contain a pod spec.
// This only returns internal objects.
func GetPodSpec(obj runtime.Object) (*kapi.PodSpec, *field.Path, error) {
	switch r := obj.(type) {
	case *kapi.Pod:
		return &r.Spec, field.NewPath("spec"), nil
	case *kapi.PodTemplate:
		return &r.Template.Spec, field.NewPath("template", "spec"), nil
	case *kapi.ReplicationController:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	case *extensions.DaemonSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensions.Deployment:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensions.ReplicaSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batch.Job:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batch.CronJob:
		return &r.Spec.JobTemplate.Spec.Template.Spec, field.NewPath("spec", "jobTemplate", "spec", "template", "spec"), nil
	case *batch.JobTemplate:
		return &r.Template.Spec.Template.Spec, field.NewPath("template", "spec", "template", "spec"), nil
	case *apps.StatefulSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapi.PodSecurityPolicyReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *appsapi.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	}
	return nil, nil, errNoPodSpec
}

// GetPodSpecV1 returns a mutable pod spec out of the provided object, including a field path
// to the field in the object, or an error if the object does not contain a pod spec.
// This only returns pod specs for v1 compatible objects.
func GetPodSpecV1(obj runtime.Object) (*kapiv1.PodSpec, *field.Path, error) {
	switch r := obj.(type) {
	case *kapiv1.Pod:
		return &r.Spec, field.NewPath("spec"), nil
	case *kapiv1.PodTemplate:
		return &r.Template.Spec, field.NewPath("template", "spec"), nil
	case *kapiv1.ReplicationController:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	case *extensionsv1beta1.DaemonSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensionsv1beta1.Deployment:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *extensionsv1beta1.ReplicaSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batchv1.Job:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *batchv2alpha1.CronJob:
		return &r.Spec.JobTemplate.Spec.Template.Spec, field.NewPath("spec", "jobTemplate", "spec", "template", "spec"), nil
	case *batchv2alpha1.JobTemplate:
		return &r.Template.Spec.Template.Spec, field.NewPath("template", "spec", "template", "spec"), nil
	case *appsv1beta1.StatefulSet:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *appsv1beta1.Deployment:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapiv1.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapiv1.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *securityapiv1.PodSecurityPolicyReview:
		return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
	case *appsapiv1.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.Spec, field.NewPath("spec", "template", "spec"), nil
		}
	}
	return nil, nil, errNoPodSpec
}

// GetTemplateMetaObject returns a mutable metav1.Object interface for the template
// the object contains, or false if no such object is available.
func GetTemplateMetaObject(obj runtime.Object) (metav1.Object, bool) {
	switch r := obj.(type) {
	case *kapiv1.PodTemplate:
		return &r.Template.ObjectMeta, true
	case *kapiv1.ReplicationController:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}
	case *extensionsv1beta1.DaemonSet:
		return &r.Spec.Template.ObjectMeta, true
	case *extensionsv1beta1.Deployment:
		return &r.Spec.Template.ObjectMeta, true
	case *extensionsv1beta1.ReplicaSet:
		return &r.Spec.Template.ObjectMeta, true
	case *batchv1.Job:
		return &r.Spec.Template.ObjectMeta, true
	case *batchv2alpha1.CronJob:
		return &r.Spec.JobTemplate.Spec.Template.ObjectMeta, true
	case *batchv2alpha1.JobTemplate:
		return &r.Template.Spec.Template.ObjectMeta, true
	case *appsv1beta1.StatefulSet:
		return &r.Spec.Template.ObjectMeta, true
	case *appsv1beta1.Deployment:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapiv1.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapiv1.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapiv1.PodSecurityPolicyReview:
		return &r.Spec.Template.ObjectMeta, true
	case *appsapiv1.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}
	case *kapi.PodTemplate:
		return &r.Template.ObjectMeta, true
	case *kapi.ReplicationController:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}
	case *extensions.DaemonSet:
		return &r.Spec.Template.ObjectMeta, true
	case *extensions.Deployment:
		return &r.Spec.Template.ObjectMeta, true
	case *extensions.ReplicaSet:
		return &r.Spec.Template.ObjectMeta, true
	case *batch.Job:
		return &r.Spec.Template.ObjectMeta, true
	case *batch.CronJob:
		return &r.Spec.JobTemplate.Spec.Template.ObjectMeta, true
	case *batch.JobTemplate:
		return &r.Template.Spec.Template.ObjectMeta, true
	case *apps.StatefulSet:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapi.PodSecurityPolicySubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapi.PodSecurityPolicySelfSubjectReview:
		return &r.Spec.Template.ObjectMeta, true
	case *securityapi.PodSecurityPolicyReview:
		return &r.Spec.Template.ObjectMeta, true
	case *appsapi.DeploymentConfig:
		if r.Spec.Template != nil {
			return &r.Spec.Template.ObjectMeta, true
		}
	}
	return nil, false
}

type containerMutator struct {
	*kapi.Container
}

func (m containerMutator) GetName() string       { return m.Name }
func (m containerMutator) GetImage() string      { return m.Image }
func (m containerMutator) SetImage(image string) { m.Image = image }

type containerV1Mutator struct {
	*kapiv1.Container
}

func (m containerV1Mutator) GetName() string       { return m.Name }
func (m containerV1Mutator) GetImage() string      { return m.Image }
func (m containerV1Mutator) SetImage(image string) { m.Image = image }

// podSpecMutator implements the mutation interface over objects with a pod spec.
type podSpecMutator struct {
	spec    *kapi.PodSpec
	oldSpec *kapi.PodSpec
	path    *field.Path
}

func (m *podSpecMutator) Path() *field.Path {
	return m.path
}

func hasIdenticalPodSpecImage(spec *kapi.PodSpec, containerName, image string) bool {
	if spec == nil {
		return false
	}
	for i := range spec.InitContainers {
		if spec.InitContainers[i].Name == containerName {
			return spec.InitContainers[i].Image == image
		}
	}
	for i := range spec.Containers {
		if spec.Containers[i].Name == containerName {
			return spec.Containers[i].Image == image
		}
	}
	return false
}

// Mutate applies fn to all containers and init containers. If fn changes the Kind to
// any value other than "DockerImage", an error is set on that field.
func (m *podSpecMutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i := range m.spec.InitContainers {
		container := &m.spec.InitContainers[i]
		if hasIdenticalPodSpecImage(m.oldSpec, container.Name, container.Image) {
			continue
		}
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: container.Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("initContainers").Index(i).Child("image")))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, fieldErrorOrInternal(fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind), m.path.Child("initContainers").Index(i).Child("image")))
			continue
		}
		container.Image = ref.Name
	}
	for i := range m.spec.Containers {
		container := &m.spec.Containers[i]
		if hasIdenticalPodSpecImage(m.oldSpec, container.Name, container.Image) {
			continue
		}
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: container.Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("containers").Index(i).Child("image")))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, fieldErrorOrInternal(fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind), m.path.Child("containers").Index(i).Child("image")))
			continue
		}
		container.Image = ref.Name
	}
	return errs
}

func (m *podSpecMutator) GetContainerByName(name string) (ContainerMutator, bool) {
	spec := m.spec
	for i := range spec.InitContainers {
		if name != spec.InitContainers[i].Name {
			continue
		}
		return containerMutator{&spec.InitContainers[i]}, true
	}
	for i := range spec.Containers {
		if name != spec.Containers[i].Name {
			continue
		}
		return containerMutator{&spec.Containers[i]}, true
	}
	return nil, false
}

func (m *podSpecMutator) GetContainerByIndex(init bool, i int) (ContainerMutator, bool) {
	var container *kapi.Container
	spec := m.spec
	if init {
		if i < 0 || i >= len(spec.InitContainers) {
			return nil, false
		}
		container = &spec.InitContainers[i]
	} else {
		if i < 0 || i >= len(spec.Containers) {
			return nil, false
		}
		container = &spec.Containers[i]
	}
	return containerMutator{container}, true
}

// podSpecV1Mutator implements the mutation interface over objects with a pod spec.
type podSpecV1Mutator struct {
	spec    *kapiv1.PodSpec
	oldSpec *kapiv1.PodSpec
	path    *field.Path
}

func (m *podSpecV1Mutator) Path() *field.Path {
	return m.path
}

func hasIdenticalPodSpecV1Image(spec *kapiv1.PodSpec, containerName, image string) bool {
	if spec == nil {
		return false
	}
	for i := range spec.InitContainers {
		if spec.InitContainers[i].Name == containerName {
			return spec.InitContainers[i].Image == image
		}
	}
	for i := range spec.Containers {
		if spec.Containers[i].Name == containerName {
			return spec.Containers[i].Image == image
		}
	}
	return false
}

// Mutate applies fn to all containers and init containers. If fn changes the Kind to
// any value other than "DockerImage", an error is set on that field.
func (m *podSpecV1Mutator) Mutate(fn ImageReferenceMutateFunc) field.ErrorList {
	var errs field.ErrorList
	for i := range m.spec.InitContainers {
		container := &m.spec.InitContainers[i]
		if hasIdenticalPodSpecV1Image(m.oldSpec, container.Name, container.Image) {
			continue
		}
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: container.Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("initContainers").Index(i).Child("image")))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, fieldErrorOrInternal(fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind), m.path.Child("initContainers").Index(i).Child("image")))
			continue
		}
		container.Image = ref.Name
	}
	for i := range m.spec.Containers {
		container := &m.spec.Containers[i]
		if hasIdenticalPodSpecV1Image(m.oldSpec, container.Name, container.Image) {
			continue
		}
		ref := kapi.ObjectReference{Kind: "DockerImage", Name: container.Image}
		if err := fn(&ref); err != nil {
			errs = append(errs, fieldErrorOrInternal(err, m.path.Child("containers").Index(i).Child("image")))
			continue
		}
		if ref.Kind != "DockerImage" {
			errs = append(errs, fieldErrorOrInternal(fmt.Errorf("pod specs may only contain references to docker images, not %q", ref.Kind), m.path.Child("containers").Index(i).Child("image")))
			continue
		}
		container.Image = ref.Name
	}
	return errs
}

func (m *podSpecV1Mutator) GetContainerByName(name string) (ContainerMutator, bool) {
	spec := m.spec
	for i := range spec.InitContainers {
		if name != spec.InitContainers[i].Name {
			continue
		}
		return containerV1Mutator{&spec.InitContainers[i]}, true
	}
	for i := range spec.Containers {
		if name != spec.Containers[i].Name {
			continue
		}
		return containerV1Mutator{&spec.Containers[i]}, true
	}
	return nil, false
}

func (m *podSpecV1Mutator) GetContainerByIndex(init bool, i int) (ContainerMutator, bool) {
	var container *kapiv1.Container
	spec := m.spec
	if init {
		if i < 0 || i >= len(spec.InitContainers) {
			return nil, false
		}
		container = &spec.InitContainers[i]
	} else {
		if i < 0 || i >= len(spec.Containers) {
			return nil, false
		}
		container = &spec.Containers[i]
	}
	return containerV1Mutator{container}, true
}
