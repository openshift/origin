package generator

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildGenerator is a central place responsible for generating new Build objects
// from BuildConfigs and other Builds.
type BuildGenerator struct {
	Client GeneratorClient
}

type GeneratorClient interface {
	GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuild(ctx kapi.Context, build *buildapi.Build) error
	GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
}

type Client struct {
	GetBuildConfigFunc    func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfigFunc func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuildFunc          func(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuildFunc       func(ctx kapi.Context, build *buildapi.Build) error
	GetImageStreamFunc    func(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
}

func (c Client) GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
	return c.GetBuildConfigFunc(ctx, name)
}

func (c Client) UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
	return c.UpdateBuildConfigFunc(ctx, buildConfig)
}

func (c Client) GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error) {
	return c.GetBuildFunc(ctx, name)
}

func (c Client) CreateBuild(ctx kapi.Context, build *buildapi.Build) error {
	return c.CreateBuildFunc(ctx, build)
}

func (c Client) GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
	return c.GetImageStreamFunc(ctx, name)
}

type fatalError struct {
	error
}

// Instantiate returns new Build object based on a BuildRequest object
func (g *BuildGenerator) Instantiate(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating build from config %s", request.Name)
	bc, err := g.Client.GetBuildConfig(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	newBuild, err := g.generateBuild(ctx, bc, request.Revision)
	if err != nil {
		return nil, err
	}

	return g.createBuild(ctx, newBuild)
}

// Clone returns clone of a Build
func (g *BuildGenerator) Clone(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating build from build %s", request.Name)
	build, err := g.Client.GetBuild(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	newBuild := generateBuildFromBuild(build)

	return g.createBuild(ctx, newBuild)
}

// createBuild is responsible for validating build object and saving it and returning newly created object
func (g *BuildGenerator) createBuild(ctx kapi.Context, build *buildapi.Build) (*buildapi.Build, error) {
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict("build", build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}
	kapi.FillObjectMetaSystemFields(ctx, &build.ObjectMeta)

	err := g.Client.CreateBuild(ctx, build)
	if err != nil {
		return nil, err
	}
	return g.Client.GetBuild(ctx, build.Name)
}

// generateBuild generates a build definition based on the current imageid
// from any ImageStream that is associated to the BuildConfig by an ImageChangeTrigger.
// Takes a BuildConfig to base the build on, an optional SourceRevision to build, and an optional
// Client to use to get ImageRepositories to check for affiliation to this BuildConfig (by way of
// an ImageChangeTrigger).  If there is a match in the image repo list, the resulting build will use
// the image tag from the corresponding image repo rather than the image field from the buildconfig
// as the base image for the build.
func (g *BuildGenerator) generateBuild(ctx kapi.Context, config *buildapi.BuildConfig, revision *buildapi.SourceRevision) (*buildapi.Build, error) {
	var build *buildapi.Build
	var err error
	glog.V(4).Infof("Generating tagged build for config %s", config.Name)

	switch {
	case config.Parameters.Strategy.Type == buildapi.STIBuildStrategyType:
		if config.Parameters.Strategy.STIStrategy.From != nil && len(config.Parameters.Strategy.STIStrategy.From.Name) > 0 {
			build, err = g.generateBuildUsingObjectReference(ctx, config, revision)
		} else {
			build, err = g.generateBuildUsingImageTriggerTag(ctx, config, revision)
		}
	case config.Parameters.Strategy.Type == buildapi.DockerBuildStrategyType:
		build, err = g.generateBuildUsingImageTriggerTag(ctx, config, revision)
	case config.Parameters.Strategy.Type == buildapi.CustomBuildStrategyType:
		build, err = g.generateBuildUsingImageTriggerTag(ctx, config, revision)
	default:
		return nil, fmt.Errorf("Build strategy type must be set")
	}
	return build, err
}

// generateBuildUsingObjectReference examines the ImageRepo referenced by the BuildConfig and resolves it to
// an imagespec, it then returns a Build object that uses that imagespec.
func (g *BuildGenerator) generateBuildUsingObjectReference(ctx kapi.Context, config *buildapi.BuildConfig, revision *buildapi.SourceRevision) (*buildapi.Build, error) {
	imageRepoSubstitutions := make(map[kapi.ObjectReference]string)
	from := config.Parameters.Strategy.STIStrategy.From
	namespace := from.Namespace
	if len(namespace) == 0 {
		namespace = config.Namespace
	}
	tag := config.Parameters.Strategy.STIStrategy.Tag
	if len(tag) == 0 {
		tag = buildapi.DefaultImageTag
	}

	imageRepo, err := g.Client.GetImageStream(kapi.WithNamespace(ctx, namespace), from.Name)
	if err != nil {
		return nil, err
	}
	if imageRepo == nil || len(imageRepo.Status.DockerImageRepository) == 0 {
		return nil, fmt.Errorf("Docker Image Repository %s missing in namespace %s", from.Name, namespace)
	}
	glog.V(4).Infof("Found image repository %s", imageRepo.Name)
	latest, err := imageapi.LatestTaggedImage(imageRepo, tag)
	if err == nil {
		glog.V(4).Infof("Using image %s for image repository %s in namespace %s", latest.DockerImageReference, from.Name, from.Namespace)
		imageRepoSubstitutions[*from] = latest.DockerImageReference
	} else {
		return nil, fmt.Errorf("Docker Image Repository %s has no tag %s", from.Name, tag)
	}

	return g.generateBuildFromConfig(ctx, config, revision, nil, imageRepoSubstitutions)
}

// generateBuildUsingImageTriggerTag examines the ImageChangeTriggers associated with the BuildConfig
// and uses them to determine the current imagespec that should be used to run this build, it then
// returns a Build object that uses that imagespec.
func (g *BuildGenerator) generateBuildUsingImageTriggerTag(ctx kapi.Context, config *buildapi.BuildConfig, revision *buildapi.SourceRevision) (*buildapi.Build, error) {
	imageSubstitutions := make(map[string]string)
	for _, trigger := range config.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		icTrigger := trigger.ImageChange
		glog.V(4).Infof("Found image change trigger with reference to repo %s", icTrigger.From.Name)
		imageRef, err := g.resolveImageRepoReference(ctx, &icTrigger.From, icTrigger.Tag, config.Namespace)
		if err != nil {
			if _, ok := err.(fatalError); ok {
				return nil, err
			}

			continue
		}
		// for the ImageChange trigger, record the image it substitutes for and get the latest
		// image id from the imagerepository.  We will substitute all images in the buildconfig
		// with the latest values from the imagerepositories.
		glog.V(4).Infof("Adding substitution %s with %s", icTrigger.Image, imageRef)
		imageSubstitutions[icTrigger.Image] = imageRef
	}

	return g.generateBuildFromConfig(ctx, config, revision, imageSubstitutions, nil)
}

// generateBuildFromConfig creates a new build based on a given BuildConfig.
// Optionally a SourceRevision for the new build can be specified.
func (g *BuildGenerator) generateBuildFromConfig(ctx kapi.Context, bc *buildapi.BuildConfig, revision *buildapi.SourceRevision,
	imageSubstitutions map[string]string, imageRepoSubstitutions map[kapi.ObjectReference]string) (*buildapi.Build, error) {
	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	obj, _ := kapi.Scheme.Copy(bc)
	bcCopy := obj.(*buildapi.BuildConfig)

	build := &buildapi.Build{
		Parameters: buildapi.BuildParameters{
			Source:   bcCopy.Parameters.Source,
			Strategy: bcCopy.Parameters.Strategy,
			Output:   bcCopy.Parameters.Output,
			Revision: revision,
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: bcCopy.Labels,
		},
		Status: buildapi.BuildStatusNew,
	}
	build.Config = &kapi.ObjectReference{Kind: "BuildConfig", Name: bc.Name, Namespace: bc.Namespace}
	build.Name = getNextBuildName(bc)
	if err := g.Client.UpdateBuildConfig(ctx, bc); err != nil {
		return nil, err
	}
	if build.Labels == nil {
		build.Labels = make(map[string]string)
	}
	build.Labels[buildapi.BuildConfigLabel] = bcCopy.Name

	for originalImage, newImage := range imageSubstitutions {
		glog.V(4).Infof("Substituting %s for %s", newImage, originalImage)
		substituteImageReferences(build, originalImage, newImage)
	}
	for imageRepo, newImage := range imageRepoSubstitutions {
		if len(imageRepo.Namespace) != 0 {
			glog.V(4).Infof("Substituting repository %s for %s/%s", newImage, imageRepo.Namespace, imageRepo.Name)
		} else {
			glog.V(4).Infof("Substituting repository %s for %s", newImage, imageRepo.Name)
		}
		substituteImageRepoReferences(build, imageRepo, newImage)
	}

	// If after doing all the substitutions for ImageChangeTriggers, the Build is still using a From reference instead
	// of a resolved image, we need to resolve that From reference to a valid image so we can run the build.  Builds do
	// not consume ImageRepo references, only image specs.
	if build.Parameters.Strategy.Type == buildapi.STIBuildStrategyType && build.Parameters.Strategy.STIStrategy.From != nil {
		image, err := g.resolveImageRepoReference(ctx, build.Parameters.Strategy.STIStrategy.From, build.Parameters.Strategy.STIStrategy.Tag, build.Namespace)
		if err != nil {
			return nil, err
		}
		build.Parameters.Strategy.STIStrategy.Image = image
		build.Parameters.Strategy.STIStrategy.From = nil
		build.Parameters.Strategy.STIStrategy.Tag = ""
	}
	return build, nil
}

func (g *BuildGenerator) resolveImageRepoReference(ctx kapi.Context, from *kapi.ObjectReference, tag string, defaultNamespace string) (string, error) {
	var namespace string
	if len(from.Namespace) != 0 {
		namespace = from.Namespace
	} else {
		namespace = defaultNamespace
	}
	imageStream, err := g.Client.GetImageStream(kapi.WithNamespace(ctx, namespace), from.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", fatalError{err}
	}

	if imageStream == nil || len(imageStream.Status.DockerImageRepository) == 0 {
		return "", fmt.Errorf("could not resolve image stream %s/%s", namespace, from.Name)
	}
	glog.V(4).Infof("Found image stream %s/%s", namespace, imageStream.Name)

	latest, err := imageapi.LatestTaggedImage(imageStream, tag)
	if err != nil {
		return "", err
	}
	return latest.DockerImageReference, nil
}

// getNextBuildName returns name of the next build and increments BuildConfig's LastVersion.
func getNextBuildName(bc *buildapi.BuildConfig) string {
	bc.LastVersion++
	return fmt.Sprintf("%s-%d", bc.Name, bc.LastVersion)
}

// substituteImageReferences replaces references to an image with a new value
func substituteImageReferences(build *buildapi.Build, oldImage string, newImage string) {
	switch {
	case build.Parameters.Strategy.Type == buildapi.DockerBuildStrategyType &&
		build.Parameters.Strategy.DockerStrategy != nil &&
		build.Parameters.Strategy.DockerStrategy.Image == oldImage:
		build.Parameters.Strategy.DockerStrategy.Image = newImage
	case build.Parameters.Strategy.Type == buildapi.STIBuildStrategyType &&
		build.Parameters.Strategy.STIStrategy != nil &&
		(build.Parameters.Strategy.STIStrategy.From == nil || build.Parameters.Strategy.STIStrategy.From.Name == "") &&
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

// substituteImageRepoReferences uses references to an image repository to set an actual image name
// It also clears the ImageRepo reference from the BuildStrategy, if one was set.  The imagereference
// field will be used explicitly.
func substituteImageRepoReferences(build *buildapi.Build, imageRepo kapi.ObjectReference, newImage string) {
	switch {
	case build.Parameters.Strategy.Type == buildapi.STIBuildStrategyType &&
		build.Parameters.Strategy.STIStrategy != nil &&
		build.Parameters.Strategy.STIStrategy.From != nil &&
		build.Parameters.Strategy.STIStrategy.From.Name == imageRepo.Name &&
		build.Parameters.Strategy.STIStrategy.From.Namespace == imageRepo.Namespace:
		build.Parameters.Strategy.STIStrategy.Image = newImage
		build.Parameters.Strategy.STIStrategy.From = nil
	}
}

// generateBuildFromBuild creates a new build based on a given Build.
func generateBuildFromBuild(build *buildapi.Build) *buildapi.Build {
	obj, _ := kapi.Scheme.Copy(build)
	buildCopy := obj.(*buildapi.Build)
	return &buildapi.Build{
		Parameters: buildCopy.Parameters,
		ObjectMeta: kapi.ObjectMeta{
			Name:   getNextBuildNameFromBuild(buildCopy),
			Labels: buildCopy.ObjectMeta.Labels,
		},
		Status: buildapi.BuildStatusNew,
	}
}

// getNextBuildNameFromBuild returns name of the next build with random uuid added at the end
func getNextBuildNameFromBuild(build *buildapi.Build) string {
	buildName := build.Name
	if matched, _ := regexp.MatchString(`^.+-\d-\d+$`, buildName); matched {
		nameElems := strings.Split(buildName, "-")
		buildName = strings.Join(nameElems[:len(nameElems)-1], "-")
	}
	return fmt.Sprintf("%s-%d", buildName, int32(util.Now().Unix()))
}
