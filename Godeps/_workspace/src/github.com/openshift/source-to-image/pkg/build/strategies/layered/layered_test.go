package layered

import (
	"errors"
	"regexp"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

type FakeExecutor struct{}

func (f *FakeExecutor) Execute(api.Script, *api.Request) error {
	return nil
}

func newFakeLayered() *Layered {
	return &Layered{
		docker:  &test.FakeDocker{},
		request: &api.Request{},
		fs:      &test.FakeFileSystem{},
		tar:     &test.FakeTar{},
		scripts: &FakeExecutor{},
	}
}

func TestBuildOK(t *testing.T) {
	l := newFakeLayered()
	l.request.BaseImage = "test/image"
	_, err := l.Build(l.request)
	if err != nil {
		t.Errorf("Unexpected error returned: %v", err)
	}
	if !l.request.LayeredBuild {
		t.Errorf("Expected LayeredBuild to be true!")
	}
	if m, _ := regexp.MatchString(`test/image-\d+`, l.request.BaseImage); !m {
		t.Errorf("Expected BaseImage test/image-withnumbers, but got %s", l.request.BaseImage)
	}
	if l.request.ExternalRequiredScripts {
		t.Errorf("Expected ExternalRequiredScripts to be false!")
	}
	if l.request.ScriptsURL != "image:///tmp/scripts" {
		t.Error("Expected ScriptsURL image:///tmp/scripts, but got %s", l.request.ScriptsURL)
	}
	if l.request.Location != "/tmp/src" {
		t.Errorf("Expected Location /tmp/src, but got %s", l.request.Location)
	}
}

func TestBuildErrorWriteDockerfile(t *testing.T) {
	l := newFakeLayered()
	l.fs.(*test.FakeFileSystem).WriteFileError = errors.New("WriteDockerfileError")
	_, err := l.Build(l.request)
	if err == nil || err.Error() != "WriteDockerfileError" {
		t.Errorf("An error was expected for WriteDockerfile, but got different: %v", err)
	}
}

func TestBuildErrorCreateTarFile(t *testing.T) {
	l := newFakeLayered()
	l.tar.(*test.FakeTar).CreateTarError = errors.New("CreateTarError")
	_, err := l.Build(l.request)
	if err == nil || err.Error() != "CreateTarError" {
		t.Error("An error was expected for CreateTar, but got different: %v", err)
	}
}

func TestBuildErrorOpenTarFile(t *testing.T) {
	l := newFakeLayered()
	l.fs.(*test.FakeFileSystem).OpenError = errors.New("OpenTarError")
	_, err := l.Build(l.request)
	if err == nil || err.Error() != "OpenTarError" {
		t.Errorf("An error was expected for OpenTarFile, but got different: %v", err)
	}
}

func TestBuildErrorBuildImage(t *testing.T) {
	l := newFakeLayered()
	l.docker.(*test.FakeDocker).BuildImageError = errors.New("BuildImageError")
	_, err := l.Build(l.request)
	if err == nil || err.Error() != "BuildImageError" {
		t.Errorf("An error was expected for BuildImage, but got different: %v", err)
	}
}
