package sti

import (
	"log"
	"os"
	"path/filepath"

	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/script"
	"github.com/openshift/source-to-image/pkg/sti/tar"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	request      *STIRequest
	result       *STIResult
	postExecutor postExecutor
	installer    script.Installer
	fs           util.FileSystem
	docker       docker.Docker
	tar          tar.Tar
}

type postExecutor interface {
	PostExecute(containerID string, cmd []string) error
}

// newRequestHandler returns a new handler for a given request.
func newRequestHandler(req *STIRequest) (*requestHandler, error) {
	if req.Verbose {
		log.Printf("Using docker socket: %s\n", req.DockerSocket)
	}

	docker, err := docker.NewDocker(req.DockerSocket, req.Verbose)
	if err != nil {
		return nil, err
	}

	return &requestHandler{
		request:   req,
		docker:    docker,
		installer: script.NewInstaller(req.BaseImage, req.ScriptsUrl, docker, req.Verbose),
		fs:        util.NewFileSystem(req.Verbose),
		tar:       tar.NewTar(req.Verbose),
	}, nil
}

func (h *requestHandler) setup(requiredScripts, optionalScripts []string) (err error) {
	if h.request.workingDir, err = h.fs.CreateWorkingDirectory(); err != nil {
		return err
	}

	h.result = &STIResult{
		Success:    false,
		WorkingDir: h.request.workingDir,
	}

	dirs := []string{"upload/scripts", "downloads/scripts", "downloads/defaultScripts"}
	for _, v := range dirs {
		if err = h.fs.MkdirAll(filepath.Join(h.request.workingDir, v)); err != nil {
			return err
		}
	}

	if err = h.installer.DownloadAndInstall(requiredScripts, h.request.workingDir, true); err != nil {
		return err
	}

	if err = h.installer.DownloadAndInstall(optionalScripts, h.request.workingDir, false); err != nil {
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
	if h.request.Verbose {
		log.Printf("Using image name %s", h.request.BaseImage)
	}

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
			log.Printf("An error occurred in post executor: %v", err)
		}
	}
	return err
}

func (h *requestHandler) cleanup() {
	if h.request.PreserveWorkingDir {
		log.Printf("Temporary directory '%s' will be saved, not deleted\n", h.request.workingDir)
	} else {
		h.fs.RemoveDirectory(h.request.workingDir)
	}
}
