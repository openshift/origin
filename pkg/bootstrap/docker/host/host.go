package host

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"
	tarhelper "github.com/openshift/source-to-image/pkg/tar"

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
	// For now, use a shared mount on Windows/Mac
	// Eventually it also needs to be used on Linux, but nsenter
	// is still needed for Docker 1.9
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		return false, nil
	}
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

func catHostFile(hostFile string) string {
	file := path.Join("/rootfs", hostFile)
	return fmt.Sprintf("cat %s", file)
}

// CopyFromHost copies a set of files from the Docker host to the local file system
func (h *HostHelper) CopyFromHost(sourceDir, destDir string) error {
	container, err := h.runner().
		Image(h.image).
		Bind(fmt.Sprintf("%[1]s:%[1]s:ro", sourceDir)).
		Create()
	if err != nil {
		return err
	}
	defer func() {
		errors.LogError(h.client.RemoveContainer(docker.RemoveContainerOptions{ID: container}))
	}()
	localTarFile, err := ioutil.TempFile("", "local-copy-tar-")
	if err != nil {
		return err
	}
	localTarClosed := false
	defer func() {
		if !localTarClosed {
			errors.LogError(localTarFile.Close())
		}
		errors.LogError(os.Remove(localTarFile.Name()))
	}()
	glog.V(4).Infof("Downloading from host path %s to local tar file: %s", sourceDir, localTarFile.Name())
	err = h.client.DownloadFromContainer(container, docker.DownloadFromContainerOptions{
		Path:         sourceDir,
		OutputStream: localTarFile,
	})
	if err != nil {
		return err
	}
	if err = localTarFile.Close(); err != nil {
		return err
	}
	localTarClosed = true
	inputTar, err := os.Open(localTarFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		errors.LogError(inputTar.Close())
	}()
	tarHelper := tarhelper.New()
	tarHelper.SetExclusionPattern(nil)
	glog.V(4).Infof("Extracting temporary tar %s to directory %s", inputTar.Name(), destDir)
	var tarLog io.Writer
	if glog.V(5) {
		tarLog = os.Stderr
	}
	return tarHelper.ExtractTarStreamWithLogging(destDir, inputTar, tarLog)
}

// makeTempCopy creates a temporary directory and places a copy of the source file
// in it. It returns the directory where the temporary copy was made.
func makeTempCopy(file string) (string, error) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}
	destPath := filepath.Join(tempDir, filepath.Base(file))
	destFile, err := os.Create(destPath)
	if err != nil {
		return "", err
	}
	defer func() {
		errors.LogError(destFile.Close())
	}()
	sourceFile, err := os.Open(file)
	if err != nil {
		return "", err
	}
	defer func() {
		errors.LogError(sourceFile.Close())
	}()
	_, err = io.Copy(destFile, sourceFile)
	return tempDir, err
}

// CopyMasterConfigToHost copies a local file to the Docker host
func (h *HostHelper) CopyMasterConfigToHost(sourceFile, destDir string) error {
	localDir, err := makeTempCopy(sourceFile)
	if err != nil {
		return err
	}
	tarHelper := tarhelper.New()
	tarHelper.SetExclusionPattern(nil)
	var tarLog io.Writer
	if glog.V(5) {
		tarLog = os.Stderr
	}
	localTarFile, err := ioutil.TempFile("", "master-config")
	if err != nil {
		return err
	}
	localTarClosed := false
	defer func() {
		if !localTarClosed {
			errors.LogError(localTarFile.Close())
		}
	}()
	glog.V(4).Infof("Creating temporary tar %s to upload to %s", localTarFile.Name(), destDir)
	err = tarHelper.CreateTarStreamWithLogging(localDir, false, localTarFile, tarLog)
	if err != nil {
		return err
	}
	err = localTarFile.Close()
	if err != nil {
		return err
	}
	localTarClosed = true
	localTarInputClosed := false
	localTarInput, err := os.Open(localTarFile.Name())
	if err != nil {
		return err
	}
	defer func() {
		if !localTarInputClosed {
			localTarInput.Close()
		}
	}()
	bind := fmt.Sprintf("%s:/var/lib/origin/openshift.local.config:z", destDir)
	container, err := h.runner().
		Image(h.image).
		Bind(bind).Create()
	_ = container
	if err != nil {
		return err
	}
	defer func() {
		errors.LogError(h.client.RemoveContainer(docker.RemoveContainerOptions{ID: container}))
	}()

	glog.V(4).Infof("Uploading tar file %s to remote dir: %s", localTarFile.Name(), destDir)
	err = h.client.UploadToContainer(container, docker.UploadToContainerOptions{
		InputStream: localTarInput,
		Path:        "/var/lib/origin/openshift.local.config/master",
	})
	if err != nil {
		glog.V(4).Infof("An error occurred uploading the file: %v", err)
	} else {
		// If the upload succeeded the local input stream will be closed automatically
		localTarInputClosed = true
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
