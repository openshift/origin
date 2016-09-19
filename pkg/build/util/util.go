package util

import (
	"fmt"
	"strconv"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/credentialprovider"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/golang/glog"
	buildapi "github.com/openshift/origin/pkg/build/api"
	buildclient "github.com/openshift/origin/pkg/build/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."
)

// GetBuildName returns name of the build pod.
func GetBuildName(pod *kapi.Pod) string {
	if pod == nil {
		return ""
	}
	return pod.Annotations[buildapi.BuildAnnotation]
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildapi.BuildStrategy) *kapi.ObjectReference {
	switch {
	case strategy.SourceStrategy != nil:
		return &strategy.SourceStrategy.From
	case strategy.DockerStrategy != nil:
		return strategy.DockerStrategy.From
	case strategy.CustomStrategy != nil:
		return &strategy.CustomStrategy.From
	default:
		return nil
	}
}

// NameFromImageStream returns a concatenated name representing an ImageStream[Tag/Image]
// reference.  If the reference does not contain a Namespace, the namespace parameter
// is used instead.
func NameFromImageStream(namespace string, ref *kapi.ObjectReference, tag string) string {
	var ret string
	if ref.Namespace == "" {
		ret = namespace
	} else {
		ret = ref.Namespace
	}
	ret = ret + "/" + ref.Name
	if tag != "" && strings.Index(ref.Name, ":") == -1 && strings.Index(ref.Name, "@") == -1 {
		ret = ret + ":" + tag
	}
	return ret
}

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildapi.Build) bool {
	return build.Status.Phase != buildapi.BuildPhaseRunning && build.Status.Phase != buildapi.BuildPhasePending && build.Status.Phase != buildapi.BuildPhaseNew
}

// IsPaused returns true if the provided BuildConfig is paused and cannot be used to create a new Build
func IsPaused(bc *buildapi.BuildConfig) bool {
	return strings.ToLower(bc.Annotations[buildapi.BuildConfigPausedAnnotation]) == "true"
}

// BuildNumber returns the given build number.
func BuildNumber(build *buildapi.Build) (int64, error) {
	annotations := build.GetAnnotations()
	if stringNumber, ok := annotations[buildapi.BuildNumberAnnotation]; ok {
		return strconv.ParseInt(stringNumber, 10, 64)
	}
	return 0, fmt.Errorf("build %s/%s does not have %s annotation", build.Namespace, build.Name, buildapi.BuildNumberAnnotation)
}

// BuildRunPolicy returns the scheduling policy for the build based on the
// "queued" label.
func BuildRunPolicy(build *buildapi.Build) buildapi.BuildRunPolicy {
	labels := build.GetLabels()
	if value, found := labels[buildapi.BuildRunPolicyLabel]; found {
		switch value {
		case "Parallel":
			return buildapi.BuildRunPolicyParallel
		case "Serial":
			return buildapi.BuildRunPolicySerial
		case "SerialLatestOnly":
			return buildapi.BuildRunPolicySerialLatestOnly
		}
	}
	glog.V(5).Infof("Build %s/%s does not have start policy label set, using default (Serial)")
	return buildapi.BuildRunPolicySerial
}

// BuildNameForConfigVersion returns the name of the version-th build
// for the config that has the provided name.
func BuildNameForConfigVersion(name string, version int) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{buildapi.BuildConfigLabel: buildapi.LabelValue(name)}.AsSelector()
}

// BuildConfigSelectorDeprecated returns a label Selector which can be used to find
// all builds for a BuildConfig that use the deprecated labels.
func BuildConfigSelectorDeprecated(name string) labels.Selector {
	return labels.Set{buildapi.BuildConfigLabelDeprecated: name}.AsSelector()
}

type buildFilter func(buildapi.Build) bool

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func BuildConfigBuilds(c buildclient.BuildLister, namespace, name string, filterFunc buildFilter) (*buildapi.BuildList, error) {
	result, err := c.List(namespace, kapi.ListOptions{
		LabelSelector: BuildConfigSelector(name),
	})
	if err != nil {
		return nil, err
	}
	if filterFunc == nil {
		return result, nil
	}
	filteredList := &buildapi.BuildList{TypeMeta: result.TypeMeta, ListMeta: result.ListMeta}
	for _, b := range result.Items {
		if filterFunc(b) {
			filteredList.Items = append(filteredList.Items, b)
		}
	}
	return filteredList, nil
}

// ConfigNameForBuild returns the name of the build config from a
// build name.
func ConfigNameForBuild(build *buildapi.Build) string {
	if build == nil {
		return ""
	}
	if build.Annotations != nil {
		if _, exists := build.Annotations[buildapi.BuildConfigAnnotation]; exists {
			return build.Annotations[buildapi.BuildConfigAnnotation]
		}
	}
	if _, exists := build.Labels[buildapi.BuildConfigLabel]; exists {
		return build.Labels[buildapi.BuildConfigLabel]
	}
	return build.Labels[buildapi.BuildConfigLabelDeprecated]
}

// VersionForBuild returns the version from the provided build name.
// If no version can be found, 0 is returned to indicate no version.
func VersionForBuild(build *buildapi.Build) int {
	if build == nil {
		return 0
	}
	versionString := build.Annotations[buildapi.BuildNumberAnnotation]
	version, err := strconv.Atoi(versionString)
	if err != nil {
		return 0
	}
	return version
}

