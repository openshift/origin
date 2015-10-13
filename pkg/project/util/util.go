package util

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/util/sets"

	oapi "github.com/openshift/origin/pkg/api"
)

// Associated returns true if the spec.finalizers contains the origin finalizer
func Associated(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if oapi.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return true
		}
	}
	return false
}

// Associate adds the origin finalizer to spec.finalizers if its not there already
func Associate(kubeClient kclient.Interface, namespace *kapi.Namespace) (*kapi.Namespace, error) {
	if Associated(namespace) {
		return namespace, nil
	}
	return finalizeInternal(kubeClient, namespace, true)
}

// Finalized returns true if the spec.finalizers does not contain the origin finalizer
func Finalized(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if oapi.FinalizerOrigin == namespace.Spec.Finalizers[i] {
			return false
		}
	}
	return true
}

// Finalize will remove the origin finalizer from the namespace
func Finalize(kubeClient kclient.Interface, namespace *kapi.Namespace) (result *kapi.Namespace, err error) {
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

		namespace, err = kubeClient.Namespaces().Get(namespace.Name)
		if err != nil {
			return nil, err
		}
	}
}

// finalizeInternal will update the namespace finalizer list to either have or not have origin finalizer
func finalizeInternal(kubeClient kclient.Interface, namespace *kapi.Namespace, withOrigin bool) (*kapi.Namespace, error) {
	namespaceFinalize := kapi.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec

	finalizerSet := sets.NewString()
	for i := range namespace.Spec.Finalizers {
		finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(oapi.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(oapi.FinalizerOrigin))
	}

	namespaceFinalize.Spec.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, kapi.FinalizerName(value))
	}
	return kubeClient.Namespaces().Finalize(&namespaceFinalize)
}
