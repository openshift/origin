package client

import (
	appsv1client "k8s.io/client-go/kubernetes/typed/apps/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	discoveryv1client "k8s.io/client-go/kubernetes/typed/discovery/v1"
	"k8s.io/client-go/tools/clientcmd"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientSet struct {
	corev1client.CoreV1Interface
	appsv1client.AppsV1Interface
	discoveryv1client.DiscoveryV1Interface
	runtimeclient.Client
}

func New(kubeconfigPath string) (*ClientSet, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfigPath)
	if err != nil {
		return nil, err
	}

	clientSet := &ClientSet{}
	clientSet.CoreV1Interface = corev1client.NewForConfigOrDie(config)
	clientSet.AppsV1Interface = appsv1client.NewForConfigOrDie(config)
	clientSet.DiscoveryV1Interface = discoveryv1client.NewForConfigOrDie(config)

	clientSet.Client, err = runtimeclient.New(config, runtimeclient.Options{})
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}
