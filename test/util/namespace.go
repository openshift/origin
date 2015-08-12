package util

import (
	"fmt"
	"time"

	"github.com/openshift/origin/pkg/cmd/util"
	kapi "k8s.io/kubernetes/pkg/api"
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
