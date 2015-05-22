package generator

import (
	"fmt"
	"regexp"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildGenerator is a central place responsible for generating new Build objects
// from BuildConfigs and other Builds.
type BuildGenerator struct {
	Client          GeneratorClient
	ServiceAccounts kclient.ServiceAccountsNamespacer
	Secrets         kclient.SecretsNamespacer
}

// GeneratorClient is the API client used by the generator
type GeneratorClient interface {
	GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuild(ctx kapi.Context, build *buildapi.Build) error
	GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	GetImageStreamImage(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error)
	GetImageStreamTag(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error)
}

// Client is an implementation of the GeneratorClient interface
type Client struct {
	GetBuildConfigFunc      func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfigFunc   func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuildFunc            func(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuildFunc         func(ctx kapi.Context, build *buildapi.Build) error
	GetImageStreamFunc      func(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	GetImageStreamImageFunc func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error)
	GetImageStreamTagFunc   func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error)
}

// GetBuildConfig retrieves a named build config
func (c Client) GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
	return c.GetBuildConfigFunc(ctx, name)
}

// UpdateBuildConfig updates a named build config
func (c Client) UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
	return c.UpdateBuildConfigFunc(ctx, buildConfig)
}

// GetBuild retrieves a build
func (c Client) GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error) {
	return c.GetBuildFunc(ctx, name)
}

// CreateBuild creates a new build
func (c Client) CreateBuild(ctx kapi.Context, build *buildapi.Build) error {
	return c.CreateBuildFunc(ctx, build)
}

// GetImageStream retrieves a named image stream
func (c Client) GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
	return c.GetImageStreamFunc(ctx, name)
}

// GetImageStreamImage retrieves an image stream image
func (c Client) GetImageStreamImage(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
	return c.GetImageStreamImageFunc(ctx, name)
}

// GetImageStreamTag retrieves and image stream tag
func (c Client) GetImageStreamTag(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
	return c.GetImageStreamTagFunc(ctx, name)
}

type fatalError struct {
	error
}

type streamRef struct {
	ref *kapi.ObjectReference
	tag string
}

