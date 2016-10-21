package host

import (
	"fmt"
	"path"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/bootstrap/docker/run"
)

const (
	cmdTestNsenterMount          = "nsenter --mount=/rootfs/proc/1/ns/mnt findmnt"
	cmdEnsureHostDirs            = "for dir in %s; do if [ ! -d \"${dir}\" ]; then mkdir -p \"${dir}\"; fi; done"
	cmdCreateVolumesDirBindMount = "cat /rootfs/proc/1/mountinfo | grep /var/lib/origin || " +
		"nsenter --mount=/rootfs/proc/1/ns/mnt mount -o bind %[1]s %[1]s"
	cmdCreateVolumesDirShare = "cat /rootfs/proc/1/mountinfo | grep %[1]s | grep shared || " +
		"nsenter --mount=/rootfs/proc/1/ns/mnt mount --make-shared %[1]s"

	DefaultVolumesDir = "/var/lib/origin/openshift.local.volumes"
	DefaultConfigDir  = "/var/lib/origin/openshift.local.config"
)

// HostHelper contains methods to help check settings on a Docker host machine
// using a privileged container
type HostHelper struct {
	runHelper  *run.RunHelper
	client     *docker.Client
	image      string
	volumesDir string
	configDir  string
	dataDir    string
}

// NewHostHelper creates a new HostHelper
func NewHostHelper(client *docker.Client, image, volumesDir, configDir, dataDir string) *HostHelper {
	return &HostHelper{
		runHelper:  run.NewRunHelper(client),
		client:     client,
		image:      image,
		volumesDir: volumesDir,
		configDir:  configDir,
		dataDir:    dataDir,
	}
}

// CanUseNsenterMounter returns true if the Docker host machine can execute findmnt through nsenter
func (h *HostHelper) CanUseNsenterMounter() (bool, error) {
	rc, err := h.runner().
		Image(h.image).
		DiscardContainer().
		Privileged().
		Bind("/:/rootfs:ro").
		Entrypoint("/bin/bash").
		Command("-c", cmdTestNsenterMount).Run()
	return err == nil && rc == 0, nil
}

// EnsureVolumeShare ensures that the host Docker machine has a shared directory that can be used
// for OpenShift volumes
func (h *HostHelper) EnsureVolumeShare() error {
	if err := h.ensureVolumesDirBindMount(); err != nil {
		return err
	}
	if err := h.ensureVolumesDirShare(); err != nil {
		return err
	}
	return nil
}

func (h *HostHelper) defaultBinds() []string {
	return []string{fmt.Sprintf("%s:/var/lib/origin/openshift.local.config:z", h.configDir)}
}

// DownloadDirFromContainer copies a set of files from the Docker host to the local file system
func (h *HostHelper) DownloadDirFromContainer(sourceDir, destDir string) error {
	container, err := h.runner().
		Image(h.image).
		Bind(h.defaultBinds()...).
		Entrypoint("/bin/true").
		Create()
	if err != nil {
		return err
	}
	defer func() {
		errors.LogError(h.client.RemoveContainer(docker.RemoveContainerOptions{ID: container}))
	}()
	err = dockerhelper.DownloadDirFromContainer(h.client, container, sourceDir, destDir)
	if err != nil {
		glog.V(4).Infof("An error occurred downloading the directory: %v", err)
	} else {
		glog.V(4).Infof("Successfully downloaded directory.")
	}
	return err
}

// UploadFileToContainer copies a local file to the Docker host
func (h *HostHelper) UploadFileToContainer(src, dst string) error {
	container, err := h.runner().
		Image(h.image).
		Bind(h.defaultBinds()...).
		Entrypoint("/bin/true").
		Create()
	if err != nil {
		return err
	}
	defer func() {
		errors.LogError(h.client.RemoveContainer(docker.RemoveContainerOptions{ID: container}))
	}()
	err = dockerhelper.UploadFileToContainer(h.client, container, src, dst)
	if err != nil {
		glog.V(4).Infof("An error occurred uploading the file: %v", err)
	} else {
		glog.V(4).Infof("Successfully uploaded file.")
	}
	return err
}

// Hostname retrieves the FQDN of the Docker host machine
func (h *HostHelper) Hostname() (string, error) {
	hostname, _, _, err := h.runner().
		Image(h.image).
		HostNetwork().
		HostPid().
		DiscardContainer().
		Privileged().
		Entrypoint("/bin/bash").
		Command("-c", "uname -n").Output()
	if err != nil {
		return "", err
	}
	return strings.ToLower(strings.TrimSpace(hostname)), nil
}

func (h *HostHelper) EnsureHostDirectories() error {
	// Attempt to create host directories only if they are
	// the default directories. If the user specifies them, then the
	// user is responsible for ensuring they exist, are mountable, etc.
	dirs := []string{}
	if h.configDir == DefaultConfigDir {
		dirs = append(dirs, path.Join("/rootfs", h.configDir))
	}
	if h.volumesDir == DefaultVolumesDir {
		dirs = append(dirs, path.Join("/rootfs", h.volumesDir))
	}
	if len(dirs) > 0 {
		cmd := fmt.Sprintf(cmdEnsureHostDirs, strings.Join(dirs, " "))
		rc, err := h.runner().
			Image(h.image).
			DiscardContainer().
			Privileged().
			Bind("/var:/rootfs/var").
			Entrypoint("/bin/bash").
			Command("-c", cmd).Run()
		if err != nil || rc != 0 {
			return errors.NewError("cannot create host volumes directory").WithCause(err)
		}
	}
	return nil
}

func (h *HostHelper) hostPidCmd(cmd string) (int, error) {
	return h.runner().
		Image(h.image).
		DiscardContainer().
		HostPid().
		Privileged().
		Bind("/proc:/rootfs/proc:ro").
		Entrypoint("/bin/bash").
		Command("-c", cmd).Run()
}

func (h *HostHelper) ensureVolumesDirBindMount() error {
	cmd := fmt.Sprintf(cmdCreateVolumesDirBindMount, h.volumesDir)
	rc, err := h.hostPidCmd(cmd)
	if err != nil || rc != 0 {
		return errors.NewError("cannot create volumes dir mount").WithCause(err)
	}
	return nil
}

func (h *HostHelper) ensureVolumesDirShare() error {
	cmd := fmt.Sprintf(cmdCreateVolumesDirShare, h.volumesDir)
	rc, err := h.hostPidCmd(cmd)
	if err != nil || rc != 0 {
		return errors.NewError("cannot create volumes dir share").WithCause(err)
	}
	return nil
}

func (h *HostHelper) runner() *run.Runner {
	return h.runHelper.New()
}
