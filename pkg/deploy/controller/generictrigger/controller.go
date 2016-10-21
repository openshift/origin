package generictrigger

import (
	"k8s.io/kubernetes/pkg/runtime"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// DeploymentTriggerController processes all triggers for a deployment config
// and kicks new deployments whenever possible.
type DeploymentTriggerController struct {
	// dn is used to update deployment configs.
	dn osclient.DeploymentConfigsNamespacer

	// queue contains deployment configs that need to be synced.
	queue workqueue.RateLimitingInterface

	// dcStore provides a local cache for deployment configs.
	dcStore oscache.StoreToDeploymentConfigLister
	// dcStoreSynced makes sure the dc store is synced before reconcling any deployment config.
	dcStoreSynced func() bool

	// codec is used for decoding a config out of a deployment.
	codec runtime.Codec
}

// Handle processes deployment triggers for a deployment config.
func (c *DeploymentTriggerController) Handle(config *deployapi.DeploymentConfig) error {
	if len(config.Spec.Triggers) == 0 || config.Spec.Paused {
		return nil
	}

	request := &deployapi.DeploymentRequest{
		Name:   config.Name,
		Latest: true,
		Force:  false,
	}

	_, err := c.dn.DeploymentConfigs(config.Namespace).Instantiate(request)
	return err
}

func (c *DeploymentTriggerController) handleErr(err error, key interface{}) {
	// TODO: "empty data" comes from the protobuf serializer when instantiate
	// returns a 204. This should be a typed error we ignore in this controller.
	if err == nil || err.Error() == "empty data" {
		c.queue.Forget(key)
		return
	}

	if c.queue.NumRequeues(key) < MaxRetries {
		glog.V(2).Infof("Error instantiating deployment config %v: %v", key, err)
		c.queue.AddRateLimited(key)
		return
	}

	utilruntime.HandleError(err)
	c.queue.Forget(key)
}
