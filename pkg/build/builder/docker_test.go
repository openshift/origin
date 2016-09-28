package builder

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/docker/docker/builder/dockerfile/parser"
	"github.com/fsouza/go-dockerclient"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/generate/git"
	"github.com/openshift/origin/pkg/util/docker/dockerfile"
	"github.com/openshift/source-to-image/pkg/tar"
)

func TestInsertEnvAfterFrom(t *testing.T) {
	tests := map[string]struct {
		original string
		env      []kapi.EnvVar
		want     string
	}{
		"no FROM instruction": {
			original: `RUN echo "invalid Dockerfile"
`,
			env: []kapi.EnvVar{
				{Name: "PATH", Value: "/bin"},
			},
			want: `RUN echo "invalid Dockerfile"
`},
		"empty env": {
			original: `FROM busybox
`,
			env: []kapi.EnvVar{},
			want: `FROM busybox
`},
		"single FROM instruction": {
			original: `FROM busybox
RUN echo "hello world"
`,
			env: []kapi.EnvVar{
				{Name: "PATH", Value: "/bin"},
			},
			want: `FROM busybox
ENV "PATH"="/bin"
RUN echo "hello world"
`},
		"multiple FROM instructions": {
			original: `FROM scratch
FROM busybox
RUN echo "hello world"
`,
			env: []kapi.EnvVar{
				{Name: "PATH", Value: "/bin"},
				{Name: "GOPATH", Value: "/go"},
				{Name: "PATH", Value: "/go/bin:$PATH"},
			},
			want: `FROM scratch
ENV "PATH"="/bin" "GOPATH"="/go" "PATH"="/go/bin:$PATH"
FROM busybox
ENV "PATH"="/bin" "GOPATH"="/go" "PATH"="/go/bin:$PATH"
RUN echo "hello world"`},
	}
	for name, test := range tests {
		got, err := parser.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want, err := parser.Parse(strings.NewReader(test.want))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		insertEnvAfterFrom(got, test.env)
		if !bytes.Equal(dockerfile.ParseTreeToDockerfile(got), dockerfile.ParseTreeToDockerfile(want)) {
			t.Errorf("%s: insertEnvAfterFrom(node, %+v) = %+v; want %+v", name, test.env, got, want)
			t.Logf("resulting Dockerfile:\n%s", dockerfile.ParseTreeToDockerfile(got))
		}
	}
}

func TestReplaceLastFrom(t *testing.T) {
	tests := []struct {
		original string
		image    string
		want     string
	}{
		{
			original: `# no FROM instruction`,
			image:    "centos",
			want:     ``,
		},
		{
			original: `FROM scratch
# FROM busybox
RUN echo "hello world"
`,
			image: "centos",
			want: `FROM centos
RUN echo "hello world"
`,
		},
		{
			original: `FROM scratch
FROM busybox
RUN echo "hello world"
`,
			image: "centos",
			want: `FROM scratch
FROM centos
RUN echo "hello world"
`,
		},
	}
	for i, test := range tests {
		got, err := parser.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("test[%d]: %v", i, err)
			continue
		}
		want, err := parser.Parse(strings.NewReader(test.want))
		if err != nil {
			t.Errorf("test[%d]: %v", i, err)
			continue
		}
		replaceLastFrom(got, test.image)
		if !bytes.Equal(dockerfile.ParseTreeToDockerfile(got), dockerfile.ParseTreeToDockerfile(want)) {
			t.Errorf("test[%d]: replaceLastFrom(node, %+v) = %+v; want %+v", i, test.image, got, want)
			t.Logf("resulting Dockerfile:\n%s", dockerfile.ParseTreeToDockerfile(got))
		}
	}
}

