package generator

import (
	"fmt"
	"regexp"
	"strings"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/credentialprovider"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// BuildGenerator is a central place responsible for generating new Build objects
// from BuildConfigs and other Builds.
type BuildGenerator struct {
	Client                    GeneratorClient
	DefaultServiceAccountName string
	ServiceAccounts           kclient.ServiceAccountsNamespacer
	Secrets                   kclient.SecretsNamespacer
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

// FetchServiceAccountSecrets retrieves the Secrets used for pushing and pulling
// images from private Docker registries.
func (g *BuildGenerator) FetchServiceAccountSecrets(namespace string) ([]kapi.Secret, error) {
	// TODO: Check for the service account specified in the build config when it
	//       will be added
	serviceAccount := g.DefaultServiceAccountName
	if len(serviceAccount) == 0 {
		serviceAccount = bootstrappolicy.BuilderServiceAccountName
	}
	var result []kapi.Secret
	sa, err := g.ServiceAccounts.ServiceAccounts(namespace).Get(serviceAccount)
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %q: %v", namespace, serviceAccount, err)
	}
	for _, ref := range sa.Secrets {
		secret, err := g.Secrets.Secrets(namespace).Get(ref.Name)
		if err != nil {
			continue
		}
		result = append(result, *secret)
	}
	return result, nil
}

// Instantiate returns new Build object based on a BuildRequest object
func (g *BuildGenerator) Instantiate(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating Build from BuildConfig %s/%s", request.Namespace, request.Name)
	bc, err := g.Client.GetBuildConfig(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	if request.TriggeredByImage != nil {
		for _, trigger := range bc.Triggers {
			if trigger.Type != buildapi.ImageChangeBuildTriggerType {
				continue
			}
			if trigger.ImageChange.LastTriggeredImageID == request.TriggeredByImage.Name {
				glog.V(2).Infof("Aborting imageid triggered build for BuildConfig %s/%s with imageid %s because the BuildConfig already matches this imageid", bc.Namespace, bc.Name, request.TriggeredByImage)
				return nil, fmt.Errorf("Build config %s/%s has already instantiated a build for imageid %s", bc.Namespace, bc.Name, request.TriggeredByImage.Name)
			}
			trigger.ImageChange.LastTriggeredImageID = request.TriggeredByImage.Name
		}
	}

	newBuild, err := g.generateBuildFromConfig(ctx, bc, request.Revision)
	if err != nil {
		return nil, err
	}
	glog.V(4).Infof("Build %s/%s has been generated from %s/%s BuildConfig", newBuild.Namespace, newBuild.ObjectMeta.Name, bc.Namespace, bc.ObjectMeta.Name)

	// need to update the BuildConfig because LastVersion and possibly LastTriggeredImageID changed
	if err := g.Client.UpdateBuildConfig(ctx, bc); err != nil {
		glog.V(4).Infof("Failed to update BuildConfig %s/%s so no Build will be created", bc.Namespace, bc.Name)
		return nil, err
	}

	// Ideally we would create the build *before* updating the BC to ensure that we don't set the LastTriggeredImageID
	// on the BC and then fail to create the corresponding build, however doing things in that order allows for a race
	// condition in which two builds get kicked off.  Doing it in this order ensures that we catch the race while
	// updating the BC.
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
	serviceAccount := g.DefaultServiceAccountName
	if len(serviceAccount) == 0 {
		serviceAccount = bootstrappolicy.BuilderServiceAccountName
	}
	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	obj, _ := kapi.Scheme.Copy(bc)
	bcCopy := obj.(*buildapi.BuildConfig)
	build := &buildapi.Build{
		ServiceAccount: serviceAccount,
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

	build.Config = &kapi.ObjectReference{Kind: "BuildConfig", Name: bc.Name, Namespace: bc.Namespace}
	build.Name = getNextBuildName(bc)
	if build.Labels == nil {
		build.Labels = make(map[string]string)
	}
	build.Labels[buildapi.BuildConfigLabel] = bcCopy.Name

	builderSecrets, err := g.FetchServiceAccountSecrets(bc.Namespace)
	if err != nil {
		return nil, err
	}
	if build.Parameters.Output.PushSecret == nil {
		build.Parameters.Output.PushSecret = g.resolveImageSecret(ctx, builderSecrets, build.Parameters.Output.To, bc.Namespace)
	}

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
		if build.Parameters.Strategy.SourceStrategy.PullSecret == nil {
			build.Parameters.Strategy.SourceStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, build.Parameters.Strategy.SourceStrategy.From, bc.Namespace)
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
		if build.Parameters.Strategy.DockerStrategy.PullSecret == nil {
			build.Parameters.Strategy.DockerStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, build.Parameters.Strategy.DockerStrategy.From, bc.Namespace)
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
		if build.Parameters.Strategy.CustomStrategy.PullSecret == nil {
			build.Parameters.Strategy.CustomStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, build.Parameters.Strategy.CustomStrategy.From, bc.Namespace)
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
	case "ImageStream":
		// NOTE: The 'ImageStream' reference should be used only for the 'output' image
		is, err := g.Client.GetImageStream(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error getting ImageStream %s/%s: %v", namespace, from.Name, err)
			return "", err
		}
		image, err := imageapi.DockerImageReferenceForStream(is)
		if err != nil {
			glog.V(2).Infof("Error resolving Docker image reference for %s/%s: %v", namespace, from.Name, err)
			return "", err
		}
		return image.String(), nil
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

// resolveImageSecret looks up the Secrets provided by the Service Account and
// attempt to find a best match for given image.
func (g *BuildGenerator) resolveImageSecret(ctx kapi.Context, secrets []kapi.Secret, imageRef *kapi.ObjectReference, buildNamespace string) *kapi.LocalObjectReference {
	if len(secrets) == 0 || imageRef == nil {
		return nil
	}
	emptyKeyring := credentialprovider.BasicDockerKeyring{}
	// Get the image pull spec from the image stream reference
	imageSpec, err := g.resolveImageStreamReference(ctx, imageRef, buildNamespace)
	if err != nil {
		glog.V(2).Infof("Unable to resolve the image name for %s/%s: %v", buildNamespace, imageRef, err)
		return nil
	}
	for _, secret := range secrets {
		keyring, err := credentialprovider.MakeDockerKeyring([]kapi.Secret{secret}, &emptyKeyring)
		if err != nil {
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
		if _, found := keyring.Lookup(imageSpec); found {
			return &kapi.LocalObjectReference{Name: secret.Name}
		}
	}
	glog.V(4).Infof("No secrets found for pushing or pulling the %s  %s/%s", imageRef.Kind, buildNamespace, imageRef.Name)
	return nil
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
