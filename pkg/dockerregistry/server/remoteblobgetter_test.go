package server

import (
	"reflect"
	"testing"

	_ "github.com/docker/distribution/registry/storage/driver/inmemory"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/diff"

	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestIdentifyCandidateRepositories(t *testing.T) {
	for _, tc := range []struct {
		name                 string
		is                   *imageapi.ImageStream
		localRegistry        string
		primary              bool
		expectedRepositories []string
		expectedSearch       map[string]imagePullthroughSpec
	}{
		{
			name:          "empty image stream",
			is:            &imageapi.ImageStream{},
			localRegistry: "localhost:5000",
		},

		{
			name: "secure image stream with one image",
			is: &imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"tag": {
							Items: []imageapi.TagEvent{{DockerImageReference: "docker.io/busybox"}},
						},
					},
				},
			},
			localRegistry:        "localhost:5000",
			primary:              true,
			expectedRepositories: []string{"docker.io/library/busybox"},
			expectedSearch: map[string]imagePullthroughSpec{
				"docker.io/library/busybox": makeTestImagePullthroughSpec(t, "docker.io/library/busybox:latest", false),
			},
		},

		{
			name: "secure image stream with one insecure image",
			is: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"insecure": {ImportPolicy: imageapi.TagImportPolicy{Insecure: true}},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"secure": {
							Items: []imageapi.TagEvent{
								{DockerImageReference: "example.org/user/app:tag"},
								{DockerImageReference: "secure.example.org/user/app"},
							},
						},
						"insecure": {
							Items: []imageapi.TagEvent{
								{DockerImageReference: "registry.example.org/user/app"},
								{DockerImageReference: "other.org/user/app"},
							},
						},
					},
				},
			},
			localRegistry:        "localhost:5000",
			primary:              true,
			expectedRepositories: []string{"example.org/user/app", "registry.example.org/user/app"},
			expectedSearch: map[string]imagePullthroughSpec{
				"example.org/user/app":          makeTestImagePullthroughSpec(t, "example.org/user/app:tag", false),
				"registry.example.org/user/app": makeTestImagePullthroughSpec(t, "registry.example.org/user/app:latest", true),
			},
		},

		{
			name: "search secondary results in insecure image stream",
			is: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{imageapi.InsecureRepositoryAnnotation: "true"},
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"insecure": {ImportPolicy: imageapi.TagImportPolicy{Insecure: false}},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"secure": {
							Items: []imageapi.TagEvent{
								{DockerImageReference: "example.org/user/app:tag"},
								{DockerImageReference: "example.org/app:tag2"},
							},
						},
						"insecure": {Items: []imageapi.TagEvent{{DockerImageReference: "registry.example.org/user/app"}}},
					},
				},
			},
			localRegistry:        "localhost:5000",
			primary:              false,
			expectedRepositories: []string{"example.org/app"},
			expectedSearch: map[string]imagePullthroughSpec{
				"example.org/app": makeTestImagePullthroughSpec(t, "example.org/app:tag2", true),
			},
		},

		{
			name: "empty secondary search",
			is: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Annotations: map[string]string{imageapi.InsecureRepositoryAnnotation: "true"},
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"insecure": {ImportPolicy: imageapi.TagImportPolicy{Insecure: false}},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"secure":   {Items: []imageapi.TagEvent{{DockerImageReference: "example.org/user/app:tag"}}},
						"insecure": {Items: []imageapi.TagEvent{{DockerImageReference: "registry.example.org/user/app"}}},
					},
				},
			},
			localRegistry: "localhost:5000",
			primary:       false,
		},

		{
			name: "insecure flag propagates to the whole registry",
			is: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"insecure": {ImportPolicy: imageapi.TagImportPolicy{Insecure: true}},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"secure":   {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/c"}}},
						"insecure": {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/app"}}},
						"foo":      {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/c/foo"}}},
						"bar":      {Items: []imageapi.TagEvent{{DockerImageReference: "other.b/bar"}}},
						"gas":      {Items: []imageapi.TagEvent{{DockerImageReference: "a.a/app"}}},
					},
				},
			},
			localRegistry:        "localhost:5000",
			primary:              true,
			expectedRepositories: []string{"a.a/app", "other.b/bar", "a.b/app", "a.b/c", "a.b/c/foo"},
			expectedSearch: map[string]imagePullthroughSpec{
				"a.a/app":     makeTestImagePullthroughSpec(t, "a.a/app:latest", false),
				"other.b/bar": makeTestImagePullthroughSpec(t, "other.b/bar:latest", false),
				"a.b/app":     makeTestImagePullthroughSpec(t, "a.b/app:latest", true),
				"a.b/c":       makeTestImagePullthroughSpec(t, "a.b/c:latest", true),
				"a.b/c/foo":   makeTestImagePullthroughSpec(t, "a.b/c/foo:latest", true),
			},
		},

		{
			name: "duplicate entries",
			is: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"insecure": {ImportPolicy: imageapi.TagImportPolicy{Insecure: true}},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"secure":   {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/foo"}}},
						"insecure": {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/app:latest"}}},
						"foo":      {Items: []imageapi.TagEvent{{DockerImageReference: "a.b/app"}}},
						"bar":      {Items: []imageapi.TagEvent{{DockerImageReference: "a.b.c/app"}}},
						"gas":      {Items: []imageapi.TagEvent{{DockerImageReference: "a.b.c/app"}}},
					},
				},
			},
			localRegistry:        "localhost:5000",
			primary:              true,
			expectedRepositories: []string{"a.b.c/app", "a.b/app", "a.b/foo"},
			expectedSearch: map[string]imagePullthroughSpec{
				"a.b.c/app": makeTestImagePullthroughSpec(t, "a.b.c/app:latest", false),
				"a.b/app":   makeTestImagePullthroughSpec(t, "a.b/app:latest", true),
				"a.b/foo":   makeTestImagePullthroughSpec(t, "a.b/foo:latest", true),
			},
		},
	} {
		repositories, search := identifyCandidateRepositories(tc.is, tc.localRegistry, tc.primary)

		if !reflect.DeepEqual(repositories, tc.expectedRepositories) {
			if len(repositories) != 0 || len(tc.expectedRepositories) != 0 {
				t.Errorf("[%s] got unexpected repositories: %s", tc.name, diff.ObjectGoPrintDiff(repositories, tc.expectedRepositories))
			}
		}

		for repo, spec := range search {
			if expSpec, exists := tc.expectedSearch[repo]; !exists {
				t.Errorf("[%s] got unexpected repository among results: %q: %#+v", tc.name, repo, spec)
			} else if !reflect.DeepEqual(spec, expSpec) {
				t.Errorf("[%s] got unexpected pull spec for repo %q: %s", tc.name, repo, diff.ObjectGoPrintDiff(spec, expSpec))
			}
		}
		for expRepo, expSpec := range tc.expectedSearch {
			if _, exists := tc.expectedSearch[expRepo]; !exists {
				t.Errorf("[%s] missing expected repository among results: %q: %#+v", tc.name, expRepo, expSpec)
			}
		}
	}
}

func makeTestImagePullthroughSpec(t *testing.T, ref string, insecure bool) imagePullthroughSpec {
	r, err := imageapi.ParseDockerImageReference(ref)
	if err != nil {
		t.Fatal(err)
	}
	return imagePullthroughSpec{dockerImageReference: &r, insecure: insecure}
}
