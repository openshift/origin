package controller

import (
	"path"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	serviceaccountadmission "k8s.io/kubernetes/plugin/pkg/admission/serviceaccount"

	deployercontroller "github.com/openshift/origin/pkg/apps/controller/deployer"
	deployconfigcontroller "github.com/openshift/origin/pkg/apps/controller/deploymentconfig"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

func envVars(host string, caData []byte, insecure bool, bearerTokenFile string) []kapi.EnvVar {
	envvars := []kapi.EnvVar{
		{Name: "KUBERNETES_MASTER", Value: host},
		{Name: "OPENSHIFT_MASTER", Value: host},
	}

	if len(bearerTokenFile) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "BEARER_TOKEN_FILE", Value: bearerTokenFile})
	}

	if len(caData) > 0 {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_CA_DATA", Value: string(caData)})
	} else if insecure {
		envvars = append(envvars, kapi.EnvVar{Name: "OPENSHIFT_INSECURE", Value: "true"})
	}

	return envvars
}

func RunDeployerController(ctx ControllerContext) (bool, error) {
	clientConfig, err := ctx.ClientBuilder.Config(bootstrappolicy.InfraDeployerControllerServiceAccountName)
	if err != nil {
		return true, err
	}

	kubeClient, err := kubernetes.NewForConfig(clientConfig)
	if err != nil {
		return true, err
	}

	vars := envVars(
		clientConfig.Host,
		clientConfig.CAData,
		clientConfig.Insecure,
		path.Join(serviceaccountadmission.DefaultAPITokenMountPath, kapi.ServiceAccountTokenKey),
	)

	groupVersion := schema.GroupVersion{Group: "", Version: "v1"}
	annotationCodec := legacyscheme.Codecs.LegacyCodec(groupVersion)

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Format
	imageTemplate.Latest = ctx.OpenshiftControllerConfig.Deployer.ImageTemplateFormat.Latest

	go deployercontroller.NewDeployerController(
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		ctx.ExternalKubeInformers.Core().V1().Pods(),
		kubeClient,
		bootstrappolicy.DeployerServiceAccountName,
		imageTemplate.ExpandOrDie("deployer"),
		vars,
		annotationCodec,
	).Run(5, ctx.Stop)

	return true, nil
}

func RunDeploymentConfigController(ctx ControllerContext) (bool, error) {
	saName := bootstrappolicy.InfraDeploymentConfigControllerServiceAccountName

	kubeClient, err := ctx.ClientBuilder.Client(saName)
	if err != nil {
		return true, err
	}

	groupVersion := schema.GroupVersion{Group: "", Version: "v1"}
	annotationCodec := legacyscheme.Codecs.LegacyCodec(groupVersion)

	go deployconfigcontroller.NewDeploymentConfigController(
		ctx.AppInformers.Apps().InternalVersion().DeploymentConfigs(),
		ctx.ExternalKubeInformers.Core().V1().ReplicationControllers(),
		ctx.ClientBuilder.OpenshiftInternalAppsClientOrDie(saName),
		kubeClient,
		annotationCodec,
	).Run(5, ctx.Stop)

	return true, nil
}
