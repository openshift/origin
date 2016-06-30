package layered

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"time"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/errors"
	"github.com/openshift/source-to-image/pkg/tar"
	"github.com/openshift/source-to-image/pkg/util"
	utilglog "github.com/openshift/source-to-image/pkg/util/glog"
)

var glog = utilglog.StderrLog

const defaultDestination = "/tmp"

// A Layered builder builds images by first performing a docker build to inject
// (layer) the source code and s2i scripts into the builder image, prior to
// running the new image with the assemble script. This is necessary when the
// builder image does not include "sh" and "tar" as those tools are needed
// during the normal source injection process.
type Layered struct {
	config     *api.Config
	docker     docker.Docker
	fs         util.FileSystem
	tar        tar.Tar
	scripts    build.ScriptsHandler
	hasOnBuild bool
}

// New creates a Layered builder.
func New(config *api.Config, scripts build.ScriptsHandler, overrides build.Overrides) (*Layered, error) {
	d, err := docker.New(config.DockerConfig, config.PullAuthentication)
	if err != nil {
		return nil, err
	}
	tarHandler := tar.New()
	tarHandler.SetExclusionPattern(regexp.MustCompile(config.ExcludeRegExp))
	return &Layered{
		docker:  d,
		config:  config,
		fs:      util.NewFileSystem(),
		tar:     tarHandler,
		scripts: scripts,
	}, nil
}

// getDestination returns the destination directory from the config.
func getDestination(config *api.Config) string {
	destination := config.Destination
	if len(destination) == 0 {
		destination = defaultDestination
	}
	return destination
}

// checkValidDirWithContents returns true if the parameter provided is a valid,
// accessible and non-empty directory.
func checkValidDirWithContents(name string) bool {
	items, err := ioutil.ReadDir(name)
	if os.IsNotExist(err) {
		glog.Warningf("Unable to access directory %q: %v", name, err)
	}
	return !(err != nil || len(items) == 0)
}

// CreateDockerfile takes the various inputs and creates the Dockerfile used by
// the docker cmd to create the image produced by s2i.
func (builder *Layered) CreateDockerfile(config *api.Config) error {
	buffer := bytes.Buffer{}

	user, err := builder.docker.GetImageUser(builder.config.BuilderImage)
	if err != nil {
		return err
	}

	scriptsDir := filepath.Join(getDestination(config), "scripts")
	sourcesDir := filepath.Join(getDestination(config), "src")

	uploadScriptsDir := path.Join(config.WorkingDir, api.UploadScripts)

	buffer.WriteString(fmt.Sprintf("FROM %s\n", builder.config.BuilderImage))
	// only COPY scripts dir if required scripts are present, i.e. the dir is not empty;
	// even if the "scripts" dir exists, the COPY would fail if it was empty
	scriptsIncluded := checkValidDirWithContents(uploadScriptsDir)
	if scriptsIncluded {
		glog.V(2).Infof("The scripts are included in %q directory", uploadScriptsDir)
		buffer.WriteString(fmt.Sprintf("COPY scripts %s\n", scriptsDir))
	} else {
		// if an err on reading or opening dir, can't copy it
		glog.V(2).Infof("Could not gather scripts from the directory %q", uploadScriptsDir)
	}
	buffer.WriteString(fmt.Sprintf("COPY src %s\n", sourcesDir))

	//TODO: We need to account for images that may not have chown. There is a proposal
	//      to specify the owner for COPY here: https://github.com/docker/docker/pull/9934
	if len(user) > 0 {
		buffer.WriteString("USER root\n")
		if scriptsIncluded {
			buffer.WriteString(fmt.Sprintf("RUN chown -R %s -- %s %s\n", user, scriptsDir, sourcesDir))
		} else {
			buffer.WriteString(fmt.Sprintf("RUN chown -R %s -- %s\n", user, sourcesDir))
		}
		buffer.WriteString(fmt.Sprintf("USER %s\n", user))
	}

	uploadDir := filepath.Join(builder.config.WorkingDir, "upload")
	if err := builder.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Writing custom Dockerfile to %s", uploadDir)
	return nil
}

