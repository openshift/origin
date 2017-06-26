package builder

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"

	docker "github.com/fsouza/go-dockerclient"

	s2iapi "github.com/openshift/source-to-image/pkg/api"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

var (
	// procCGroupPattern is a regular expression that parses the entries in /proc/self/cgroup
	procCGroupPattern = regexp.MustCompile(`\d+:([a-z_,]+):/.*/(docker-|)([a-z0-9]+).*`)
)

// readNetClsCGroup parses /proc/self/cgroup in order to determine the container id that can be used
// the network namespace that this process is running on.
func readNetClsCGroup(reader io.Reader) string {
	cgroups := make(map[string]string)

	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		if match := procCGroupPattern.FindStringSubmatch(scanner.Text()); match != nil {
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
			return value
		}
	}

	return ""
}

// getDockerNetworkMode determines whether the builder is running as a container
// by examining /proc/self/cgroup. This context is then passed to source-to-image.
func getDockerNetworkMode() s2iapi.DockerNetworkMode {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	defer file.Close()

	if id := readNetClsCGroup(file); id != "" {
		return s2iapi.NewDockerNetworkModeContainer(id)
	}
	return ""
}

// GetCGroupLimits returns a struct populated with cgroup limit values gathered
// from the local /sys/fs/cgroup filesystem.  Overflow values are set to
// math.MaxInt64.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	byteLimit, err := readInt64("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		// for systems without cgroups builds should succeed
		if _, err := os.Stat("/sys/fs/cgroup"); os.IsNotExist(err) {
			return &s2iapi.CGroupLimits{}, nil
		}
		return nil, fmt.Errorf("cannot determine cgroup limits: %v", err)
	}
	// math.MaxInt64 seems to give cgroups trouble, this value is
	// still 92 terabytes, so it ought to be sufficiently large for
	// our purposes.
	if byteLimit > 92233720368547 {
		byteLimit = 92233720368547
	}

	parent, err := getCgroupParent()
	if err != nil {
		return nil, fmt.Errorf("read cgroup parent: %v", err)
	}

	return &s2iapi.CGroupLimits{
		// Though we are capped on memory and cpu at the cgroup parent level,
		// some build containers care what their memory limit is so they can
		// adapt, thus we need to set the memory limit at the container level
		// too, so that information is available to them.
		MemoryLimitBytes: byteLimit,
		// Set memoryswap==memorylimit, this ensures no swapping occurs.
		// see: https://docs.docker.com/engine/reference/run/#runtime-constraints-on-cpu-and-memory
		MemorySwap: byteLimit,
		Parent:     parent,
	}, nil
}

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
func addBuildLabels(labels map[string]string, build *buildapi.Build) {
	labels[buildapi.DefaultDockerLabelNamespace+"build.name"] = build.Name
	labels[buildapi.DefaultDockerLabelNamespace+"build.namespace"] = build.Namespace
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
