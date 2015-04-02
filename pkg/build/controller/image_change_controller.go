package controller

import (
	"fmt"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImageChangeControllerFatalError struct {
	Reason string
	Err    error
}

func (e ImageChangeControllerFatalError) Error() string {
	return "fatal error handling ImageStream change: " + e.Reason
}

// ImageChangeController watches for changes to ImageRepositories and triggers
// builds when a new version of a tag referenced by a BuildConfig
// is available.
type ImageChangeController struct {
	BuildConfigStore        cache.Store
	BuildConfigInstantiator buildclient.BuildConfigInstantiator
	BuildConfigUpdater      buildclient.BuildConfigUpdater
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

// HandleImageRepo processes the next ImageStream event.
func (c *ImageChangeController) HandleImageRepo(repo *imageapi.ImageStream) error {
	glog.V(4).Infof("Build image change controller detected imagerepo change %s", repo.Status.DockerImageRepository)

	// TODO: this is inefficient
	for _, bc := range c.BuildConfigStore.List() {
		config := bc.(*buildapi.BuildConfig)

		shouldBuild := false
		// For every ImageChange trigger find the latest tagged image from the image repository and replace that value
		// throughout the build strategies. A new build is triggered only if the latest tagged image id or pull spec
		// differs from the last triggered build recorded on the build config.
		for _, trigger := range config.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			change := trigger.ImageChange
			// only trigger a build if this image repo matches the name and namespace of the ref in the build trigger
			// also do not trigger if the imagerepo does not have a valid DockerImageRepository value for us to pull
			// the image from
			if len(repo.Status.DockerImageRepository) == 0 || change.From.Name != repo.Name ||
				(len(change.From.Namespace) != 0 && change.From.Namespace != repo.Namespace) {
				continue
			}
			latest, err := imageapi.LatestTaggedImage(repo, change.Tag)
			if err != nil {
				util.HandleError(fmt.Errorf("unable to find tagged image: %v", err))
				continue
			}

			// (must be different) to trigger a build
			last := change.LastTriggeredImageID
			next := latest.DockerImageReference

			if len(last) == 0 || (len(next) > 0 && next != last) {
				change.LastTriggeredImageID = next
				shouldBuild = true
			}
		}

		if shouldBuild {
			glog.V(4).Infof("Running build for buildConfig %s in namespace %s", config.Name, config.Namespace)
			// instantiate new build
			request := &buildapi.BuildRequest{ObjectMeta: kapi.ObjectMeta{Name: config.Name}}
			if _, err := c.BuildConfigInstantiator.Instantiate(config.Namespace, request); err != nil {
				return fmt.Errorf("Error instantiating build from config %s: %v", config.Name, err)
			}
			// and update the config
			if err := c.updateConfig(config); err != nil {
				// This is not a retryable error. The worst case outcome of not updating the buildconfig
				// is that we might rerun a build for the same "new" imageid change in the future,
				// which is better than guaranteeing we run the build 2+ times by retrying it here.
				return ImageChangeControllerFatalError{
					Reason: fmt.Sprintf("Error updating buildConfig %s with new LastTriggeredImageID", config.Name),
					Err:    err,
				}
			}
		}
	}
	return nil
}

// updateConfig is responsible for updating current BuildConfig object which was changed
// during instantiate call, it basically copies LastTriggeredImageID to fresh copy
// of the BuildConfig object
func (c *ImageChangeController) updateConfig(config *buildapi.BuildConfig) error {
	item, _, err := c.BuildConfigStore.Get(config)
	newConfig := item.(*buildapi.BuildConfig)
	if err != nil {
		return err
	}
	for i, trigger := range newConfig.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		change := trigger.ImageChange
		change.LastTriggeredImageID = config.Triggers[i].ImageChange.LastTriggeredImageID
	}

	return c.BuildConfigUpdater.Update(newConfig)
}
