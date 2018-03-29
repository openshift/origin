package controller

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	webconsoleclient "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned"
)

type WebConsoleOperatorStarter struct {
	ClientConfig *rest.Config
}

func (o *WebConsoleOperatorStarter) Run(stopCh <-chan struct{}) {
	kubeClient, err := kubernetes.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	webconsoleClient, err := webconsoleclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}

	operator := NewWebConsoleOperator(
		webconsoleClient.WebconsoleV1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
	)

	operator.Run(1, stopCh)
}
