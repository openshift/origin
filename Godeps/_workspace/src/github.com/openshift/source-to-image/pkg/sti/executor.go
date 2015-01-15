package sti

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/api"
	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/git"
	"github.com/openshift/source-to-image/pkg/sti/script"
	"github.com/openshift/source-to-image/pkg/sti/tar"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

const (
	// maxErrorOutput is the maximum length of the error output saved for processing
	maxErrorOutput = 1024
	// defaultLocation is the default location of the scripts and sources in image
	defaultLocation = "/tmp"
)

// requestHandler encapsulates dependencies needed to fulfill requests.
type requestHandler struct {
	request      *api.Request
	result       *api.Result
	postExecutor postExecutor
	errorChecker errorChecker
	installer    script.Installer
	git          git.Git
	fs           util.FileSystem
	docker       docker.Docker
	tar          tar.Tar
}

type postExecutor interface {
	PostExecute(containerID string, location string) error
}

type errorChecker interface {
	wasExpectedError(text string) bool
}

// newRequestHandler returns a new handler for a given request.
func newRequestHandler(req *api.Request) (*requestHandler, error) {
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

func (h *requestHandler) setup(requiredScripts, optionalScripts []api.Script) (err error) {
	if h.request.WorkingDir, err = h.fs.CreateWorkingDirectory(); err != nil {
		return err
	}

	h.result = &api.Result{
		Success:    false,
		WorkingDir: h.request.WorkingDir,
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
		if err = h.fs.MkdirAll(filepath.Join(h.request.WorkingDir, v)); err != nil {
			return err
		}
	}

	if h.request.ExternalRequiredScripts, err = h.installer.DownloadAndInstall(
		requiredScripts, h.request.WorkingDir, true); err != nil {
		return err
	}
	if h.request.ExternalOptionalScripts, err = h.installer.DownloadAndInstall(
		optionalScripts, h.request.WorkingDir, false); err != nil {
		glog.Warningf("Failed downloading optional scripts: %v", err)
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

func (h *requestHandler) execute(command api.Script) error {
	glog.V(2).Infof("Using image name %s", h.request.BaseImage)

	uploadDir := filepath.Join(h.request.WorkingDir, "upload")
	tarFileName, err := h.tar.CreateTarFile(h.request.WorkingDir, uploadDir)
	if err != nil {
		return err
	}

	tarFile, err := h.fs.Open(tarFileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	expectedError := ""
	outReader, outWriter := io.Pipe()
	errReader, errWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	defer errReader.Close()
	defer errWriter.Close()
	opts := docker.RunContainerOptions{
		Image:           h.request.BaseImage,
		Stdout:          outWriter,
		Stderr:          errWriter,
		PullImage:       h.request.ForcePull,
		ExternalScripts: h.request.ExternalRequiredScripts,
		ScriptsURL:      h.request.ScriptsURL,
		Location:        h.request.Location,
		Command:         command,
		Env:             h.generateConfigEnv(),
		PostExec:        h,
	}
	if !h.request.LayeredBuild {
		opts.Stdin = tarFile
	}
	// goroutine to stream container's output
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			if glog.V(2) || command == api.Usage {
				glog.Info(scanner.Text())
			}
		}
	}(outReader)
	// goroutine to stream container's error
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			text := scanner.Text()
			if glog.V(1) {
				glog.Errorf(text)
			}
			if h.errorChecker != nil && h.errorChecker.wasExpectedError(text) &&
				len(expectedError) < maxErrorOutput {
				expectedError += text + "; "
			}
		}
	}(errReader)

	err = h.docker.RunContainer(opts)
	if e, ok := err.(errors.ContainerError); ok {
		return errors.NewContainerError(h.request.BaseImage, e.ErrorCode, expectedError)
	}
	return err
}

func (h *requestHandler) build() error {
	// create Dockerfile
	buffer := bytes.Buffer{}
	location := h.request.Location
	if len(location) == 0 {
		location = defaultLocation
	}
	buffer.WriteString(fmt.Sprintf("FROM %s\n", h.request.BaseImage))
	buffer.WriteString(fmt.Sprintf("ADD scripts %s\n", filepath.Join(location, "scripts")))
	buffer.WriteString(fmt.Sprintf("ADD src %s\n", filepath.Join(location, "src")))
	uploadDir := filepath.Join(h.request.WorkingDir, "upload")
	if err := h.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Writing custom Dockerfile to %s", uploadDir)

	tarFileName, err := h.tar.CreateTarFile(h.request.WorkingDir, uploadDir)
	if err != nil {
		return err
	}
	tarFile, err := h.fs.Open(tarFileName)
	if err != nil {
		return err
	}
	defer tarFile.Close()

	newBaseImage := fmt.Sprintf("%s-%d", h.request.BaseImage, time.Now().UnixNano())
	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	opts := docker.BuildImageOptions{
		Name:   newBaseImage,
		Stdin:  tarFile,
		Stdout: outWriter,
	}
	// goroutine to stream container's output
	go func(reader io.Reader) {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			glog.V(2).Info(scanner.Text())
		}
	}(outReader)
	glog.V(2).Infof("Building new image %s with scripts and sources already inside", newBaseImage)
	if err = h.docker.BuildImage(opts); err != nil {
		return err
	}

	// upon successful build we need to modify current request
	h.request.LayeredBuild = true
	// new image name
	h.request.BaseImage = newBaseImage
	// the scripts are inside the image
	h.request.ExternalRequiredScripts = false
	h.request.ScriptsURL = "image://" + filepath.Join(location, "scripts")
	// the source is also inside the image
	h.request.Location = filepath.Join(location, "src")
	return nil
}

func (h *requestHandler) PostExecute(containerID string, location string) (err error) {
	h.result.Success = true
	if h.postExecutor != nil {
		err = h.postExecutor.PostExecute(containerID, location)
		if err != nil {
			glog.Errorf("An error occurred in post executor: %v", err)
		}
	}
	return err
}

func (h *requestHandler) cleanup() {
	if h.request.PreserveWorkingDir {
		glog.Infof("Temporary directory '%s' will be saved, not deleted", h.request.WorkingDir)
	} else {
		glog.V(2).Infof("Removing temporary directory %s", h.request.WorkingDir)
		h.fs.RemoveDirectory(h.request.WorkingDir)
	}
	if h.request.LayeredBuild {
		glog.V(2).Infof("Removing temporary image %s", h.request.BaseImage)
		h.docker.RemoveImage(h.request.BaseImage)
	}
}

func (h *requestHandler) fetchSource() error {
	targetSourceDir := filepath.Join(h.request.WorkingDir, "upload", "src")
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
