package builder

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/builder/parser"
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
RUN echo "hello world"
`},
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
		if !reflect.DeepEqual(got, want) {
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
		if !reflect.DeepEqual(got, want) {
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
		if err = dockerBuilder.dockerBuild(buildDir); err != nil {
			t.Errorf("failed to build: %v", err)
			continue
		}
	}
}
