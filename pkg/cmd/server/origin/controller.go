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
	codec := kapi.Codecs.LegacyCodec(groupVersion)

	buildControllerConfig := controller.BuildControllerConfig{
		DockerImage:           c.ImageFor("docker-builder"),
		STIImage:              c.ImageFor("sti-builder"),
		AdmissionPluginConfig: c.Options.AdmissionConfig.PluginConfig,
		Codec: codec,
	}

	ret["build"] = buildControllerConfig.RunController

	return ret, nil
}
