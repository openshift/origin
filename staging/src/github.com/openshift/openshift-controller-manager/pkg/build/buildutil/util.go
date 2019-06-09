package buildutil

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/validation"
	v1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"

	buildv1 "github.com/openshift/api/build/v1"
	buildclientv1 "github.com/openshift/client-go/build/clientset/versioned/typed/build/v1"
	buildlisterv1 "github.com/openshift/client-go/build/listers/build/v1"
	"github.com/openshift/library-go/pkg/build/buildutil"
	"github.com/openshift/library-go/pkg/build/naming"
)

const (

	// BuildWorkDirMount is the working directory within the build pod, mounted as a volume.
	BuildWorkDirMount = "/tmp/build"

	// BuilderServiceAccountName is the name of the account used to run build pods by default.
	BuilderServiceAccountName = "builder"

	// BuildBlobsMetaCache is the directory used to store a cache for the blobs metadata to be
	// reused across builds.
	BuildBlobsMetaCache = "/var/lib/containers/cache"

	// BuildBlobsContentCache is the directory used to store a cache for the blobs content to be
	// reused within a build pod.
	BuildBlobsContentCache = "/var/cache/blobs"

	// buildPodSuffix is the suffix used to append to a build pod name given a build name
	buildPodSuffix           = "build"
	caConfigMapSuffix        = "ca"
	sysConfigConfigMapSuffix = "sys-config"
)

func HasTriggerType(triggerType buildv1.BuildTriggerType, bc *buildv1.BuildConfig) bool {
	matches := buildutil.FindTriggerPolicy(triggerType, bc)
	return len(matches) > 0
}

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildv1.Build) bool {
	return IsTerminalPhase(build.Status.Phase)
}

// IsTerminalPhase returns true if the provided phase is terminal
func IsTerminalPhase(phase buildv1.BuildPhase) bool {
	switch phase {
	case buildv1.BuildPhaseNew,
		buildv1.BuildPhasePending,
		buildv1.BuildPhaseRunning:
		return false
	}
	return true
}

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{buildv1.BuildConfigLabel: LabelValue(name)}.AsSelector()
}

type buildFilter func(*buildv1.Build) bool

func BuildConfigBuildsFromLister(lister buildlisterv1.BuildLister, namespace, name string, filterFunc buildFilter) ([]*buildv1.Build, error) {
	result, err := lister.Builds(namespace).List(BuildConfigSelector(name))
	if err != nil {
		return nil, err
	}
	if filterFunc == nil {
		return result, nil
	}
	var filteredList []*buildv1.Build
	for _, b := range result {
		if filterFunc(b) {
			filteredList = append(filteredList, b)
		}
	}
	return filteredList, nil
}

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func BuildConfigBuilds(c buildclientv1.BuildsGetter, namespace, name string, filterFunc buildFilter) ([]*buildv1.Build, error) {
	result, err := c.Builds(namespace).List(metav1.ListOptions{LabelSelector: BuildConfigSelector(name).String()})
	if err != nil {
		return nil, err
	}
	builds := make([]*buildv1.Build, len(result.Items))
	for i := range result.Items {
		builds[i] = &result.Items[i]
	}
	if filterFunc == nil {
		return builds, nil
	}
	var filteredList []*buildv1.Build
	for _, b := range builds {
		if filterFunc(b) {
			filteredList = append(filteredList, b)
		}
	}
	return filteredList, nil
}

// MergeTrustedEnvWithoutDuplicates merges two environment lists without having
// duplicate items in the output list.  The source list will be filtered
// such that only whitelisted environment variables are merged into the
// output list.  If sourcePrecedence is true, keys in the source list
// will override keys in the output list.
func MergeTrustedEnvWithoutDuplicates(source []corev1.EnvVar, output *[]corev1.EnvVar, sourcePrecedence bool) {
	MergeEnvWithoutDuplicates(source, output, sourcePrecedence, buildv1.WhitelistEnvVarNames)
}

