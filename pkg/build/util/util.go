package util

import (
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/golang/glog"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapiv1 "k8s.io/kubernetes/pkg/apis/core/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"
	credentialprovidersecrets "k8s.io/kubernetes/pkg/credentialprovider/secrets"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
)

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."

	// WorkDir is the working directory within the build pod, mounted as a volume.
	BuildWorkDirMount = "/tmp/build"

	// BuilderServiceAccountName is the name of the account used to run build pods by default.
	BuilderServiceAccountName = "builder"
)

var (
	// InputContentPath is the path at which the build inputs will be available
	// to all the build containers.
	InputContentPath = filepath.Join(BuildWorkDirMount, "inputs")
	proxyRegex       = regexp.MustCompile("(?i)proxy")
)

// IsBuildComplete returns whether the provided build is complete or not
func IsBuildComplete(build *buildapi.Build) bool {
	return IsTerminalPhase(build.Status.Phase)
}

// IsTerminalPhase returns true if the provided phase is terminal
func IsTerminalPhase(phase buildapi.BuildPhase) bool {
	switch phase {
	case buildapi.BuildPhaseNew,
		buildapi.BuildPhasePending,
		buildapi.BuildPhaseRunning:
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
	return labels.Set{buildapi.BuildConfigLabel: buildapi.LabelValue(name)}.AsSelector()
}

// BuildConfigSelectorDeprecated returns a label Selector which can be used to find
// all builds for a BuildConfig that use the deprecated labels.
func BuildConfigSelectorDeprecated(name string) labels.Selector {
	return labels.Set{buildapi.BuildConfigLabelDeprecated: name}.AsSelector()
}

type buildFilter func(*buildapi.Build) bool

// BuildConfigBuilds return a list of builds for the given build config.
// Optionally you can specify a filter function to select only builds that
// matches your criteria.
func BuildConfigBuilds(c buildlister.BuildLister, namespace, name string, filterFunc buildFilter) ([]*buildapi.Build, error) {
	result, err := c.Builds(namespace).List(BuildConfigSelector(name))
	if err != nil {
		return nil, err
	}
	if filterFunc == nil {
		return result, nil
	}
	var filteredList []*buildapi.Build
	for _, b := range result {
		if filterFunc(b) {
			filteredList = append(filteredList, b)
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

func CopyApiResourcesToV1Resources(in *kapi.ResourceRequirements) corev1.ResourceRequirements {
	in = in.DeepCopy()
	out := corev1.ResourceRequirements{}
	if err := kapiv1.Convert_core_ResourceRequirements_To_v1_ResourceRequirements(in, &out, nil); err != nil {
		panic(err)
	}
	return out
}

func CopyApiEnvVarToV1EnvVar(in []kapi.EnvVar) []corev1.EnvVar {
	out := make([]corev1.EnvVar, len(in))
	for i := range in {
		item := in[i].DeepCopy()
		if err := kapiv1.Convert_core_EnvVar_To_v1_EnvVar(item, &out[i], nil); err != nil {
			panic(err)
		}
	}
	return out
}

// MergeTrustedEnvWithoutDuplicates merges two environment lists without having
// duplicate items in the output list.  The source list will be filtered
// such that only whitelisted environment variables are merged into the
// output list.  If sourcePrecedence is true, keys in the source list
// will override keys in the output list.
func MergeTrustedEnvWithoutDuplicates(source []corev1.EnvVar, output *[]corev1.EnvVar, sourcePrecedence bool) {
	// filter out all environment variables except trusted/well known
	// values, because we do not want random environment variables being
	// fed into the privileged STI container via the BuildConfig definition.
	type sourceMapItem struct {
		index int
		value string
	}

	index := 0
	filteredSourceMap := make(map[string]sourceMapItem)
	filteredSource := []corev1.EnvVar{}
	for _, env := range source {
		for _, acceptable := range buildapi.WhitelistEnvVarNames {
			if env.Name == acceptable {
				filteredSource = append(filteredSource, env)
				filteredSourceMap[env.Name] = sourceMapItem{index, env.Value}
				index++
				break
			}
		}
	}

	result := *output
	for i, env := range result {
		// If the value exists in output, override it and remove it
		// from the source list
		if v, found := filteredSourceMap[env.Name]; found {
			if sourcePrecedence {
				result[i].Value = v.value
			}
			filteredSource = append(filteredSource[:v.index], filteredSource[v.index+1:]...)
		}
	}
	*output = append(result, filteredSource...)
}

// GetBuildEnv gets the build strategy environment
func GetBuildEnv(build *buildapi.Build) []kapi.EnvVar {
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
func SetBuildEnv(build *buildapi.Build, env []kapi.EnvVar) {
	var oldEnv *[]kapi.EnvVar

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
func UpdateBuildEnv(build *buildapi.Build, env []kapi.EnvVar) {
	buildEnv := GetBuildEnv(build)

	newEnv := []kapi.EnvVar{}
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
func FindDockerSecretAsReference(secrets []kapi.Secret, image string) *kapi.LocalObjectReference {
	emptyKeyring := credentialprovider.BasicDockerKeyring{}
	for _, secret := range secrets {
		secretsv1 := make([]corev1.Secret, 1)
		err := kapiv1.Convert_core_Secret_To_v1_Secret(&secret, &secretsv1[0], nil)
		if err != nil {
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
		keyring, err := credentialprovidersecrets.MakeDockerKeyring(secretsv1, &emptyKeyring)
		if err != nil {
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
		if _, found := keyring.Lookup(image); found {
			return &kapi.LocalObjectReference{Name: secret.Name}
		}
	}
	return nil
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
