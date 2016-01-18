package builder

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
	"regexp"
	"strconv"
	"strings"

	s2iapi "github.com/openshift/source-to-image/pkg/api"
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
// MAX_INT_64.
func GetCGroupLimits() (*s2iapi.CGroupLimits, error) {
	byteLimit, err := readInt64("/sys/fs/cgroup/memory/memory.limit_in_bytes")
	if err != nil {
		return nil, err
	}

	cpuQuota, err := readInt64("/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_quota_us")
	if err != nil {
		return nil, err
	}

	cpuShares, err := readInt64("/sys/fs/cgroup/cpu,cpuacct/cpu.shares")
	if err != nil {
		return nil, err
	}

	cpuPeriod, err := readInt64("/sys/fs/cgroup/cpu,cpuacct/cpu.cfs_period_us")
	if err != nil {
		return nil, err
	}
	return &s2iapi.CGroupLimits{
		CPUShares:        cpuShares,
		CPUPeriod:        cpuPeriod,
		CPUQuota:         cpuQuota,
		MemoryLimitBytes: byteLimit,
		MemorySwap:       -1,
	}, nil
}

func readInt64(filePath string) (int64, error) {
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return -1, err
	}
	s := strings.TrimSpace(string(data))
	val, err := strconv.ParseInt(s, 10, 64)
	// overflow errors are ok because we'll get a MAX_INT_64 value which is more
	// than enough anyway.
	if err != nil && err.(*strconv.NumError).Err != strconv.ErrRange {
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
