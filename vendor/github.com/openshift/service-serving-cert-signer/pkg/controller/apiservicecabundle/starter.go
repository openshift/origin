package apiservicecabundle

import (
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/client-go/rest"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
)

type APIServiceCABundleInjectorOptions struct {
	Config *servicecertsignerv1alpha1.APIServiceCABundleInjectorConfig
}

func (o *APIServiceCABundleInjectorOptions) RunAPIServiceCABundleInjector(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	apiServiceClient, err := apiserviceclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	apiServiceInformers := apiserviceinformer.NewSharedInformerFactory(apiServiceClient, 2*time.Minute)

	caBundleFile := o.Config.CABundleFile
	if len(caBundleFile) == 0 {
		return fmt.Errorf("no signing cert/key pair provided")
	}
	caBundleContent, err := ioutil.ReadFile(caBundleFile)
	if err != nil {
		return err
	}

	servingCertUpdateController := NewAPIServiceCABundleInjector(
		apiServiceInformers.Apiregistration().V1().APIServices(),
		apiServiceClient.ApiregistrationV1(),
		caBundleContent,
	)

	apiServiceInformers.Start(stopCh)

	go servingCertUpdateController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
