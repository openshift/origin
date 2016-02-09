package generator

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/credentialprovider"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"

	buildapi "github.com/openshift/origin/pkg/build/api"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/util/namer"
)

// GeneratorFatalError represents a fatal error while generating a build.
// An operation that fails because of a fatal error should not be retried.
type GeneratorFatalError struct {
	// Reason the fatal error occurred
	Reason string
}

// Error returns the error string for this fatal error
func (e GeneratorFatalError) Error() string {
	return fmt.Sprintf("fatal error generating Build from BuildConfig: %s", e.Reason)
}

// IsFatal returns true if err is a fatal error
func IsFatal(err error) bool {
	_, isFatal := err.(GeneratorFatalError)
	return isFatal
}

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

type streamRef struct {
	ref *kapi.ObjectReference
	tag string
}

// FetchServiceAccountSecrets retrieves the Secrets used for pushing and pulling
// images from private Docker registries.
func (g *BuildGenerator) FetchServiceAccountSecrets(namespace, serviceAccount string) ([]kapi.Secret, error) {
	var result []kapi.Secret
	sa, err := g.ServiceAccounts.ServiceAccounts(namespace).Get(serviceAccount)
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", namespace, serviceAccount, err)
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

// findImageChangeTrigger finds an image change trigger that has a from that matches the passed in ref
// if no match is found but there is an image change trigger with a null from, that trigger is returned
func findImageChangeTrigger(bc *buildapi.BuildConfig, ref *kapi.ObjectReference) *buildapi.ImageChangeTrigger {
	if ref == nil {
		return nil
	}
	for _, trigger := range bc.Spec.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		imageChange := trigger.ImageChange
		triggerRef := imageChange.From
		if triggerRef == nil {
			triggerRef = buildutil.GetImageStreamForStrategy(bc.Spec.Strategy)
			if triggerRef == nil || triggerRef.Kind != "ImageStreamTag" {
				continue
			}
		}
		triggerNs := triggerRef.Namespace
		if triggerNs == "" {
			triggerNs = bc.Namespace
		}
		refNs := ref.Namespace
		if refNs == "" {
			refNs = bc.Namespace
		}
		if triggerRef.Name == ref.Name && triggerNs == refNs {
			return imageChange
		}
	}
	return nil
}

func describeBuildRequest(request *buildapi.BuildRequest) string {
	desc := fmt.Sprintf("BuildConfig: %s/%s", request.Namespace, request.Name)
	if request.Revision != nil {
		desc += fmt.Sprintf(", Revision: %#v", request.Revision.Git)
	}
	if request.TriggeredByImage != nil {
		desc += fmt.Sprintf(", TriggeredBy: %s/%s with stream: %s/%s",
			request.TriggeredByImage.Kind, request.TriggeredByImage.Name,
			request.From.Kind, request.From.Name)
	}
	if request.LastVersion != nil {
		desc += fmt.Sprintf(", LastVersion: %d", *request.LastVersion)
	}
	return desc
}

// updateBuildEnv updates the strategy environment
// This will replace the existing variable definitions with provided env
func updateBuildEnv(strategy *buildapi.BuildStrategy, env []kapi.EnvVar) {
	var buildEnv *[]kapi.EnvVar

	switch {
	case strategy.SourceStrategy != nil:
		buildEnv = &strategy.SourceStrategy.Env
	case strategy.DockerStrategy != nil:
		buildEnv = &strategy.DockerStrategy.Env
	case strategy.CustomStrategy != nil:
		buildEnv = &strategy.CustomStrategy.Env
	}

	newEnv := []kapi.EnvVar{}
	for _, e := range *buildEnv {
		exists := false
		for _, n := range env {
			if e.Name == n.Name {
				exists = true
				break
			}
		}
		if !exists {
			newEnv = append(newEnv, e)
		}
	}
	newEnv = append(newEnv, env...)
	*buildEnv = newEnv
}

