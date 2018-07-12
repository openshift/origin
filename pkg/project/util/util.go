package util

import (
	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

// ConvertNamespace transforms a Namespace into a Project
func ConvertNamespace(namespace *kapi.Namespace) *projectapi.Project {
	return &projectapi.Project{
		ObjectMeta: namespace.ObjectMeta,
		Spec: projectapi.ProjectSpec{
			Finalizers: namespace.Spec.Finalizers,
		},
		Status: projectapi.ProjectStatus{
			Phase: namespace.Status.Phase,
		},
	}
}

// convertProject transforms a Project into a Namespace
func ConvertProject(project *projectapi.Project) *kapi.Namespace {
	namespace := &kapi.Namespace{
		ObjectMeta: project.ObjectMeta,
		Spec: kapi.NamespaceSpec{
			Finalizers: project.Spec.Finalizers,
		},
		Status: kapi.NamespaceStatus{
			Phase: project.Status.Phase,
		},
	}
	if namespace.Annotations == nil {
		namespace.Annotations = map[string]string{}
	}
	namespace.Annotations[oapi.OpenShiftDisplayName] = project.Annotations[oapi.OpenShiftDisplayName]
	return namespace
}

// ConvertNamespaceList transforms a NamespaceList into a ProjectList
func ConvertNamespaceList(namespaceList *kapi.NamespaceList) *projectapi.ProjectList {
	projects := &projectapi.ProjectList{}
	for _, n := range namespaceList.Items {
		projects.Items = append(projects.Items, *ConvertNamespace(&n))
	}
	return projects
}