// BuildClient is the API client used by various build components to
// retrieve information about builds/buildconfigs/imagestreams.
type BuildClient interface {
	GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuild(ctx kapi.Context, build *buildapi.Build) error
	GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	GetImageStreamImage(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error)
	GetImageStreamTag(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error)
}

// BuildClientImpl is an implementation of the BuildClientInf interface
type BuildClientImpl struct {
	GetBuildConfigFunc      func(ctx kapi.Context, name string) (*buildapi.BuildConfig, error)
	UpdateBuildConfigFunc   func(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error
	GetBuildFunc            func(ctx kapi.Context, name string) (*buildapi.Build, error)
	CreateBuildFunc         func(ctx kapi.Context, build *buildapi.Build) error
	GetImageStreamFunc      func(ctx kapi.Context, name string) (*imageapi.ImageStream, error)
	GetImageStreamImageFunc func(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error)
	GetImageStreamTagFunc   func(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error)
}

// GetBuildConfig retrieves a named build config
func (c BuildClientImpl) GetBuildConfig(ctx kapi.Context, name string) (*buildapi.BuildConfig, error) {
	return c.GetBuildConfigFunc(ctx, name)
}

// UpdateBuildConfig updates a named build config
func (c BuildClientImpl) UpdateBuildConfig(ctx kapi.Context, buildConfig *buildapi.BuildConfig) error {
	return c.UpdateBuildConfigFunc(ctx, buildConfig)
}

// GetBuild retrieves a build
func (c BuildClientImpl) GetBuild(ctx kapi.Context, name string) (*buildapi.Build, error) {
	return c.GetBuildFunc(ctx, name)
}

// CreateBuild creates a new build
func (c BuildClientImpl) CreateBuild(ctx kapi.Context, build *buildapi.Build) error {
	return c.CreateBuildFunc(ctx, build)
}

// GetImageStream retrieves a named image stream
func (c BuildClientImpl) GetImageStream(ctx kapi.Context, name string) (*imageapi.ImageStream, error) {
	return c.GetImageStreamFunc(ctx, name)
}

// GetImageStreamImage retrieves an image stream image
func (c BuildClientImpl) GetImageStreamImage(ctx kapi.Context, name string) (*imageapi.ImageStreamImage, error) {
	return c.GetImageStreamImageFunc(ctx, name)
}

// GetImageStreamTag retrieves and image stream tag
func (c BuildClientImpl) GetImageStreamTag(ctx kapi.Context, name string) (*imageapi.ImageStreamTag, error) {
	return c.GetImageStreamTagFunc(ctx, name)
}

// FetchServiceAccountSecrets retrieves the Secrets used for pushing and pulling
// images from private Docker registries.
func FetchServiceAccountSecrets(saClient kclient.ServiceAccountsNamespacer, secretsClient kclient.SecretsNamespacer, namespace, serviceAccount string) ([]kapi.Secret, error) {
	var result []kapi.Secret
	sa, err := saClient.ServiceAccounts(namespace).Get(serviceAccount)
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", namespace, serviceAccount, err)
	}
	for _, ref := range sa.Secrets {
		secret, err := secretsClient.Secrets(namespace).Get(ref.Name)
		if err != nil {
			continue
		}
		result = append(result, *secret)
	}
	return result, nil
}

// ResolveImageSecret looks up the Secrets provided by the Service Account and
// attempt to find a best match for given image.
func ResolveImageSecret(client BuildClient, ctx kapi.Context, secrets []kapi.Secret, imageRef *kapi.ObjectReference, buildNamespace string) *kapi.LocalObjectReference {
	if len(secrets) == 0 || imageRef == nil {
		return nil
	}
	emptyKeyring := credentialprovider.BasicDockerKeyring{}
	// Get the image pull spec from the image stream reference
	imageSpec, err := ResolveImageStreamDockerRepository(client, ctx, *imageRef, buildNamespace)
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

// ResolveImageStreamDockerRepository looks up the ImageStream[Tag/Image] and converts it to a
// the docker repository reference with no tag information
func ResolveImageStreamDockerRepository(client BuildClient, ctx kapi.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	namespace := defaultNamespace
	if len(from.Namespace) > 0 {
		namespace = from.Namespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		imageStreamImage, err := client.GetImageStreamImage(kapi.WithNamespace(ctx, namespace), from.Name)
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
		is, err := client.GetImageStream(kapi.WithNamespace(ctx, namespace), name)
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

// ResolveImageStreamReference looks up the ImageStream[Tag/Image] and converts it to a
// docker pull spec that can be used in an Image field.
func ResolveImageStreamReference(client BuildClient, ctx kapi.Context, from kapi.ObjectReference, defaultNamespace string) (string, error) {
	var namespace string
	if len(from.Namespace) != 0 {
		namespace = from.Namespace
	} else {
		namespace = defaultNamespace
	}

	glog.V(4).Infof("Resolving ImageStreamReference %s of Kind %s in namespace %s", from.Name, from.Kind, namespace)
	switch from.Kind {
	case "ImageStreamImage":
		imageStreamImage, err := client.GetImageStreamImage(kapi.WithNamespace(ctx, namespace), from.Name)
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
		imageStreamTag, err := client.GetImageStreamTag(kapi.WithNamespace(ctx, namespace), from.Name)
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
