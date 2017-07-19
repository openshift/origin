package builder

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fsouza/go-dockerclient"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/source-to-image/pkg/tar"
	s2ifs "github.com/openshift/source-to-image/pkg/util/fs"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/build/util/dockerfile"
	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/generate/git"
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
		got, err := dockerfile.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("%s: %v", name, err)
			continue
		}
		want, err := dockerfile.Parse(strings.NewReader(test.want))
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
		got, err := dockerfile.Parse(strings.NewReader(test.original))
		if err != nil {
			t.Errorf("test[%d]: %v", i, err)
			continue
		}
		want, err := dockerfile.Parse(strings.NewReader(test.want))
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

// TestDockerfilePath validates that we can use a Dockerfile with a custom name, and in a sub-directory
func TestDockerfilePath(t *testing.T) {
	tests := []struct {
		contextDir     string
		dockerfilePath string
		dockerStrategy *buildapi.DockerBuildStrategy
	}{
		// default Dockerfile path
		{
			dockerfilePath: "Dockerfile",
			dockerStrategy: &buildapi.DockerBuildStrategy{},
		},
		// custom Dockerfile path in the root context
		{
			dockerfilePath: "mydockerfile",
			dockerStrategy: &buildapi.DockerBuildStrategy{
				DockerfilePath: "mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		{
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &buildapi.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
		// custom Dockerfile path in a sub directory
		// with a contextDir
		{
			contextDir:     "somedir",
			dockerfilePath: "dockerfiles/mydockerfile",
			dockerStrategy: &buildapi.DockerBuildStrategy{
				DockerfilePath: "dockerfiles/mydockerfile",
			},
		},
	}

	from := "FROM openshift/origin-base"
	expected := []string{
		from,
		// expected env variables
		"\"OPENSHIFT_BUILD_NAME\"=\"name\"",
		"\"OPENSHIFT_BUILD_NAMESPACE\"=\"namespace\"",
		"\"OPENSHIFT_BUILD_SOURCE\"=\"http://github.com/openshift/origin.git\"",
		"\"OPENSHIFT_BUILD_COMMIT\"=\"commitid\"",
		// expected labels
		"\"io.openshift.build.commit.author\"=\"test user \\u003ctest@email.com\\u003e\"",
		"\"io.openshift.build.commit.date\"=\"date\"",
		"\"io.openshift.build.commit.id\"=\"commitid\"",
		"\"io.openshift.build.commit.ref\"=\"ref\"",
		"\"io.openshift.build.commit.message\"=\"message\"",
		"\"io.openshift.build.name\"=\"name\"",
		"\"io.openshift.build.namespace\"=\"namespace\"",
	}

	for _, test := range tests {
		buildDir, err := ioutil.TempDir("", "dockerfile-path")
		if err != nil {
			t.Errorf("failed to create tmpdir: %v", err)
			continue
		}
		defer func() {
			if err := os.RemoveAll(buildDir); err != nil {
				t.Fatal(err)
			}
		}()

		absoluteDockerfilePath := filepath.Join(buildDir, test.contextDir, test.dockerfilePath)
		if err = os.MkdirAll(filepath.Dir(absoluteDockerfilePath), os.FileMode(0750)); err != nil {
			t.Errorf("failed to create directory %s: %v", filepath.Dir(absoluteDockerfilePath), err)
			continue
		}
		if err = ioutil.WriteFile(absoluteDockerfilePath, []byte(from), os.FileMode(0644)); err != nil {
			t.Errorf("failed to write dockerfile to %s: %v", absoluteDockerfilePath, err)
			continue
		}

		build := &buildapi.Build{
			Spec: buildapi.BuildSpec{
				CommonSpec: buildapi.CommonSpec{
					Source: buildapi.BuildSource{
						Git: &buildapi.GitBuildSource{
							URI: "http://github.com/openshift/origin.git",
						},
						ContextDir: test.contextDir,
					},
					Strategy: buildapi.BuildStrategy{
						DockerStrategy: test.dockerStrategy,
					},
					Output: buildapi.BuildOutput{
						To: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "test/test-result:latest",
						},
					},
				},
			},
		}
		build.Name = "name"
		build.Namespace = "namespace"

		sourceInfo := &git.SourceInfo{}
		sourceInfo.AuthorName = "test user"
		sourceInfo.AuthorEmail = "test@email.com"
		sourceInfo.Date = "date"
		sourceInfo.CommitID = "commitid"
		sourceInfo.Ref = "ref"
		sourceInfo.Message = "message"
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
			tar:          tar.New(s2ifs.NewFileSystem()),
		}

		// this will validate that the Dockerfile is readable
		// and append some labels to the Dockerfile
		if err = dockerBuilder.addBuildParameters(buildDir, sourceInfo); err != nil {
			t.Errorf("failed to add build parameters: %v", err)
			continue
		}

		// check that our Dockerfile has been modified
		dockerfileData, err := ioutil.ReadFile(absoluteDockerfilePath)
		if err != nil {
			t.Errorf("failed to read dockerfile %s: %v", absoluteDockerfilePath, err)
			continue
		}
		for _, value := range expected {
			if !strings.Contains(string(dockerfileData), value) {
				t.Errorf("Updated Dockerfile content does not contain expected value:\n%s\n\nUpdated content:\n%s\n", value, string(dockerfileData))

			}
		}

		// check that the docker client is called with the right Dockerfile parameter
		if err = dockerBuilder.dockerBuild(buildDir, "", []buildapi.SecretBuildSource{}); err != nil {
			t.Errorf("failed to build: %v", err)
			continue
		}
		os.RemoveAll(buildDir)
	}
}

