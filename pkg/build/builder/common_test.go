package builder

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MakeNowJust/heredoc"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"

	buildapiv1 "github.com/openshift/api/build/v1"
	"github.com/openshift/origin/pkg/build/builder/util/dockerfile"
	"github.com/openshift/origin/pkg/git"
)

func TestBuildInfo(t *testing.T) {
	b := &buildapiv1.Build{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "sample-app",
			Namespace: "default",
		},
		Spec: buildapiv1.BuildSpec{
			CommonSpec: buildapiv1.CommonSpec{
				Source: buildapiv1.BuildSource{
					Git: &buildapiv1.GitBuildSource{
						URI: "github.com/openshift/sample-app",
						Ref: "master",
					},
				},
				Strategy: buildapiv1.BuildStrategy{
					SourceStrategy: &buildapiv1.SourceBuildStrategy{
						Env: []corev1.EnvVar{
							{Name: "RAILS_ENV", Value: "production"},
						},
					},
				},
			},
		},
	}
	sourceInfo := &git.SourceInfo{}
	sourceInfo.CommitID = "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee"
	got := buildInfo(b, sourceInfo)
	want := []KeyValue{
		{"OPENSHIFT_BUILD_NAME", "sample-app"},
		{"OPENSHIFT_BUILD_NAMESPACE", "default"},
		{"OPENSHIFT_BUILD_SOURCE", "github.com/openshift/sample-app"},
		{"OPENSHIFT_BUILD_REFERENCE", "master"},
		{"OPENSHIFT_BUILD_COMMIT", "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee"},
		{"RAILS_ENV", "production"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildInfo(%+v) = %+v; want %+v", b, got, want)
	}

	b.Spec.Revision = &buildapiv1.SourceRevision{
		Git: &buildapiv1.GitSourceRevision{
			Commit: "1575a90c569a7cc0eea84fbd3304d9df37c9f5ee",
		},
	}
	got = buildInfo(b, nil)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("buildInfo(%+v) = %+v; want %+v", b, got, want)
	}

}

func TestRandomBuildTag(t *testing.T) {
	tests := []struct {
		namespace, name string
		want            string
	}{
		{"test", "build-1", "temp.builder.openshift.io/test/build-1:f1f85ff5"},
		// For long build namespace + build name, the returned random build tag
		// would be longer than the limit of reference.NameTotalLengthMax (255
		// chars). We do not truncate the repository name because it could create an
		// invalid repository name (e.g., namespace=abc, name=d, repo=abc/d,
		// trucated=abc/ -> invalid), so we simply take a SHA1 hash of the
		// repository name (which is guaranteed to be a valid repository name) and
		// preserve the random tag.
		{
			"namespace" + strings.Repeat(".namespace", 20),
			"name" + strings.Repeat(".name", 20),
			"8a0f9d66cde28a0ebb1e3ee8ef9a484ce687afe0:f1f85ff5",
		},
	}
	for _, tt := range tests {
		rand.Seed(0)
		got := randomBuildTag(tt.namespace, tt.name)
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("randomBuildTag(%q, %q) = %q, want %q", tt.namespace, tt.name, got, tt.want)
		}
	}
}

func TestRandomBuildTagNoDupes(t *testing.T) {
	rand.Seed(0)
	previous := make(map[string]struct{})
	for i := 0; i < 100; i++ {
		tag := randomBuildTag("test", "build-1")
		_, exists := previous[tag]
		if exists {
			t.Errorf("randomBuildTag returned a recently seen tag: %q", tag)
		}
		previous[tag] = struct{}{}
	}
}

func TestContainerName(t *testing.T) {
	rand.Seed(0)
	got := containerName("test-strategy", "my-build", "ns", "hook")
	want := "openshift_test-strategy-build_my-build_ns_hook_f1f85ff5"
	if got != want {
		t.Errorf("got %v, want %v", got, want)
	}
}

