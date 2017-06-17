package controller

import (
	"fmt"
	"time"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/server/crypto"
	servingcertcontroller "github.com/openshift/origin/pkg/service/controller/servingcert"
)

type ServiceServingCertsControllerOptions struct {
	Signer *configapi.CertInfo
}

func (c *ServiceServingCertsControllerOptions) RunController(ctx ControllerContext) (bool, error) {
	if c.Signer == nil || len(c.Signer.CertFile) == 0 || len(c.Signer.KeyFile) == 0 {
		return false, nil
	}
	ca, err := crypto.GetCA(c.Signer.CertFile, c.Signer.KeyFile, "")
	if err != nil {
		return true, fmt.Errorf("service serving cert controller: %v", err)
	}

	servingCertController := servingcertcontroller.NewServiceServingCertController(
		ctx.InternalKubeInformers.Core().InternalVersion().Services(),
		ctx.InternalKubeInformers.Core().InternalVersion().Secrets(),
		ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ca,
		"cluster.local",
		2*time.Minute,
	)
	servingCertUpdateController := servingcertcontroller.NewServiceServingCertUpdateController(
		ctx.InternalKubeInformers.Core().InternalVersion().Services(),
		ctx.InternalKubeInformers.Core().InternalVersion().Secrets(),
		ctx.ClientBuilder.KubeInternalClientOrDie(bootstrappolicy.InfraServiceServingCertServiceAccountName).Core(),
		ca,
		"cluster.local",
		20*time.Minute,
	)

	go servingCertController.Run(1, ctx.Stop)
	go servingCertUpdateController.Run(5, ctx.Stop)

	return true, nil
}
