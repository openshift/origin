package builder

import (
	"reflect"
	"strings"
	"testing"

	"github.com/docker/docker/builder/parser"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/util/docker/dockerfile"
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
