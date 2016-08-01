package util

import (
	"fmt"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/cmd/util"
)

// Namespace returns the test namespace. The default namespace is set to
// 'integration-test'. You can override it by setting the 'OS_TEST_NAMESPACE'
// environment variable
func Namespace() string {
	return util.Env("OS_TEST_NAMESPACE", "integration")
}

// RandomNamespace provides random Kubernetes namespace name based on the UNIX
// timestamp. Optionally you can set the prefix.
func RandomNamespace(prefix string) string {
	return prefix + string([]byte(fmt.Sprintf("%d", time.Now().UnixNano()))[3:12])
}

// CreateNamespace creates a namespace with the specified name using the provided kubeconfig
// DO NOT USE, use create project instead
func CreateNamespace(clusterAdminKubeConfig, name string) (err error) {
	clusterAdminKubeClient, err := GetClusterAdminKubeClient(clusterAdminKubeConfig)
	if err != nil {
		return err
	}
	_, err = clusterAdminKubeClient.Namespaces().Create(&kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{Name: name},
	})
	return err
}

func DeleteAndWaitForNamespaceTermination(c *kclient.Client, name string) error {
	w, err := c.Namespaces().Watch(kapi.ListOptions{})
	if err != nil {
		return err
	}
	if err := c.Namespaces().Delete(name); err != nil {
		return err
	}
	_, err = watch.Until(30*time.Second, w, func(event watch.Event) (bool, error) {
		if event.Type != watch.Deleted {
			return false, nil
		}
		namespace, ok := event.Object.(*kapi.Namespace)
		if !ok {
			return false, nil
		}
		return namespace.Name == name, nil
	})
	return err
}