// SourceTar returns a stream to the source tar file.
// TODO: this should stop generating a file, and instead stream the tar.
func (builder *Layered) SourceTar(config *api.Config) (io.ReadCloser, error) {
	uploadDir := filepath.Join(config.WorkingDir, "upload")
	tarFileName, err := builder.tar.CreateTarFile(builder.config.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return builder.fs.Open(tarFileName)
}

// Build handles the `docker build` equivalent execution, returning the
// success/failure details.
func (builder *Layered) Build(config *api.Config) (*api.Result, error) {
	if config.HasOnBuild && config.BlockOnBuild {
		return nil, fmt.Errorf("builder image uses ONBUILD instructions but ONBUILD is not allowed")
	}

	if err := builder.CreateDockerfile(config); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := builder.SourceTar(config)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	dockerImageReference, err := docker.ParseImageReference(builder.config.BuilderImage)
	if err != nil {
		return nil, err
	}
	// if we fall down this path via oc new-app, the builder image will be a docker image ref ending
	// with a @<hex image id> instead of a tag; simply appending the time stamp to the end of a
	// hex image id ref is not kosher with the docker API; so we remove the ID piece, and then
	// construct the new image name
	var newBuilderImage string
	if len(dockerImageReference.ID) == 0 {
		newBuilderImage = fmt.Sprintf("%s-%d", builder.config.BuilderImage, time.Now().UnixNano())
	} else {
		if len(dockerImageReference.Registry) > 0 {
			newBuilderImage = fmt.Sprintf("%s/", dockerImageReference.Registry)
		}
		if len(dockerImageReference.Namespace) > 0 {
			newBuilderImage = fmt.Sprintf("%s%s/", newBuilderImage, dockerImageReference.Namespace)
		}
		newBuilderImage = fmt.Sprintf("%s%s:s2i-layered-%d", newBuilderImage, dockerImageReference.Name, time.Now().UnixNano())
	}

	outReader, outWriter := io.Pipe()
	defer outReader.Close()
	defer outWriter.Close()
	opts := docker.BuildImageOptions{
		Name:         newBuilderImage,
		Stdin:        tarStream,
		Stdout:       outWriter,
		CGroupLimits: config.CGroupLimits,
	}
	// goroutine to stream container's output
	go func(reader io.Reader) {
		scanner := bufio.NewReader(reader)
		for {
			text, err := scanner.ReadString('\n')
			if err != nil {
				// we're ignoring ErrClosedPipe, as this is information
				// the docker container ended streaming logs
				if glog.Is(2) && err != io.ErrClosedPipe && err != io.EOF {
					glog.Errorf("Error reading docker stdout, %v", err)
				}
				break
			}
			glog.V(2).Info(text)
		}
	}(outReader)

	glog.V(2).Infof("Building new image %s with scripts and sources already inside", newBuilderImage)
	if err = builder.docker.BuildImage(opts); err != nil {
		return nil, err
	}

	// upon successful build we need to modify current config
	builder.config.LayeredBuild = true
	// new image name
	builder.config.BuilderImage = newBuilderImage
	// see CreateDockerfile, conditional copy, location of scripts
	scriptsIncluded := checkValidDirWithContents(path.Join(config.WorkingDir, api.UploadScripts))
	glog.V(2).Infof("Scripts dir has contents %v", scriptsIncluded)
	if scriptsIncluded {
		builder.config.ScriptsURL = "image://" + path.Join(getDestination(config), "scripts")
	} else {
		builder.config.ScriptsURL, err = builder.docker.GetScriptsURL(newBuilderImage)
		if err != nil {
			return nil, err
		}
	}

	glog.V(2).Infof("Building %s using sti-enabled image", builder.config.Tag)
	if err := builder.scripts.Execute(api.Assemble, config.AssembleUser, builder.config); err != nil {
		switch e := err.(type) {
		case errors.ContainerError:
			return nil, errors.NewAssembleError(builder.config.Tag, e.Output, e)
		default:
			return nil, err
		}
	}

	return &api.Result{
		Success: true,
	}, nil
}