// Instantiate returns new Build object based on a BuildRequest object
func (g *BuildGenerator) Instantiate(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating Build from BuildConfig %s/%s", request.Namespace, request.Name)
	bc, err := g.Client.GetBuildConfig(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	newBuild, err := g.generateBuildFromConfig(ctx, bc, request.Revision)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("Build %s/%s has been generated from %s/%s BuildConfig", newBuild.Namespace, newBuild.ObjectMeta.Name, bc.Namespace, bc.ObjectMeta.Name)
	return g.createBuild(ctx, newBuild)
}

// Clone returns clone of a Build
func (g *BuildGenerator) Clone(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating build from build %s/%s", request.Namespace, request.Name)
	build, err := g.Client.GetBuild(ctx, request.Name)
	if err != nil {
		return nil, err
	}
	newBuild := generateBuildFromBuild(build)
	glog.V(4).Infof("Build %s/%s has been generated from Build %s/%s", newBuild.Namespace, newBuild.ObjectMeta.Name, build.Namespace, build.ObjectMeta.Name)
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

// generateBuildFromConfig generates a build definition based on the current imageid
// from any ImageStream that is associated to the BuildConfig by From reference in
// the Strategy, or uses the Image field of the Strategy.
// Takes a BuildConfig to base the build on, and an optional SourceRevision to build.
func (g *BuildGenerator) generateBuildFromConfig(ctx kapi.Context, bc *buildapi.BuildConfig, revision *buildapi.SourceRevision) (*buildapi.Build, error) {
	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	obj, _ := kapi.Scheme.Copy(bc)
	bcCopy := obj.(*buildapi.BuildConfig)
	build := &buildapi.Build{
		Parameters: buildapi.BuildParameters{
			Source:    bcCopy.Parameters.Source,
			Strategy:  bcCopy.Parameters.Strategy,
			Output:    bcCopy.Parameters.Output,
			Revision:  revision,
			Resources: bcCopy.Parameters.Resources,
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: bcCopy.Labels,
		},
		Status: buildapi.BuildStatusNew,
	}

	if bc.Parameters.Output.PushSecret == nil {
		sa, err := g.ServiceAccounts.ServiceAccounts(bc.Namespace).Get(bootstrappolicy.BuilderServiceAccountName)
		if err != nil {
			return nil, err
		}

		for _, secretRef := range sa.Secrets {
			secret, err := g.Secrets.Secrets(bc.Namespace).Get(secretRef.Name)
			if err != nil {
				return nil, err
			}

			if secret.Type == kapi.SecretTypeDockercfg {
				build.Parameters.Output.PushSecret = &kapi.LocalObjectReference{Name: secret.Name}
			}
		}
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

	// If the Build is using a From reference instead of a resolved image, we need to resolve that From
	// reference to a valid image so we can run the build.  Builds do not consume ImageStream references,
	// only image specs.
	switch {
	case build.Parameters.Strategy.Type == buildapi.SourceBuildStrategyType &&
		build.Parameters.Strategy.SourceStrategy.From != nil:
		image, err := g.resolveImageStreamReference(ctx, build.Parameters.Strategy.SourceStrategy.From, build.Config.Namespace)
		if err != nil {
			return nil, err
		}
		build.Parameters.Strategy.SourceStrategy.From = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
	case build.Parameters.Strategy.Type == buildapi.DockerBuildStrategyType &&
		build.Parameters.Strategy.DockerStrategy.From != nil:
		image, err := g.resolveImageStreamReference(ctx, build.Parameters.Strategy.DockerStrategy.From, build.Config.Namespace)
		if err != nil {
			return nil, err
		}
		build.Parameters.Strategy.DockerStrategy.From = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
	case build.Parameters.Strategy.Type == buildapi.CustomBuildStrategyType &&
		build.Parameters.Strategy.CustomStrategy.From != nil:
		image, err := g.resolveImageStreamReference(ctx, build.Parameters.Strategy.CustomStrategy.From, build.Config.Namespace)
		if err != nil {
			return nil, err
		}
		build.Parameters.Strategy.CustomStrategy.From = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		updateCustomImageEnv(build.Parameters.Strategy.CustomStrategy, image)
	}
	return build, nil
}

// resolveImageStreamReference looks up the ImageStream[Tag/Image] and converts it to a
// docker pull spec that can be used in an Image field.
func (g *BuildGenerator) resolveImageStreamReference(ctx kapi.Context, from *kapi.ObjectReference, defaultNamespace string) (string, error) {
	var namespace string
	if len(from.Namespace) != 0 {
		namespace = from.Namespace
	} else {
		namespace = defaultNamespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		image, err := g.Client.GetImageStreamImage(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error ImageStreamReference %s in namespace %s: %v", from.Name, namespace, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", fatalError{err}
		}
		glog.V(4).Infof("Resolved ImageStreamReference %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "ImageStreamTag":
		image, err := g.Client.GetImageStreamTag(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error resolving ImageStreamTag reference %s in namespace %s: %v", from.Name, namespace, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", fatalError{err}
		}
		glog.V(4).Infof("Resolved ImageStreamTag %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "DockerImage":
		return from.Name, nil
	default:
		return "", fatalError{fmt.Errorf("Unknown From Kind %s", from.Kind)}
	}
}

// getNextBuildName returns name of the next build and increments BuildConfig's LastVersion.
func getNextBuildName(bc *buildapi.BuildConfig) string {
	bc.LastVersion++
	return fmt.Sprintf("%s-%d", bc.Name, bc.LastVersion)
}

// For a custom build strategy, update base image env variable reference with the new image.
// If no env variable reference exists, create a new env variable.
func updateCustomImageEnv(strategy *buildapi.CustomBuildStrategy, newImage string) {
	if strategy.Env == nil {
		strategy.Env = make([]kapi.EnvVar, 1)
		strategy.Env[0] = kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: newImage}
	} else {
		found := false
		for i := range strategy.Env {
			glog.V(4).Infof("Checking env variable %s %s", strategy.Env[i].Name, strategy.Env[i].Value)
			if strategy.Env[i].Name == buildapi.CustomBuildStrategyBaseImageKey {
				found = true
				strategy.Env[i].Value = newImage
				glog.V(4).Infof("Updated env variable %s to %s", strategy.Env[i].Name, strategy.Env[i].Value)
				break
			}
		}
		if !found {
			strategy.Env = append(strategy.Env, kapi.EnvVar{Name: buildapi.CustomBuildStrategyBaseImageKey, Value: newImage})
		}
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
