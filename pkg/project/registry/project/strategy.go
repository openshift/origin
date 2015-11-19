package project

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/project/api/validation"
)

// projectStrategy implements behavior for projects
type projectStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Project
// objects via the REST API.
var Strategy = projectStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is false for projects.
func (projectStrategy) NamespaceScoped() bool {
	return false
}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (projectStrategy) PrepareForCreate(obj runtime.Object) {
	project := obj.(*api.Project)
	hasProjectFinalizer := false
	for i := range project.Spec.Finalizers {
		if project.Spec.Finalizers[i] == api.FinalizerOrigin {
			hasProjectFinalizer = true
			break
		}
	}
	if !hasProjectFinalizer {
		if len(project.Spec.Finalizers) == 0 {
			project.Spec.Finalizers = []kapi.FinalizerName{api.FinalizerOrigin}
		} else {
			project.Spec.Finalizers = append(project.Spec.Finalizers, api.FinalizerOrigin)
		}
	}
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (projectStrategy) PrepareForUpdate(obj, old runtime.Object) {
	newProject := obj.(*api.Project)
	oldProject := old.(*api.Project)
	newProject.Spec.Finalizers = oldProject.Spec.Finalizers
	newProject.Status = oldProject.Status
}

// Validate validates a new project.
func (projectStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateProject(obj.(*api.Project))
}

// AllowCreateOnUpdate is false for project.
func (projectStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (projectStrategy) AllowUnconditionalUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (projectStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateProjectUpdate(obj.(*api.Project), old.(*api.Project))
}
