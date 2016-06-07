package install

import (
	"fmt"
	"io"
	"io/ioutil"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kubeclient "k8s.io/kubernetes/pkg/client/unversioned"
)

// copyFile copies the source file to the specified target
func copyFile(src, target string) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(target, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// createNamespaceIfNotFound creates the namespace if it does not yet exist.
func createNamespaceIfNotFound(output io.Writer, kubeClient kubeclient.Interface, item *kapi.Namespace) error {
	fmt.Fprintf(output, "-- Check if namespace %s exists ... ", item.Name)
	if _, err := kubeClient.Namespaces().Get(item.Name); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		if _, err = kubeClient.Namespaces().Create(item); err != nil {
			return err
		}
		fmt.Fprintf(output, "\n-- Created namespace\n")
	}
	fmt.Fprintf(output, "   OK\n")
	return nil
}

// createReplicationControllerIfNotFound creates the object if it doesn't exist.
func createReplicationControllerIfNotFound(output io.Writer, client kubeclient.ReplicationControllerInterface, item *kapi.ReplicationController) error {
	fmt.Fprintf(output, "-- Check if replication controller %s exists ... ", item.Name)
	if _, err := client.Get(item.Name); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		if _, err = client.Create(item); err != nil {
			return err
		}
		fmt.Fprintf(output, "\n-- Created replication controller\n")
	}
	fmt.Fprintf(output, "   OK\n")
	return nil
}

// createServiceIfNotFound creates the object if it doesn't exist.
func createServiceIfNotFound(output io.Writer, client kubeclient.ServiceInterface, item *kapi.Service) error {
	fmt.Fprintf(output, "-- Check if service %s exists ... ", item.Name)
	if _, err := client.Get(item.Name); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		if _, err = client.Create(item); err != nil {
			return err
		}
		fmt.Fprintf(output, "\n-- Created service\n")
	}
	fmt.Fprintf(output, "   OK\n")
	return nil
}

// createSecretIfNotFound creates the object if it doesn't exist.
func createSecretIfNotFound(output io.Writer, client kubeclient.SecretsInterface, item *kapi.Secret) error {
	fmt.Fprintf(output, "-- Check if secret %s exists ... ", item.Name)
	if _, err := client.Get(item.Name); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
		if _, err = client.Create(item); err != nil {
			return err
		}
		fmt.Fprintf(output, "\n-- Created secret\n")
	}
	fmt.Fprintf(output, "   OK\n")
	return nil
}
