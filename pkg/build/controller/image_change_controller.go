package controller

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/golang/glog"

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
func (c *ImageChangeController) HandleImageRepo(imageRepo *imageapi.ImageRepository) error {
	glog.V(4).Infof("Build image change controller detected imagerepo change %s", imageRepo.DockerImageRepository)
	imageSubstitutions := make(map[string]string)

	// TODO: this is inefficient
	for _, bc := range c.BuildConfigStore.List() {
		config := bc.(*buildapi.BuildConfig)
		glog.V(4).Infof("Detecting changed images for buildConfig %s", config.Name)

		// Extract relevant triggers for this imageRepo for this config
		shouldTriggerBuild := false
		for _, trigger := range config.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			icTrigger := trigger.ImageChange
			// only trigger a build if this image repo matches the name and namespace of the ref in the build trigger
			// also do not trigger if the imagerepo does not have a valid DockerImageRepository value for us to pull
			// the image from
			if imageRepo.Status.DockerImageRepository == "" || icTrigger.From.Name != imageRepo.Name || (len(icTrigger.From.Namespace) != 0 && icTrigger.From.Namespace != imageRepo.Namespace) {
				continue
			}
			// for every ImageChange trigger, record the image it substitutes for and get the latest
			// image id from the imagerepository.  We will substitute all images in the buildconfig
			// with the latest values from the imagerepositories.
			tag := icTrigger.Tag
			if len(tag) == 0 {
				tag = buildapi.DefaultImageTag
			}
			latest, err := imageapi.LatestTaggedImage(*imageRepo, tag)
			if err != nil {
				glog.V(2).Info(err)
				continue
			}

			// (must be different) to trigger a build
			if icTrigger.LastTriggeredImageID != latest.Image {
				imageSubstitutions[icTrigger.Image] = latest.DockerImageReference
				shouldTriggerBuild = true
				icTrigger.LastTriggeredImageID = latest.Image
			}
		}

		if shouldTriggerBuild {
			glog.V(4).Infof("Running build for buildConfig %s in namespace %s", config.Name, config.Namespace)
			b := buildutil.GenerateBuildFromConfig(config, nil, imageSubstitutions)
			if err := c.BuildCreator.Create(config.Namespace, b); err != nil {
				return fmt.Errorf("Error starting build for buildConfig %s: %v", config.Name, err)
			} else {
				if err := c.BuildConfigUpdater.Update(config); err != nil {
					// This is not a retryable error because the build has been created.  The worst case
					// outcome of not updating the buildconfig is that we might rerun a build for the
					// same "new" imageid change in the future, which is better than guaranteeing we
					// run the build 2+ times by retrying it here.
					glog.V(2).Infof("Error updating buildConfig %v: %v", config.Name, err)
					return ImageChangeControllerFatalError{Reason: fmt.Sprintf("Error updating buildConfig %s with new LastTriggeredImageID", config.Name), Err: err}
				}
			}
		}
	}
	return nil
}
