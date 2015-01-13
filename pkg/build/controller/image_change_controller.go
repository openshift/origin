package controller

import (
	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/cache"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeController watches for changes to ImageRepositories and triggers
// builds when a new version of a tag referenced by a BuildConfig
// is available.
type ImageChangeController struct {
	NextImageRepository func() *imageapi.ImageRepository
	BuildConfigStore    cache.Store
	BuildConfigUpdater  buildConfigUpdater
	BuildCreator        buildCreator
	// Stop is an optional channel that controls when the controller exits
	Stop <-chan struct{}
}

type buildConfigUpdater interface {
	UpdateBuildConfig(buildConfig *buildapi.BuildConfig) error
}

type buildCreator interface {
	CreateBuild(build *buildapi.BuildConfig, imageSubstitutions map[string]string) error
}

// Run processes ImageRepository events one by one.
func (c *ImageChangeController) Run() {
	go util.Until(c.HandleImageRepo, 0, c.Stop)
}

// HandleImageRepo processes the next ImageRepository event.
func (c *ImageChangeController) HandleImageRepo() {
	glog.V(4).Infof("Waiting for imagerepo change")
	imageRepo := c.NextImageRepository()
	glog.V(4).Infof("Build image change controller detected imagerepo change %s", imageRepo.DockerImageRepository)
	imageSubstitutions := make(map[string]string)

	for _, bc := range c.BuildConfigStore.List() {
		config := bc.(*buildapi.BuildConfig)
		glog.V(4).Infof("Detecting changed images for buildConfig %s", config.Name)

		// Extract relevant triggers for this imageRepo for this config
		shouldTriggerBuild := false
		for _, trigger := range config.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			// for every ImageChange trigger, record the image it substitutes for and get the latest
			// image id from the imagerepository.  We will substitute all images in the buildconfig
			// with the latest values from the imagerepositories.
			icTrigger := trigger.ImageChange
			tag := icTrigger.Tag
			if len(tag) == 0 {
				tag = buildapi.DefaultImageTag
			}
			imageID, hasTag := imageRepo.Tags[tag]
			if !hasTag {
				continue
			}

			// (must be different) to trigger a build
			if icTrigger.ImageRepositoryRef.Name == imageRepo.Name &&
				icTrigger.LastTriggeredImageID != imageID {
				imageSubstitutions[icTrigger.Image] = imageRepo.DockerImageRepository + ":" + imageID
				shouldTriggerBuild = true
				icTrigger.LastTriggeredImageID = imageID
			}
		}

		if shouldTriggerBuild {
			glog.V(4).Infof("Running build for buildConfig %s", config.Name)
			if err := c.BuildCreator.CreateBuild(config, imageSubstitutions); err != nil {
				glog.V(2).Infof("Error starting build for buildConfig %v: %v", config.Name, err)
			} else {
				if err := c.BuildConfigUpdater.UpdateBuildConfig(config); err != nil {
					glog.V(2).Infof("Error updating buildConfig %v: %v", config.Name, err)
				}
			}
		}
	}
}
