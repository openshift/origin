package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestCreateImageImport(t *testing.T) {
	testCases := map[string]struct {
		name               string
		from               string
		stream             *imageapi.ImageStream
		all                bool
		confirm            bool
		insecure           *bool
		err                string
		expectedImages     []imageapi.ImageImportSpec
		expectedRepository *imageapi.RepositoryImportSpec
	}{
		"import from non-existing": {
			name: "nonexisting",
			err:  "pass --confirm to create and import",
		},
		"confirmed import from non-existing": {
			name:    "nonexisting",
			confirm: true,
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "nonexisting"},
				To:   &kapi.LocalObjectReference{Name: "latest"},
			}},
		},
		"confirmed import all from non-existing": {
			name:    "nonexisting",
			all:     true,
			confirm: true,
			expectedRepository: &imageapi.RepositoryImportSpec{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "nonexisting"},
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
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				To:   &kapi.LocalObjectReference{Name: "latest"},
			}},
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
			expectedRepository: &imageapi.RepositoryImportSpec{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
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
			expectedRepository: &imageapi.RepositoryImportSpec{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "totally/different/spec"},
			},
		},
		"import all from .spec.tags": {
			name: "testis",
			all:  true,
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
						"other":  {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
					To:   &kapi.LocalObjectReference{Name: "latest"},
				},
				{
					From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:   &kapi.LocalObjectReference{Name: "other"},
				},
			},
		},
		"import all from .spec.tags with insecure annotation": {
			name: "testis",
			all:  true,
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "testis",
					Namespace:   "other",
					Annotations: map[string]string{imageapi.InsecureRepositoryAnnotation: "true"},
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
						"other":  {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
					To:           &kapi.LocalObjectReference{Name: "latest"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				},
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:           &kapi.LocalObjectReference{Name: "other"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				},
			},
		},
		"import all from .spec.tags with insecure flag": {
			name:     "testis",
			all:      true,
			insecure: newBool(true),
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"}},
						"other":  {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
					To:           &kapi.LocalObjectReference{Name: "latest"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				},
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:           &kapi.LocalObjectReference{Name: "other"},
					ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
				},
			},
		},
		"import all from .spec.tags no DockerImage tags": {
			name: "testis",
			all:  true,
			err:  "does not have tags pointing to external docker images",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "otheris:latest"}},
					},
				},
			},
		},
		"empty image stream": {
			name: "testis",
			err:  "does not have valid docker images",
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
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
				To:   &kapi.LocalObjectReference{Name: "latest"},
			}},
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
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:latest"},
				To:   &kapi.LocalObjectReference{Name: "existing"},
			}},
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
		"import tag from .spec.tags": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
						},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
				To:   &kapi.LocalObjectReference{Name: "mytag"},
			}},
		},
		"import tag from .spec.tags with Kind != DockerImage": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "someimage:mytag"},
						},
					},
				},
			},
			err: "tag \"mytag\" points to existing ImageStreamTag \"someimage:mytag\", it cannot be re-imported",
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
			expectedImages: []imageapi.ImageImportSpec{{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				To:           &kapi.LocalObjectReference{Name: "latest"},
				ImportPolicy: imageapi.TagImportPolicy{Insecure: true},
			}},
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
			expectedImages: []imageapi.ImageImportSpec{{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				To:           &kapi.LocalObjectReference{Name: "latest"},
				ImportPolicy: imageapi.TagImportPolicy{Insecure: false},
			}},
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
		if !listEqual(isi.Spec.Images, test.expectedImages) {
			t.Errorf("%s: unexpected import images, expected %#v, got %#v", name, test.expectedImages, isi.Spec.Images)
		}
		if !kapi.Semantic.DeepEqual(isi.Spec.Repository, test.expectedRepository) {
			t.Errorf("%s: unexpected import repository, expected %#v, got %#v", name, test.expectedRepository, isi.Spec.Repository)
		}
	}
}

func TestWasError(t *testing.T) {
	testCases := map[string]struct {
		isi      *imageapi.ImageStreamImport
		expected bool
	}{
		"no error": {
			isi:      &imageapi.ImageStreamImport{},
			expected: false,
		},
		"error importing images": {
			isi: &imageapi.ImageStreamImport{
				Status: imageapi.ImageStreamImportStatus{
					Images: []imageapi.ImageImportStatus{
						{Status: unversioned.Status{Status: unversioned.StatusFailure}},
					},
				},
			},
			expected: true,
		},
		"error importing repository": {
			isi: &imageapi.ImageStreamImport{
				Status: imageapi.ImageStreamImportStatus{
					Repository: &imageapi.RepositoryImportStatus{
						Status: unversioned.Status{Status: unversioned.StatusFailure},
					},
				},
			},
			expected: true,
		},
	}

	for name, test := range testCases {
		if a, e := wasError(test.isi), test.expected; a != e {
			t.Errorf("%s: expected %v, got %v", name, e, a)
		}
	}
}

func listEqual(actual, expected []imageapi.ImageImportSpec) bool {
	if len(actual) != len(expected) {
		return false
	}

	for _, a := range actual {
		found := false
		for _, e := range expected {
			if kapi.Semantic.DeepEqual(a, e) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func newBool(a bool) *bool {
	r := new(bool)
	*r = a
	return r
}
