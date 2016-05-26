package imagechange

import (
	"fmt"

	"github.com/golang/glog"

	apierror "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/client/record"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/workqueue"

	"github.com/openshift/origin/pkg/client"
	oscache "github.com/openshift/origin/pkg/client/cache"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// fatalError is an error which can't be retried.
type fatalError string

func (e fatalError) Error() string {
	return fmt.Sprintf("fatal error handling image stream: %s", string(e))
}

// ImageChangeController watches image streams for any image updates and updates
// any deployment configs that point to updated images.
type ImageChangeController struct {
	// queue contains image streams that need to be synced.
	queue workqueue.RateLimitingInterface

	// dn is used for updating deployment configs.
	dn client.DeploymentConfigsNamespacer

	// dcStore provides a local cache for deploymentconfigs.
	dcStore oscache.StoreToDeploymentConfigLister
	// dcController watches for changes to all deploymentconfigs.
	dcController *framework.Controller
	// streamStore provides a local cache for image streams.
	streamStore oscache.StoreToImageStreamLister
	// streamController watches for changes to all image streams.
	streamController *framework.Controller

	// recorder is used to record events.
	// TODO: Use this to emit events on successful updates
	recorder record.EventRecorder
}

// Handle processes image change triggers associated with imagestream.
func (c *ImageChangeController) Handle(stream *imageapi.ImageStream) error {
	// TODO: Build an index from image stream to deploymentconfigs and avoid listing all dcs
	configs, err := c.dcStore.List()
	if err != nil {
		return fmt.Errorf("couldn't get list of deployment configs while handling image stream %q: %v", imageapi.LabelForStream(stream), err)
	}

	// Find any configs which should be updated based on the new image state
	configsToUpdate := []*deployapi.DeploymentConfig{}
	for i := range configs {
		config := &configs[i]
		glog.V(4).Infof("Detecting image changes for deployment config %q", deployutil.LabelForDeploymentConfig(config))
		hasImageChange := false

		for _, trigger := range config.Spec.Triggers {
			params := trigger.ImageChangeParams

			// Only automatic image change triggers should fire
			if trigger.Type != deployapi.DeploymentTriggerOnImageChange {
				continue
			}

			// All initial deployments (latestVersion == 0) should have their images resolved in order
			// to be able to work and not try to pull non-existent images from DockerHub.
			// Deployments with automatic set to false that have been deployed at least once (latestVersion > 0)
			// shouldn't have their images updated.
			if !params.Automatic && config.Status.LatestVersion != 0 {
				continue
			}

			// Check if the image stream matches the trigger
			if !triggerMatchesImage(config, params, stream) {
				continue
			}

			_, tag, ok := imageapi.SplitImageStreamTag(params.From.Name)
			if !ok {
				glog.Warningf("Invalid image stream tag %q in %q", params.From.Name, deployutil.LabelForDeploymentConfig(config))
				continue
			}

			// Find the latest tag event for the trigger tag
			latestEvent := imageapi.LatestTaggedImage(stream, tag)
			if latestEvent == nil {
				glog.V(5).Infof("Couldn't find latest tag event for tag %q in image stream %q", tag, imageapi.LabelForStream(stream))
				continue
			}

			// Ensure a change occurred
			if len(latestEvent.DockerImageReference) == 0 || latestEvent.DockerImageReference == params.LastTriggeredImage {
				glog.V(4).Infof("No image changes for deployment config %q were detected", deployutil.LabelForDeploymentConfig(config))
				continue
			}

			names := sets.NewString(params.ContainerNames...)
			for i := range config.Spec.Template.Spec.Containers {
				container := &config.Spec.Template.Spec.Containers[i]
				if !names.Has(container.Name) {
					continue
				}
				// Update the image
				container.Image = latestEvent.DockerImageReference
				// Log the last triggered image ID
				params.LastTriggeredImage = latestEvent.DockerImageReference
				hasImageChange = true
			}
		}

		if hasImageChange {
			configsToUpdate = append(configsToUpdate, config)
		}
	}

	// Attempt to regenerate all configs which may contain image updates
	for i := range configsToUpdate {
		config := configsToUpdate[i]
		if _, err := c.dn.DeploymentConfigs(config.Namespace).Update(config); err != nil {
			c.handleErr(err, config, stream)
		}
	}

	return nil
}

func (c *ImageChangeController) handleErr(err error, config *deployapi.DeploymentConfig, stream *imageapi.ImageStream) {
	switch {
	case apierror.IsConflict(err):
		// Retry since the cache needs some time to catch up. Conflict errors are common
		// when a deployment config with an image change trigger is created.
		c.enqueueImageStream(stream)
	case apierror.IsNotFound(err):
		// Do nothing
	default:
		glog.Infof("Cannot update deployment config %q for trigger on image stream %q: %v", deployutil.LabelForDeploymentConfig(config), imageapi.LabelForStream(stream), err)
	}
}

// triggerMatchesImages decides whether a given trigger for config matches the provided image stream.
func triggerMatchesImage(config *deployapi.DeploymentConfig, params *deployapi.DeploymentTriggerImageChangeParams, stream *imageapi.ImageStream) bool {
	namespace := params.From.Namespace
	if len(namespace) == 0 {
		namespace = config.Namespace
	}
	name, _, ok := imageapi.SplitImageStreamTag(params.From.Name)
	return stream.Namespace == namespace && stream.Name == name && ok
}
