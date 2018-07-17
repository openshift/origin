package util

import (
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/golang/glog"

	kexec "k8s.io/utils/exec"
)

const (
	crioRuntimeName         = "crio"
	crioRuntimeType         = "cri-o"
	dockerRuntimeName       = "docker"
	dockerRuntimeType       = "docker"
	defaultCRIOShimSocket   = "unix:///var/run/crio/crio.sock"
	defaultDockerShimSocket = "unix:///var/run/dockershim.sock"
)

type Runtime struct {
	Name     string
	Type     string
	Endpoint string
}

func GetRuntime() (*Runtime, error) {
	runtimeName, runtimeType, runtimeEndpoint, err := getDefaultRuntimeEndpoint()
	if err != nil {
		return nil, err
	}

	return &Runtime{
		Name:     runtimeName,
		Type:     runtimeType,
		Endpoint: runtimeEndpoint,
	}, nil
}

func (r *Runtime) GetContainerPid(data string) (string, error) {
	var pid string
	kexecer := kexec.New()

	containerID, err := r.GetContainerID(data)
	if err != nil {
		return pid, err
	}

	switch r.Name {
	case crioRuntimeName:
		output, err := kexecer.Command("runc", "state", containerID).CombinedOutput()
		if err != nil {
			return pid, err
		}

		re := regexp.MustCompile("\"pid\": ([0-9]+),")
		match := re.FindStringSubmatch(string(output))
		if len(match) < 1 {
			return pid, fmt.Errorf("failed to find pid for container: %s", containerID)
		}
		pid = match[1]
	case dockerRuntimeName:
		output, err := kexecer.Command("docker", "inspect", "-f", "{{.State.Pid}}", containerID).CombinedOutput()
		if err != nil {
			return pid, err
		}
		pid = string(output)
	default:
		return "", fmt.Errorf("invalid runtime name %s", r.Name)
	}

	return pid, nil
}

func (r *Runtime) GetContainerID(data string) (string, error) {
	// Trim the quotes and split the type and ID.
	parts := strings.Split(strings.Trim(data, "\""), "://")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid container ID: %q", data)
	}
	containerType, containerID := parts[0], parts[1]

	if r.Type != containerType {
		return "", fmt.Errorf("expected runtime type %q but found %q", r.Type, containerType)
	}

	return containerID, nil
}

func (r *Runtime) GetRuntimeVersion() (string, error) {
	var versionInfo string
	kexecer := kexec.New()

	switch r.Name {
	case crioRuntimeName:
		output, err := kexecer.Command("crictl", "version").CombinedOutput()
		if err != nil {
			return "", err
		}
		versionInfo = string(output)
	case dockerRuntimeName:
		output, err := kexecer.Command("docker", "version").CombinedOutput()
		if err != nil {
			return "", err
		}
		versionInfo = string(output)
	default:
		return "", fmt.Errorf("invalid runtime name %s", r.Name)
	}

	return versionInfo, nil
}

func getDefaultRuntimeEndpoint() (string, string, string, error) {
	isCRIO, err := filePathExists(defaultCRIOShimSocket)
	if err != nil {
		return "", "", "", err
	}

	isDocker, err := filePathExists(defaultDockerShimSocket)
	if err != nil {
		return "", "", "", err
	}

	// TODO: Instead of trying to detect the runtime make this as config option
	if isDocker && isCRIO {
		glog.Warningf("Found both crio and docker socket files, defaulting to crio")
		return crioRuntimeName, crioRuntimeType, defaultCRIOShimSocket, nil
	} else if isCRIO {
		return crioRuntimeName, crioRuntimeType, defaultCRIOShimSocket, nil
	} else if isDocker {
		return dockerRuntimeName, dockerRuntimeType, defaultDockerShimSocket, nil
	}

	return "", "", "", fmt.Errorf("supported runtime socket files not found")
}

func filePathExists(endpoint string) (bool, error) {
	u, err := url.Parse(endpoint)
	if err != nil {
		return false, err
	}

	if _, err := os.Stat(u.Path); err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
