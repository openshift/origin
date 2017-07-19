package controller

import (
	kubecontroller "k8s.io/kubernetes/cmd/kube-controller-manager/app"
)

// KubeControllerConfig is the runtime (non-serializable) config object used to
// launch the set of kube (not openshift) controllers.
type KubeControllerConfig struct {
	RecyclerImage string

	// TODO the scheduler should move out into its own logical component
	SchedulerControllerConfig SchedulerControllerConfig

	HeapsterNamespace string
}

// GetControllerInitializers return the controller initializer functions for kube controllers
// TODO in 3.7, CloudProvider is on the context
func (c KubeControllerConfig) GetControllerInitializers() (map[string]kubecontroller.InitFunc, error) {
	ret := kubecontroller.NewControllerInitializers()

	// Remove the "normal" resource quota, because we run it as an openshift controller to cover all types
	// TODO split openshift separately so we get upstream initialization here
	delete(ret, "resourcequota")
	// "serviceaccount-token" is used to create SA tokens for everyone else.  We special case this one.
	delete(ret, "serviceaccount-token")

	// TODO once the cloudProvider moves, move the configs out of here to where they need to be constructed
	persistentVolumeController := PersistentVolumeControllerConfig{
		RecyclerImage: c.RecyclerImage,
	}
	ret["persistentvolume-binder"] = persistentVolumeController.RunController

	// FIXME: Move this under openshift controller intialization once we figure out
	// deployment (options).
	ret["openshift.io/scheduler"] = c.SchedulerControllerConfig.RunController

	// overrides the Kube HPA controller config, so that we can point it at an HTTPS Heapster
	// in openshift-infra, and pass it a scale client that knows how to scale DCs
	hpaControllerConfig := HorizontalPodAutoscalerControllerConfig{
		HeapsterNamespace: c.HeapsterNamespace,
	}
	ret["horizontalpodautoscaling"] = hpaControllerConfig.RunController

	return ret, nil
}