// Instantiate returns new Build object based on a BuildRequest object
func (g *BuildGenerator) Instantiate(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating Build from %s", describeBuildRequest(request))
	bc, err := g.Client.GetBuildConfig(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	if buildutil.IsPaused(bc) {
		return nil, &GeneratorFatalError{fmt.Sprintf("can't instantiate from BuildConfig %s/%s: BuildConfig is paused", bc.Namespace, bc.Name)}
	}

	if err := g.checkLastVersion(bc, request.LastVersion); err != nil {
		return nil, err
	}

	if err := g.updateImageTriggers(ctx, bc, request.From, request.TriggeredByImage); err != nil {
		return nil, err
	}

	newBuild, err := g.generateBuildFromConfig(ctx, bc, request.Revision, request.Binary)
	if err != nil {
		return nil, err
	}

	if len(request.Env) > 0 {
		updateBuildEnv(&newBuild.Spec.Strategy, request.Env)
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

// checkBuildConfigLastVersion will return an error if the BuildConfig's LastVersion doesn't match the passed in lastVersion
// when lastVersion is not nil
func (g *BuildGenerator) checkLastVersion(bc *buildapi.BuildConfig, lastVersion *int) error {
	if lastVersion != nil && bc.Status.LastVersion != *lastVersion {
		glog.V(2).Infof("Aborting version triggered build for BuildConfig %s/%s because the BuildConfig LastVersion (%d) does not match the requested LastVersion (%d)", bc.Namespace, bc.Name, bc.Status.LastVersion, *lastVersion)
		return fmt.Errorf("the LastVersion(%v) on build config %s/%s does not match the build request LastVersion(%d)",
			bc.Status.LastVersion, bc.Namespace, bc.Name, *lastVersion)
	}
	return nil
}

// updateImageTriggers sets the LastTriggeredImageID on all the ImageChangeTriggers on the BuildConfig and
// updates the From reference of the strategy if the strategy uses an ImageStream or ImageStreamTag reference
func (g *BuildGenerator) updateImageTriggers(ctx kapi.Context, bc *buildapi.BuildConfig, from, triggeredBy *kapi.ObjectReference) error {
	var requestTrigger *buildapi.ImageChangeTrigger
	if from != nil {
		requestTrigger = findImageChangeTrigger(bc, from)
	}
	if requestTrigger != nil && triggeredBy != nil && requestTrigger.LastTriggeredImageID == triggeredBy.Name {
		glog.V(2).Infof("Aborting imageid triggered build for BuildConfig %s/%s with imageid %s because the BuildConfig already matches this imageid", bc.Namespace, bc.Name, triggeredBy.Name)
		return fmt.Errorf("build config %s/%s has already instantiated a build for imageid %s", bc.Namespace, bc.Name, triggeredBy.Name)
	}
	// Update last triggered image id for all image change triggers
	for _, trigger := range bc.Spec.Triggers {
		if trigger.Type != buildapi.ImageChangeBuildTriggerType {
			continue
		}
		// Use the requested image id for the trigger that caused the build, otherwise resolve to the latest
		if triggeredBy != nil && trigger.ImageChange == requestTrigger {
			trigger.ImageChange.LastTriggeredImageID = triggeredBy.Name
			continue
		}

		triggerImageRef := trigger.ImageChange.From
		if triggerImageRef == nil {
			triggerImageRef = buildutil.GetImageStreamForStrategy(bc.Spec.Strategy)
		}
		if triggerImageRef == nil {
			glog.Warningf("Could not get ImageStream reference for default ImageChangeTrigger on BuildConfig %s/%s", bc.Namespace, bc.Name)
			continue
		}
		image, err := g.resolveImageStreamReference(ctx, *triggerImageRef, bc.Namespace)
		if err != nil {
			// If the trigger is for the strategy from ref, return an error
			if trigger.ImageChange.From == nil {
				return err
			}
			// Otherwise, warn that an error occurred, but continue
			glog.Warningf("Could not resolve trigger reference for build config %s/%s: %#v", bc.Namespace, bc.Name, triggerImageRef)
		}
		trigger.ImageChange.LastTriggeredImageID = image
	}
	return nil
}

// Clone returns clone of a Build
func (g *BuildGenerator) Clone(ctx kapi.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating build from build %s/%s", request.Namespace, request.Name)
	build, err := g.Client.GetBuild(ctx, request.Name)
	if err != nil {
		return nil, err
	}

	var buildConfig *buildapi.BuildConfig
	if build.Status.Config != nil {
		buildConfig, err = g.Client.GetBuildConfig(ctx, build.Status.Config.Name)
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}

		if buildutil.IsPaused(buildConfig) {
			return nil, &GeneratorFatalError{fmt.Sprintf("can't instantiate from BuildConfig %s/%s: BuildConfig is paused", buildConfig.Namespace, buildConfig.Name)}
		}
	}

	newBuild := generateBuildFromBuild(build, buildConfig)
	glog.V(4).Infof("Build %s/%s has been generated from Build %s/%s", newBuild.Namespace, newBuild.ObjectMeta.Name, build.Namespace, build.ObjectMeta.Name)

	// need to update the BuildConfig because LastVersion changed
	if buildConfig != nil {
		if err := g.Client.UpdateBuildConfig(ctx, buildConfig); err != nil {
			glog.V(4).Infof("Failed to update BuildConfig %s/%s so no Build will be created", buildConfig.Namespace, buildConfig.Name)
			return nil, err
		}
	}

	return g.createBuild(ctx, newBuild)
}

// createBuild is responsible for validating build object and saving it and returning newly created object
func (g *BuildGenerator) createBuild(ctx kapi.Context, build *buildapi.Build) (*buildapi.Build, error) {
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict(buildapi.Resource("build"), build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
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
// the Strategy, or uses the Image field of the Strategy. If binary is provided, override
// the current build strategy with a binary artifact for this specific build.
// Takes a BuildConfig to base the build on, and an optional SourceRevision to build.
func (g *BuildGenerator) generateBuildFromConfig(ctx kapi.Context, bc *buildapi.BuildConfig, revision *buildapi.SourceRevision, binary *buildapi.BinaryBuildSource) (*buildapi.Build, error) {
	serviceAccount := bc.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = g.DefaultServiceAccountName
	}
	if len(serviceAccount) == 0 {
		serviceAccount = bootstrappolicy.BuilderServiceAccountName
	}
	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	obj, _ := kapi.Scheme.Copy(bc)
	bcCopy := obj.(*buildapi.BuildConfig)
	build := &buildapi.Build{
		Spec: buildapi.BuildSpec{
			ServiceAccount:            serviceAccount,
			Source:                    bcCopy.Spec.Source,
			Strategy:                  bcCopy.Spec.Strategy,
			Output:                    bcCopy.Spec.Output,
			Revision:                  revision,
			Resources:                 bcCopy.Spec.Resources,
			CompletionDeadlineSeconds: bcCopy.Spec.CompletionDeadlineSeconds,
		},
		ObjectMeta: kapi.ObjectMeta{
			Labels: bcCopy.Labels,
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
			Config: &kapi.ObjectReference{
				Kind:      "BuildConfig",
				Name:      bc.Name,
				Namespace: bc.Namespace,
			},
		},
	}

	if binary != nil {
		build.Spec.Source.Git = nil
		build.Spec.Source.Binary = binary
		if build.Spec.Source.Dockerfile != nil && binary.AsFile == "Dockerfile" {
			build.Spec.Source.Dockerfile = nil
		}
	}

	build.Name = getNextBuildName(bc)
	if build.Annotations == nil {
		build.Annotations = make(map[string]string)
	}
	build.Annotations[buildapi.BuildNumberAnnotation] = strconv.Itoa(bc.Status.LastVersion)
	if build.Labels == nil {
		build.Labels = make(map[string]string)
	}
	build.Labels[buildapi.BuildConfigLabelDeprecated] = bcCopy.Name
	build.Labels[buildapi.BuildConfigLabel] = bcCopy.Name

	builderSecrets, err := g.FetchServiceAccountSecrets(bc.Namespace, serviceAccount)
	if err != nil {
		return nil, err
	}
	if build.Spec.Output.PushSecret == nil {
		build.Spec.Output.PushSecret = g.resolveImageSecret(ctx, builderSecrets, build.Spec.Output.To, bc.Namespace)
	}
	strategyImageChangeTrigger := getStrategyImageChangeTrigger(bc)

	// Resolve image source if present
	for i, sourceImage := range build.Spec.Source.Images {
		if sourceImage.PullSecret == nil {
			sourceImage.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &sourceImage.From, bc.Namespace)
		}
		sourceImageSpec, err := g.resolveImageStreamReference(ctx, sourceImage.From, bc.Namespace)
		if err != nil {
			return nil, err
		}
		sourceImage.From.Kind = "DockerImage"
		sourceImage.From.Name = sourceImageSpec
		sourceImage.From.Namespace = ""
		build.Spec.Source.Images[i] = sourceImage
	}

	// If the Build is using a From reference instead of a resolved image, we need to resolve that From
	// reference to a valid image so we can run the build.  Builds do not consume ImageStream references,
	// only image specs.
	var image string
	if strategyImageChangeTrigger != nil {
		image = strategyImageChangeTrigger.LastTriggeredImageID
	}

	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, build.Spec.Strategy.SourceStrategy.From, build.Status.Config.Namespace)
			if err != nil {
				return nil, err
			}
		}
		build.Spec.Strategy.SourceStrategy.From = kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if build.Spec.Strategy.SourceStrategy.PullSecret == nil {
			build.Spec.Strategy.SourceStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &build.Spec.Strategy.SourceStrategy.From, bc.Namespace)
		}
	case build.Spec.Strategy.DockerStrategy != nil &&
		build.Spec.Strategy.DockerStrategy.From != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, *build.Spec.Strategy.DockerStrategy.From, build.Status.Config.Namespace)
			if err != nil {
				return nil, err
			}
		}
		build.Spec.Strategy.DockerStrategy.From = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if build.Spec.Strategy.DockerStrategy.PullSecret == nil {
			build.Spec.Strategy.DockerStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, build.Spec.Strategy.DockerStrategy.From, bc.Namespace)
		}
	case build.Spec.Strategy.CustomStrategy != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, build.Spec.Strategy.CustomStrategy.From, build.Status.Config.Namespace)
			if err != nil {
				return nil, err
			}
		}
		build.Spec.Strategy.CustomStrategy.From = kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if build.Spec.Strategy.CustomStrategy.PullSecret == nil {
			build.Spec.Strategy.CustomStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &build.Spec.Strategy.CustomStrategy.From, bc.Namespace)
		}
		updateCustomImageEnv(build.Spec.Strategy.CustomStrategy, image)
	}
	return build, nil
}

