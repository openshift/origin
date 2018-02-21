package generator

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildclient "github.com/openshift/origin/pkg/build/generated/internalclientset/typed/build/internalversion"
	buildutil "github.com/openshift/origin/pkg/build/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
)

const conflictRetries = 3

// GeneratorFatalError represents a fatal error while generating a build.
// An operation that fails because of a fatal error should not be retried.
type GeneratorFatalError struct {
	// Reason the fatal error occurred
	Reason string
}

// Error returns the error string for this fatal error
func (e *GeneratorFatalError) Error() string {
	return fmt.Sprintf("fatal error generating Build from BuildConfig: %s", e.Reason)
}

// IsFatal returns true if err is a fatal error
func IsFatal(err error) bool {
	_, isFatal := err.(*GeneratorFatalError)
	return isFatal
}

// BuildGenerator is a central place responsible for generating new Build objects
// from BuildConfigs and other Builds.
type BuildGenerator struct {
	Client          GeneratorClient
	ServiceAccounts kcoreclient.ServiceAccountsGetter
	Secrets         kcoreclient.SecretsGetter
}

// GeneratorClient is the API client used by the generator
type GeneratorClient interface {
	GetBuildConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error
	GetBuild(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error)
	CreateBuild(ctx apirequest.Context, build *buildapi.Build) error
	UpdateBuild(ctx apirequest.Context, build *buildapi.Build) error
	GetImageStream(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error)
	GetImageStreamImage(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error)
	GetImageStreamTag(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error)
}

// Client is an implementation of the GeneratorClient interface
type Client struct {
	BuildConfigs      buildclient.BuildConfigsGetter
	Builds            buildclient.BuildsGetter
	ImageStreams      imageclient.ImageStreamsGetter
	ImageStreamImages imageclient.ImageStreamImagesGetter
	ImageStreamTags   imageclient.ImageStreamTagsGetter
}

