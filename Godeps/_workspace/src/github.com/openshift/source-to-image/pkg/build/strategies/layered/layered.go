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

//getDestination returns the destination directory from the config
func getDestination(config *api.Config) string {
	destination := config.Destination
	if len(destination) == 0 {
		destination = defaultDestination
	}
	return destination
}

//checkValidDirWithContents will return true if the parameter provided is a valid, accessible directory that has contents (i.e. is not empty
func checkValidDirWithContents(name string) bool {
	items, err := ioutil.ReadDir(name)
	if os.IsNotExist(err) {
		glog.Warningf("Unable to access directory %q: %v", name, err)
	}
	return !(err != nil || len(items) == 0)
}

//CreateDockerfile takes the various inputs and creates the Dockerfile used by the docker cmd
// to create the image produces by s2i
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
	// only COPY scripts dir if required scripts are present, i.e. the dir is not empty;
	// even if the "scripts" dir exists, the COPY would fail if it was empty
	scriptsIncluded := checkValidDirWithContents(path.Join(config.WorkingDir, api.UploadScripts))
	if scriptsIncluded {
		glog.V(2).Infof("The scripts are included in %q directory", path.Join(config.WorkingDir, api.UploadScripts))
		buffer.WriteString(fmt.Sprintf("COPY scripts %s\n", locations[0]))
	} else {
		// if an err on reading or opening dir, can't copy it
		glog.V(2).Infof("Could not gather scripts from the directory %q", path.Join(config.WorkingDir, api.UploadScripts))
	}
	buffer.WriteString(fmt.Sprintf("COPY src %s\n", locations[1]))

	//TODO: We need to account for images that may not have chown. There is a proposal
	//      to specify the owner for COPY here: https://github.com/docker/docker/pull/9934
	if len(user) > 0 {
		buffer.WriteString("USER root\n")
		if scriptsIncluded {
			buffer.WriteString(fmt.Sprintf("RUN chown -R %s %s %s\n", user, locations[0], locations[1]))
		} else {
			buffer.WriteString(fmt.Sprintf("RUN chown -R %s %s\n", user, locations[1]))
		}
		buffer.WriteString(fmt.Sprintf("USER %s\n", user))
	}

	uploadDir := filepath.Join(b.config.WorkingDir, "upload")
	if err := b.fs.WriteFile(filepath.Join(uploadDir, "Dockerfile"), buffer.Bytes()); err != nil {
		return err
	}
	glog.V(2).Infof("Writing custom Dockerfile to %s", uploadDir)
	return nil
}

// TODO: this should stop generating a file, and instead stream the tar.
//SourceTar returns a stream to the source tar file
func (b *Layered) SourceTar(config *api.Config) (io.ReadCloser, error) {
	uploadDir := filepath.Join(config.WorkingDir, "upload")
	tarFileName, err := b.tar.CreateTarFile(b.config.WorkingDir, uploadDir)
	if err != nil {
		return nil, err
	}
	return b.fs.Open(tarFileName)
}

//Build handles the `docker build` equivalent execution, returning the success/failure details
func (b *Layered) Build(config *api.Config) (*api.Result, error) {
	if config.DisableImplicitBuild {
		return nil, fmt.Errorf("builder image is missing basic requirements (sh or tar), but implicit Docker builds are disabled so a layered build cannot be performed.")
	}
	if err := b.CreateDockerfile(config); err != nil {
		return nil, err
	}

	glog.V(2).Info("Creating application source code image")
	tarStream, err := b.SourceTar(config)
	if err != nil {
		return nil, err
	}
	defer tarStream.Close()

	dockerImageReference, err := docker.ParseDockerImageReference(b.config.BuilderImage)
	if err != nil {
		return nil, err
	}
	// if we fall down this path via oc new-app, the builder image will be a docker image ref ending
	// with a @<hex image id> instead of a tag; simply appending the time stamp to the end of a
	// hex image id ref is not kosher with the docker API; so we remove the ID piece, and then
	// construct the new image name
	var newBuilderImage string
	if len(dockerImageReference.ID) == 0 {
		newBuilderImage = fmt.Sprintf("%s-%d", b.config.BuilderImage, time.Now().UnixNano())
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
				if glog.V(2) && err != io.ErrClosedPipe && err != io.EOF {
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
	// see CreateDockerfile, conditional copy, location of scripts
	scriptsIncluded := checkValidDirWithContents(path.Join(config.WorkingDir, api.UploadScripts))
	glog.V(2).Infof("Scripts dir has contents %v", scriptsIncluded)
	if scriptsIncluded {
		b.config.ScriptsURL = "image://" + path.Join(getDestination(config), "scripts")
	} else {
		b.config.ScriptsURL, err = b.docker.GetScriptsURL(newBuilderImage)
		if err != nil {
			return nil, err
		}
	}

	glog.V(2).Infof("Building %s using sti-enabled image", b.config.Tag)
	if err := b.scripts.Execute(api.Assemble, config.AssembleUser, b.config); err != nil {
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
