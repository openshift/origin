package util

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	buildapi "github.com/openshift/origin/pkg/build/api"
	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// GenerateBuildFromConfig creates a new build based on a given BuildConfig. Optionally a SourceRevision for the new
// build can be specified.  Also optionally a list of image names to be substituted can be supplied.  Values in the BuildConfig
// that have a substitution provided will be replaced in the resulting Build
func GenerateBuildFromConfig(bc *buildapi.BuildConfig, r *buildapi.SourceRevision, imageSubstitutions map[string]string) (build *buildapi.Build) {
	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	obj, _ := kapi.Scheme.Copy(bc)
	bcCopy := obj.(*buildapi.BuildConfig)

	b := &buildapi.Build{
		Parameters: buildapi.BuildParameters{
			Source:   bcCopy.Parameters.Source,
			Strategy: bcCopy.Parameters.Strategy,
			Output:   bcCopy.Parameters.Output,
			Revision: r,
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: bcCopy.Labels,
		},
	}
	if b.Labels == nil {
		b.Labels = make(map[string]string)
	}
	b.Labels[buildapi.BuildConfigLabel] = bcCopy.Name

	for originalImage, newImage := range imageSubstitutions {
		glog.V(4).Infof("Substituting %s for %s", newImage, originalImage)
		SubstituteImageReferences(b, originalImage, newImage)
	}
	return b
}

// GenerateBuildFromBuild creates a new build based on a given Build.
func GenerateBuildFromBuild(build *buildapi.Build) *buildapi.Build {
	obj, _ := kapi.Scheme.Copy(build)
	buildCopy := obj.(*buildapi.Build)
	return &buildapi.Build{
		Parameters: buildCopy.Parameters,
		ObjectMeta: kapi.ObjectMeta{
			Labels: buildCopy.ObjectMeta.Labels,
		},
	}
}

// GenerateBuildWithImageTag generates a build definition based on the current imageid
// from any ImageRepository that is associated to the BuildConfig by an ImageChangeTrigger.
// Takes a BuildConfig to base the build on, an optional SourceRevision to build, and an optional
// Client to use to get ImageRepositories to check for affiliation to this BuildConfig (by way of
// an ImageChangeTrigger).  If there is a match in the image repo list, the resulting build will use
// the image tag from the corresponding image repo rather than the image field from the buildconfig
// as the base image for the build.
func GenerateBuildWithImageTag(config *buildapi.BuildConfig, revision *buildapi.SourceRevision, imageRepoGetter osclient.ImageRepositoryNamespaceGetter) (*buildapi.Build, error) {

	imageSubstitutions := make(map[string]string)
	glog.V(4).Infof("Generating tagged build for config %s", config.Name)

	for _, trigger := range config.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		icTrigger := trigger.ImageChange
		glog.V(4).Infof("Found image change trigger with reference to repo %s", icTrigger.From.Name)

		var imageRepo *imageapi.ImageRepository
		var namespace string
		if len(icTrigger.From.Namespace) != 0 {
			namespace = icTrigger.From.Namespace
		} else {
			namespace = config.Namespace
		}

		var err error
		imageRepo, err = imageRepoGetter.GetByNamespace(namespace, icTrigger.From.Name)
		if err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return nil, err
		}

		if imageRepo == nil || imageRepo.Status.DockerImageRepository == "" {
			continue
		}
		glog.V(4).Infof("Found image repo %s", imageRepo.Name)

		// for the ImageChange trigger, record the image it substitutes for and get the latest
		// image id from the imagerepository.  We will substitute all images in the buildconfig
		// with the latest values from the imagerepositories.
		tag := icTrigger.Tag
		if len(tag) == 0 {
			tag = buildapi.DefaultImageTag
		}
		latest, err := imageapi.LatestTaggedImage(imageRepo, tag)
		if err != nil {
			continue
		}
		imageRef := latest.DockerImageReference
		glog.V(4).Infof("Adding substitution %s with %s", icTrigger.Image, imageRef)
		imageSubstitutions[icTrigger.Image] = imageRef
	}
	glog.V(4).Infof("Generating build from config for build config %s", config.Name)
	build := GenerateBuildFromConfig(config, revision, imageSubstitutions)
	return build, nil
}

// SubstituteImageReferences replaces references to an image with a new value
func SubstituteImageReferences(build *buildapi.Build, oldImage string, newImage string) {
	switch {
	case build.Parameters.Strategy.Type == buildapi.DockerBuildStrategyType &&
		build.Parameters.Strategy.DockerStrategy != nil &&
		build.Parameters.Strategy.DockerStrategy.Image == oldImage:
		build.Parameters.Strategy.DockerStrategy.Image = newImage
	case build.Parameters.Strategy.Type == buildapi.STIBuildStrategyType &&
		build.Parameters.Strategy.STIStrategy != nil &&
		build.Parameters.Strategy.STIStrategy.Image == oldImage:
		build.Parameters.Strategy.STIStrategy.Image = newImage
	case build.Parameters.Strategy.Type == buildapi.CustomBuildStrategyType:
		// update env variable references to the old image with the new image
		strategy := build.Parameters.Strategy.CustomStrategy
		if strategy.Env == nil {
			strategy.Env = make([]kapi.EnvVar, 1)
			strategy.Env[0] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: newImage}
		} else {
			found := false
			for i := range strategy.Env {
				glog.V(4).Infof("Checking env variable %s %s", strategy.Env[i].Name, strategy.Env[i].Value)
				if strategy.Env[i].Name == buildapi.CustomBuildStrategyBaseImageKey {
					found = true
					if strategy.Env[i].Value == oldImage {
						strategy.Env[i].Value = newImage
						glog.V(4).Infof("Updated env variable %s %s", strategy.Env[i].Name, strategy.Env[i].Value)
						break
					}
				}
			}
			if !found {
				strategy.Env = append(strategy.Env, kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: newImage})
			}
		}
		// update the actual custom build image with the new image, if applicable
		if strategy.Image == oldImage {
			strategy.Image = newImage
		}
	}
}
