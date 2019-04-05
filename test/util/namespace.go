package util

import (
	"github.com/openshift/origin/pkg/cmd/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Namespace returns the test namespace. The default namespace is set to
// 'integration-test'. You can override it by setting the 'OS_TEST_NAMESPACE'
// environment variable
func Namespace() string {
	return util.Env("OS_TEST_NAMESPACE", "integration")
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
