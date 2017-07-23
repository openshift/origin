package util

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/glog"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2iutil "github.com/openshift/source-to-image/pkg/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/v1"
	"k8s.io/kubernetes/pkg/credentialprovider"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	buildlister "github.com/openshift/origin/pkg/build/generated/listers/build/internalversion"
)

const (
	// NoBuildLogsMessage reports that no build logs are available
	NoBuildLogsMessage = "No logs are available."
)

// GetBuildName returns name of the build pod.
func GetBuildName(pod metav1.Object) string {
	if pod == nil {
		return ""
	}
	return pod.GetAnnotations()[buildapi.BuildAnnotation]
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
	glog.V(5).Infof("Build %s/%s does not have start policy label set, using default (Serial)", build.Namespace, build.Name)
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

func BuildDeepCopy(build *buildapi.Build) (*buildapi.Build, error) {
	objCopy, err := kapi.Scheme.DeepCopy(build)
	if err != nil {
		return nil, err
	}
	copied, ok := objCopy.(*buildapi.Build)
	if !ok {
		return nil, fmt.Errorf("expected Build, got %#v", objCopy)
	}
	return copied, nil
}

func CopyApiResourcesToV1Resources(in *kapi.ResourceRequirements) v1.ResourceRequirements {
	copied, err := kapi.Scheme.DeepCopy(in)
	if err != nil {
		panic(err)
	}
	in = copied.(*kapi.ResourceRequirements)
	out := v1.ResourceRequirements{}
	if err := v1.Convert_api_ResourceRequirements_To_v1_ResourceRequirements(in, &out, nil); err != nil {
		panic(err)
	}
	return out
}

func CopyApiEnvVarToV1EnvVar(in []kapi.EnvVar) []v1.EnvVar {
	copied, err := kapi.Scheme.DeepCopy(in)
	if err != nil {
		panic(err)
	}
	in = copied.([]kapi.EnvVar)
	out := make([]v1.EnvVar, len(in))
	for i := range in {
		if err := v1.Convert_api_EnvVar_To_v1_EnvVar(&in[i], &out[i], nil); err != nil {
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
func MergeTrustedEnvWithoutDuplicates(source []v1.EnvVar, output *[]v1.EnvVar, sourcePrecedence bool) {
	// filter out all environment variables except trusted/well known
	// values, because we do not want random environment variables being
	// fed into the privileged STI container via the BuildConfig definition.
	type sourceMapItem struct {
		index int
		value string
	}

	index := 0
	filteredSourceMap := make(map[string]sourceMapItem)
	filteredSource := []v1.EnvVar{}
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

// SafeForLoggingURL removes the user:password section of
// a url if present.  If not present the value is returned unchanged.
func SafeForLoggingURL(u *url.URL) *url.URL {
	if u == nil {
		return nil
	}
	newUrl := *u
	newUrl.User = url.User("redacted")
	return &newUrl
}

// SafeForLoggingEnvVar returns a copy of an EnvVar array with
// proxy credential values redacted.
func SafeForLoggingEnvVar(env []kapi.EnvVar) []kapi.EnvVar {
	newEnv := make([]kapi.EnvVar, len(env))
	copy(newEnv, env)
	proxyRegex := regexp.MustCompile("(?i)proxy")
	for i, env := range newEnv {
		if proxyRegex.MatchString(env.Name) {
			newEnv[i].Value, _ = s2iutil.SafeForLoggingURL(env.Value)
		}
	}
	return newEnv
}

// SafeForLoggingBuildCommonSpec returns a copy of a CommonSpec with
// proxy credential env variable values redacted.
func SafeForLoggingBuildCommonSpec(spec *buildapi.CommonSpec) *buildapi.CommonSpec {
	newSpec := *spec
	if newSpec.Source.Git != nil {
		if newSpec.Source.Git.HTTPProxy != nil {
			s, _ := s2iutil.SafeForLoggingURL(*newSpec.Source.Git.HTTPProxy)
			newSpec.Source.Git.HTTPProxy = &s
		}

		if newSpec.Source.Git.HTTPSProxy != nil {
			s, _ := s2iutil.SafeForLoggingURL(*newSpec.Source.Git.HTTPSProxy)
			newSpec.Source.Git.HTTPSProxy = &s
		}
	}

	if newSpec.Strategy.SourceStrategy != nil {
		newSpec.Strategy.SourceStrategy.Env = SafeForLoggingEnvVar(newSpec.Strategy.SourceStrategy.Env)
	}
	if newSpec.Strategy.DockerStrategy != nil {
		newSpec.Strategy.DockerStrategy.Env = SafeForLoggingEnvVar(newSpec.Strategy.DockerStrategy.Env)
	}
	if newSpec.Strategy.CustomStrategy != nil {
		newSpec.Strategy.CustomStrategy.Env = SafeForLoggingEnvVar(newSpec.Strategy.CustomStrategy.Env)
	}
	if newSpec.Strategy.JenkinsPipelineStrategy != nil {
		newSpec.Strategy.JenkinsPipelineStrategy.Env = SafeForLoggingEnvVar(newSpec.Strategy.JenkinsPipelineStrategy.Env)
	}
	return &newSpec
}

// SafeForLoggingBuild returns a copy of a Build with
// proxy credentials redacted.
func SafeForLoggingBuild(build *buildapi.Build) *buildapi.Build {
	newBuild := *build
	newSpec := SafeForLoggingBuildCommonSpec(&build.Spec.CommonSpec)
	newBuild.Spec.CommonSpec = *newSpec
	return &newBuild
}

// SafeForLoggingEnvironmentList returns a copy of an s2i EnvironmentList array with
// proxy credential values redacted.
func SafeForLoggingEnvironmentList(env s2iapi.EnvironmentList) s2iapi.EnvironmentList {
	newEnv := make(s2iapi.EnvironmentList, len(env))
	copy(newEnv, env)
	proxyRegex := regexp.MustCompile("(?i)proxy")
	for i, env := range newEnv {
		if proxyRegex.MatchString(env.Name) {
			newEnv[i].Value, _ = s2iutil.SafeForLoggingURL(env.Value)
		}
	}
	return newEnv
}

// SafeForLoggingS2IConfig returns a copy of an s2i Config with
// proxy credentials redacted.
func SafeForLoggingS2IConfig(config *s2iapi.Config) *s2iapi.Config {
	newConfig := *config
	newConfig.Environment = SafeForLoggingEnvironmentList(config.Environment)
	if config.ScriptDownloadProxyConfig != nil {
		newProxy := *config.ScriptDownloadProxyConfig
		newConfig.ScriptDownloadProxyConfig = &newProxy
		if newConfig.ScriptDownloadProxyConfig.HTTPProxy != nil {
			newConfig.ScriptDownloadProxyConfig.HTTPProxy = SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}

		if newConfig.ScriptDownloadProxyConfig.HTTPProxy != nil {
			newConfig.ScriptDownloadProxyConfig.HTTPSProxy = SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}
	}
	newConfig.ScriptsURL, _ = s2iutil.SafeForLoggingURL(newConfig.ScriptsURL)
	return &newConfig
}

// GetBuildConfigEnv gets the buildconfig strategy environment
func GetBuildConfigEnv(buildConfig *buildapi.BuildConfig) []kapi.EnvVar {
	switch {
	case buildConfig.Spec.Strategy.SourceStrategy != nil:
		return buildConfig.Spec.Strategy.SourceStrategy.Env
	case buildConfig.Spec.Strategy.DockerStrategy != nil:
		return buildConfig.Spec.Strategy.DockerStrategy.Env
	case buildConfig.Spec.Strategy.CustomStrategy != nil:
		return buildConfig.Spec.Strategy.CustomStrategy.Env
	case buildConfig.Spec.Strategy.JenkinsPipelineStrategy != nil:
		return buildConfig.Spec.Strategy.JenkinsPipelineStrategy.Env
	default:
		return nil
	}
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

// SetBuildConfigEnv replaces the current buildconfig environment
func SetBuildConfigEnv(buildConfig *buildapi.BuildConfig, env []kapi.EnvVar) {
	var oldEnv *[]kapi.EnvVar

	switch {
	case buildConfig.Spec.Strategy.SourceStrategy != nil:
		oldEnv = &buildConfig.Spec.Strategy.SourceStrategy.Env
	case buildConfig.Spec.Strategy.DockerStrategy != nil:
		oldEnv = &buildConfig.Spec.Strategy.DockerStrategy.Env
	case buildConfig.Spec.Strategy.CustomStrategy != nil:
		oldEnv = &buildConfig.Spec.Strategy.CustomStrategy.Env
	case buildConfig.Spec.Strategy.JenkinsPipelineStrategy != nil:
		oldEnv = &buildConfig.Spec.Strategy.JenkinsPipelineStrategy.Env
	}
	*oldEnv = env
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
		secretsv1 := make([]v1.Secret, 1)
		err := v1.Convert_api_Secret_To_v1_Secret(&secret, &secretsv1[0], nil)
		if err != nil {
			glog.V(2).Infof("Unable to make the Docker keyring for %s/%s secret: %v", secret.Name, secret.Namespace, err)
			continue
		}
		keyring, err := credentialprovider.MakeDockerKeyring(secretsv1, &emptyKeyring)
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
