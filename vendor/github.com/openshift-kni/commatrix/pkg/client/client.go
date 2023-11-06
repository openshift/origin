package client

import (
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	runtimeclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type ClientSet struct {
	runtimeclient.Client
	corev1client.CoreV1Interface
	Config *rest.Config
}

func New() (*ClientSet, error) {
	var err error

	restConfig := ctrl.GetConfigOrDie()
	clientSet := &ClientSet{}

	clientSet.CoreV1Interface = corev1client.NewForConfigOrDie(restConfig)
	clientSet.Client, err = runtimeclient.New(restConfig, runtimeclient.Options{})
	clientSet.Config = restConfig
	if err != nil {
		return nil, err
	}

	return clientSet, nil
}