// TestDockerfilePath validates that we can use a Dockefile with a custom name, and in a sub-directory
func TestDockerfilePath(t *testing.T) {
	tests := []struct {
		contextDir     string
		dockerfilePath string
		dockerStrategy *api.DockerBuildStrategy
	}{
		// default Dockerfile path
		{
			dockerfilePath: "Dockerfile",
			dockerStrategy: &api.DockerBuildStrategy{},
		},
		// custom Dockerfile path in the root context
		{
			dockerfilePath: "mydockerfile",
			dockerStrategy: &api.DockerBuildStrategy{
				DockerfilePath: "mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		{
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &api.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		// with a contextDir
		{
			contextDir:     "somedir",
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &api.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
	}

	for _, test := range tests {
		buildDir, err := ioutil.TempDir("", "dockerfile-path")
		if err != nil {
			t.Errorf("failed to create tmpdir: %v", err)
			continue
		}
		absoluteDockerfilePath := filepath.Join(buildDir, test.contextDir, test.dockerfilePath)
		dockerfileContent := "FROM openshift/origin-base"
		if err = os.MkdirAll(filepath.Dir(absoluteDockerfilePath), os.FileMode(0750)); err != nil {
			t.Errorf("failed to create directory %s: %v", filepath.Dir(absoluteDockerfilePath), err)
			continue
		}
		if err = ioutil.WriteFile(absoluteDockerfilePath, []byte(dockerfileContent), os.FileMode(0644)); err != nil {
			t.Errorf("failed to write dockerfile to %s: %v", absoluteDockerfilePath, err)
			continue
		}

		build := &api.Build{
			Spec: api.BuildSpec{
				CommonSpec: api.CommonSpec{
					Source: api.BuildSource{
						Git: &api.GitBuildSource{
							URI: "http://github.com/openshift/origin.git",
						},
						ContextDir: test.contextDir,
					},
					Strategy: api.BuildStrategy{
						DockerStrategy: test.dockerStrategy,
					},
					Output: api.BuildOutput{
						To: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "test/test-result:latest",
						},
					},
				},
			},
		}

		dockerClient := &FakeDocker{
			buildImageFunc: func(opts docker.BuildImageOptions) error {
				if opts.Dockerfile != test.dockerfilePath {
					t.Errorf("Unexpected dockerfile path: %s (expected: %s)", opts.Dockerfile, test.dockerfilePath)
				}
				return nil
			},
		}

		dockerBuilder := &DockerBuilder{
			dockerClient: dockerClient,
			build:        build,
			gitClient:    git.NewRepository(),
			tar:          tar.New(),
		}

		// this will validate that the Dockerfile is readable
		// and append some labels to the Dockerfile
		if err = dockerBuilder.addBuildParameters(buildDir); err != nil {
			t.Errorf("failed to add build parameters: %v", err)
			continue
		}

		// check that our Dockerfile has been modified
		dockerfileData, err := ioutil.ReadFile(absoluteDockerfilePath)
		if err != nil {
			t.Errorf("failed to read dockerfile %s: %v", absoluteDockerfilePath, err)
			continue
		}
		if !strings.Contains(string(dockerfileData), dockerfileContent) {
			t.Errorf("Updated Dockerfile content does not contains the original Dockerfile content.\n\nOriginal content:\n%s\n\nUpdated content:\n%s\n", dockerfileContent, string(dockerfileData))
			continue
		}

		// check that the docker client is called with the right Dockerfile parameter
		if err = dockerBuilder.dockerBuild(buildDir, "", []api.SecretBuildSource{}); err != nil {
			t.Errorf("failed to build: %v", err)
			continue
		}
	}
}

func TestEmptySource(t *testing.T) {
	build := &api.Build{
		Spec: api.BuildSpec{
			CommonSpec: api.CommonSpec{
				Source: api.BuildSource{},
				Strategy: api.BuildStrategy{
					DockerStrategy: &api.DockerBuildStrategy{},
				},
				Output: api.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "test/test-result:latest",
					},
				},
			},
		},
	}

	dockerBuilder := &DockerBuilder{
		build: build,
	}

	if err := dockerBuilder.Build(); err == nil {
		t.Error("Should have received error on docker build")
	} else {
		if !strings.Contains(err.Error(), "must provide a value for at least one of source, binary, images, or dockerfile") {
			t.Errorf("Did not receive correct error: %v", err)
		}
	}
}
func TestGetDockerfileFrom(t *testing.T) {
	tests := map[string]struct {
		dockerfileContent string
		want              []string
	}{
		"no FROM instruction": {
			dockerfileContent: `RUN echo "invalid Dockerfile"
`,
			want: []string{},
		},
		"single FROM instruction": {
			dockerfileContent: `FROM scratch
RUN echo "hello world"
`,
			want: []string{"scratch"},
		},
		"multi FROM instruction": {
			dockerfileContent: `FROM scratch
FROM busybox
RUN echo "hello world"
`,
			want: []string{"scratch", "busybox"},
		},
	}
	for i, test := range tests {
		buildDir, err := ioutil.TempDir("", "dockerfile-path")
		if err != nil {
			t.Errorf("failed to create tmpdir: %v", err)
			continue
		}
		dockerfilePath := filepath.Join(buildDir, defaultDockerfilePath)
		dockerfileContent := test.dockerfileContent
		if err = os.MkdirAll(filepath.Dir(dockerfilePath), os.FileMode(0750)); err != nil {
			t.Errorf("failed to create directory %s: %v", filepath.Dir(dockerfilePath), err)
			continue
		}
		if err = ioutil.WriteFile(dockerfilePath, []byte(dockerfileContent), os.FileMode(0644)); err != nil {
			t.Errorf("failed to write dockerfile to %s: %v", dockerfilePath, err)
			continue
		}
		froms := getDockerfileFrom(dockerfilePath)
		if len(froms) != len(test.want) {
			t.Errorf("test[%s]: getDockerfileFrom(dockerfilepath, %s) = %+v; want %+v", i, dockerfilePath, froms, test.want)
			t.Logf("Dockerfile froms::\n%v", froms)
			continue
		}
		for fi := range froms {
			if froms[fi] != test.want[fi] {
				t.Errorf("test[%s]: getDockerfileFrom(dockerfilepath, %s) = %+v; want %+v", i, dockerfilePath, froms, test.want)
				t.Logf("Dockerfile froms::\n%v", froms)
				break
			}
		}
	}
}
