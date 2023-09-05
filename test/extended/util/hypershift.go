package util

import (
	"fmt"
	"os"
	"sync"
)

const (
	hypershiftManagementClusterKubeconfigEnvVar = "HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG"
	hypershiftManagementClusterNamespaceEnvVar  = "HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE"
)

var (
	hypershiftManagementClusterKubeconfig string
	hypershiftManagementClusterNamespace  string
	hypershiftMutex                       sync.Mutex
)

// GetHypershiftManagementClusterConfigAndNamespace retrieves the management cluster
// kubeconfig and namespace, or an error in case of a failure
func GetHypershiftManagementClusterConfigAndNamespace() (string, string, error) {
	hypershiftMutex.Lock()
	defer hypershiftMutex.Unlock()

	if hypershiftManagementClusterKubeconfig == "" && hypershiftManagementClusterNamespace != "" {
		return hypershiftManagementClusterKubeconfig, hypershiftManagementClusterNamespace, nil
	}

	kubeconfig, namespace := os.Getenv(hypershiftManagementClusterKubeconfigEnvVar), os.Getenv(hypershiftManagementClusterNamespaceEnvVar)
	if kubeconfig == "" || namespace == "" {
		return "", "", fmt.Errorf("both the %s and the %s env var must be set", hypershiftManagementClusterKubeconfigEnvVar, hypershiftManagementClusterNamespaceEnvVar)
	}

	hypershiftManagementClusterKubeconfig = kubeconfig
	hypershiftManagementClusterNamespace = namespace

	return hypershiftManagementClusterKubeconfig, hypershiftManagementClusterNamespace, nil
}
