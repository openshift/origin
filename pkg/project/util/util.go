package util

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client"
	"k8s.io/kubernetes/pkg/util"

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
func Associate(kubeClient kclient.Interface, namespace *kapi.Namespace) (*kapi.Namespace, error) {
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
func Finalize(kubeClient kclient.Interface, namespace *kapi.Namespace) (*kapi.Namespace, error) {
	if Finalized(namespace) {
		return namespace, nil
	}
	return finalizeInternal(kubeClient, namespace, false)
}

// finalizeInternal will update the namespace finalizer list to either have or not have origin finalizer
func finalizeInternal(kubeClient kclient.Interface, namespace *kapi.Namespace, withOrigin bool) (*kapi.Namespace, error) {
	namespaceFinalize := kapi.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec

	finalizerSet := util.NewStringSet()
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
	return kubeClient.Namespaces().Finalize(&namespaceFinalize)
}
