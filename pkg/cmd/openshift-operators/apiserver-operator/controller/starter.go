package controller

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	apiregistrationclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"

	apiserverclient "github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator/generated/clientset/versioned"
)

type APIServerOperatorStarter struct {
	ClientConfig *rest.Config
}

func (o *APIServerOperatorStarter) Run(stopCh <-chan struct{}) {
	kubeClient, err := kubernetes.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	apiregistrationClient, err := apiregistrationclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}
	apiserverClient, err := apiserverclient.NewForConfig(o.ClientConfig)
	if err != nil {
		panic(err)
	}

	operator := NewAPIServerOperator(
		apiserverClient.ApiserverV1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
		apiregistrationClient.ApiregistrationV1beta1(),
	)

	operator.Run(1, stopCh)
}
