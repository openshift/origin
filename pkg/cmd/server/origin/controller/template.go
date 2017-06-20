package controller

import (
	"k8s.io/client-go/tools/cache"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatecontroller "github.com/openshift/origin/pkg/template/controller"
)

func RunTemplateInstanceController(ctx ControllerContext) (bool, error) {
	err := ctx.TemplateInformers.Template().InternalVersion().Templates().Informer().AddIndexers(cache.Indexers{
		templateapi.TemplateUIDIndex: func(obj interface{}) ([]string, error) {
			return []string{string(obj.(*templateapi.Template).UID)}, nil
		},
	})
	if err != nil {
		return true, err
	}

	saName := bootstrappolicy.InfraTemplateInstanceControllerServiceAccountName

	restConfig, err := ctx.ClientBuilder.Config(saName)
	if err != nil {
		return true, err
	}

	internalKubeClient, err := ctx.ClientBuilder.KubeInternalClient(saName)
	if err != nil {
		return true, err
	}

	deprecatedOcClient, err := ctx.ClientBuilder.DeprecatedOpenshiftClient(saName)
	if err != nil {
		return true, err
	}

	templateClient, err := ctx.ClientBuilder.OpenshiftTemplateClient(saName)
	if err != nil {
		return true, err
	}

	go templatecontroller.NewTemplateInstanceController(
		restConfig,
		deprecatedOcClient,
		internalKubeClient,
		templateClient.Template(),
		ctx.TemplateInformers.Template().InternalVersion().TemplateInstances(),
	).Run(5, ctx.Stop)

	return true, nil
}
