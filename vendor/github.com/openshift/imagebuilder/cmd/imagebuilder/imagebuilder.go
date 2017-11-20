package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	dockertypes "github.com/docker/docker/api/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/golang/glog"

	"github.com/openshift/imagebuilder"
	"github.com/openshift/imagebuilder/dockerclient"
)

func main() {
	log.SetFlags(0)
	options := dockerclient.NewClientExecutor(nil)
	var tags stringSliceFlag
	var dockerfilePath string
	var imageFrom string
	var mountSpecs stringSliceFlag

	flag.Var(&tags, "t", "The name to assign this image, if any. May be specified multiple times.")
	flag.Var(&tags, "tag", "The name to assign this image, if any. May be specified multiple times.")
	flag.StringVar(&dockerfilePath, "f", dockerfilePath, "An optional path to a Dockerfile to use. You may pass multiple docker files using the operating system delimiter.")
	flag.StringVar(&dockerfilePath, "file", dockerfilePath, "An optional path to a Dockerfile to use. You may pass multiple docker files using the operating system delimiter.")
	flag.StringVar(&imageFrom, "from", imageFrom, "An optional FROM to use instead of the one in the Dockerfile.")
	flag.Var(&mountSpecs, "mount", "An optional list of files and directories to mount during the build. Use SRC:DST syntax for each path.")
	flag.BoolVar(&options.AllowPull, "allow-pull", true, "Pull the images that are not present.")
	flag.BoolVar(&options.IgnoreUnrecognizedInstructions, "ignore-unrecognized-instructions", true, "If an unrecognized Docker instruction is encountered, warn but do not fail the build.")
	flag.BoolVar(&options.StrictVolumeOwnership, "strict-volume-ownership", false, "Due to limitations in docker `cp`, owner permissions on volumes are lost. This flag will fail builds that might fall victim to this.")

	flag.Parse()

	args := flag.Args()
	if len(args) != 1 {
		log.Fatalf("You must provide one argument, the name of a directory to build")
	}

	options.Directory = args[0]
	if len(tags) > 0 {
		options.Tag = tags[0]
		options.AdditionalTags = tags[1:]
	}
	if len(dockerfilePath) == 0 {
		dockerfilePath = filepath.Join(options.Directory, "Dockerfile")
	}

	var mounts []dockerclient.Mount
	for _, s := range mountSpecs {
		segments := strings.Split(s, ":")
		if len(segments) != 2 {
			log.Fatalf("--mount must be of the form SOURCE:DEST")
		}
		mounts = append(mounts, dockerclient.Mount{SourcePath: segments[0], DestinationPath: segments[1]})
	}
	options.TransientMounts = mounts

	options.Out, options.ErrOut = os.Stdout, os.Stderr
	options.AuthFn = func(name string) ([]dockertypes.AuthConfig, bool) {
		return nil, false
	}
	options.LogFn = func(format string, args ...interface{}) {
		if glog.V(2) {
			log.Printf("Builder: "+format, args...)
		} else {
			fmt.Fprintf(options.ErrOut, "--> %s\n", fmt.Sprintf(format, args...))
		}
	}

	// Accept ARGS on the command line
	arguments := make(map[string]string)

	dockerfiles := filepath.SplitList(dockerfilePath)
	if len(dockerfiles) == 0 {
		dockerfiles = []string{filepath.Join(options.Directory, "Dockerfile")}
	}

	if err := build(dockerfiles[0], dockerfiles[1:], arguments, imageFrom, options); err != nil {
		log.Fatal(err.Error())
	}
}

func build(dockerfile string, additionalDockerfiles []string, arguments map[string]string, from string, e *dockerclient.ClientExecutor) error {
	if err := e.DefaultExcludes(); err != nil {
		return fmt.Errorf("error: Could not parse default .dockerignore: %v", err)
	}

	client, err := docker.NewClientFromEnv()
	if err != nil {
		return fmt.Errorf("error: No connection to Docker available: %v", err)
	}
	e.Client = client

	// TODO: handle signals
	defer func() {
		for _, err := range e.Release() {
			fmt.Fprintf(e.ErrOut, "error: Unable to clean up build: %v\n", err)
		}
	}()

	b, node, err := imagebuilder.NewBuilderForFile(dockerfile, arguments)
	if err != nil {
		return err
	}
	if err := e.Prepare(b, node, from); err != nil {
		return err
	}
	if err := e.Execute(b, node); err != nil {
		return err
	}

	for _, s := range additionalDockerfiles {
		_, node, err := imagebuilder.NewBuilderForFile(s, arguments)
		if err != nil {
			return err
		}
		if err := e.Execute(b, node); err != nil {
			return err
		}
	}

	return e.Commit(b)
}

type stringSliceFlag []string

func (f *stringSliceFlag) Set(s string) error {
	*f = append(*f, s)
	return nil
}

func (f *stringSliceFlag) String() string {
	return strings.Join(*f, " ")
}
