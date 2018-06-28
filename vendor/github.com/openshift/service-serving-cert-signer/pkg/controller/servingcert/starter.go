package servingcert

import (
	"fmt"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	servicecertsignerv1alpha1 "github.com/openshift/api/servicecertsigner/v1alpha1"
	"github.com/openshift/library-go/pkg/crypto"
	"k8s.io/client-go/informers"
)

type ServingCertOptions struct {
	Config *servicecertsignerv1alpha1.ServiceServingCertSignerConfig
}

func (o *ServingCertOptions) RunServingCert(clientConfig *rest.Config, stopCh <-chan struct{}) error {
	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	kubeInformers := informers.NewSharedInformerFactory(kubeClient, 2*time.Minute)

	signer := o.Config.Signer
	if len(signer.CertFile) == 0 || len(signer.KeyFile) == 0 {
		return fmt.Errorf("no signing cert/key pair provided")
	}
	ca, err := crypto.GetCA(signer.CertFile, signer.KeyFile, "")
	if err != nil {
		return err
	}

	servingCertController := NewServiceServingCertController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
		2*time.Minute,
	)
	servingCertUpdateController := NewServiceServingCertUpdateController(
		kubeInformers.Core().V1().Services(),
		kubeInformers.Core().V1().Secrets(),
		kubeClient.CoreV1(),
		ca,
		// TODO this needs to be configurable
		"cluster.local",
		20*time.Minute,
	)

	kubeInformers.Start(stopCh)

	go servingCertController.Run(1, stopCh)
	go servingCertUpdateController.Run(5, stopCh)

	<-stopCh

	return fmt.Errorf("stopped")
}
