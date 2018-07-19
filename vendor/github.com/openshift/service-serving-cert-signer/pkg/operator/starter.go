package operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	scsclient "github.com/openshift/client-go/servicecertsigner/clientset/versioned"
	scsinformers "github.com/openshift/client-go/servicecertsigner/informers/externalversions"
)

func RunOperator(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}
	scsClient, err := scsclient.NewForConfig(clientConfig)
	if err != nil {
		panic(err)
	}

	operatorInformers := scsinformers.NewSharedInformerFactory(scsClient, 10*time.Minute)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, 10*time.Minute, targetNamespaceName, nil)

	operator := NewServiceCertSignerOperator(
		operatorInformers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs(),
		kubeInformersNamespaced,
		scsClient.ServicecertsignerV1alpha1(),
		kubeClient.AppsV1(),
		kubeClient.CoreV1(),
		kubeClient.RbacV1(),
	)

	operatorInformers.Start(stopCh)
	kubeInformersNamespaced.Start(stopCh)

	operator.Run(1, stopCh)
	return fmt.Errorf("stopped")
}
