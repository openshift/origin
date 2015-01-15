package sti

import (
	"io"
	"path/filepath"
	"regexp"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/sti/api"
	"github.com/openshift/source-to-image/pkg/sti/docker"
	"github.com/openshift/source-to-image/pkg/sti/errors"
	"github.com/openshift/source-to-image/pkg/sti/util"
)

// Builder provides a simple Build interface
type Builder struct {
	handler buildHandlerInterface
}

type buildHandlerInterface interface {
	cleanup()
	setup(required []api.Script, optional []api.Script) error
	determineIncremental() error
	Request() *api.Request
	Result() *api.Result
	saveArtifacts() error
	fetchSource() error
	execute(command api.Script) error
	wasExpectedError(text string) bool
	build() error
}

type buildHandler struct {
	*requestHandler
	callbackInvoker util.CallbackInvoker
}

// NewBuilder returns a new Builder
func NewBuilder(req *api.Request) (*Builder, error) {
	handler, err := newBuildHandler(req)
	if err != nil {
		return nil, err
	}
	return &Builder{
		handler: handler,
	}, nil
}

func newBuildHandler(req *api.Request) (*buildHandler, error) {
	rh, err := newRequestHandler(req)
	if err != nil {
		return nil, err
	}
	bh := &buildHandler{
		requestHandler:  rh,
		callbackInvoker: util.NewCallbackInvoker(),
	}
	rh.postExecutor = bh
	rh.errorChecker = bh
	return bh, nil
}

// Build processes a Request and returns a *api.Result and an error.
// An error represents a failure performing the build rather than a failure
// of the build itself.  Callers should check the Success field of the result
// to determine whether a build succeeded or not.
func (b *Builder) Build() (*api.Result, error) {
	bh := b.handler
	defer bh.cleanup()

	err := bh.setup([]api.Script{api.Assemble, api.Run}, []api.Script{api.SaveArtifacts})
	if err != nil {
		return nil, err
	}

	err = bh.determineIncremental()
	if err != nil {
		return nil, err
	}
	if bh.Request().Incremental {
		glog.V(1).Infof("Existing image for tag %s detected for incremental build.", bh.Request().Tag)
	} else {
		glog.V(1).Infof("Clean build will be performed")
	}

	glog.V(2).Infof("Performing source build from %s", bh.Request().Source)
	if bh.Request().Incremental {
		if err = bh.saveArtifacts(); err != nil {
			glog.Warning("Error saving previous build artifacts: %v", err)
			glog.Warning("Clean build will be performed!")
		}
	}

	glog.V(1).Infof("Building %s", bh.Request().Tag)
	err = bh.execute(api.Assemble)
	if e, ok := err.(errors.ContainerError); ok && bh.wasExpectedError(e.ExpectedError) {
		glog.Warningf("Image %s does not have tar! Performing additional build to add the scripts and sources.",
			bh.Request().BaseImage)
		if err := bh.build(); err != nil {
			return nil, err
		}
		glog.V(2).Infof("Building %s using sti-enabled image", bh.Request().Tag)
		if err := bh.execute(api.Assemble); err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	return bh.Result(), nil
}

// wasExpectedError is used for determining whether the error that appeared
// authorizes us to do the additional build injecting the scripts and sources.
func (h *buildHandler) wasExpectedError(text string) bool {
	tar, _ := regexp.MatchString(`.*tar.*not found`, text)
	sh, _ := regexp.MatchString(`.*/bin/sh.*no such file or directory`, text)
	return tar || sh
}

func (h *buildHandler) PostExecute(containerID string, location string) error {
	var (
		err             error
		previousImageID string
	)
	if h.request.Incremental && h.request.RemovePreviousImage {
		if previousImageID, err = h.docker.GetImageID(h.request.Tag); err != nil {
			glog.Errorf("Error retrieving previous image's metadata: %v", err)
		}
	}

	cmd := []string{}
	opts := docker.CommitContainerOptions{
		Command:     append(cmd, filepath.Join(location, string(api.Run))),
		Env:         h.generateConfigEnv(),
		ContainerID: containerID,
		Repository:  h.request.Tag,
	}
	imageID, err := h.docker.CommitContainer(opts)
	if err != nil {
		return errors.NewBuildError(h.request.Tag, err)
	}

	h.result.ImageID = imageID
	glog.V(1).Infof("Tagged %s as %s", imageID, h.request.Tag)

	if h.request.Incremental && h.request.RemovePreviousImage && previousImageID != "" {
		glog.V(1).Infof("Removing previously-tagged image %s", previousImageID)
		if err = h.docker.RemoveImage(previousImageID); err != nil {
			glog.Errorf("Unable to remove previous image: %v", err)
		}
	}

	if h.request.CallbackURL != "" {
		h.result.Messages = h.callbackInvoker.ExecuteCallback(h.request.CallbackURL,
			h.result.Success, h.result.Messages)
	}

	glog.Infof("Successfully built %s", h.request.Tag)
	return nil
}

func (h *buildHandler) determineIncremental() (err error) {
	h.request.Incremental = false
	if h.request.Clean {
		return
	}

	// can only do incremental build if runtime image exists
	previousImageExists, err := h.docker.IsImageInLocalRegistry(h.request.Tag)
	if err != nil {
		return
	}

	// we're assuming save-artifacts to exists for embedded scripts (if not we'll
	// warn a user upon container failure and proceed with clean build)
	// for external save-artifacts - check its existence
	saveArtifactsExists := !h.request.ExternalOptionalScripts ||
		h.fs.Exists(filepath.Join(h.request.WorkingDir, "upload", "scripts", string(api.SaveArtifacts)))
	h.request.Incremental = previousImageExists && saveArtifactsExists
	return nil
}

func (h *buildHandler) saveArtifacts() (err error) {
	artifactTmpDir := filepath.Join(h.request.WorkingDir, "upload", "artifacts")
	if err = h.fs.Mkdir(artifactTmpDir); err != nil {
		return err
	}

	image := h.request.Tag
	reader, writer := io.Pipe()
	glog.V(1).Infof("Saving build artifacts from image %s to path %s", image, artifactTmpDir)
	extractFunc := func() error {
		defer reader.Close()
		return h.tar.ExtractTarStream(artifactTmpDir, reader)
	}

	opts := docker.RunContainerOptions{
		Image:           image,
		ExternalScripts: h.request.ExternalRequiredScripts,
		ScriptsURL:      h.request.ScriptsURL,
		Location:        h.request.Location,
		Command:         api.SaveArtifacts,
		Stdout:          writer,
		OnStart:         extractFunc,
	}
	err = h.docker.RunContainer(opts)
	writer.Close()
	if _, ok := err.(errors.ContainerError); ok {
		return errors.NewSaveArtifactsError(image, err)
	}
	return err
}

func (h *buildHandler) Request() *api.Request {
	return h.request
}

func (h *buildHandler) Result() *api.Result {
	return h.result
}