// MergeEnvWithoutDuplicates merges two environment lists without having
// duplicate items in the output list.  If sourcePrecedence is true, keys in the source list
// will override keys in the output list.
func MergeEnvWithoutDuplicates(source []corev1.EnvVar, output *[]corev1.EnvVar, sourcePrecedence bool, whitelist []string) {
	// filter out all environment variables except trusted/well known
	// values, because we do not want random environment variables being
	// fed into the privileged STI container via the BuildConfig definition.

	filteredSourceMap := make(map[string]corev1.EnvVar)
	for _, env := range source {
		allowed := false
		if len(whitelist) == 0 {
			allowed = true
		} else {
			for _, acceptable := range buildv1.WhitelistEnvVarNames {
				if env.Name == acceptable {
					allowed = true
					break
				}
			}
		}
		if allowed {
			filteredSourceMap[env.Name] = env
		}
	}
	result := *output
	for i, env := range result {
		// If the value exists in output, optionally override it and remove it
		// from the source list
		if v, found := filteredSourceMap[env.Name]; found {
			if sourcePrecedence {
				result[i].Value = v.Value
			}
			delete(filteredSourceMap, env.Name)
		}
	}

	// iterate the original list so we retain the order of the inputs
	// when we append them to the output.
	for _, v := range source {
		if v, ok := filteredSourceMap[v.Name]; ok {
			result = append(result, v)
		}
	}
	*output = result
}

// FindDockerSecretAsReference looks through a set of k8s Secrets to find one that represents Docker credentials
// and which contains credentials that are associated with the registry identified by the image.  It returns
// a LocalObjectReference to the Secret, or nil if no match was found.
func FindDockerSecretAsReference(secrets []corev1.Secret, image string) *corev1.LocalObjectReference {
	emptyKeyring := credentialprovider.BasicDockerKeyring{}
	for _, secret := range secrets {
		secretList := []corev1.Secret{secret}
		keyring, err := credentialprovidersecrets.MakeDockerKeyring(secretList, &emptyKeyring)
		if err != nil {
			klog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
		if _, found := keyring.Lookup(image); found {
			return &corev1.LocalObjectReference{Name: secret.Name}
		}
	}
	return nil
}

// FetchServiceAccountSecrets retrieves the Secrets used for pushing and pulling
// images from private Docker registries.
func FetchServiceAccountSecrets(secretStore v1lister.SecretLister, serviceAccountStore v1lister.ServiceAccountLister, namespace, serviceAccount string) ([]corev1.Secret, error) {
	var result []corev1.Secret
	sa, err := serviceAccountStore.ServiceAccounts(namespace).Get(serviceAccount)
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", namespace, serviceAccount, err)
	}
	for _, ref := range sa.Secrets {
		secret, err := secretStore.Secrets(namespace).Get(ref.Name)
		if err != nil {
			continue
		}
		result = append(result, *secret)
	}
	return result, nil
}

// UpdateCustomImageEnv updates base image env variable reference with the new image for a custom build strategy.
// If no env variable reference exists, create a new env variable.
func UpdateCustomImageEnv(strategy *buildv1.CustomBuildStrategy, newImage string) {
	if strategy.Env == nil {
		strategy.Env = make([]corev1.EnvVar, 1)
		strategy.Env[0] = corev1.EnvVar{Name: buildv1.CustomBuildStrategyBaseImageKey, Value: newImage}
	} else {
		found := false
		for i := range strategy.Env {
			klog.V(4).Infof("Checking env variable %s %s", strategy.Env[i].Name, strategy.Env[i].Value)
			if strategy.Env[i].Name == buildv1.CustomBuildStrategyBaseImageKey {
				found = true
				strategy.Env[i].Value = newImage
				klog.V(4).Infof("Updated env variable %s to %s", strategy.Env[i].Name, strategy.Env[i].Value)
				break
			}
		}
		if !found {
			strategy.Env = append(strategy.Env, corev1.EnvVar{Name: buildv1.CustomBuildStrategyBaseImageKey, Value: newImage})
		}
	}
}

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildv1.Build) string {
	return naming.GetPodName(build.Name, buildPodSuffix)
}

// GetBuildCAConfigMapName returns the name of the ConfigMap containing the build's
// certificate authority bundles.
func GetBuildCAConfigMapName(build *buildv1.Build) string {
	return naming.GetConfigMapName(build.Name, caConfigMapSuffix)
}

// GetBuildSystemConfigMapName returns the name of the ConfigMap containing the build's
// registry configuration.
func GetBuildSystemConfigMapName(build *buildv1.Build) string {
	return naming.GetConfigMapName(build.Name, sysConfigConfigMapSuffix)
}

// LabelValue returns a string to use as a value for the Build
// label in a pod. If the length of the string parameter exceeds
// the maximum label length, the value will be truncated.
func LabelValue(name string) string {
	if len(name) <= validation.DNS1123LabelMaxLength {
		return name
	}
	return name[:validation.DNS1123LabelMaxLength]
}
