package util

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/project/api"
)

// Associated returns true if the spec.finalizers contains the origin finalizer
func Associated(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if api.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return true
		}
	}
	return false
}

// Associate adds the origin finalizer to spec.finalizers if its not there already
func Associate(kubeClient clientset.Interface, namespace *kapi.Namespace) (*kapi.Namespace, error) {
	if Associated(namespace) {
		return namespace, nil
	}
	return finalizeInternal(kubeClient, namespace, true)
}

// Finalized returns true if the spec.finalizers does not contain the origin finalizer
func Finalized(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if api.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return false
		}
	}
	return true
}

// Finalize will remove the origin finalizer from the namespace
func Finalize(kubeClient clientset.Interface, namespace *kapi.Namespace) (result *kapi.Namespace, err error) {
	if Finalized(namespace) {
		return namespace, nil
	}

	// there is a potential for a resource conflict with base kubernetes finalizer
	// as a result, we handle resource conflicts in case multiple finalizers try
	// to finalize at same time
	for {
		result, err = finalizeInternal(kubeClient, namespace, false)
		if err == nil {
			return result, nil
		}

		if !kerrors.IsConflict(err) {
			return nil, err
		}

		namespace, err = kubeClient.Core().Namespaces().Get(namespace.Name)
		if err != nil {
			return nil, err
		}
	}
}

// finalizeInternal will update the namespace finalizer list to either have or not have origin finalizer
func finalizeInternal(kubeClient clientset.Interface, namespace *kapi.Namespace, withOrigin bool) (*kapi.Namespace, error) {
	namespaceFinalize := kapi.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec

	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(api.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(api.FinalizerOrigin))
	}

	namespaceFinalize.Spec.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, kapi.FinalizerName(value))
	}
	return kubeClient.Core().Namespaces().Finalize(&namespaceFinalize)
}

// ConvertNamespace transforms a Namespace into a Project
func ConvertNamespace(namespace *kapi.Namespace) *api.Project {
	return &api.Project{
		ObjectMeta: namespace.ObjectMeta,
		Spec: api.ProjectSpec{
			Finalizers: namespace.Spec.Finalizers,
		},
		Status: api.ProjectStatus{
			Phase: namespace.Status.Phase,
		},
	}
}

// convertProject transforms a Project into a Namespace
func ConvertProject(project *api.Project) *kapi.Namespace {
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
	namespace.Annotations[api.ProjectDisplayName] = project.Annotations[api.ProjectDisplayName]
	return namespace
}

// ConvertNamespaceList transforms a NamespaceList into a ProjectList
func ConvertNamespaceList(namespaceList *kapi.NamespaceList) *api.ProjectList {
	projects := &api.ProjectList{}
	for _, n := range namespaceList.Items {
		projects.Items = append(projects.Items, *ConvertNamespace(&n))
	}
	return projects
}
