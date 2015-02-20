package onbuild

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

type fakeSourceHandler struct{}

func (*fakeSourceHandler) Prepare(r *api.Request) error {
	return nil
}

func (*fakeSourceHandler) Download(r *api.Request) error {
	return nil
}

type fakeCleaner struct{}

func (*fakeCleaner) Cleanup(*api.Request) {}

func newFakeOnBuild() *OnBuild {
	return &OnBuild{
		docker:  &test.FakeDocker{},
		git:     &test.FakeGit{},
		fs:      &test.FakeFileSystem{},
		tar:     &test.FakeTar{},
		source:  &fakeSourceHandler{},
		garbage: &fakeCleaner{},
	}
}

func checkDockerfile(fs *test.FakeFileSystem, t *testing.T) {
	if fs.WriteFileError != nil {
		t.Errorf("%v", fs.WriteFileError)
	}
	if fs.WriteFileName != "upload/src/Dockerfile" {
		t.Errorf("Expected Dockerfile in 'upload/src/Dockerfile', got %v", fs.WriteFileName)
	}
	if !strings.Contains(fs.WriteFileContent, `ENTRYPOINT ["./run"]`) {
		t.Errorf("The Dockerfile does not set correct entrypoint:\n %s\n", fs.WriteFileContent)
	}
}

func TestCreateDockerfile(t *testing.T) {
	fakeRequest := &api.Request{
		BaseImage: "fake:onbuild",
	}
	b := newFakeOnBuild()
	fakeFs := &test.FakeFileSystem{
		Files: []os.FileInfo{
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", false, 0777},
		},
	}
	b.fs = fakeFs
	err := b.CreateDockerfile(fakeRequest)
	if err != nil {
		t.Errorf("%v", err)
	}
	checkDockerfile(fakeFs, t)
}

func TestCreateDockerfileWithAssemble(t *testing.T) {
	fakeRequest := &api.Request{
		BaseImage: "fake:onbuild",
	}
	b := newFakeOnBuild()
	fakeFs := &test.FakeFileSystem{
		Files: []os.FileInfo{
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", false, 0777},
			&test.FakeFile{"assemble", false, 0777},
		},
	}
	b.fs = fakeFs
	err := b.CreateDockerfile(fakeRequest)
	if err != nil {
		t.Errorf("%v", err)
	}
	checkDockerfile(fakeFs, t)
	if !strings.Contains(fakeFs.WriteFileContent, `RUN sh assemble`) {
		t.Errorf("The Dockerfile does not run assemble:\n%s\n", fakeFs.WriteFileContent)
	}
}

func TestBuild(t *testing.T) {
	fakeRequest := &api.Request{
		BaseImage: "fake:onbuild",
		Tag:       "fakeapp",
	}
	b := newFakeOnBuild()
	fakeFs := &test.FakeFileSystem{
		Files: []os.FileInfo{
			&test.FakeFile{"config.ru", false, 0600},
			&test.FakeFile{"app.rb", false, 0600},
			&test.FakeFile{"run", false, 0777},
		},
	}
	b.fs = fakeFs
	result, err := b.Build(fakeRequest)
	if err != nil {
		t.Errorf("%v", err)
	}
	if !result.Success {
		t.Errorf("Expected successfull build, got: %v", result)
	}
	checkDockerfile(fakeFs, t)
	fmt.Printf("result: %v\n", result)
}