func Test_addBuildParameters(t *testing.T) {
	type want struct {
		Err bool
		Out string
	}
	tests := []struct {
		original string
		from     *corev1.ObjectReference
		build    []buildapiv1.ImageSource
		want     want
	}{
		{
			original: `# no FROM instruction`,
			want:     want{},
		},
		{
			original: heredoc.Doc(`
				FROM scratch
				# FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Out: heredoc.Doc(`
				FROM scratch
				RUN echo "hello world"
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch
				FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Out: heredoc.Doc(`
				FROM scratch
				FROM busybox
				RUN echo "hello world"
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				RUN echo "hello world"
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				COPY --from=test /a /b
				COPY --from=nginx /a /c
				COPY --from=nginx:latest /a /c
				RUN echo "hello world"
				`),
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				COPY --from=test /a /b
				COPY --from=nginx /a /c
				COPY --from=nginx:latest /a /c
				RUN echo "hello world"
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY -from=test /a /b
				`),
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				COPY -from=test /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM other
				COPY --from=test /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				FROM other
				COPY --from=test /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test:latest"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				COPY --from=nginx:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			from: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: "from-image:v1",
			},
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"scratch", "test:latest"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM from-image:v1 as test
				COPY --from=nginx:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			from: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: "from-image:v1",
			},
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"scratch", "test:latest", "from-image:v1"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM nginx:latest as test
				COPY --from=nginx:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			from: &corev1.ObjectReference{
				Kind: "ImageStreamTag",
				Name: "from-image:v1",
			},
			want: want{
				Out: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM test
				FROM scratch as test
				COPY --from=test /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test", "scratch"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM nginx:latest
				FROM nginx:latest as test
				COPY --from=test /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM other
				COPY --from=test /a /b
				FROM scratch as test
				COPY --from=test /a /b
				`),
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test", "scratch"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM other
				COPY --from=nginx:latest /a /b
				FROM nginx:latest as test
				COPY --from=test /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM other
				COPY --from=test /a /b
				FROM scratch as test
				COPY --from=test /a /b
				`),
			from: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: "nginx:v1",
			},
			build: []buildapiv1.ImageSource{
				{From: corev1.ObjectReference{Kind: "DockerImage", Name: "nginx:latest"}, As: []string{"test", "scratch"}},
			},
			want: want{
				Out: heredoc.Doc(`
				FROM other
				COPY --from=nginx:latest /a /b
				FROM nginx:v1 as test
				COPY --from=test /a /b
				`),
			},
		},
		{
			original: heredoc.Doc(`
				FROM other
				COPY --from=test /a /b
				FROM scratch as test
				COPY --from=test /a /b
				`),
			from: &corev1.ObjectReference{
				Kind: "DockerImage",
				Name: "nginx:v1",
			},
			want: want{
				Out: heredoc.Doc(`
				FROM other
				COPY --from=test /a /b
				FROM nginx:v1 as test
				COPY --from=test /a /b
				`),
			},
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			f, err := ioutil.TempFile("", "builder-dockertest")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())
			defer f.Close()
			if _, err := f.Write([]byte(test.original)); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if _, err := dockerfile.Parse(strings.NewReader(test.original)); err != nil {
				t.Fatal(err)
			}
			if _, err := dockerfile.Parse(strings.NewReader(test.want.Out)); err != nil {
				t.Fatal(err)
			}
			build := &buildapiv1.Build{}
			build.Spec.Strategy.DockerStrategy = &buildapiv1.DockerBuildStrategy{
				DockerfilePath: filepath.Base(f.Name()),
			}
			if test.from != nil {
				build.Spec.Strategy.DockerStrategy.From = test.from
			}
			build.Spec.Source.Images = test.build
			sourceInfo := &git.SourceInfo{}
			testErr := addBuildParameters(filepath.Dir(f.Name()), build, sourceInfo)
			out, err := ioutil.ReadFile(f.Name())
			if err != nil {
				t.Fatal(err)
			}
			got := want{
				Err: testErr != nil,
				Out: string(out),
			}
			extra := "ENV \"OPENSHIFT_BUILD_NAME\"=\"\" \"OPENSHIFT_BUILD_NAMESPACE\"=\"\"\nLABEL \"io.openshift.build.name\"=\"\" \"io.openshift.build.namespace\"=\"\"\n"
			test.want.Out += extra
			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected: %s", diff.ObjectReflectDiff(test.want, got))
			}
		})
	}
}

func Test_findReferencedImages(t *testing.T) {
	type want struct {
		Images     []string
		Multistage bool
		Err        bool
	}
	tests := []struct {
		original string
		want     want
	}{
		{
			original: `# no FROM instruction`,
			want: want{
				Images: []string{},
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch
				# FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Images: []string{"scratch"},
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch
				FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Images:     []string{"busybox", "scratch"},
				Multistage: true,
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				RUN echo "hello world"
				`),
			want: want{
				Images:     []string{"busybox", "scratch"},
				Multistage: true,
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM busybox
				COPY --from=test /a /b
				COPY --from=nginx /a /c
				COPY --from=nginx:latest /a /c
				RUN echo "hello world"
				`),
			want: want{
				Images:     []string{"busybox", "nginx", "nginx:latest", "scratch"},
				Multistage: true,
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				COPY --from=test:latest /a /b
				`),
			want: want{
				Images: []string{"scratch", "test:latest"},
			},
		},
		{
			original: heredoc.Doc(`
				FROM scratch as test
				FROM other
				COPY --from=test /a /b
				`),
			want: want{
				Images:     []string{"other", "scratch"},
				Multistage: true,
			},
		},
		{
			original: heredoc.Doc(`
				FROM other
				COPY --from=test /a /b
				FROM scratch as test
				COPY --from=test /a /b
				`),
			want: want{
				Images:     []string{"other", "scratch", "test"},
				Multistage: true,
			},
		},
	}
	for i, test := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			f, err := ioutil.TempFile("", "builder-dockertest")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(f.Name())
			defer f.Close()
			if _, err := f.Write([]byte(test.original)); err != nil {
				t.Fatal(err)
			}
			f.Close()
			if _, err := dockerfile.Parse(strings.NewReader(test.original)); err != nil {
				t.Fatal(err)
			}
			images, multistage, err := findReferencedImages(f.Name())
			got := want{
				Images:     images,
				Multistage: multistage,
				Err:        err != nil,
			}
			if !reflect.DeepEqual(test.want, got) {
				t.Errorf("unexpected: %s", diff.ObjectReflectDiff(test.want, got))
			}
		})
	}
}
