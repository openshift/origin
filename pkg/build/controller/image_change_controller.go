package controller

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	buildutil "github.com/openshift/origin/pkg/build/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ImageChangeControllerFatalError struct {
	Reason string
	Err    error
}

func (e ImageChangeControllerFatalError) Error() string {
	return "fatal error handling ImageRepository change: " + e.Reason
}

// ImageChangeController watches for changes to ImageRepositories and triggers
// builds when a new version of a tag referenced by a BuildConfig
// is available.
type ImageChangeController struct {
	BuildConfigStore   cache.Store
	BuildCreator       buildclient.BuildCreator
	BuildConfigUpdater buildclient.BuildConfigUpdater
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

// HandleImageRepo processes the next ImageRepository event.
func (c *ImageChangeController) HandleImageRepo(repo *imageapi.ImageRepository) error {
	glog.V(4).Infof("Build image change controller detected imagerepo change %s", repo.Status.DockerImageRepository)
	subs := make(map[string]string)

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
			if repo.Status.DockerImageRepository == "" || change.From.Name != repo.Name || (len(change.From.Namespace) != 0 && change.From.Namespace != repo.Namespace) {
				continue
			}
			latest, err := imageapi.LatestTaggedImage(repo, change.Tag)
			if err != nil {
				util.HandleError(fmt.Errorf("unable to find tagged image: %v", err))
				continue
			}

			// (must be different) to trigger a build
			last := change.LastTriggeredImageID
			next := latest.Image
			if len(next) == 0 {
				// tags without images should still trigger builds (when going from a pure tag to an image
				// based tag, we should rebuild)
				next = latest.DockerImageReference
			}
			if len(last) == 0 || next != last {
				subs[change.Image] = latest.DockerImageReference
				change.LastTriggeredImageID = next
				shouldBuild = true
			}
		}

		if shouldBuild {
			glog.V(4).Infof("Running build for buildConfig %s in namespace %s", config.Name, config.Namespace)
			b := buildutil.GenerateBuildFromConfig(config, nil, subs)
			if err := c.BuildCreator.Create(config.Namespace, b); err != nil {
				return fmt.Errorf("error starting build for buildConfig %s: %v", config.Name, err)
			}
			if err := c.BuildConfigUpdater.Update(config); err != nil {
				// This is not a retryable error because the build has been created.  The worst case
				// outcome of not updating the buildconfig is that we might rerun a build for the
				// same "new" imageid change in the future, which is better than running the build
				// 2+ times by retrying it here.
				return ImageChangeControllerFatalError{Reason: fmt.Sprintf("Error updating buildConfig %s with new LastTriggeredImageID", config.Name), Err: err}
			}
		}
	}
	return nil
}
