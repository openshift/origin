package libkpod

import (
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/pkg/errors"
)

// ContainerStop stops a running container with a grace period (i.e., timeout).
func (c *ContainerServer) ContainerStop(container string, timeout int64) (string, error) {
	ctr, err := c.LookupContainer(container)
	if err != nil {
		return "", errors.Wrapf(err, "failed to find container %s", container)
	}

	cStatus := c.runtime.ContainerStatus(ctr)
	if cStatus.Status != oci.ContainerStateStopped {
		if err := c.runtime.StopContainer(ctr, timeout); err != nil {
			return "", errors.Wrapf(err, "failed to stop container %s", ctr.ID())
		}
		if err := c.storageRuntimeServer.StopContainer(ctr.ID()); err != nil {
			return "", errors.Wrapf(err, "failed to unmount container %s", ctr.ID())
		}
	}

	c.ContainerStateToDisk(ctr)

	return ctr.ID(), nil
}