func TestEmptySource(t *testing.T) {
	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildid",
			Namespace: "default",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "test/test-result:latest",
					},
				},
			},
		},
	}

	client := testclient.Fake{}

	dockerBuilder := &DockerBuilder{
		client: client.Builds(""),
		build:  build,
	}

	if err := dockerBuilder.Build(); err == nil {
		t.Error("Should have received error on docker build")
	} else {
		if !strings.Contains(err.Error(), "must provide a value for at least one of source, binary, images, or dockerfile") {
			t.Errorf("Did not receive correct error: %v", err)
		}
	}
}

// We should not be able to try to pull from scratch
func TestDockerfileFromScratch(t *testing.T) {
	dockerFile := `FROM scratch
USER 1001`

	dockerClient := &FakeDocker{
		buildImageFunc: func(opts docker.BuildImageOptions) error {
			return nil
		},
		pullImageFunc: func(opts docker.PullImageOptions, auth docker.AuthConfiguration) error {
			if opts.Repository == "scratch" && opts.Registry == "" {
				return fmt.Errorf("cannot pull scratch")
			}
			return nil
		},
	}

	build := &buildapi.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "buildid",
			Namespace: "default",
		},
		Spec: buildapi.BuildSpec{
			CommonSpec: buildapi.CommonSpec{
				Source: buildapi.BuildSource{
					ContextDir: "",
					Dockerfile: &dockerFile,
				},
				Strategy: buildapi.BuildStrategy{
					DockerStrategy: &buildapi.DockerBuildStrategy{
						DockerfilePath: "",
						From: &kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "scratch",
						},
					},
				},
				Output: buildapi.BuildOutput{
					To: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "scratch",
					},
				},
			},
		},
	}

	client := testclient.Fake{}

	dockerBuilder := &DockerBuilder{
		client:       client.Builds(""),
		build:        build,
		dockerClient: dockerClient,
		gitClient:    git.NewRepository(),
		tar:          tar.New(s2ifs.NewFileSystem()),
	}

	if err := dockerBuilder.Build(); err != nil {
		if strings.Contains(err.Error(), "cannot pull scratch") {
			t.Errorf("Docker build should not have attempted to pull from scratch")
		} else {
			t.Errorf("Received unexpected error: %v", err)
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
		defer func() {
			if err := os.RemoveAll(buildDir); err != nil {
				t.Fatal(err)
			}
		}()
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
		os.RemoveAll(buildDir)
	}
}
