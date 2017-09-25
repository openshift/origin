package controller

import (
	"github.com/golang/glog"

	kapiv1 "k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/controller"
	sacontroller "k8s.io/kubernetes/pkg/controller/serviceaccount"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	serviceaccountcontrollers "github.com/openshift/origin/pkg/serviceaccounts/controllers"
)

type ServiceAccountControllerOptions struct {
	ManagedNames []string
}

func (c *ServiceAccountControllerOptions) RunController(ctx ControllerContext) (bool, error) {
	if len(c.ManagedNames) == 0 {
		glog.Infof("Skipped starting Service Account Manager, no managed names specified")
		return false, nil
	}

	options := sacontroller.DefaultServiceAccountsControllerOptions()
	options.ServiceAccounts = []kapiv1.ServiceAccount{}

	for _, saName := range c.ManagedNames {
		// the upstream controller does this one, so we don't have to
		if saName == "default" {
			continue
		}
		sa := kapiv1.ServiceAccount{}
		sa.Name = saName

		options.ServiceAccounts = append(options.ServiceAccounts, sa)
	}

	go sacontroller.NewServiceAccountsController(
		ctx.ExternalKubeInformers.Core().V1().ServiceAccounts(),
		ctx.ExternalKubeInformers.Core().V1().Namespaces(),
		ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceAccountControllerServiceAccountName),
		options).Run(3, ctx.Stop)

	return true, nil
}

type ServiceAccountTokenControllerOptions struct {
	RootCA           []byte
	ServiceServingCA []byte
	PrivateKey       interface{}

	RootClientBuilder controller.SimpleControllerClientBuilder
}

func (c *ServiceAccountTokenControllerOptions) RunController(ctx ControllerContext) (bool, error) {
	if c.PrivateKey == nil {
		glog.Infof("Skipped starting Service Account Token Manager, no private key specified")
		return false, nil
	}

	go sacontroller.NewTokensController(
		ctx.ExternalKubeInformers.Core().V1().ServiceAccounts(),
		ctx.ExternalKubeInformers.Core().V1().Secrets(),
		c.RootClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceAccountTokensControllerServiceAccountName),
		sacontroller.TokensControllerOptions{
			TokenGenerator:   serviceaccount.JWTTokenGenerator(c.PrivateKey),
			RootCA:           c.RootCA,
			ServiceServingCA: c.ServiceServingCA,
		},
	).Run(int(ctx.OpenshiftControllerOptions.ServiceAccountTokenOptions.ConcurrentSyncs), ctx.Stop)
	return true, nil
}

func RunServiceAccountPullSecretsController(ctx ControllerContext) (bool, error) {
	kc := ctx.ClientBuilder.ClientOrDie(bootstrappolicy.InfraServiceAccountPullSecretsControllerServiceAccountName)

	go serviceaccountcontrollers.NewDockercfgDeletedController(
		ctx.ExternalKubeInformers.Core().V1().Secrets(),
		kc,
		serviceaccountcontrollers.DockercfgDeletedControllerOptions{},
	).Run(ctx.Stop)

	go serviceaccountcontrollers.NewDockercfgTokenDeletedController(
		ctx.ExternalKubeInformers.Core().V1().Secrets(),
		kc,
		serviceaccountcontrollers.DockercfgTokenDeletedControllerOptions{},
	).Run(ctx.Stop)

	dockerURLsInitialized := make(chan struct{})
	dockercfgController := serviceaccountcontrollers.NewDockercfgController(
		ctx.ExternalKubeInformers.Core().V1().ServiceAccounts(),
		ctx.ExternalKubeInformers.Core().V1().Secrets(),
		kc,
		serviceaccountcontrollers.DockercfgControllerOptions{DockerURLsInitialized: dockerURLsInitialized},
	)
	go dockercfgController.Run(5, ctx.Stop)

	dockerRegistryControllerOptions := serviceaccountcontrollers.DockerRegistryServiceControllerOptions{
		RegistryNamespace:     "default",
		RegistryServiceName:   "docker-registry",
		DockercfgController:   dockercfgController,
		DockerURLsInitialized: dockerURLsInitialized,
	}
	go serviceaccountcontrollers.NewDockerRegistryServiceController(
		ctx.ExternalKubeInformers.Core().V1().Secrets(),
		kc,
		dockerRegistryControllerOptions,
	).Run(10, ctx.Stop)

	return true, nil
}