// GetBuildConfig retrieves a named build config
func (c Client) GetBuildConfig(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.BuildConfig, error) {
	return c.BuildConfigs.BuildConfigs(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

// UpdateBuildConfig updates a named build config
func (c Client) UpdateBuildConfig(ctx apirequest.Context, buildConfig *buildapi.BuildConfig) error {
	_, err := c.BuildConfigs.BuildConfigs(apirequest.NamespaceValue(ctx)).Update(buildConfig)
	return err
}

// GetBuild retrieves a build
func (c Client) GetBuild(ctx apirequest.Context, name string, options *metav1.GetOptions) (*buildapi.Build, error) {
	return c.Builds.Builds(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

// CreateBuild creates a new build
func (c Client) CreateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	_, err := c.Builds.Builds(apirequest.NamespaceValue(ctx)).Create(build)
	return err
}

// UpdateBuild updates a build
func (c Client) UpdateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	_, err := c.Builds.Builds(apirequest.NamespaceValue(ctx)).Update(build)
	return err
}

// GetImageStream retrieves a named image stream
func (c Client) GetImageStream(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStream, error) {
	return c.ImageStreams.ImageStreams(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

// GetImageStreamImage retrieves an image stream image
func (c Client) GetImageStreamImage(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamImage, error) {
	return c.ImageStreamImages.ImageStreamImages(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

// GetImageStreamTag retrieves and image stream tag
func (c Client) GetImageStreamTag(ctx apirequest.Context, name string, options *metav1.GetOptions) (*imageapi.ImageStreamTag, error) {
	return c.ImageStreamTags.ImageStreamTags(apirequest.NamespaceValue(ctx)).Get(name, *options)
}

// FetchServiceAccountSecrets retrieves the Secrets used for pushing and pulling
// images from private Docker registries.
func FetchServiceAccountSecrets(secrets kcoreclient.SecretsGetter, serviceAccounts kcoreclient.ServiceAccountsGetter, namespace, serviceAccount string) ([]kapi.Secret, error) {
	var result []kapi.Secret
	sa, err := serviceAccounts.ServiceAccounts(namespace).Get(serviceAccount, metav1.GetOptions{})
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", namespace, serviceAccount, err)
	}
	for _, ref := range sa.Secrets {
		secret, err := secrets.Secrets(namespace).Get(ref.Name, metav1.GetOptions{})
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
			triggerRef = buildapi.GetInputReference(bc.Spec.Strategy)
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

// Adds new Build Args to existing Build Args. Overwrites existing ones
func updateBuildArgs(oldArgs *[]kapi.EnvVar, newArgs []kapi.EnvVar) []kapi.EnvVar {
	combined := make(map[string]string)

	// Change oldArgs into a map
	for _, o := range *oldArgs {
		combined[o.Name] = o.Value
	}

	// Add new args, this overwrites old
	for _, n := range newArgs {
		combined[n.Name] = n.Value
	}

	// Change back into an array
	var result []kapi.EnvVar
	for k, v := range combined {
		result = append(result, kapi.EnvVar{Name: k, Value: v})
	}

	return result
}

// Instantiate returns a new Build object based on a BuildRequest object
func (g *BuildGenerator) Instantiate(ctx apirequest.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	var build *buildapi.Build
	var err error

	for i := 0; i < conflictRetries; i++ {
		build, err = g.instantiate(ctx, request)
		if err == nil || !errors.IsConflict(err) {
			break
		}
		glog.V(4).Infof("instantiate returned conflict, try %d/%d", i+1, conflictRetries)
	}

	return build, err
}

func (g *BuildGenerator) instantiate(ctx apirequest.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating Build from %s", describeBuildRequest(request))
	bc, err := g.Client.GetBuildConfig(ctx, request.Name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if buildutil.IsPaused(bc) {
		return nil, errors.NewBadRequest(fmt.Sprintf("can't instantiate from BuildConfig %s/%s: BuildConfig is paused", bc.Namespace, bc.Name))
	}

	if err := g.checkLastVersion(bc, request.LastVersion); err != nil {
		return nil, errors.NewBadRequest(err.Error())
	}

	if err := g.updateImageTriggers(ctx, bc, request.From, request.TriggeredByImage); err != nil {
		if _, ok := err.(errors.APIStatus); ok {
			return nil, err
		}
		return nil, errors.NewInternalError(err)
	}

	newBuild, err := g.generateBuildFromConfig(ctx, bc, request.Revision, request.Binary)
	if err != nil {
		if _, ok := err.(errors.APIStatus); ok {
			return nil, err
		}
		return nil, errors.NewInternalError(err)
	}

	// Add labels and annotations from the buildrequest.  Existing
	// label/annotations will take precedence because we don't want system
	// annotations/labels (eg buildname) to get stomped on.
	newBuild.Annotations = mergeMaps(request.Annotations, newBuild.Annotations)
	newBuild.Labels = mergeMaps(request.Labels, newBuild.Labels)

	// Copy build trigger information and build arguments to the build object.
	newBuild.Spec.TriggeredBy = request.TriggeredBy

	if len(request.Env) > 0 {
		buildutil.UpdateBuildEnv(newBuild, request.Env)
	}

	// Update the Docker strategy options
	if request.DockerStrategyOptions != nil {
		dockerOpts := request.DockerStrategyOptions

		// Update the Docker build args
		if dockerOpts.BuildArgs != nil && len(dockerOpts.BuildArgs) > 0 {
			if newBuild.Spec.Strategy.DockerStrategy == nil {
				return nil, errors.NewBadRequest(fmt.Sprintf("Cannot specify Docker build specific options on %s/%s, not a Docker build.", bc.Namespace, bc.ObjectMeta.Name))
			}
			newBuild.Spec.Strategy.DockerStrategy.BuildArgs = updateBuildArgs(&newBuild.Spec.Strategy.DockerStrategy.BuildArgs, dockerOpts.BuildArgs)
		}

		// Update the Docker noCache option
		if dockerOpts.NoCache != nil {
			if newBuild.Spec.Strategy.DockerStrategy == nil {
				return nil, errors.NewBadRequest(fmt.Sprintf("Cannot specify Docker build specific options on %s/%s, not a Docker build.", bc.Namespace, bc.ObjectMeta.Name))
			}
			newBuild.Spec.Strategy.DockerStrategy.NoCache = *dockerOpts.NoCache
		}
	}

	// Update the Source strategy options
	if request.SourceStrategyOptions != nil {
		sourceOpts := request.SourceStrategyOptions

		// Update the Source incremental option
		if sourceOpts.Incremental != nil {
			if newBuild.Spec.Strategy.SourceStrategy == nil {
				return nil, errors.NewBadRequest(fmt.Sprintf("Cannot specify Source build specific options on %s/%s, not a Source build.", bc.Namespace, bc.ObjectMeta.Name))
			}
			newBuild.Spec.Strategy.SourceStrategy.Incremental = sourceOpts.Incremental
		}
	}
	glog.V(4).Infof("Build %s/%s has been generated from %s/%s BuildConfig", newBuild.Namespace, newBuild.ObjectMeta.Name, bc.Namespace, bc.ObjectMeta.Name)

	// need to update the BuildConfig because LastVersion and possibly
	// LastTriggeredImageID changed
	if err := g.Client.UpdateBuildConfig(ctx, bc); err != nil {
		glog.V(4).Infof("Failed to update BuildConfig %s/%s so no Build will be created", bc.Namespace, bc.Name)
		return nil, err
	}

	// Ideally we would create the build *before* updating the BC to ensure
	// that we don't set the LastTriggeredImageID on the BC and then fail to
	// create the corresponding build, however doing things in that order
	// allows for a race condition in which two builds get kicked off.  Doing
	// it in this order ensures that we catch the race while updating the BC.
	return g.createBuild(ctx, newBuild)
}

// checkLastVersion will return an error if the BuildConfig's LastVersion doesn't match the passed in lastVersion
// when lastVersion is not nil
func (g *BuildGenerator) checkLastVersion(bc *buildapi.BuildConfig, lastVersion *int64) error {
	if lastVersion != nil && bc.Status.LastVersion != *lastVersion {
		glog.V(2).Infof("Aborting version triggered build for BuildConfig %s/%s because the BuildConfig LastVersion (%d) does not match the requested LastVersion (%d)", bc.Namespace, bc.Name, bc.Status.LastVersion, *lastVersion)
		return fmt.Errorf("the LastVersion(%v) on build config %s/%s does not match the build request LastVersion(%d)",
			bc.Status.LastVersion, bc.Namespace, bc.Name, *lastVersion)
	}
	return nil
}

// updateImageTriggers sets the LastTriggeredImageID on all the ImageChangeTriggers on the BuildConfig and
// updates the From reference of the strategy if the strategy uses an ImageStream or ImageStreamTag reference
func (g *BuildGenerator) updateImageTriggers(ctx apirequest.Context, bc *buildapi.BuildConfig, from, triggeredBy *kapi.ObjectReference) error {
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
			triggerImageRef = buildapi.GetInputReference(bc.Spec.Strategy)
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
func (g *BuildGenerator) Clone(ctx apirequest.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	var build *buildapi.Build
	var err error

	for i := 0; i < conflictRetries; i++ {
		build, err = g.clone(ctx, request)
		if err == nil || !errors.IsConflict(err) {
			break
		}
		glog.V(4).Infof("clone returned conflict, try %d/%d", i+1, conflictRetries)
	}

	return build, err
}

func (g *BuildGenerator) clone(ctx apirequest.Context, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	glog.V(4).Infof("Generating build from build %s/%s", request.Namespace, request.Name)
	build, err := g.Client.GetBuild(ctx, request.Name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	var buildConfig *buildapi.BuildConfig
	if build.Status.Config != nil {
		buildConfig, err = g.Client.GetBuildConfig(ctx, build.Status.Config.Name, &metav1.GetOptions{})
		if err != nil && !errors.IsNotFound(err) {
			return nil, err
		}
		if buildutil.IsPaused(buildConfig) {
			return nil, errors.NewInternalError(&GeneratorFatalError{fmt.Sprintf("can't instantiate from BuildConfig %s/%s: BuildConfig is paused", buildConfig.Namespace, buildConfig.Name)})
		}
	}

	newBuild := generateBuildFromBuild(build, buildConfig)
	glog.V(4).Infof("Build %s/%s has been generated from Build %s/%s", newBuild.Namespace, newBuild.ObjectMeta.Name, build.Namespace, build.ObjectMeta.Name)

	// Copy build trigger information to the build object.
	newBuild.Spec.TriggeredBy = request.TriggeredBy

	if len(request.Env) > 0 {
		buildutil.UpdateBuildEnv(newBuild, request.Env)
	}

	// Update the Docker build args
	if request.DockerStrategyOptions != nil {
		dockerOpts := request.DockerStrategyOptions
		if dockerOpts.BuildArgs != nil && len(dockerOpts.BuildArgs) > 0 {
			if newBuild.Spec.Strategy.DockerStrategy == nil {
				return nil, errors.NewBadRequest(fmt.Sprintf("Cannot specify build args on %s/%s, not a Docker build.", buildConfig.Namespace, buildConfig.ObjectMeta.Name))
			}
			newBuild.Spec.Strategy.DockerStrategy.BuildArgs = updateBuildArgs(&newBuild.Spec.Strategy.DockerStrategy.BuildArgs, dockerOpts.BuildArgs)
		}
	}

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
func (g *BuildGenerator) createBuild(ctx apirequest.Context, build *buildapi.Build) (*buildapi.Build, error) {
	if !rest.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict(buildapi.Resource("build"), build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}
	rest.FillObjectMetaSystemFields(ctx, &build.ObjectMeta)
	err := g.Client.CreateBuild(ctx, build)
	if err != nil {
		return nil, err
	}
	return g.Client.GetBuild(ctx, build.Name, &metav1.GetOptions{})
}

// generateBuildFromConfig generates a build definition based on the current imageid
// from any ImageStream that is associated to the BuildConfig by From reference in
// the Strategy, or uses the Image field of the Strategy. If binary is provided, override
// the current build strategy with a binary artifact for this specific build.
// Takes a BuildConfig to base the build on, and an optional SourceRevision to build.
func (g *BuildGenerator) generateBuildFromConfig(ctx apirequest.Context, bc *buildapi.BuildConfig, revision *buildapi.SourceRevision, binary *buildapi.BinaryBuildSource) (*buildapi.Build, error) {

	// Need to copy the buildConfig here so that it doesn't share pointers with
	// the build object which could be (will be) modified later.
	buildName := getNextBuildName(bc)
	bcCopy := bc.DeepCopy()
	serviceAccount := bcCopy.Spec.ServiceAccount
	if len(serviceAccount) == 0 {
		serviceAccount = bootstrappolicy.BuilderServiceAccountName
	}
	t := true
	build := &buildapi.Build{
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				ServiceAccount:            serviceAccount,
				Source:                    bcCopy.Spec.Source,
				Strategy:                  bcCopy.Spec.Strategy,
				Output:                    bcCopy.Spec.Output,
				Revision:                  revision,
				Resources:                 bcCopy.Spec.Resources,
				PostCommit:                bcCopy.Spec.PostCommit,
				CompletionDeadlineSeconds: bcCopy.Spec.CompletionDeadlineSeconds,
				NodeSelector:              bcCopy.Spec.NodeSelector,
			},
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   buildName,
			Labels: bcCopy.Labels,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: buildapiv1.SchemeGroupVersion.String(), // BuildConfig.APIVersion is not populated
					Kind:       "BuildConfig",                          // BuildConfig.Kind is not populated
					Name:       bcCopy.Name,
					UID:        bcCopy.UID,
					Controller: &t,
				},
			},
		},
		Status: buildapi.BuildStatus{
			Phase: buildapi.BuildPhaseNew,
			Config: &kapi.ObjectReference{
				Kind:      "BuildConfig",
				Name:      bcCopy.Name,
				Namespace: bcCopy.Namespace,
			},
		},
	}

	setBuildSource(binary, build)
	setBuildAnnotationAndLabel(bcCopy, build)

	var builderSecrets []kapi.Secret
	var err error
	if builderSecrets, err = FetchServiceAccountSecrets(g.Secrets, g.ServiceAccounts, bcCopy.Namespace, serviceAccount); err != nil {
		return nil, err
	}

	// Resolve image source if present
	if err = g.setBuildSourceImage(ctx, builderSecrets, bcCopy, &build.Spec.Source); err != nil {
		return nil, err
	}
	if err = g.setBaseImageAndPullSecretForBuildStrategy(ctx, builderSecrets, bcCopy, &build.Spec.Strategy); err != nil {
		return nil, err
	}

	return build, nil
}

// setBuildSourceImage set BuildSource Image item for new build
func (g *BuildGenerator) setBuildSourceImage(ctx apirequest.Context, builderSecrets []kapi.Secret, bcCopy *buildapi.BuildConfig, Source *buildapi.BuildSource) error {
	var err error

	strategyImageChangeTrigger := getStrategyImageChangeTrigger(bcCopy)
	for i, sourceImage := range Source.Images {
		if sourceImage.PullSecret == nil {
			sourceImage.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &sourceImage.From, bcCopy.Namespace)
		}

		var sourceImageSpec string
		// if the imagesource matches the strategy from, and we have a trigger for the strategy from,
		// use the imageid from the trigger rather than resolving it.
		if strategyFrom := buildapi.GetInputReference(bcCopy.Spec.Strategy); strategyFrom != nil &&
			reflect.DeepEqual(sourceImage.From, *strategyFrom) &&
			strategyImageChangeTrigger != nil {
			sourceImageSpec = strategyImageChangeTrigger.LastTriggeredImageID
		} else {
			refImageChangeTrigger := getImageChangeTriggerForRef(bcCopy, &sourceImage.From)
			// if there is no trigger associated with this imagesource, resolve the imagesource reference now.
			// otherwise use the imageid from the imagesource trigger.
			if refImageChangeTrigger == nil {
				sourceImageSpec, err = g.resolveImageStreamReference(ctx, sourceImage.From, bcCopy.Namespace)
				if err != nil {
					return err
				}
			} else {
				sourceImageSpec = refImageChangeTrigger.LastTriggeredImageID
			}
		}

		sourceImage.From.Kind = "DockerImage"
		sourceImage.From.Name = sourceImageSpec
		sourceImage.From.Namespace = ""
		Source.Images[i] = sourceImage
	}

	return nil
}

// setBaseImageAndPullSecretForBuildStrategy sets base image and pullSecret items used in buildStrategy for new builds
func (g *BuildGenerator) setBaseImageAndPullSecretForBuildStrategy(ctx apirequest.Context, builderSecrets []kapi.Secret, bcCopy *buildapi.BuildConfig, strategy *buildapi.BuildStrategy) error {
	var err error
	var image string

	if strategyImageChangeTrigger := getStrategyImageChangeTrigger(bcCopy); strategyImageChangeTrigger != nil {
		image = strategyImageChangeTrigger.LastTriggeredImageID
	}
	// If the Build is using a From reference instead of a resolved image, we need to resolve that From
	// reference to a valid image so we can run the build.  Builds do not consume ImageStream references,
	// only image specs.
	switch {
	case strategy.SourceStrategy != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, strategy.SourceStrategy.From, bcCopy.Namespace)
			if err != nil {
				return err
			}
		}
		strategy.SourceStrategy.From = kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if strategy.SourceStrategy.PullSecret == nil {
			strategy.SourceStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &strategy.SourceStrategy.From, bcCopy.Namespace)
		}
	case strategy.DockerStrategy != nil &&
		strategy.DockerStrategy.From != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, *strategy.DockerStrategy.From, bcCopy.Namespace)
			if err != nil {
				return err
			}
		}
		strategy.DockerStrategy.From = &kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if strategy.DockerStrategy.PullSecret == nil {
			strategy.DockerStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, strategy.DockerStrategy.From, bcCopy.Namespace)
		}
	case strategy.CustomStrategy != nil:
		if image == "" {
			image, err = g.resolveImageStreamReference(ctx, strategy.CustomStrategy.From, bcCopy.Namespace)
			if err != nil {
				return err
			}
		}
		strategy.CustomStrategy.From = kapi.ObjectReference{
			Kind: "DockerImage",
			Name: image,
		}
		if strategy.CustomStrategy.PullSecret == nil {
			strategy.CustomStrategy.PullSecret = g.resolveImageSecret(ctx, builderSecrets, &strategy.CustomStrategy.From, bcCopy.Namespace)
		}
		UpdateCustomImageEnv(strategy.CustomStrategy, image)
	}
	return nil
}

