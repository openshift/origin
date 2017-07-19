package plugin

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	dockertypes "github.com/docker/engine-api/types"

	kapi "k8s.io/kubernetes/pkg/api"
	kcontainer "k8s.io/kubernetes/pkg/kubelet/container"
	dockertools "k8s.io/kubernetes/pkg/kubelet/dockershim/libdocker"
	"k8s.io/kubernetes/pkg/kubelet/leaky"
)

func formatPod(pod *kapi.Pod) string {
	return fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
}

// Copied from pkg/kubelet/dockershim/naming.go::parseSandboxName()
func dockerSandboxNameToInfraPodNamePrefix(name string) (string, error) {
	// Docker adds a "/" prefix to names. so trim it.
	name = strings.TrimPrefix(name, "/")

	parts := strings.Split(name, "_")
	// Tolerate the random suffix.
	// TODO(random-liu): Remove 7 field case when docker 1.11 is deprecated.
	if len(parts) != 6 && len(parts) != 7 {
		return "", fmt.Errorf("failed to parse the sandbox name: %q", name)
	}
	if parts[0] != "k8s" {
		return "", fmt.Errorf("container is not managed by kubernetes: %q", name)
	}

	// Return /k8s_POD_name_namespace_uid
	return fmt.Sprintf("/k8s_%s_%s_%s_%s", leaky.PodInfraContainerName, parts[2], parts[3], parts[4]), nil
}

func killInfraContainerForPod(docker dockertools.Interface, containers []dockertypes.Container, cid kcontainer.ContainerID) error {
	// FIXME: handle CRI-O; but unfortunately CRI-O supports multiple
	// "runtimes" which depend on the filename of that runtime binary,
	// so we have no idea what cid.Type will be.
	if cid.Type != "docker" {
		return fmt.Errorf("unhandled runtime %q", cid.Type)
	}

	var err error
	var infraPrefix string
	for _, c := range containers {
		if c.ID == cid.ID {
			infraPrefix, err = dockerSandboxNameToInfraPodNamePrefix(c.Names[0])
			if err != nil {
				return err
			}
			break
		}
	}
	if infraPrefix == "" {
		return fmt.Errorf("failed to generate infra container prefix from %q", cid.ID)
	}
	// Find and kill the infra container
	for _, c := range containers {
		if strings.HasPrefix(c.Names[0], infraPrefix) {
			if err := docker.StopContainer(c.ID, 10); err != nil {
				glog.Warningf("failed to stop infra container %q", c.ID)
			}
		}
	}

	return nil
}

// This function finds the ContainerID of a failed pod, parses it, and kills
// any matching Infra container for that pod.
func killUpdateFailedPods(docker dockertools.Interface, pods []kapi.Pod) error {
	containers, err := docker.ListContainers(dockertypes.ContainerListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list docker containers: %v", err)
	}

	for _, pod := range pods {
		// Find the first ready container in the pod and use it to find the infra container
		var cid kcontainer.ContainerID
		for i := range pod.Status.ContainerStatuses {
			if pod.Status.ContainerStatuses[i].State.Running != nil && pod.Status.ContainerStatuses[i].ContainerID != "" {
				cid = kcontainer.ParseContainerID(pod.Status.ContainerStatuses[i].ContainerID)
				break
			}
		}
		if cid.IsEmpty() {
			continue
		}
		glog.V(5).Infof("Killing pod %q sandbox on restart", formatPod(&pod))
		if err := killInfraContainerForPod(docker, containers, cid); err != nil {
			glog.Warningf("Failed to kill pod %q sandbox: %v", formatPod(&pod), err)
			continue
		}
	}
	return nil
}
