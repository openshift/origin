package util

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	ktypedclient "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"

	buildv1 "github.com/openshift/api/build/v1"
	buildlister "github.com/openshift/client-go/build/listers/build/v1"
	"github.com/openshift/origin/pkg/api/apihelpers"
	"github.com/openshift/origin/pkg/build/buildapihelpers"
)

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."

	// WorkDir is the working directory within the build pod, mounted as a volume.
	BuildWorkDirMount = "/tmp/build"

	// BuilderServiceAccountName is the name of the account used to run build pods by default.
	BuilderServiceAccountName = "builder"

	// buildPodSuffix is the suffix used to append to a build pod name given a build name
	buildPodSuffix = "build"
)

var (
	// InputContentPath is the path at which the build inputs will be available
	// to all the build containers.
	InputContentPath = filepath.Join(BuildWorkDirMount, "inputs")
)

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
func IsFatalGeneratorError(err error) bool {
	_, isFatal := err.(*GeneratorFatalError)
	return isFatal
}

// GetBuildPodName returns name of the build pod.
func GetBuildPodName(build *buildv1.Build) string {
	return apihelpers.GetPodName(build.Name, buildPodSuffix)
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

// BuildNameForConfigVersion returns the name of the version-th build
// for the config that has the provided name.
func BuildNameForConfigVersion(name string, version int) string {
	return fmt.Sprintf("%s-%d", name, version)
}

// BuildConfigSelector returns a label Selector which can be used to find all
// builds for a BuildConfig.
func BuildConfigSelector(name string) labels.Selector {
	return labels.Set{BuildConfigLabel: buildapihelpers.LabelValue(name)}.AsSelector()
}

type buildFilter func(*buildv1.Build) bool

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func BuildConfigBuilds(c buildlister.BuildLister, namespace, name string, filterFunc buildFilter) ([]*buildv1.Build, error) {
	result, err := c.Builds(namespace).List(BuildConfigSelector(name))
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

// ConfigNameForBuild returns the name of the build config from a
// build name.
func ConfigNameForBuild(build *buildv1.Build) string {
	if build == nil {
		return ""
	}
	if build.Annotations != nil {
		if _, exists := build.Annotations[BuildConfigAnnotation]; exists {
			return build.Annotations[BuildConfigAnnotation]
		}
	}
	if _, exists := build.Labels[BuildConfigLabel]; exists {
		return build.Labels[BuildConfigLabel]
	}
	return build.Labels[BuildConfigLabelDeprecated]
}

// MergeTrustedEnvWithoutDuplicates merges two environment lists without having
// duplicate items in the output list.  The source list will be filtered
// such that only whitelisted environment variables are merged into the
// output list.  If sourcePrecedence is true, keys in the source list
// will override keys in the output list.
func MergeTrustedEnvWithoutDuplicates(source []corev1.EnvVar, output *[]corev1.EnvVar, sourcePrecedence bool) {
	MergeEnvWithoutDuplicates(source, output, sourcePrecedence, WhitelistEnvVarNames)
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
			for _, acceptable := range WhitelistEnvVarNames {
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

// GetBuildEnv gets the build strategy environment
func GetBuildEnv(build *buildv1.Build) []corev1.EnvVar {
	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		return build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.DockerStrategy != nil:
		return build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		return build.Spec.Strategy.CustomStrategy.Env
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		return build.Spec.Strategy.JenkinsPipelineStrategy.Env
	default:
		return nil
	}
}

// SetBuildEnv replaces the current build environment
func SetBuildEnv(build *buildv1.Build, env []corev1.EnvVar) {
	var oldEnv *[]corev1.EnvVar

	switch {
	case build.Spec.Strategy.SourceStrategy != nil:
		oldEnv = &build.Spec.Strategy.SourceStrategy.Env
	case build.Spec.Strategy.DockerStrategy != nil:
		oldEnv = &build.Spec.Strategy.DockerStrategy.Env
	case build.Spec.Strategy.CustomStrategy != nil:
		oldEnv = &build.Spec.Strategy.CustomStrategy.Env
	case build.Spec.Strategy.JenkinsPipelineStrategy != nil:
		oldEnv = &build.Spec.Strategy.JenkinsPipelineStrategy.Env
	default:
		return
	}
	*oldEnv = env
}

// UpdateBuildEnv updates the strategy environment
// This will replace the existing variable definitions with provided env
func UpdateBuildEnv(build *buildv1.Build, env []corev1.EnvVar) {
	buildEnv := GetBuildEnv(build)

	newEnv := []corev1.EnvVar{}
	for _, e := range buildEnv {
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
	SetBuildEnv(build, newEnv)
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
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
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
func FetchServiceAccountSecrets(client ktypedclient.CoreV1Interface, namespace, serviceAccount string) ([]corev1.Secret, error) {
	var result []corev1.Secret
	sa, err := client.ServiceAccounts(namespace).Get(serviceAccount, metav1.GetOptions{})
	if err != nil {
		return result, fmt.Errorf("Error getting push/pull secrets for service account %s/%s: %v", namespace, serviceAccount, err)
	}
	for _, ref := range sa.Secrets {
		secret, err := client.Secrets(namespace).Get(ref.Name, metav1.GetOptions{})
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
		strategy.Env[0] = corev1.EnvVar{Name: CustomBuildStrategyBaseImageKey, Value: newImage}
	} else {
		found := false
		for i := range strategy.Env {
			glog.V(4).Infof("Checking env variable %s %s", strategy.Env[i].Name, strategy.Env[i].Value)
			if strategy.Env[i].Name == CustomBuildStrategyBaseImageKey {
				found = true
				strategy.Env[i].Value = newImage
				glog.V(4).Infof("Updated env variable %s to %s", strategy.Env[i].Name, strategy.Env[i].Value)
				break
			}
		}
		if !found {
			strategy.Env = append(strategy.Env, corev1.EnvVar{Name: CustomBuildStrategyBaseImageKey, Value: newImage})
		}
	}
}

// ParseProxyURL parses a proxy URL and allows fallback to non-URLs like
// myproxy:80 (for example) which url.Parse no longer accepts in Go 1.8.  The
// logic is copied from net/http.ProxyFromEnvironment to try to maintain
// backwards compatibility.
func ParseProxyURL(proxy string) (*url.URL, error) {
	proxyURL, err := url.Parse(proxy)

	// logic copied from net/http.ProxyFromEnvironment
	if err != nil || !strings.HasPrefix(proxyURL.Scheme, "http") {
		// proxy was bogus. Try prepending "http://" to it and see if that
		// parses correctly. If not, we fall through and complain about the
		// original one.
		if proxyURL, err := url.Parse("http://" + proxy); err == nil {
			return proxyURL, nil
		}
	}

	return proxyURL, err
}

// GetInputReference returns the From ObjectReference associated with the
// BuildStrategy.
func GetInputReference(strategy buildv1.BuildStrategy) *corev1.ObjectReference {
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
