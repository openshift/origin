package openshift_service_serving_cert_signer

import (
	"fmt"
	"time"

	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/cmd/openshift-service-serving-cert-signer/apis/serviceservingcertsigner/v1alpha1"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	servingcertcontroller "github.com/openshift/origin/pkg/service/controller/servingcert"
)

func RunOpenShiftSSCS(config *v1alpha1.ServiceServingCertSignerConfig, clientConfig *rest.Config, stop <-chan struct{}) error {
	kubeExternal, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	defaultInformerResyncPeriod := 10 * time.Minute
	kubeInformers := informers.NewSharedInformerFactory(kubeExternal, defaultInformerResyncPeriod)

	ca, err := crypto.GetCA(config.Signer.CertFile, config.Signer.KeyFile, "")
	if err != nil {
		return fmt.Errorf("service serving cert controller: %v", err)
	}

	ctx := &controllerContext{
		kubeClient:    kubeExternal,
		kubeInformers: kubeInformers,
		ca:            ca,
	}

	// TODO serve endpoints
	//glog.Infof("Starting controllers on %s (%s)", config.ServingInfo.BindAddress, version.Get().String())
	//if err := origincontrollers.RunControllerServer(*config.ServingInfo, kubeExternal); err != nil {
	//	return err
	//}

	ctx.RunServiceServingCertsController(stop)

	return nil
}

type controllerContext struct {
	kubeClient    kubernetes.Interface
	kubeInformers informers.SharedInformerFactory
	ca            *crypto.CA
}

func (ctx *controllerContext) RunServiceServingCertsController(stop <-chan struct{}) {
	servingCertController := servingcertcontroller.NewServiceServingCertController(
		ctx.kubeInformers.Core().V1().Services(),
		ctx.kubeInformers.Core().V1().Secrets(),
		ctx.kubeClient.CoreV1(),
		ctx.kubeClient.CoreV1(),
		ctx.ca,
		"cluster.local",
		2*time.Minute,
	)
	servingCertUpdateController := servingcertcontroller.NewServiceServingCertUpdateController(
		ctx.kubeInformers.Core().V1().Services(),
		ctx.kubeInformers.Core().V1().Secrets(),
		ctx.kubeClient.CoreV1(),
		ctx.ca,
		"cluster.local",
		20*time.Minute,
	)

	go servingCertController.Run(1, stop)
	go servingCertUpdateController.Run(5, stop)

}
