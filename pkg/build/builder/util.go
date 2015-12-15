package builder

import (
	"bufio"
	"io"
	"os"
	"regexp"
	"strings"

	stiapi "github.com/openshift/source-to-image/pkg/api"
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
// by examining /proc/self/cgroup. This contenxt is then passed to source-to-image.
func getDockerNetworkMode() stiapi.DockerNetworkMode {
	file, err := os.Open("/proc/self/cgroup")
	if err != nil {
		return ""
	}
	defer file.Close()

	if id := readNetClsCGroup(file); id != "" {
		return stiapi.NewDockerNetworkModeContainer(id)
	}
	return ""
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
