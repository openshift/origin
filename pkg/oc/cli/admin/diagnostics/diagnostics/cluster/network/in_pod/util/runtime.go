package util

import (
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/golang/glog"

	kwait "k8s.io/apimachinery/pkg/util/wait"
	kubeletapi "k8s.io/kubernetes/pkg/kubelet/apis/cri"
	kruntimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletremote "k8s.io/kubernetes/pkg/kubelet/remote"
)

const (
	crioRuntimeName         = "crio"
	dockerRuntimeName       = "docker"
	defaultCRIOShimSocket   = "unix:///var/run/crio/crio.sock"
	defaultDockerShimSocket = "unix:///var/run/dockershim.sock"

	// 2 minutes is the current default value used in kubelet
	defaultRuntimeRequestTimeout = 2 * time.Minute
)

type Runtime struct {
	Name    string
	Service kubeletapi.RuntimeService
}

func GetRuntime() (*Runtime, error) {
	runtimeName, runtimeEndpoint, err := getDefaultRuntimeEndpoint()
	if err != nil {
		return nil, err
	}

	runtimeService, err := kubeletremote.NewRemoteRuntimeService(runtimeEndpoint, defaultRuntimeRequestTimeout)
	if err != nil {
		return nil, err
	}

	// Timeout ~30 seconds
	err = kwait.ExponentialBackoff(
		kwait.Backoff{
			Duration: 100 * time.Millisecond,
			Factor:   1.2,
			Steps:    24,
		},
		func() (bool, error) {
			// Ensure the runtime is actually alive; gRPC may create the client but
			// it may not be responding to requests yet
			if _, err := runtimeService.ListPodSandbox(&kruntimeapi.PodSandboxFilter{}); err != nil {
				// Wait longer
				return false, nil
			}
			return true, nil
		})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch runtime service: %v", err)
	}

	return &Runtime{
		Name:    runtimeName,
		Service: runtimeService,
	}, nil
}

func getDefaultRuntimeEndpoint() (string, string, error) {
	isCRIO, err := filePathExists(defaultCRIOShimSocket)
	if err != nil {
		return "", "", err
	}

	isDocker, err := filePathExists(defaultDockerShimSocket)
	if err != nil {
		return "", "", err
	}

	// TODO: Instead of trying to detect the runtime make this as config option
	if isDocker && isCRIO {
		glog.Warningf("Found both crio and docker socket files, defaulting to crio")
		return crioRuntimeName, defaultCRIOShimSocket, nil
	} else if isCRIO {
		return crioRuntimeName, defaultCRIOShimSocket, nil
	} else if isDocker {
		return dockerRuntimeName, defaultDockerShimSocket, nil
	}

	return "", "", fmt.Errorf("supported runtime socket files not found")
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
