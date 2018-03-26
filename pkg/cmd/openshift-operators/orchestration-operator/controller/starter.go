package controller

import (
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	apiserverclient "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned"
	controllerclient "github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator/generated/clientset/versioned"
	orchestrationclient "github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator/generated/clientset/versioned"
	webconsoleclient "github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator/generated/clientset/versioned"
)

type OrchestrationOperatorStarter struct {
	ClientConfig *rest.Config
}

func (o *OrchestrationOperatorStarter) Run(stopCh <-chan struct{}) {
	kubeClient, err := kubernetes.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	orchestrationClient, err := orchestrationclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	apiserverClient, err := apiserverclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	controllerClient, err := controllerclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	webconsoleClient, err := webconsoleclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	apiextensionsClient, err := apiextensionsclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}

	operator := NewOrchestrationOperator(
		orchestrationClient.OrchestrationV1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
		kubeClient.RbacV1(),
		apiextensionsClient.ApiextensionsV1beta1(),
		controllerClient.ControllerV1(),
		apiserverClient.ApiserverV1(),
		webconsoleClient.WebconsoleV1(),
	)

	operator.Run(1, stopCh)
}
