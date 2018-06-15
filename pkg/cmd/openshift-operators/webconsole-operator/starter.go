package webconsole_operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	webconsoleclient "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/clientset/versioned"
	webconsoleinformers "github.com/openshift/origin/pkg/cmd/openshift-operators/generated/informers/externalversions"
)

func RunWebConsoleOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	webconsoleClient, err := webconsoleclient.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}

	operatorInformers := webconsoleinformers.NewSharedInformerFactory(webconsoleClient, 10*time.Minute)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, 10*time.Minute, targetNamespaceName, nil)

	operator := NewWebConsoleOperator(
		operatorInformers.Webconsole().V1alpha1().OpenShiftWebConsoleConfigs(),
		kubeInformersNamespaced,
		webconsoleClient.WebconsoleV1alpha1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
	)

	operatorInformers.Start(stopCh)
	kubeInformersNamespaced.Start(stopCh)

	operator.Run(1, stopCh)

	return fmt.Errorf("stopped")
}
