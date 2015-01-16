package util

import (
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
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
			Labels: map[string]string{buildapi.BuildConfigLabel: bcCopy.Name},
		},
	}
	for originalImage, newImage := range imageSubstitutions {
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

// SubstituteImageReferences replaces references to an image with a new value
func SubstituteImageReferences(build *buildapi.Build, oldImage string, newImage string) {
	switch {
	case build.Parameters.Strategy.Type == buildapi.DockerBuildStrategyType &&
		build.Parameters.Strategy.DockerStrategy != nil &&
		build.Parameters.Strategy.DockerStrategy.BaseImage == oldImage:
		build.Parameters.Strategy.DockerStrategy.BaseImage = newImage
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
