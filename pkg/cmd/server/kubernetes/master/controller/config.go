package controller

import (
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"k8s.io/kubernetes/pkg/volume"
)

// KubeControllerConfig is the runtime (non-serializable) config object used to
// launch the set of kube (not openshift) controllers.
type KubeControllerConfig struct {
	HorizontalPodAutoscalerControllerConfig HorizontalPodAutoscalerControllerConfig
}

// GetControllerInitializers return the controller initializer functions for kube controllers
// TODO in 3.7, CloudProvider is on the context
func (c KubeControllerConfig) GetControllerInitializers() (map[string]kubecontroller.InitFunc, error) {
	ret := kubecontroller.NewControllerInitializers()

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	ret["horizontalpodautoscaling"] = c.HorizontalPodAutoscalerControllerConfig.RunController

	return ret, nil
}

// BuildKubeControllerConfig builds the struct to create the controller initializers.  Eventually we want this to be fully
// stock kube with no modification.
func BuildKubeControllerConfig(options configapi.MasterConfig) (*KubeControllerConfig, error) {
	ret := &KubeControllerConfig{}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest
	volume.NewPersistentVolumeRecyclerPodTemplate = newPersistentVolumeRecyclerPodTemplate(imageTemplate.ExpandOrDie("recycler"))

	ret.HorizontalPodAutoscalerControllerConfig = HorizontalPodAutoscalerControllerConfig{
		HeapsterNamespace: options.PolicyConfig.OpenShiftInfrastructureNamespace,
	}

	return ret, nil
}
