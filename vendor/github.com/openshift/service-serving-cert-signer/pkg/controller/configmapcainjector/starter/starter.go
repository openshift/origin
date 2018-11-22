package starter

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/openshift/service-serving-cert-signer/pkg/controller/configmapcainjector/controller"
)

func ToStartFunc(config *servicecertsignerv1alpha1.ConfigMapCABundleInjectorConfig) (controllercmd.StartFunc, error) {
	if len(config.CABundleFile) == 0 {
		return nil, fmt.Errorf("no ca bundle provided")
	}

	ca, err := ioutil.ReadFile(config.CABundleFile)
	if err != nil {
		return nil, err
	}

	// Verify that there is at least one cert in the bundle file
	block, _ := pem.Decode(ca)
	if block == nil {
		return nil, fmt.Errorf("failed to parse CA bundle file as pem")
	}
	if _, err = x509.ParseCertificate(block.Bytes); err != nil {
		return nil, err
	}

	opts := &configMapCABundleInjectorOptions{ca: string(ca)}
	return opts.runConfigMapCABundleInjector, nil
}

type configMapCABundleInjectorOptions struct {
	ca string
}

func (o *configMapCABundleInjectorOptions) runConfigMapCABundleInjector(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 20*time.Minute)

	configMapInjectorController := controller.NewConfigMapCABundleInjectionController(
		kubeInformers.Core().V1().ConfigMaps(),
		kubeClient.CoreV1(),
		o.ca,
	)

	kubeInformers.Start(stopCh)

	go configMapInjectorController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
