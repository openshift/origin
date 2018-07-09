package registry_operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	registryclient "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/clientset/versioned"
	registryinformers "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/informers/externalversions"
)

func RunDockerRegistryOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	registryClient, err := registryclient.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}

	operatorInformers := registryinformers.NewSharedInformerFactory(registryClient, 10*time.Minute)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, 10*time.Minute, targetNamespaceName, nil)

	operator := NewDockerRegistryOperator(
		operatorInformers.Dockerregistry().V1alpha1().OpenShiftDockerRegistryConfigs(),
		kubeInformersNamespaced,
		registryClient.DockerregistryV1alpha1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
	)

	operatorInformers.Start(stopCh)
	kubeInformersNamespaced.Start(stopCh)

	operator.Run(1, stopCh)

	return fmt.Errorf("stopped")
}
