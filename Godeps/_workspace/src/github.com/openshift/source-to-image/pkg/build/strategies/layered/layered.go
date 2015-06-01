package layered

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/golang/glog"
	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
)

const defaultDestination = "/tmp"

type Layered struct {
	config  *api.Config
	docker  docker.Docker
	fs      util.FileSystem
	tar     tar.Tar
	scripts build.ScriptsHandler
}

func New(config *api.Config, scripts build.ScriptsHandler) (*Layered, error) {
	d, err := docker.New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return nil, err
	}
	return &Layered{
		docker:  d,
		config:  config,
		fs:      util.NewFileSystem(),
		tar:     tar.New(),
		scripts: scripts,
	}, nil
}

func getDestination(config *api.Config) string {
	destination := config.Destination
	if len(destination) == 0 {
		destination = defaultDestination
	}
	return destination
}

func (b *Layered) CreateDockerfile(config *api.Config) error {
	buffer := bytes.Buffer{}

	user, err := b.docker.GetImageUser(b.config.BuilderImage)
	if err != nil {
		return err
	}

	locations := []string{
		filepath.Join(getDestination(config), "scripts"),
		filepath.Join(getDestination(config), "src"),
	}

	buffer.WriteString(fmt.Sprintf("FROM %s\n", b.config.BuilderImage))
	buffer.WriteString(fmt.Sprintf("COPY scripts %s\n", locations[0]))
	buffer.WriteString(fmt.Sprintf("COPY src %s\n", locations[1]))

	//TODO: We need to account for images that may not have chown. There is a proposal
	//      to specify the owner for COPY here: https://github.com/docker/docker/pull/9934
	if len(user) > 0 {
		buffer.WriteString("USER root\n")
		buffer.WriteString(fmt.Sprintf("RUN chown -R %s %s %s\n", user, locations[0], locations[1]))
		buffer.WriteString(fmt.Sprintf("USER %s\n", user))
	}

	uploadDir := filepath.Join(b.config.WorkingDir, "upload")
	if err := b.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Writing custom Dockerfile to %s", uploadDir)
	return nil
}

func (b *Layered) SourceTar(config *api.Config) (io.ReadCloser, error) {
	uploadDir := filepath.Join(config.WorkingDir, "upload")
	tarFileName, err := b.tar.CreateTarFile(b.config.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return b.fs.Open(tarFileName)
}

func (b *Layered) Build(config *api.Config) (*api.Result, error) {
	if err := b.CreateDockerfile(config); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := b.SourceTar(config)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	newBuilderImage := fmt.Sprintf("%s-%d", b.config.BuilderImage, time.Now().UnixNano())
	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	opts := docker.BuildImageOptions{
		Name:   newBuilderImage,
		Stdin:  tarStream,
		Stdout: outWriter,
	}

	// goroutine to stream container's output
	go func(reader io.Reader) {
		scanner := bufio.NewReader(reader)
		for {
			text, err := scanner.ReadString('\n')
			if err != nil {
				// we're ignoring ErrClosedPipe, as this is information
				// the docker container ended streaming logs
				if glog.V(2) && err != io.ErrClosedPipe {
					glog.Errorf("Error reading docker stdout, %v", err)
				}
				break
			}
			glog.V(2).Info(text)
		}
	}(outReader)

	glog.V(2).Infof("Building new image %s with scripts and sources already inside", newBuilderImage)
	if err = b.docker.BuildImage(opts); err != nil {
		return nil, err
	}

	// upon successful build we need to modify current config
	b.config.LayeredBuild = true
	// new image name
	b.config.BuilderImage = newBuilderImage
	// the scripts are inside the image
	b.config.ScriptsURL = "image://" + filepath.Join(getDestination(config), "scripts")

	glog.V(2).Infof("Building %s using sti-enabled image", b.config.Tag)
	if err := b.scripts.Execute(api.Assemble, b.config); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			return nil, errors.NewAssembleError(b.config.Tag, e.Output, e)
		default:
			return nil, err
		}
	}

	return &api.Result{
		Success: true,
	}, nil
}
