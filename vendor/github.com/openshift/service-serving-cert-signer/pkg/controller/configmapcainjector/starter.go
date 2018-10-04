package configmapcainjector

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"crypto/x509"
	"encoding/pem"
	"io/ioutil"

	"github.com/golang/glog"
	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"k8s.io/client-go/informers"
)

type ConfigMapCABundleInjectorOptions struct {
	Config *servicecertsignerv1alpha1.ConfigMapCABundleInjectorConfig
}

// These might need adjustment
const (
	InformerResyncInterval = 2 * time.Minute
	ControllerResyncInterval = 20 * time.Minute
)

func (o *ConfigMapCABundleInjectorOptions) RunConfigMapCABundleInjector(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, InformerResyncInterval)

	caPem, err := ioutil.ReadFile(o.Config.CABundleFile)
	if err != nil {
		return err
	}

	// Verify that there is at least one cert in the bundle file
	block, _ := pem.Decode(caPem)
	if block == nil {
		return fmt.Errorf("failed to parse CA bundle file as pem")
	}
	if _, err = x509.ParseCertificate(block.Bytes); err != nil {
		return err
	}
	glog.V(4).Infof("using ca PEM: %s", string(caPem))

	configMapInjectorController := NewConfigMapCABundleInjectionController(
		kubeInformers.Core().V1().ConfigMaps(),
		kubeClient.CoreV1(),
		caPem,
		ControllerResyncInterval,
	)

	kubeInformers.Start(stopCh)

	go configMapInjectorController.Run(1, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
