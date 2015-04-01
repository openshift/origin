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

const defaultLocation = "/tmp"

type Layered struct {
	request *api.Request
	docker  docker.Docker
	fs      util.FileSystem
	tar     tar.Tar
	scripts build.ScriptsHandler
}

func New(request *api.Request, scripts build.ScriptsHandler) (*Layered, error) {
	d, err := docker.New(request.DockerSocket)
	if err != nil {
		return nil, err
	}
	return &Layered{
		docker:  d,
		request: request,
		fs:      util.NewFileSystem(),
		tar:     tar.New(),
		scripts: scripts,
	}, nil
}

func getLocation(request *api.Request) string {
	location := request.Location
	if len(location) == 0 {
		location = defaultLocation
	}
	return location
}

func (b *Layered) CreateDockerfile(request *api.Request) error {
	buffer := bytes.Buffer{}

	user, err := b.docker.GetImageUser(b.request.BaseImage)
	if err != nil {
		return err
	}

	locations := []string{
		filepath.Join(getLocation(request), "scripts"),
		filepath.Join(getLocation(request), "src"),
	}

	buffer.WriteString(fmt.Sprintf("FROM %s\n", b.request.BaseImage))
	buffer.WriteString(fmt.Sprintf("COPY scripts %s\n", locations[0]))
	buffer.WriteString(fmt.Sprintf("COPY src %s\n", locations[1]))

	//TODO: We need to account for images that may not have chown. There is a proposal
	//      to specify the owner for COPY here: https://github.com/docker/docker/pull/9934
	if len(user) > 0 {
		buffer.WriteString("USER root\n")
		buffer.WriteString(fmt.Sprintf("RUN chown -R %s %s %s\n", user, locations[0], locations[1]))
		buffer.WriteString(fmt.Sprintf("USER %s\n", user))
	}

	uploadDir := filepath.Join(b.request.WorkingDir, "upload")
	if err := b.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Writing custom Dockerfile to %s", uploadDir)
	return nil
}

func (b *Layered) SourceTar(request *api.Request) (io.ReadCloser, error) {
	uploadDir := filepath.Join(request.WorkingDir, "upload")
	tarFileName, err := b.tar.CreateTarFile(b.request.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return b.fs.Open(tarFileName)
}

func (b *Layered) Build(request *api.Request) (*api.Result, error) {
	if err := b.CreateDockerfile(request); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := b.SourceTar(request)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	newBaseImage := fmt.Sprintf("%s-%d", b.request.BaseImage, time.Now().UnixNano())
	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	opts := docker.BuildImageOptions{
		Name:   newBaseImage,
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

	glog.V(2).Infof("Building new image %s with scripts and sources already inside", newBaseImage)
	if err = b.docker.BuildImage(opts); err != nil {
		return nil, err
	}

	// upon successful build we need to modify current request
	b.request.LayeredBuild = true
	// new image name
	b.request.BaseImage = newBaseImage
	// the scripts are inside the image
	b.request.ScriptsURL = "image://" + filepath.Join(getLocation(request), "scripts")
	// the source is also inside the image
	b.request.Location = filepath.Join(getLocation(request), "src")

	glog.V(2).Infof("Building %s using sti-enabled image", b.request.Tag)
	if err := b.scripts.Execute(api.Assemble, b.request); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			return nil, errors.NewAssembleError(b.request.Tag, e.Output, e)
		default:
			return nil, err
		}
	}

	return &api.Result{
		Success: true,
	}, nil
}
