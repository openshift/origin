package controller

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	controllerclient "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned"
)

type ControllerOperatorStarter struct {
	ClientConfig *rest.Config
}

func (o *ControllerOperatorStarter) Run(stopCh <-chan struct{}) {
	kubeClient, err := kubernetes.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	controllerClient, err := controllerclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}

	operator := NewControllerOperator(
		controllerClient.ControllerV1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
	)

	operator.Run(1, stopCh)
}
