package util

import (
	"k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	projectapiv1 "github.com/openshift/api/project/v1"
	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

// Associated returns true if the spec.finalizers contains the origin finalizer
func Associated(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if projectapi.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return true
		}
	}
	return false
}

// Associate adds the origin finalizer to spec.finalizers if its not there already
func Associate(kubeClient internalclientset.Interface, namespace *kapi.Namespace) (*kapi.Namespace, error) {
	if Associated(namespace) {
		return namespace, nil
	}
	return finalizeInternal(kubeClient, namespace, true)
}

// Finalized returns true if the spec.finalizers does not contain the origin finalizer
func Finalized(namespace *v1.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if projectapiv1.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return false
		}
	}
	return true
}

// Finalize will remove the origin finalizer from the namespace
func Finalize(kubeClient clientset.Interface, namespace *v1.Namespace) (result *v1.Namespace, err error) {
	if Finalized(namespace) {
		return namespace, nil
	}

	// there is a potential for a resource conflict with base kubernetes finalizer
	// as a result, we handle resource conflicts in case multiple finalizers try
	// to finalize at same time
	for {
		result, err = finalizeInternalV1(kubeClient, namespace, false)
		if err == nil {
			return result, nil
		}

		if !kerrors.IsConflict(err) {
			return nil, err
		}

		namespace, err = kubeClient.Core().Namespaces().Get(namespace.Name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
	}
}

// finalizeInternal will update the namespace finalizer list to either have or not have origin finalizer
// TODO: remove me
func finalizeInternal(kubeClient internalclientset.Interface, namespace *kapi.Namespace, withOrigin bool) (*kapi.Namespace, error) {
	namespaceFinalize := kapi.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec

	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(projectapi.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(projectapi.FinalizerOrigin))
	}

	namespaceFinalize.Spec.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, kapi.FinalizerName(value))
	}
	return kubeClient.Core().Namespaces().Finalize(&namespaceFinalize)
}

// finalizeInternalV1 will update the namespace finalizer list to either have or not have origin finalizer
func finalizeInternalV1(kubeClient clientset.Interface, namespace *v1.Namespace, withOrigin bool) (*v1.Namespace, error) {
	namespaceFinalize := v1.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec

	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(projectapiv1.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(projectapiv1.FinalizerOrigin))
	}

	namespaceFinalize.Spec.Finalizers = make([]v1.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, v1.FinalizerName(value))
	}
	return kubeClient.Core().Namespaces().Finalize(&namespaceFinalize)
}

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
