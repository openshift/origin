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

func TestCheckNoRoot(t *testing.T) {
	tests := []struct {
		name      string
		noRoot    bool
		user      string
		onbuild   []string
		expectErr bool
	}{
		{
			name:      "NoRoot is not set",
			noRoot:    false,
			user:      "root",
			onbuild:   []string{},
			expectErr: false,
		},
		{
			name:      "NoRoot is set, non-numeric user",
			noRoot:    true,
			user:      "default",
			onbuild:   []string{},
			expectErr: true,
		},
		{
			name:      "NoRoot is set, user 0",
			noRoot:    true,
			user:      "0",
			onbuild:   []string{},
			expectErr: true,
		},
		{
			name:      "NoRoot is set, numeric user, non-numeric onbuild",
			noRoot:    true,
			user:      "100",
			onbuild:   []string{"COPY test test", "USER default"},
			expectErr: true,
		},
		{
			name:      "NoRoot is set, numeric user, no onbuild user directive",
			noRoot:    true,
			user:      "200",
			onbuild:   []string{"VOLUME /data"},
			expectErr: false,
		},
		{
			name:      "NoRoot is set, numeric user, onbuild numeric user directive",
			noRoot:    true,
			user:      "200",
			onbuild:   []string{"USER 500", "VOLUME /data"},
			expectErr: false,
		},
		{
			name:      "NoRoot is set, numeric user, onbuild user 0",
			noRoot:    true,
			user:      "200",
			onbuild:   []string{"RUN echo \"hello world\"", "USER 0"},
			expectErr: true,
		},
	}

	for _, tc := range tests {
		cfg := &api.Config{
			NoRoot: tc.noRoot,
		}
		onbuild := &OnBuild{
			docker: &test.FakeDocker{
				GetImageUserResult: tc.user,
				GetOnBuildResult:   tc.onbuild,
			},
		}
		err := onbuild.checkNoRoot(cfg)
		if err != nil && !tc.expectErr {
			t.Errorf("%s: unexpected error: %v", tc.name, err)
		}
		if err == nil && tc.expectErr {
			t.Errorf("%s: expected error, but did not get any", tc.name)
		}
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
