package util

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	watchtools "k8s.io/client-go/tools/watch"

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
	_, err = clusterAdminKubeClient.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: name},
	})
	return err
}

func DeleteAndWaitForNamespaceTermination(c kubernetes.Interface, name string) error {
	w, err := c.CoreV1().Namespaces().Watch(metav1.ListOptions{})
	if err != nil {
		return err
	}
	defer w.Stop()

	if err := c.CoreV1().Namespaces().Delete(name, nil); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, err = watchtools.UntilWithoutRetry(ctx, w, func(event watch.Event) (bool, error) {
		if event.Type != watch.Deleted {
			return false, nil
		}
		namespace, ok := event.Object.(*corev1.Namespace)
		if !ok {
			return false, nil
		}
		return namespace.Name == name, nil
	})
	return err
}
