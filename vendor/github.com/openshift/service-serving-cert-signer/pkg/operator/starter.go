package operator

import (
	"fmt"
	"time"

	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"

	operatorv1alpha1 "github.com/openshift/api/operator/v1alpha1"
	scsclient "github.com/openshift/client-go/servicecertsigner/clientset/versioned"
	scsinformers "github.com/openshift/client-go/servicecertsigner/informers/externalversions"
	"github.com/openshift/library-go/pkg/operator/status"
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
	dynamicClient, err := dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	operatorInformers := scsinformers.NewSharedInformerFactory(scsClient, 10*time.Minute)
	kubeInformersNamespaced := informers.NewFilteredSharedInformerFactory(kubeClient, 10*time.Minute, targetNamespaceName, nil)

	clusterOperatorStatus := status.NewClusterOperatorStatusController(
		"openshift-service-cert-signer",
		"openshift-service-cert-signer",
		dynamicClient,
		&operatorStatusProvider{informers: operatorInformers},
	)

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

	go operator.Run(stopCh)
	go clusterOperatorStatus.Run(1, stopCh)

	<-stopCh
	return fmt.Errorf("stopped")
}

type operatorStatusProvider struct {
	informers scsinformers.SharedInformerFactory
}

func (p *operatorStatusProvider) Informer() cache.SharedIndexInformer {
	return p.informers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs().Informer()
}

func (p *operatorStatusProvider) CurrentStatus() (operatorv1alpha1.OperatorStatus, error) {
	instance, err := p.informers.Servicecertsigner().V1alpha1().ServiceCertSignerOperatorConfigs().Lister().Get("instance")
	if err != nil {
		return operatorv1alpha1.OperatorStatus{}, err
	}
	return instance.Status.OperatorStatus, nil
}