// resolveImageStreamReference looks up the ImageStream[Tag/Image] and converts it to a
// docker pull spec that can be used in an Image field.
func (g *BuildGenerator) resolveImageStreamReference(ctx kapi.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	var namespace string
	if len(from.Namespace) != 0 {
		namespace = from.Namespace
	} else {
		namespace = defaultNamespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		imageStreamImage, err := g.Client.GetImageStreamImage(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error ImageStreamReference %s in namespace %s: %v", from.Name, namespace, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", err
		}
		image := imageStreamImage.Image
		glog.V(4).Infof("Resolved ImageStreamReference %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "ImageStreamTag":
		imageStreamTag, err := g.Client.GetImageStreamTag(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error resolving ImageStreamTag reference %s in namespace %s: %v", from.Name, namespace, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", err
		}
		image := imageStreamTag.Image
		glog.V(4).Infof("Resolved ImageStreamTag %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "DockerImage":
		return from.Name, nil
	default:
		return "", fmt.Errorf("Unknown From Kind %s", from.Kind)
	}
}

// resolveImageStreamDockerRepository looks up the ImageStream[Tag/Image] and converts it to a
// the docker repository reference with no tag information
func (g *BuildGenerator) resolveImageStreamDockerRepository(ctx kapi.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	namespace := defaultNamespace
	if len(from.Namespace) > 0 {
		namespace = from.Namespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		imageStreamImage, err := g.Client.GetImageStreamImage(kapi.WithNamespace(ctx, namespace), from.Name)
		if err != nil {
			glog.V(2).Infof("Error ImageStreamReference %s in namespace %s: %v", from.Name, namespace, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", err
		}
		image := imageStreamImage.Image
		glog.V(4).Infof("Resolved ImageStreamReference %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "ImageStreamTag":
		name := strings.Split(from.Name, ":")[0]
		is, err := g.Client.GetImageStream(kapi.WithNamespace(ctx, namespace), name)
		if err != nil {
			glog.V(2).Infof("Error getting ImageStream %s/%s: %v", namespace, name, err)
			if errors.IsNotFound(err) {
				return "", err
			}
			return "", err
		}
		image, err := imageapi.DockerImageReferenceForStream(is)
		if err != nil {
			glog.V(2).Infof("Error resolving Docker image reference for %s/%s: %v", namespace, name, err)
			return "", err
		}
		glog.V(4).Infof("Resolved ImageStreamTag %s/%s to repository %s", namespace, from.Name, image)
		return image.String(), nil
	case "DockerImage":
		return from.Name, nil
	default:
		return "", fmt.Errorf("Unknown From Kind %s", from.Kind)
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
	imageSpec, err := g.resolveImageStreamDockerRepository(ctx, *imageRef, buildNamespace)
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
func getNextBuildName(buildConfig *buildapi.BuildConfig) string {
	buildConfig.Status.LastVersion++
	return fmt.Sprintf("%s-%d", buildConfig.Name, buildConfig.Status.LastVersion)
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
func generateBuildFromBuild(build *buildapi.Build, buildConfig *buildapi.BuildConfig) *buildapi.Build {
	obj, _ := kapi.Scheme.Copy(build)
	buildCopy := obj.(*buildapi.Build)

	newBuild := &buildapi.Build{
		Spec: buildCopy.Spec,
		ObjectMeta: kapi.ObjectMeta{
			Name:        getNextBuildNameFromBuild(buildCopy, buildConfig),
			Labels:      buildCopy.ObjectMeta.Labels,
			Annotations: buildCopy.ObjectMeta.Annotations,
		},
		Status: buildapi.BuildStatus{
			Phase:  buildapi.BuildPhaseNew,
			Config: buildCopy.Status.Config,
		},
	}
	if newBuild.Annotations == nil {
		newBuild.Annotations = make(map[string]string)
	}
	newBuild.Annotations[buildapi.BuildCloneAnnotation] = build.Name
	if buildConfig != nil {
		newBuild.Annotations[buildapi.BuildNumberAnnotation] = strconv.Itoa(buildConfig.Status.LastVersion)
	} else {
		// builds without a buildconfig don't have build numbers.
		delete(newBuild.Annotations, buildapi.BuildNumberAnnotation)
	}
	return newBuild
}

// getNextBuildNameFromBuild returns name of the next build with random uuid added at the end
func getNextBuildNameFromBuild(build *buildapi.Build, buildConfig *buildapi.BuildConfig) string {
	var buildName string
	if buildConfig != nil {
		return getNextBuildName(buildConfig)
	}
	// for builds created by hand, append a timestamp when cloning/rebuilding them
	// because we don't have a sequence number to bump.
	buildName = build.Name
	// remove the old timestamp if we're cloning a build that is itself a clone.
	if matched, _ := regexp.MatchString(`^.+-\d{10}$`, buildName); matched {
		nameElems := strings.Split(buildName, "-")
		buildName = strings.Join(nameElems[:len(nameElems)-1], "-")
	}
	suffix := fmt.Sprintf("%v", unversioned.Now().UnixNano())
	if len(suffix) > 10 {
		suffix = suffix[len(suffix)-10:]
	}
	return namer.GetName(buildName, suffix, kvalidation.DNS1123SubdomainMaxLength)

}

// getStrategyImageChangeTrigger returns the ImageChangeTrigger that corresponds to the BuildConfig's strategy
func getStrategyImageChangeTrigger(bc *buildapi.BuildConfig) *buildapi.ImageChangeTrigger {
	for _, trigger := range bc.Spec.Triggers {
		if trigger.Type == buildapi.ImageChangeBuildTriggerType && trigger.ImageChange.From == nil {
			return trigger.ImageChange
		}
	}
	return nil
}
