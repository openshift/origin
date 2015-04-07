package controller

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/project/api"
)

// NamespaceController is responsible for participating in Kubernetes Namespace termination
// Use the NamespaceControllerFactory to create this controller.
type NamespaceController struct {
	// Client is an OpenShift client.
	Client osclient.Interface
	// KubeClient is a Kubernetes client.
	KubeClient kclient.Interface
}

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string { return "fatal error handling namespace: " + string(e) }

// Handle processes a namespace and deletes content in origin if its terminating
func (c *NamespaceController) Handle(namespace *kapi.Namespace) (err error) {
	// if namespace is not terminating, ignore it
	if namespace.Status.Phase != kapi.NamespaceTerminating {
		return nil
	}

	// if we already processed this namespace, ignore it
	if finalized(namespace) {
		return nil
	}

	// there may still be content for us to remove
	err = deleteAllContent(c.Client, namespace.Name)
	if err != nil {
		return err
	}

	// we have removed content, so mark it finalized by us
	err = finalize(c.KubeClient, namespace)
	if err != nil {
		return err
	}

	return nil
}

// finalized returns true if the spec.finalizers does not contain the project finalizer
func finalized(namespace *kapi.Namespace) bool {
	for i := range namespace.Spec.Finalizers {
		if api.FinalizerProject == namespace.Spec.Finalizers[i] {
			return false
		}
	}
	return true
}

// finalize will finalize the namespace for kubernetes
func finalize(kubeClient kclient.Interface, namespace *kapi.Namespace) error {
	namespaceFinalize := kapi.Namespace{}
	namespaceFinalize.ObjectMeta = namespace.ObjectMeta
	namespaceFinalize.Spec = namespace.Spec
	finalizerSet := util.NewStringSet()
	for i := range namespace.Spec.Finalizers {
		if namespace.Spec.Finalizers[i] != api.FinalizerProject {
			finalizerSet.Insert(string(namespace.Spec.Finalizers[i]))
		}
	}
	namespaceFinalize.Spec.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		namespaceFinalize.Spec.Finalizers = append(namespaceFinalize.Spec.Finalizers, kapi.FinalizerName(value))
	}
	_, err := kubeClient.Namespaces().Finalize(&namespaceFinalize)
	return err
}

// deleteAllContent will purge all content in openshift in the specified namespace
func deleteAllContent(client osclient.Interface, namespace string) (err error) {
	err = deleteBuildConfigs(client, namespace)
	if err != nil {
		return err
	}
	err = deleteBuilds(client, namespace)
	if err != nil {
		return err
	}
	err = deleteDeploymentConfigs(client, namespace)
	if err != nil {
		return err
	}
	err = deleteDeployments(client, namespace)
	if err != nil {
		return err
	}
	err = deleteImageStreams(client, namespace)
	if err != nil {
		return err
	}
	err = deletePolicies(client, namespace)
	if err != nil {
		return err
	}
	err = deletePolicyBindings(client, namespace)
	if err != nil {
		return err
	}
	err = deleteRoleBindings(client, namespace)
	if err != nil {
		return err
	}
	err = deleteRoles(client, namespace)
	if err != nil {
		return err
	}
	err = deleteRoutes(client, namespace)
	if err != nil {
		return err
	}
	return nil
}

func deleteRoutes(client osclient.Interface, ns string) error {
	items, err := client.Routes(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.Routes(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteRoles(client osclient.Interface, ns string) error {
	items, err := client.Roles(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.Roles(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteRoleBindings(client osclient.Interface, ns string) error {
	items, err := client.RoleBindings(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.RoleBindings(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deletePolicyBindings(client osclient.Interface, ns string) error {
	items, err := client.PolicyBindings(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.PolicyBindings(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deletePolicies(client osclient.Interface, ns string) error {
	items, err := client.Policies(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.Policies(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteImageStreams(client osclient.Interface, ns string) error {
	items, err := client.ImageStreams(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.ImageStreams(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteDeployments(client osclient.Interface, ns string) error {
	items, err := client.Deployments(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.Deployments(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteDeploymentConfigs(client osclient.Interface, ns string) error {
	items, err := client.DeploymentConfigs(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.DeploymentConfigs(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteBuilds(client osclient.Interface, ns string) error {
	items, err := client.Builds(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.Builds(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}

func deleteBuildConfigs(client osclient.Interface, ns string) error {
	items, err := client.BuildConfigs(ns).List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}
	for i := range items.Items {
		err := client.BuildConfigs(ns).Delete(items.Items[i].Name)
		if err != nil {
			return err
		}
	}
	return nil
}