// resolveImageStreamReference looks up the ImageStream[Tag/Image] and converts it to a
// docker pull spec that can be used in an Image field.
func (g *BuildGenerator) resolveImageStreamReference(ctx apirequest.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	var namespace string
	if len(from.Namespace) != 0 {
		namespace = from.Namespace
	} else {
		namespace = defaultNamespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		name, id, err := imageapi.ParseImageStreamImageName(from.Name)
		if err != nil {
			err = resolveError(from.Kind, namespace, from.Name, err)
			glog.V(2).Info(err)
			return "", err
		}
		stream, err := g.Client.GetImageStream(apirequest.WithNamespace(ctx, namespace), name, &metav1.GetOptions{})
		if err != nil {
			err = resolveError(from.Kind, namespace, from.Name, err)
			glog.V(2).Info(err)
			return "", err
		}
		reference, ok := imageapi.DockerImageReferenceForImage(stream, id)
		if !ok {
			err = resolveError(from.Kind, namespace, from.Name, fmt.Errorf("unable to find corresponding tag for image %q", id))
			glog.V(2).Info(err)
			return "", err
		}
		glog.V(4).Infof("Resolved ImageStreamImage %s to image %q", from.Name, reference)
		return reference, nil

	case "ImageStreamTag":
		name, tag, err := imageapi.ParseImageStreamTagName(from.Name)
		if err != nil {
			err = resolveError(from.Kind, namespace, from.Name, err)
			glog.V(2).Info(err)
			return "", err
		}
		stream, err := g.Client.GetImageStream(apirequest.WithNamespace(ctx, namespace), name, &metav1.GetOptions{})
		if err != nil {
			err = resolveError(from.Kind, namespace, from.Name, err)
			glog.V(2).Info(err)
			return "", err
		}
		reference, ok := imageapi.ResolveLatestTaggedImage(stream, tag)
		if !ok {
			err = resolveError(from.Kind, namespace, from.Name, fmt.Errorf("unable to find latest tagged image"))
			glog.V(2).Info(err)
			return "", err
		}
		glog.V(4).Infof("Resolved ImageStreamTag %s to image %q", from.Name, reference)
		return reference, nil
	case "DockerImage":
		return from.Name, nil
	default:
		return "", fmt.Errorf("Unknown From Kind %s", from.Kind)
	}
}

