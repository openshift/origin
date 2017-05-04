package controller

import (
	kapi "k8s.io/kubernetes/pkg/api"

	projectutil "github.com/openshift/origin/pkg/project/util"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
)

// NamespaceController is responsible for participating in Kubernetes Namespace termination
// Use the NamespaceControllerFactory to create this controller.
type NamespaceController struct {
	// KubeClient is a Kubernetes client.
	KubeClient internalclientset.Interface
}

// Handle processes a namespace and deletes content in origin if its terminating
func (c *NamespaceController) Handle(namespace *kapi.Namespace) (err error) {
	// if namespace is not terminating, ignore it
	if namespace.Status.Phase != kapi.NamespaceTerminating {
		return nil
	}

	// if we already processed this namespace, ignore it
	if projectutil.Finalized(namespace) {
		return nil
	}

	// we have removed content, so mark it finalized by us
	_, err = projectutil.Finalize(c.KubeClient, namespace)
	if err != nil {
		return err
	}

	return nil
}
