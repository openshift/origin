package validation

import (
	"reflect"
	"strings"

	"k8s.io/apimachinery/pkg/api/validation/path"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	"github.com/openshift/origin/pkg/util/labelselector"
)

func ValidateProjectName(name string, prefix bool) []string {
	if reasons := path.ValidatePathSegmentName(name, prefix); len(reasons) != 0 {
		return reasons
	}

	if len(name) < 2 {
		return []string{"must be at least 2 characters long"}
	}

	if reasons := validation.ValidateNamespaceName(name, false); len(reasons) != 0 {
		return reasons
	}

	return nil
}

// ValidateProject tests required fields for a Project.
// This should only be called when creating a project (not on update),
// since its name validation is more restrictive than default namespace name validation
func ValidateProject(project *projectapi.Project) field.ErrorList {
	result := validation.ValidateObjectMeta(&project.ObjectMeta, false, ValidateProjectName, field.NewPath("metadata"))

	if !validateNoNewLineOrTab(project.Annotations[oapi.OpenShiftDisplayName]) {
		result = append(result, field.Invalid(field.NewPath("metadata", "annotations").Key(oapi.OpenShiftDisplayName),
			project.Annotations[oapi.OpenShiftDisplayName], "may not contain a new line or tab"))
	}
	result = append(result, validateNodeSelector(project)...)
	return result
}

// validateNoNewLineOrTab ensures a string has no new-line or tab
func validateNoNewLineOrTab(s string) bool {
	return !(strings.Contains(s, "\n") || strings.Contains(s, "\t"))
}

// ValidateProjectUpdate tests to make sure a project update can be applied.  Modifies newProject with immutable fields.
func ValidateProjectUpdate(newProject *projectapi.Project, oldProject *projectapi.Project) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&newProject.ObjectMeta, &oldProject.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateProject(newProject)...)

	if !reflect.DeepEqual(newProject.Spec.Finalizers, oldProject.Spec.Finalizers) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("spec", "finalizers"), oldProject.Spec.Finalizers, "field is immutable"))
	}
	if !reflect.DeepEqual(newProject.Status, oldProject.Status) {
		allErrs = append(allErrs, field.Invalid(field.NewPath("status"), oldProject.Spec.Finalizers, "field is immutable"))
	}

	// TODO this restriction exists because our authorizer/admission cannot properly express and restrict mutation on the field level.
	for name, value := range newProject.Annotations {
		if name == oapi.OpenShiftDisplayName || name == oapi.OpenShiftDescription {
			continue
		}

		if value != oldProject.Annotations[name] {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "annotations").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}
	// check for deletions
	for name, value := range oldProject.Annotations {
		if name == oapi.OpenShiftDisplayName || name == oapi.OpenShiftDescription {
			continue
		}
		if _, inNew := newProject.Annotations[name]; !inNew {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "annotations").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}

	for name, value := range newProject.Labels {
		if value != oldProject.Labels[name] {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "labels").Key(name), value, "field is immutable, , try updating the namespace"))
		}
	}
	for name, value := range oldProject.Labels {
		if _, inNew := newProject.Labels[name]; !inNew {
			allErrs = append(allErrs, field.Invalid(field.NewPath("metadata", "labels").Key(name), value, "field is immutable, try updating the namespace"))
		}
	}

	return allErrs
}

func ValidateProjectRequest(request *projectapi.ProjectRequest) field.ErrorList {
	project := &projectapi.Project{}
	project.ObjectMeta = request.ObjectMeta

	return ValidateProject(project)
}

func validateNodeSelector(p *projectapi.Project) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(p.Annotations) > 0 {
		if selector, ok := p.Annotations[projectapi.ProjectNodeSelector]; ok {
			if _, err := labelselector.Parse(selector); err != nil {
				allErrs = append(allErrs, field.Invalid(field.NewPath("nodeSelector"),
					p.Annotations[projectapi.ProjectNodeSelector], "must be a valid label selector"))
			}
		}
	}
	return allErrs
}
