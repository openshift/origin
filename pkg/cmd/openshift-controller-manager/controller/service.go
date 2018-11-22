package controller

import (
	"fmt"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	servingcertcontroller "github.com/openshift/service-serving-cert-signer/pkg/controller/servingcert/controller"
)

func RunServiceServingCertsController(ctx *ControllerContext) (bool, error) {
	signer := ctx.OpenshiftControllerConfig.ServiceServingCert.Signer
	if signer == nil || len(signer.CertFile) == 0 || len(signer.KeyFile) == 0 {
		return false, nil
	}
	ca, err := crypto.GetCA(signer.CertFile, signer.KeyFile, "")
	if err != nil {
		return true, fmt.Errorf("service serving cert controller: %v", err)
	}

	servingCertController := servingcertcontroller.NewServiceServingCertController(
		ctx.KubernetesInformers.Core().V1().Services(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ca,
		"cluster.local",
	)
	servingCertUpdateController := servingcertcontroller.NewServiceServingCertUpdateController(
		ctx.KubernetesInformers.Core().V1().Services(),
		ctx.KubernetesInformers.Core().V1().Secrets(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ca,
		"cluster.local",
	)

	go servingCertController.Run(1, ctx.Stop)
	go servingCertUpdateController.Run(5, ctx.Stop)

	return true, nil
}
