package builder

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"regexp"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	buildapiv1 "github.com/openshift/api/build/v1"
	builderutil "github.com/openshift/origin/pkg/build/builder/util"
	buildutil "github.com/openshift/origin/pkg/build/util"
	s2iapi "github.com/openshift/source-to-image/pkg/api"
	s2iutil "github.com/openshift/source-to-image/pkg/util"
)

var (
	// procCGroupPattern is a regular expression that parses the entries in /proc/self/cgroup
	procCGroupPattern = regexp.MustCompile(`\d+:([a-z_,]+):/.*/(\w+-|)([a-z0-9]+).*`)
)

// MergeEnv will take an existing environment and merge it with a new set of
// variables. For variables with the same name in both, only the one in the
// new environment will be kept.
func MergeEnv(oldEnv, newEnv []string) []string {
	key := func(e string) string {
		i := strings.Index(e, "=")
		if i == -1 {
			return e
		}
		return e[:i]
	}
	result := []string{}
	newVars := map[string]struct{}{}
	for _, e := range newEnv {
		newVars[key(e)] = struct{}{}
	}
	result = append(result, newEnv...)
	for _, e := range oldEnv {
		if _, exists := newVars[key(e)]; exists {
			continue
		}
		result = append(result, e)
	}
	return result
}

func reportPushFailure(err error, authPresent bool, pushAuthConfig docker.AuthConfiguration) error {
	// write extended error message to assist in problem resolution
	if authPresent {
		glog.V(0).Infof("Registry server Address: %s", pushAuthConfig.ServerAddress)
		glog.V(0).Infof("Registry server User Name: %s", pushAuthConfig.Username)
		glog.V(0).Infof("Registry server Email: %s", pushAuthConfig.Email)
		passwordPresent := "<<empty>>"
		if len(pushAuthConfig.Password) > 0 {
			passwordPresent = "<<non-empty>>"
		}
		glog.V(0).Infof("Registry server Password: %s", passwordPresent)
	}
	return fmt.Errorf("Failed to push image: %v", err)
}

// addBuildLabels adds some common image labels describing the build that produced
// this image.
func addBuildLabels(labels map[string]string, build *buildapiv1.Build) {
	labels[builderutil.DefaultDockerLabelNamespace+"build.name"] = build.Name
	labels[builderutil.DefaultDockerLabelNamespace+"build.namespace"] = build.Namespace
}

// readInt64 reads a file containing a 64 bit integer value
// and returns the value as an int64.  If the file contains
// a value larger than an int64, it returns MaxInt64,
// if the value is smaller than an int64, it returns MinInt64.
func readInt64(filePath string) (int64, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return -1, err
	}
	s := strings.TrimSpace(string(data))
	val, err := strconv.ParseInt(s, 10, 64)
	// overflow errors are ok, we'll get return a math.MaxInt64 value which is more
	// than enough anyway.  For underflow we'll return MinInt64 and the error.
	if err != nil && err.(*strconv.NumError).Err == strconv.ErrRange {
		if s[0] == '-' {
			return math.MinInt64, err
		}
		return math.MaxInt64, nil
	} else if err != nil {
		return -1, err
	}
	return val, nil
}

// readNetClsCGroup parses /proc/self/cgroup in order to determine the container id that can be used
// the network namespace that this process is running on, it returns the cgroup and container type
// (docker vs crio).
func readNetClsCGroup(reader io.Reader) (string, string) {

	containerType := "docker"

	cgroups := make(map[string]string)
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if match := procCGroupPattern.FindStringSubmatch(scanner.Text()); match != nil {
			containerType = strings.TrimSuffix(match[2], "-")

			list := strings.Split(match[1], ",")
			containerId := match[3]
			if len(list) > 0 {
				for _, key := range list {
					cgroups[key] = containerId
				}
			} else {
				cgroups[match[1]] = containerId
			}
		}
	}

	names := []string{"net_cls", "cpu"}
	for _, group := range names {
		if value, ok := cgroups[group]; ok {
			return value, containerType
		}
	}

	return "", containerType
}

// extractParentFromCgroupMap finds the cgroup parent in the cgroup map
func extractParentFromCgroupMap(cgMap map[string]string) (string, error) {
	memory, ok := cgMap["memory"]
	if !ok {
		return "", fmt.Errorf("could not find memory cgroup subsystem in map %v", cgMap)
	}
	glog.V(6).Infof("cgroup memory subsystem value: %s", memory)

	parts := strings.Split(memory, "/")
	if len(parts) < 2 {
		return "", fmt.Errorf("unprocessable cgroup memory value: %s", memory)
	}

	var cgroupParent string
	if strings.HasSuffix(memory, ".scope") {
		// systemd system, take the second to last segment.
		cgroupParent = parts[len(parts)-2]
	} else {
		// non-systemd, take everything except the last segment.
		cgroupParent = strings.Join(parts[:len(parts)-1], "/")
	}
	glog.V(5).Infof("found cgroup parent %v", cgroupParent)
	return cgroupParent, nil
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
			newConfig.ScriptDownloadProxyConfig.HTTPProxy = buildutil.SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}

		if newConfig.ScriptDownloadProxyConfig.HTTPProxy != nil {
			newConfig.ScriptDownloadProxyConfig.HTTPSProxy = buildutil.SafeForLoggingURL(newConfig.ScriptDownloadProxyConfig.HTTPProxy)
		}
	}
	newConfig.ScriptsURL, _ = s2iutil.SafeForLoggingURL(newConfig.ScriptsURL)
	return &newConfig
}
