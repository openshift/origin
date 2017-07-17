package layered

import (
	"errors"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"regexp/syntax"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/build"
	"github.com/openshift/source-to-image/pkg/docker"
	"github.com/openshift/source-to-image/pkg/test"
)

type FakeExecutor struct{}

func (f *FakeExecutor) Execute(string, string, *api.Config) error {
	return nil
}

func newFakeLayered() *Layered {
	return &Layered{
		docker:  &docker.FakeDocker{},
		config:  &api.Config{},
		fs:      &test.FakeFileSystem{},
		tar:     &test.FakeTar{},
		scripts: &FakeExecutor{},
	}
}

func newFakeLayeredWithScripts(workDir string) *Layered {
	return &Layered{
		docker:  &docker.FakeDocker{},
		config:  &api.Config{WorkingDir: workDir},
		fs:      &test.FakeFileSystem{},
		tar:     &test.FakeTar{},
		scripts: &FakeExecutor{},
	}
}

func TestBuildOK(t *testing.T) {
	workDir, _ := ioutil.TempDir("", "sti")
	scriptDir := filepath.Join(workDir, api.UploadScripts)
	err := os.MkdirAll(scriptDir, 0700)
	assemble := filepath.Join(scriptDir, api.Assemble)
	file, err := os.Create(assemble)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	defer file.Close()
	defer os.RemoveAll(workDir)
	l := newFakeLayeredWithScripts(workDir)
	l.config.BuilderImage = "test/image"
	_, err = l.Build(l.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.config.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if m, _ := regexp.MatchString(`s2i-layered-temp-image-\d+`, l.config.BuilderImage); !m {
		t.Errorf("Expected BuilderImage s2i-layered-temp-image-withnumbers, but got %s", l.config.BuilderImage)
	}
	// without config.Destination explicitly set, we should get /tmp/scripts for the scripts url
	// assuming the assemble script we created above is off the working dir
	if l.config.ScriptsURL != "image:///tmp/scripts" {
		t.Errorf("Expected ScriptsURL image:///tmp/scripts, but got %s", l.config.ScriptsURL)
	}
	if len(l.config.Destination) != 0 {
		t.Errorf("Unexpected Destination %s", l.config.Destination)
	}
}

func TestBuildOKWithImageRef(t *testing.T) {
	workDir, _ := ioutil.TempDir("", "sti")
	scriptDir := filepath.Join(workDir, api.UploadScripts)
	err := os.MkdirAll(scriptDir, 0700)
	assemble := filepath.Join(scriptDir, api.Assemble)
	file, err := os.Create(assemble)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	defer file.Close()
	defer os.RemoveAll(workDir)
	l := newFakeLayeredWithScripts(workDir)
	l.config.BuilderImage = "docker.io/uptoknow/ruby-20-centos7@sha256:d6f5718b85126954d98931e654483ee794ac357e0a98f4a680c1e848d78863a1"
	_, err = l.Build(l.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.config.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if !strings.HasPrefix(l.config.BuilderImage, "s2i-layered-temp-image-") {
		t.Errorf("Expected BuilderImage to start with s2i-layered-temp-image-, but got %s", l.config.BuilderImage)
	}
	l.config.BuilderImage = "uptoknow/ruby-20-centos7@sha256:d6f5718b85126954d98931e654483ee794ac357e0a98f4a680c1e848d78863a1"
	_, err = l.Build(l.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.config.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if !strings.HasPrefix(l.config.BuilderImage, "s2i-layered-temp-image-") {
		t.Errorf("Expected BuilderImage to start with s2i-layered-temp-image-, but got %s", l.config.BuilderImage)
	}
	l.config.BuilderImage = "ruby-20-centos7@sha256:d6f5718b85126954d98931e654483ee794ac357e0a98f4a680c1e848d78863a1"
	_, err = l.Build(l.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.config.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if !strings.HasPrefix(l.config.BuilderImage, "s2i-layered-temp-image-") {
		t.Errorf("Expected BuilderImage to start with s2i-layered-temp-image-, but got %s", l.config.BuilderImage)
	}
}

func TestBuildNoScriptsProvided(t *testing.T) {
	l := newFakeLayered()
	l.config.BuilderImage = "test/image"
	_, err := l.Build(l.config)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.config.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if m, _ := regexp.MatchString(`s2i-layered-temp-image-\d+`, l.config.BuilderImage); !m {
		t.Errorf("Expected BuilderImage s2i-layered-temp-image-withnumbers, but got %s", l.config.BuilderImage)
	}
	if len(l.config.Destination) != 0 {
		t.Errorf("Unexpected Destination %s", l.config.Destination)
	}
}

func TestBuildErrorWriteDockerfile(t *testing.T) {
	l := newFakeLayered()
	l.config.BuilderImage = "test/image"
	l.fs.(*test.FakeFileSystem).WriteFileError = errors.New("WriteDockerfileError")
	_, err := l.Build(l.config)
	if err == nil || err.Error() != "WriteDockerfileError" {
		t.Errorf("An error was expected for WriteDockerfile, but got different: %v", err)
	}
}

func TestBuildErrorCreateTarFile(t *testing.T) {
	l := newFakeLayered()
	l.config.BuilderImage = "test/image"
	l.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	_, err := l.Build(l.config)
	if err == nil || err.Error() != "CreateTarError" {
		t.Errorf("An error was expected for CreateTar, but got different: %v", err)
	}
}

func TestBuildErrorBuildImage(t *testing.T) {
	l := newFakeLayered()
	l.config.BuilderImage = "test/image"
	l.docker.(*docker.FakeDocker).BuildImageError = errors.New("BuildImageError")
	_, err := l.Build(l.config)
	if err == nil || err.Error() != "BuildImageError" {
		t.Errorf("An error was expected for BuildImage, but got different: %v", err)
	}
}

func TestBuildErrorBadImageName(t *testing.T) {
	l := newFakeLayered()
	_, err := l.Build(l.config)
	if err == nil || !strings.Contains(err.Error(), "builder image name cannot be empty") {
		t.Errorf("A builder image name cannot be empty error was expected, but got different: %v", err)
	}
}

func TestBuildErrorOnBuildBlocked(t *testing.T) {
	l := newFakeLayered()
	l.config.BlockOnBuild = true
	l.config.HasOnBuild = true
	_, err := l.Build(l.config)
	if err == nil || !strings.Contains(err.Error(), "builder image uses ONBUILD instructions but ONBUILD is not allowed") {
		t.Errorf("expected error from onbuild due to blocked ONBUILD, got: %v", err)
	}
}

func TestNewWithInvalidExcludeRegExp(t *testing.T) {
	_, err := New(nil, &api.Config{
		DockerConfig:  docker.GetDefaultDockerConfig(),
		ExcludeRegExp: "[",
	}, nil, nil, build.Overrides{})
	if syntaxErr, ok := err.(*syntax.Error); ok && syntaxErr.Code != syntax.ErrMissingBracket {
		t.Errorf("expected regexp compilation error, got %v", err)
	}
}
