package controller

import (
	"fmt"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	scheduleroptions "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app/options"
	schedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api"
	latestschedulerapi "k8s.io/kubernetes/plugin/pkg/scheduler/api/latest"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
	"github.com/openshift/origin/pkg/cmd/util/variable"
	"k8s.io/kubernetes/pkg/volume"
)

// KubeControllerConfig is the runtime (non-serializable) config object used to
// launch the set of kube (not openshift) controllers.
type KubeControllerConfig struct {
	HorizontalPodAutoscalerControllerConfig HorizontalPodAutoscalerControllerConfig

	// TODO the scheduler should move out into its own logical component
	SchedulerControllerConfig SchedulerControllerConfig
}

// GetControllerInitializers return the controller initializer functions for kube controllers
// TODO in 3.7, CloudProvider is on the context
func (c KubeControllerConfig) GetControllerInitializers() (map[string]kubecontroller.InitFunc, error) {
	ret := kubecontroller.NewControllerInitializers()

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	ret["horizontalpodautoscaling"] = c.HorizontalPodAutoscalerControllerConfig.RunController

	// FIXME: Move this under openshift controller intialization once we figure out
	// deployment (options).
	ret["openshift.io/scheduler"] = c.SchedulerControllerConfig.RunController

	return ret, nil
}

// BuildKubeControllerConfig builds the struct to create the controller initializers.  Eventually we want this to be fully
// stock kube with no modification.
func BuildKubeControllerConfig(options configapi.MasterConfig) (*KubeControllerConfig, error) {
	var err error
	ret := &KubeControllerConfig{}

	kubeExternal, _, err := configapi.GetExternalKubeClient(options.MasterClients.OpenShiftLoopbackKubeConfig, options.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return nil, err
	}

	var schedulerPolicy *schedulerapi.Policy
	if _, err := os.Stat(options.KubernetesMasterConfig.SchedulerConfigFile); err == nil {
		schedulerPolicy = &schedulerapi.Policy{}
		configData, err := ioutil.ReadFile(options.KubernetesMasterConfig.SchedulerConfigFile)
		if err != nil {
			return nil, fmt.Errorf("unable to read scheduler config: %v", err)
		}
		if err := runtime.DecodeInto(latestschedulerapi.Codec, configData, schedulerPolicy); err != nil {
			return nil, fmt.Errorf("invalid scheduler configuration: %v", err)
		}
	}
	// resolve extended arguments
	// TODO: this should be done in config validation (along with the above) so we can provide
	// proper errors
	schedulerserver := scheduleroptions.NewSchedulerServer()
	schedulerserver.PolicyConfigFile = options.KubernetesMasterConfig.SchedulerConfigFile
	if err := cmdflags.Resolve(options.KubernetesMasterConfig.SchedulerArguments, schedulerserver.AddFlags); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}
	ret.SchedulerControllerConfig = SchedulerControllerConfig{
		PrivilegedClient: kubeExternal,
		SchedulerServer:  schedulerserver,
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = options.ImageConfig.Format
	imageTemplate.Latest = options.ImageConfig.Latest
	volume.NewPersistentVolumeRecyclerPodTemplate = newPersistentVolumeRecyclerPodTemplate(imageTemplate.ExpandOrDie("recycler"))

	ret.HorizontalPodAutoscalerControllerConfig = HorizontalPodAutoscalerControllerConfig{
		HeapsterNamespace: options.PolicyConfig.OpenShiftInfrastructureNamespace,
	}

	return ret, nil
}
