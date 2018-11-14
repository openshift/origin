package starter

import (
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/client-go/rest"
	apiserviceclient "k8s.io/kube-aggregator/pkg/client/clientset_generated/clientset"
	apiserviceinformer "k8s.io/kube-aggregator/pkg/client/informers/externalversions"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/apiservicecabundle/controller"
)

func ToStartFunc(config *servicecertsignerv1alpha1.APIServiceCABundleInjectorConfig) (controllercmd.StartFunc, error) {
	if len(config.CABundleFile) == 0 {
		return nil, fmt.Errorf("no signing cert/key pair provided")
	}

	caBundleContent, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return nil, err
	}

	opts := &apiServiceCABundleInjectorOptions{caBundleContent: caBundleContent}
	return opts.runAPIServiceCABundleInjector, nil
}

type apiServiceCABundleInjectorOptions struct {
	caBundleContent []byte
}

func (o *apiServiceCABundleInjectorOptions) runAPIServiceCABundleInjector(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	apiServiceClient, err := apiserviceclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	apiServiceInformers := apiserviceinformer.NewSharedInformerFactory(apiServiceClient, 2*time.Minute)

	servingCertUpdateController := controller.NewAPIServiceCABundleInjector(
		apiServiceInformers.Apiregistration().V1().APIServices(),
		apiServiceClient.ApiregistrationV1(),
		o.caBundleContent,
	)

	apiServiceInformers.Start(stopCh)

	go servingCertUpdateController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
