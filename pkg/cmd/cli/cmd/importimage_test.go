package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestCreateImageImport(t *testing.T) {
	testCases := map[string]struct {
		name     string
		from     string
		stream   *imageapi.ImageStream
		all      bool
		confirm  bool
		insecure *bool
		err      string
		expected *imageapi.ImageStreamImportSpec
	}{
		"import from non-existing": {
			name: "nonexisting",
			err:  "pass --confirm to create and import",
		},
		"confirmed import from non-existing": {
			name:    "nonexisting",
			confirm: true,
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "nonexisting"},
					To:   &kapi.LocalObjectReference{Name: "latest"},
				}},
			},
		},
		"confirmed import all from non-existing": {
			name:    "nonexisting",
			all:     true,
			confirm: true,
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "nonexisting"},
				},
			},
		},
		"import from .spec.dockerImageRepository": {
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
					To:   &kapi.LocalObjectReference{Name: "latest"},
				}},
			},
		},
		"import all from .spec.dockerImageRepository": {
			name: "testis",
			all:  true,
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				},
			},
		},
		"import all from .spec.dockerImageRepository with different from": {
			name: "testis",
			from: "totally_different_spec",
			all:  true,
			err:  "different import spec",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
		},
		"import all from .spec.dockerImageRepository with confirmed different from": {
			name:    "testis",
			from:    "totally/different/spec",
			all:     true,
			confirm: true,
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "totally/different/spec"},
				},
			},
		},
		"import all error for .spec.dockerImageRepository": {
			name: "testis",
			all:  true,
			err:  "all is applicable only to images with spec.dockerImageRepository",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
					},
				},
			},
		},
		"empty image stream": {
			name: "testis",
			err:  "image stream has not defined anything to import",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
			},
		},
		"import latest tag": {
			name: "testis:latest",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
					},
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
					To:   &kapi.LocalObjectReference{Name: "latest"},
				}},
			},
		},
		"import existing tag": {
			name: "testis:existing",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"existing": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
					},
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
					To:   &kapi.LocalObjectReference{Name: "existing"},
				}},
			},
		},
		"import non-existing tag": {
			name: "testis:latest",
			err:  "does not exist on the image stream",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"nonlatest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
					},
				},
			},
		},
		"use insecure annotation": {
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "testis",
					Namespace:   "other",
					Annotations: map[string]string{imageapi.InsecureRepositoryAnnotation: "true"},
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
					To:           &kapi.LocalObjectReference{Name: "latest"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				}},
			},
		},
		"insecure flag overrides insecure annotation": {
			name:     "testis",
			insecure: newBool(false),
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "testis",
					Namespace:   "other",
					Annotations: map[string]string{imageapi.InsecureRepositoryAnnotation: "true"},
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expected: &imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
					To:           &kapi.LocalObjectReference{Name: "latest"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: false},
				}},
			},
		},
	}

	for name, test := range testCases {
		var fake *testclient.Fake
		if test.stream == nil {
			fake = testclient.NewSimpleFake()
		} else {
			fake = testclient.NewSimpleFake(test.stream)
		}
		o := ImportImageOptions{
			Target:   test.name,
			From:     test.from,
			All:      test.all,
			Insecure: test.insecure,
			Confirm:  test.confirm,
			isClient: fake.ImageStreams(""),
		}
		// we need to run Validate, because it sets appropriate Name and Tag
		if err := o.Validate(&cobra.Command{}); err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
		}

		_, isi, err := o.createImageImport()
		// check errors
		if len(test.err) > 0 {
			if err == nil || !strings.Contains(err.Error(), test.err) {
				t.Errorf("%s: unexpected error: expected %s, got %v", name, test.err, err)
			}
			if isi != nil {
				t.Errorf("%s: unexpected import spec: expected nil, got %#v", name, isi)
			}
			continue
		}
		if len(test.err) == 0 && err != nil {
			t.Errorf("%s: unexpected error: %v", name, err)
			continue
		}
		// check values
		if !kapi.Semantic.DeepEqual(isi.Spec, *test.expected) {
			t.Errorf("%s: unexpected import spec, expected %#v, got %#v", name, test.expected, isi.Spec)
		}
	}
}

func newBool(a bool) *bool {
	r := new(bool)
	*r = a
	return r
}
