package host

import (
	"fmt"
	"path"

	"github.com/docker/docker/api/types"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
)

const (
	cmdTestNsenterMount = "nsenter --mount=/rootfs/proc/1/ns/mnt findmnt"

	ensureVolumeShareCmd = `#/bin/bash
set -x
nsenter --mount=/rootfs/proc/1/ns/mnt mkdir -p %[1]s
grep -F %[1]s /rootfs/proc/1/mountinfo || nsenter --mount=/rootfs/proc/1/ns/mnt mount -o bind %[1]s %[1]s
grep -F %[1]s /rootfs/proc/1/mountinfo | grep shared || nsenter --mount=/rootfs/proc/1/ns/mnt mount --make-shared %[1]s
`

	// RemoteHostOriginDir is a directory on the remote machine that runs Docker
	RemoteHostOriginDir = "/var/lib/origin/cluster-up"
)

// HostHelper contains methods to help check settings on a Docker host machine
// using a privileged container
type HostHelper struct {
	client    dockerhelper.Interface
	runHelper *run.RunHelper
	image     string
}

// NewHostHelper creates a new HostHelper
func NewHostHelper(dockerHelper *dockerhelper.Helper, image string) *HostHelper {
	return &HostHelper{
		runHelper: run.NewRunHelper(dockerHelper),
		client:    dockerHelper.Client(),
		image:     image,
	}
}

// CanUseNsenterMounter returns true if the Docker host machine can execute findmnt through nsenter
func (h *HostHelper) CanUseNsenterMounter() (bool, error) {
	_, rc, err := h.runner().
		Image(h.image).
		DiscardContainer().
		Privileged().
		Bind("/:/rootfs:ro").
		Entrypoint("/bin/bash").
		Command("-c", cmdTestNsenterMount).Run()
	return err == nil && rc == 0, err
}

// MakeRemoteHostDir creates a specified directory on the remote host filesystem.
// This is used when Docker runs in VM.
func (h *HostHelper) MakeRemoteHostDir(dst string) error {
	_, rc, err := h.runner().Image(h.image).
		DiscardContainer().
		Privileged().
		HostPid().
		Bind("/:/rootfs:rw").
		Entrypoint("/bin/bash").
		Command("-c", fmt.Sprintf("mkdir -p %q", path.Join("/rootfs", dst))).Run()
	if err != nil {
		return err
	}
	if rc != 0 {
		return fmt.Errorf("unable to make remote host directory %q: %d", dst, rc)
	}
	return nil
}

// CopyToRemoteHost copies source directory to remote host filesystem.
// This is used when Docker runs in VM.
func (h *HostHelper) CopyToRemoteHost(src, dst string) error {
	if err := h.MakeRemoteHostDir(dst); err != nil {
		return err
	}
	containerID, err := h.runner().
		Image(h.image).
		DiscardContainer().
		Privileged().
		HostPid().
		Bind("/:/rootfs:rw").
		Entrypoint("/bin/bash").
		Command("-c", "sleep infinity").Start()
	if err != nil {
		return err
	}
	defer func() {
		h.client.ContainerStop(containerID, 1)
		h.client.ContainerRemove(containerID, types.ContainerRemoveOptions{})
	}()
	err = dockerhelper.UploadFileToContainer(h.client, containerID, src, path.Join("/rootfs", dst))
	if err != nil {
		return err
	}
	glog.V(2).Infof("Succesfully copied %s to %s:%s", src, containerID, dst)
	return nil
}

// CopyFromRemoteHost copies the remote host directory into local filesystem.
// This is used when Docker runs in VM.
// TODO: Do we actually need this? We are not syncing anything back from the remote host.
func (h *HostHelper) CopyFromRemoteHost(src, dst string) error {
	containerID, err := h.runner().
		Image(h.image).
		DiscardContainer().
		Privileged().
		HostPid().
		Bind("/:/rootfs:rw").
		Entrypoint("/bin/bash").
		Command("-c", "sleep infinity").Start()
	if err != nil {
		return err
	}
	defer h.client.ContainerStop(containerID, 1)
	err = dockerhelper.DownloadDirFromContainer(h.client, containerID, src, path.Join("/rootfs", dst))
	if err != nil {
		return err
	}
	glog.V(2).Infof("Succesfully copied %s to %s:%s", src, containerID, dst)
	return nil
}

// EnsureVolumeUseShareMount ensures that the host Docker VM has a shared directory that can be used
// for OpenShift volumes. This is needed for Docker for Mac.
func (h *HostHelper) EnsureVolumeUseShareMount(volumesDir string) error {
	cmd := fmt.Sprintf(ensureVolumeShareCmd, volumesDir)
	_, rc, err := h.runner().
		Image(h.image).
		DiscardContainer().
		HostPid().
		Privileged().
		Bind("/proc:/rootfs/proc:ro").
		Entrypoint("/bin/bash").
		Command("-c", cmd).Run()
	if err != nil || rc != 0 {
		return errors.NewError("cannot create volume share").WithCause(err)
	}
	return nil
}

func (h *HostHelper) runner() *run.Runner {
	return h.runHelper.New()
}
