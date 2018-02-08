package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

func TestCreateImageImport(t *testing.T) {
	testCases := map[string]struct {
		name               string
		from               string
		stream             *imageapi.ImageStream
		all                bool
		confirm            bool
		scheduled          bool
		insecure           *bool
		referencePolicy    string
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
		"import from .spec.dockerImageRepository non-existing tag": {
			name: "testis:nonexisting",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			err: `"nonexisting" does not exist on the image stream`,
		},
		"import all from .spec.dockerImageRepository": {
			name: "testis",
			all:  true,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
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
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "otheris:latest"}},
					},
				},
			},
		},
		"empty image stream": {
			name: "testis",
			err:  "the tag \"latest\" does not exist on the image stream - choose an existing tag to import or use the 'tag' command to create a new tag",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
			},
		},
		"import latest tag": {
			name: "testis:latest",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
		"use tag aliases": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag":  {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"}},
						"other1": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "testis:mytag"}},
						"other2": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "mytag"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{{
				From: kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
				To:   &kapi.LocalObjectReference{Name: "mytag"},
			}},
		},
		"import tag from alias of cross-image-stream": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "otherimage:mytag"},
						},
					},
				},
			},
			err: "tag \"mytag\" points to an imagestreamtag from another ImageStream",
		},
		"import tag from alias of circular reference": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "mytag"},
						},
					},
				},
			},
			err: "tag \"mytag\" on the image stream is a reference to same tag",
		},
		"import tag from non existing alias": {
			name: "testis:mytag",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testis",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "nonexisting"},
						},
					},
				},
			},
			err: "the tag \"mytag\" does not exist on the image stream - choose an existing tag to import or use the 'tag' command to create a new tag",
		},
		"use insecure annotation": {
			name: "testis",
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
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
				ObjectMeta: metav1.ObjectMeta{
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
		"import tag setting referencePolicy": {
			name:            "testis:mytag",
			referencePolicy: localReferencePolicy,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
						},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{{
				From:            kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
				To:              &kapi.LocalObjectReference{Name: "mytag"},
				ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.LocalTagReferencePolicy},
			}},
		},
		"import all from .spec.tags setting referencePolicy": {
			name:            "testis",
			all:             true,
			referencePolicy: localReferencePolicy,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"}},
						"other": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From:            kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
					To:              &kapi.LocalObjectReference{Name: "mytag"},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.LocalTagReferencePolicy},
				},
				{
					From:            kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:              &kapi.LocalObjectReference{Name: "other"},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.LocalTagReferencePolicy},
				},
			},
		},
		"import all from .spec.dockerImageRepository setting referencePolicy": {
			name:            "testis",
			all:             true,
			referencePolicy: localReferencePolicy,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expectedRepository: &imageapi.RepositoryImportSpec{
				From:            kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.LocalTagReferencePolicy},
			},
		},
		"import tag setting scheduled": {
			name:      "testis:mytag",
			scheduled: true,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
						},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
				To:           &kapi.LocalObjectReference{Name: "mytag"},
				ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
			}},
		},
		"import already scheduled tag without setting scheduled": {
			name:      "testis:mytag",
			scheduled: false,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From:         &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
				To:           &kapi.LocalObjectReference{Name: "mytag"},
				ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
			}},
		},
		"import all from .spec.tags setting scheduled": {
			name:      "testis",
			all:       true,
			scheduled: true,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"}},
						"other": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
					To:           &kapi.LocalObjectReference{Name: "mytag"},
					ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
				},
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:           &kapi.LocalObjectReference{Name: "other"},
					ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
				},
			},
		},
		"import all from .spec.tags, some already scheduled, without setting scheduled": {
			name:      "testis",
			all:       true,
			scheduled: false,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							From:         &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
						"other": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"}},
					},
				},
			},
			expectedImages: []imageapi.ImageImportSpec{
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"},
					To:           &kapi.LocalObjectReference{Name: "mytag"},
					ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
				},
				{
					From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:other"},
					To:           &kapi.LocalObjectReference{Name: "other"},
					ImportPolicy: imageapi.TagImportPolicy{Scheduled: false},
				},
			},
		},
		"import all from .spec.dockerImageRepository setting scheduled": {
			name:      "testis",
			all:       true,
			scheduled: true,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "testis", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "repo.com/somens/someimage",
					Tags: make(map[string]imageapi.TagReference),
				},
			},
			expectedRepository: &imageapi.RepositoryImportSpec{
				From:         kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage"},
				ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
			},
		},
	}

	for name, test := range testCases {
		var fake *imagefake.Clientset
		if test.stream == nil {
			fake = imagefake.NewSimpleClientset()
		} else {
			fake = imagefake.NewSimpleClientset(test.stream)
		}
		o := ImportImageOptions{
			Target:          test.name,
			From:            test.from,
			All:             test.all,
			Scheduled:       test.scheduled,
			Insecure:        test.insecure,
			ReferencePolicy: test.referencePolicy,
			Confirm:         test.confirm,
			isClient:        fake.Image().ImageStreams("other"),
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
		if !kapihelper.Semantic.DeepEqual(isi.Spec.Repository, test.expectedRepository) {
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
						{Status: metav1.Status{Status: metav1.StatusFailure}},
					},
				},
			},
			expected: true,
		},
		"error importing repository": {
			isi: &imageapi.ImageStreamImport{
				Status: imageapi.ImageStreamImportStatus{
					Repository: &imageapi.RepositoryImportStatus{
						Status: metav1.Status{Status: metav1.StatusFailure},
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
			if kapihelper.Semantic.DeepEqual(a, e) {
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