// resolveImageStreamDockerRepository looks up the ImageStream[Tag/Image] and converts it to a
// the docker repository reference with no tag information
func (g *BuildGenerator) resolveImageStreamDockerRepository(ctx apirequest.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	namespace := defaultNamespace
	if len(from.Namespace) > 0 {
		namespace = from.Namespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		imageStreamImage, err := g.Client.GetImageStreamImage(apirequest.WithNamespace(ctx, namespace), from.Name, &metav1.GetOptions{})
		if err != nil {
			err = resolveError(from.Kind, namespace, from.Name, err)
			glog.V(2).Info(err)
			return "", err
		}
		image := imageStreamImage.Image
		glog.V(4).Infof("Resolved ImageStreamReference %s to image %s with reference %s in namespace %s", from.Name, image.Name, image.DockerImageReference, namespace)
		return image.DockerImageReference, nil
	case "ImageStreamTag":
		name := strings.Split(from.Name, ":")[0]
		is, err := g.Client.GetImageStream(apirequest.WithNamespace(ctx, namespace), name, &metav1.GetOptions{})
		if err != nil {
			err = resolveError("ImageStream", namespace, from.Name, err)
			glog.V(2).Info(err)
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
func (g *BuildGenerator) resolveImageSecret(ctx apirequest.Context, secrets []kapi.Secret, imageRef *kapi.ObjectReference, buildNamespace string) *kapi.LocalObjectReference {
	if len(secrets) == 0 || imageRef == nil {
		return nil
	}
	// Get the image pull spec from the image stream reference
	imageSpec, err := g.resolveImageStreamDockerRepository(ctx, *imageRef, buildNamespace)
	if err != nil {
		glog.V(2).Infof("Unable to resolve the image name for %s/%s: %v", buildNamespace, imageRef, err)
		return nil
	}
	s := buildutil.FindDockerSecretAsReference(secrets, imageSpec)
	if s == nil {
		glog.V(4).Infof("No secrets found for pushing or pulling the %s  %s/%s", imageRef.Kind, buildNamespace, imageRef.Name)
	}
	return s
}

func resolveError(kind string, namespace string, name string, err error) error {
	msg := fmt.Sprintf("Error resolving %s %s in namespace %s: %v", kind, name, namespace, err)
	return &errors.StatusError{ErrStatus: metav1.Status{
		Status:  metav1.StatusFailure,
		Code:    http.StatusUnprocessableEntity,
		Reason:  metav1.StatusReasonInvalid,
		Message: msg,
		Details: &metav1.StatusDetails{
			Kind: kind,
			Name: name,
			Causes: []metav1.StatusCause{{
				Field:   "from",
				Message: msg,
			}},
		},
	}}
}

// getNextBuildName returns name of the next build and increments BuildConfig's LastVersion.
func getNextBuildName(buildConfig *buildapi.BuildConfig) string {
	buildConfig.Status.LastVersion++
	return apihelpers.GetName(buildConfig.Name, strconv.FormatInt(buildConfig.Status.LastVersion, 10), kvalidation.DNS1123SubdomainMaxLength)
}

// UpdateCustomImageEnv updates base image env variable reference with the new image for a custom build strategy.
// If no env variable reference exists, create a new env variable.
func UpdateCustomImageEnv(strategy *buildapi.CustomBuildStrategy, newImage string) {
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
	buildCopy := build.DeepCopy()

	newBuild := &buildapi.Build{
		Spec: buildCopy.Spec,
		ObjectMeta: metav1.ObjectMeta{
			Name:            getNextBuildNameFromBuild(buildCopy, buildConfig),
			Labels:          buildCopy.ObjectMeta.Labels,
			Annotations:     buildCopy.ObjectMeta.Annotations,
			OwnerReferences: buildCopy.ObjectMeta.OwnerReferences,
		},
		Status: buildapi.BuildStatus{
			Phase:  buildapi.BuildPhaseNew,
			Config: buildCopy.Status.Config,
		},
	}
	// TODO remove/update this when we support cloning binary builds
	newBuild.Spec.Source.Binary = nil
	if newBuild.Annotations == nil {
		newBuild.Annotations = make(map[string]string)
	}
	newBuild.Annotations[buildapi.BuildCloneAnnotation] = build.Name
	if buildConfig != nil {
		newBuild.Annotations[buildapi.BuildNumberAnnotation] = strconv.FormatInt(buildConfig.Status.LastVersion, 10)
	} else {
		// builds without a buildconfig don't have build numbers.
		delete(newBuild.Annotations, buildapi.BuildNumberAnnotation)
	}

	// if they exist, Jenkins reporting annotations must be removed when cloning.
	delete(newBuild.Annotations, buildapi.BuildJenkinsStatusJSONAnnotation)
	delete(newBuild.Annotations, buildapi.BuildJenkinsLogURLAnnotation)
	delete(newBuild.Annotations, buildapi.BuildJenkinsConsoleLogURLAnnotation)
	delete(newBuild.Annotations, buildapi.BuildJenkinsBlueOceanLogURLAnnotation)
	delete(newBuild.Annotations, buildapi.BuildJenkinsBuildURIAnnotation)

	// remove the BuildPodNameAnnotation for good measure.
	delete(newBuild.Annotations, buildapi.BuildPodNameAnnotation)

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
	suffix := fmt.Sprintf("%v", metav1.Now().UnixNano())
	if len(suffix) > 10 {
		suffix = suffix[len(suffix)-10:]
	}
	return apihelpers.GetName(buildName, suffix, kvalidation.DNS1123SubdomainMaxLength)

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

// getImageChangeTriggerForRef returns the ImageChangeTrigger that is triggered by a change to
// the provided object reference, if any
func getImageChangeTriggerForRef(bc *buildapi.BuildConfig, ref *kapi.ObjectReference) *buildapi.ImageChangeTrigger {
	if ref == nil || ref.Kind != "ImageStreamTag" {
		return nil
	}
	for _, trigger := range bc.Spec.Triggers {
		if trigger.Type == buildapi.ImageChangeBuildTriggerType && trigger.ImageChange.From != nil &&
			trigger.ImageChange.From.Name == ref.Name && trigger.ImageChange.From.Namespace == ref.Namespace {
			return trigger.ImageChange
		}
	}
	return nil
}

//setBuildSource update build source by binary status
func setBuildSource(binary *buildapi.BinaryBuildSource, build *buildapi.Build) {
	if binary != nil {
		build.Spec.Source.Git = nil
		build.Spec.Source.Binary = binary
		if build.Spec.Source.Dockerfile != nil && binary.AsFile == "Dockerfile" {
			build.Spec.Source.Dockerfile = nil
		}
	} else {
		// must explicitly set this because we copied the source values from the buildconfig.
		build.Spec.Source.Binary = nil
	}
}

//setBuildAnnotationAndLabel set annotations and label info of this build
func setBuildAnnotationAndLabel(bcCopy *buildapi.BuildConfig, build *buildapi.Build) {
	if build.Annotations == nil {
		build.Annotations = make(map[string]string)
	}
	//bcCopy.Status.LastVersion has been increased
	build.Annotations[buildapi.BuildNumberAnnotation] = strconv.FormatInt(bcCopy.Status.LastVersion, 10)
	build.Annotations[buildapi.BuildConfigAnnotation] = bcCopy.Name
	if build.Labels == nil {
		build.Labels = make(map[string]string)
	}
	build.Labels[buildapi.BuildConfigLabelDeprecated] = buildapi.LabelValue(bcCopy.Name)
	build.Labels[buildapi.BuildConfigLabel] = buildapi.LabelValue(bcCopy.Name)
	build.Labels[buildapi.BuildRunPolicyLabel] = string(bcCopy.Spec.RunPolicy)
}

// mergeMaps will merge to map[string]string instances, with
// keys from the second argument overwriting keys from the
// first argument, in case of duplicates.
func mergeMaps(a, b map[string]string) map[string]string {
	if a == nil && b == nil {
		return nil
	}

	res := make(map[string]string)

	for k, v := range a {
		res[k] = v
	}

	for k, v := range b {
		res[k] = v
	}

	return res
}
