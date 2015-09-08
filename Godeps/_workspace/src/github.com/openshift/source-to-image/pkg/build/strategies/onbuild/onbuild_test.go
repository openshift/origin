package onbuild

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/docker/docker/builder/parser"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/test"
)

type fakeSourceHandler struct{}

func (*fakeSourceHandler) Prepare(r *api.Config) error {
	return nil
}

func (*fakeSourceHandler) Ignore(r *api.Config) error {
	return nil
}

func (*fakeSourceHandler) Download(r *api.Config) (*api.SourceInfo, error) {
	return &api.SourceInfo{}, nil
}

type fakeCleaner struct{}

func (*fakeCleaner) Cleanup(*api.Config) {}

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

	buf := bytes.NewBuffer([]byte(fs.WriteFileContent))
	if _, err := parser.Parse(buf); err != nil {
		t.Errorf("cannot parse new Dockerfile: " + err.Error())
	}

}

func TestCreateDockerfile(t *testing.T) {
	fakeRequest := &api.Config{
		BuilderImage: "fake:onbuild",
		Environment:  map[string]string{"FOO": "BAR", "TEST": "A VALUE"},
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
	fakeRequest := &api.Config{
		BuilderImage: "fake:onbuild",
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
	fakeRequest := &api.Config{
		BuilderImage: "fake:onbuild",
		Tag:          "fakeapp",
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
