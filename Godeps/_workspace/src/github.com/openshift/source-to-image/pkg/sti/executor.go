package sti

import (
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/git"
	"github.com/openshift/source-to-image/pkg/sti/script"
	"github.com/openshift/source-to-image/pkg/sti/tar"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	request      *Request
	result       *Result
	postExecutor postExecutor
	installer    script.Installer
	git          git.Git
	fs           util.FileSystem
	docker       docker.Docker
	tar          tar.Tar
}

type postExecutor interface {
	PostExecute(containerID string, cmd []string) error
}

// newRequestHandler returns a new handler for a given request.
func newRequestHandler(req *Request) (*requestHandler, error) {
	glog.V(2).Infof("Using docker socket: %s", req.DockerSocket)

	docker, err := docker.NewDocker(req.DockerSocket)
	if err != nil {
		return nil, err
	}

	return &requestHandler{
		request:   req,
		docker:    docker,
		installer: script.NewInstaller(req.BaseImage, req.ScriptsURL, docker),
		git:       git.NewGit(),
		fs:        util.NewFileSystem(),
		tar:       tar.NewTar(),
	}, nil
}

func (h *requestHandler) setup(requiredScripts, optionalScripts []string) (err error) {
	if h.request.workingDir, err = h.fs.CreateWorkingDirectory(); err != nil {
		return err
	}

	h.result = &Result{
		Success:    false,
		WorkingDir: h.request.workingDir,
	}

	// immediately pull the image if forcepull is true, that way later code that
	// references the image will have it pre-pulled and can just inspect the image.
	if h.request.ForcePull {
		err = h.docker.PullImage(h.request.BaseImage)
	} else {
		_, err = h.docker.CheckAndPull(h.request.BaseImage)
	}
	if err != nil {
		return
	}

	// fetch sources, for theirs .sti/bin might contain sti scripts
	if len(h.request.Source) > 0 {
		if err = h.fetchSource(); err != nil {
			return err
		}
	}

	dirs := []string{"upload/scripts", "downloads/scripts", "downloads/defaultScripts"}
	for _, v := range dirs {
		if err = h.fs.MkdirAll(filepath.Join(h.request.workingDir, v)); err != nil {
			return err
		}
	}

	if h.request.externalRequiredScripts, err = h.installer.DownloadAndInstall(
		requiredScripts, h.request.workingDir, true); err != nil {
		return err
	}
	if h.request.externalOptionalScripts, err = h.installer.DownloadAndInstall(
		optionalScripts, h.request.workingDir, false); err != nil {
		return err
	}

	return nil
}

func (h *requestHandler) generateConfigEnv() (configEnv []string) {
	if len(h.request.Environment) > 0 {
		for key, val := range h.request.Environment {
			configEnv = append(configEnv, key+"="+val)
		}
	}
	return
}

func (h *requestHandler) execute(command string) error {
	glog.V(2).Infof("Using image name %s", h.request.BaseImage)

	uploadDir := filepath.Join(h.request.workingDir, "upload")
	tarFileName, err := h.tar.CreateTarFile(h.request.workingDir, uploadDir)
	if err != nil {
		return err
	}

	tarFile, err := h.fs.Open(tarFileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	opts := docker.RunContainerOptions{
		Image:     h.request.BaseImage,
		Stdin:     tarFile,
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		PullImage: true,
		Command:   command,
		Env:       h.generateConfigEnv(),
		PostExec:  h,
	}
	return h.docker.RunContainer(opts)
}

func (h *requestHandler) PostExecute(containerID string, cmd []string) (err error) {
	h.result.Success = true
	if h.postExecutor != nil {
		err = h.postExecutor.PostExecute(containerID, cmd)
		if err != nil {
			glog.Errorf("An error occurred in post executor: %v", err)
		}
	}
	return err
}

func (h *requestHandler) cleanup() {
	if h.request.PreserveWorkingDir {
		glog.Infof("Temporary directory '%s' will be saved, not deleted", h.request.workingDir)
	} else {
		h.fs.RemoveDirectory(h.request.workingDir)
	}
}

func (h *requestHandler) fetchSource() error {
	targetSourceDir := filepath.Join(h.request.workingDir, "upload", "src")
	glog.V(1).Infof("Downloading %s to directory %s", h.request.Source, targetSourceDir)
	if h.git.ValidCloneSpec(h.request.Source) {
		if err := h.git.Clone(h.request.Source, targetSourceDir); err != nil {
			glog.Errorf("Git clone failed: %+v", err)
			return err
		}

		if h.request.Ref != "" {
			glog.V(1).Infof("Checking out ref %s", h.request.Ref)

			if err := h.git.Checkout(targetSourceDir, h.request.Ref); err != nil {
				return err
			}
		}
	} else {
		h.fs.Copy(h.request.Source, targetSourceDir)
	}

	return nil
}
