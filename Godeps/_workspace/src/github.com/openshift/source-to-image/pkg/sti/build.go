package sti

import (
	"io"
	"log"
	"path/filepath"

	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/git"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

type Builder struct {
	handler buildHandlerInterface
}

type buildHandlerInterface interface {
	cleanup()
	setup(required []string, optional []string) error
	determineIncremental() error
	Request() *STIRequest
	Result() *STIResult
	saveArtifacts() error
	fetchSource() error
	execute(command string) error
}

type buildHandler struct {
	*requestHandler
	git             git.Git
	callbackInvoker util.CallbackInvoker
}

func NewBuilder(req *STIRequest) (*Builder, error) {
	handler, err := newBuildHandler(req)
	if err != nil {
		return nil, err
	}
	return &Builder{
		handler: handler,
	}, nil
}

func newBuildHandler(req *STIRequest) (*buildHandler, error) {
	rh, err := newRequestHandler(req)
	if err != nil {
		return nil, err
	}
	bh := &buildHandler{
		requestHandler:  rh,
		git:             git.NewGit(),
		callbackInvoker: util.NewCallbackInvoker(),
	}
	rh.postExecutor = bh
	return bh, nil
}

// Build processes a Request and returns a *Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (b *Builder) Build() (*STIResult, error) {
	bh := b.handler
	defer bh.cleanup()

	err := bh.setup([]string{"assemble", "run"}, []string{"save-artifacts"})
	if err != nil {
		return nil, err
	}

	err = bh.determineIncremental()
	if err != nil {
		return nil, err
	}
	if bh.Request().incremental {
		log.Printf("Existing image for tag %s detected for incremental build.\n", bh.Request().Tag)
	} else {
		log.Println("Clean build will be performed")
	}

	if bh.Request().Verbose {
		log.Printf("Performing source build from %s\n", bh.Request().Source)
	}

	if bh.Request().incremental {
		if err = bh.saveArtifacts(); err != nil {
			log.Printf("Error saving previous build artifacts: %v", err)
		}
	}

	if err = bh.fetchSource(); err != nil {
		return nil, err
	}

	if err = bh.execute("assemble"); err != nil {
		return nil, err
	}

	return bh.Result(), nil
}

func (h *buildHandler) PostExecute(containerID string, cmd []string) error {
	var err error
	var previousImageID = ""
	if h.request.incremental && h.request.RemovePreviousImage {
		if previousImageID, err = h.docker.GetImageId(h.request.Tag); err != nil {
			log.Printf("Error retrieving previous image's metadata: %v", err)
		}
	}

	opts := docker.CommitContainerOptions{
		Command:     append(cmd, "run"),
		Env:         h.generateConfigEnv(),
		ContainerID: containerID,
		Repository:  h.request.Tag,
	}
	imageID, err := h.docker.CommitContainer(opts)
	if err != nil {
		return errors.ErrBuildFailed
	}

	h.result.ImageID = imageID
	log.Printf("Tagged %s as %s\n", imageID, h.request.Tag)

	if h.request.incremental && h.request.RemovePreviousImage && previousImageID != "" {
		log.Printf("Removing previously-tagged image %s\n", previousImageID)
		if err = h.docker.RemoveImage(previousImageID); err != nil {
			log.Printf("Unable to remove previous image: %v\n", err)
		}
	}

	if h.request.CallbackUrl != "" {
		h.result.Messages = h.callbackInvoker.ExecuteCallback(h.request.CallbackUrl,
			h.result.Success, h.result.Messages)
	}

	return nil
}

func (h *buildHandler) determineIncremental() (err error) {
	h.request.incremental = false
	if h.request.Clean {
		return
	}

	// can only do incremental build if runtime image exists
	incremental, err := h.docker.IsImageInLocalRegistry(h.request.Tag)
	if err != nil {
		return
	}

	// check if a save-artifacts script exists in anything provided to the build
	// without it, we cannot do incremental builds
	if incremental && h.fs.Exists(
		filepath.Join(h.request.workingDir, "upload", "scripts", "save-artifacts")) {
		h.request.incremental = true
	}

	return nil
}

func (h *buildHandler) saveArtifacts() (err error) {
	artifactTmpDir := filepath.Join(h.request.workingDir, "upload", "artifacts")
	if err = h.fs.Mkdir(artifactTmpDir); err != nil {
		return err
	}

	image := h.request.Tag

	log.Printf("Saving build artifacts from image %s to path %s\n", image, artifactTmpDir)

	reader, writer := io.Pipe()

	extractFunc := func() error {
		defer reader.Close()
		return h.tar.ExtractTarStream(artifactTmpDir, reader)
	}

	opts := docker.RunContainerOptions{
		Image:        image,
		OverwriteCmd: true,
		Command:      "save-artifacts",
		Stdout:       writer,
		OnStart:      extractFunc,
	}
	err = h.docker.RunContainer(opts)
	writer.Close()
	if err != nil {
		switch e := err.(type) {
		case errors.StiContainerError:
			if h.request.Verbose {
				log.Printf("Exit code: %d", e.ExitCode)
			}
			return errors.ErrSaveArtifactsFailed
		}
	}
	return err
}

func (h *buildHandler) fetchSource() error {
	targetSourceDir := filepath.Join(h.request.workingDir, "upload", "src")

	log.Printf("Downloading %s to directory %s\n", h.request.Source, targetSourceDir)

	if h.git.ValidCloneSpec(h.request.Source) {
		if err := h.git.Clone(h.request.Source, targetSourceDir); err != nil {
			log.Printf("Git clone failed: %+v", err)
			return err
		}

		if h.request.Ref != "" {
			log.Printf("Checking out ref %s", h.request.Ref)

			if err := h.git.Checkout(targetSourceDir, h.request.Ref); err != nil {
				return err
			}
		}
	} else {
		h.fs.Copy(h.request.Source, targetSourceDir)
	}

	return nil
}

func (h *buildHandler) Request() *STIRequest {
	return h.request
}

func (h *buildHandler) Result() *STIResult {
	return h.result
}
