package origin

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/cmd/server/origin/controller"
)

func (c *MasterConfig) NewOpenshiftControllerInitializers() (map[string]controller.InitFunc, error) {
	ret := map[string]controller.InitFunc{}

	// initialize build controller
	storageVersion := c.Options.EtcdStorageConfig.OpenShiftStorageVersion
	groupVersion := schema.GroupVersion{Group: "", Version: storageVersion}
	// TODO: add codec to the controller context
	codec := kapi.Codecs.LegacyCodec(groupVersion)

	buildControllerConfig := controller.BuildControllerConfig{
		DockerImage:           c.ImageFor("docker-builder"),
		STIImage:              c.ImageFor("sti-builder"),
		AdmissionPluginConfig: c.Options.AdmissionConfig.PluginConfig,
		Codec: codec,
	}
	ret["build"] = buildControllerConfig.RunController

	// initialize apps.openshift.io controllers
	vars, err := c.GetOpenShiftClientEnvVars()
	if err != nil {
		return nil, err
	}
	deployer := controller.DeployerControllerConfig{ImageName: c.ImageFor("deployer"), Codec: codec, ClientEnvVars: vars}
	ret["deployer"] = deployer.RunController

	deploymentConfig := controller.DeploymentConfigControllerConfig{Codec: codec}
	ret["deploymentconfig"] = deploymentConfig.RunController

	deploymentTrigger := controller.DeploymentTriggerControllerConfig{Codec: codec}
	ret["deploymenttrigger"] = deploymentTrigger.RunController

	templateInstance := controller.TemplateInstanceControllerConfig{}
	ret["templateinstance"] = templateInstance.RunController

	return ret, nil
}
