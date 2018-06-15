package image

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestParseImageStreamImageName(t *testing.T) {
	tests := map[string]struct {
		input        string
		expectedRepo string
		expectedId   string
		expectError  bool
	}{
		"empty string": {
			input:       "",
			expectError: true,
		},
		"one part": {
			input:       "a",
			expectError: true,
		},
		"more than 2 parts": {
			input:       "a@b@c",
			expectError: true,
		},
		"empty name part": {
			input:       "@id",
			expectError: true,
		},
		"empty id part": {
			input:       "name@",
			expectError: true,
		},
		"valid input": {
			input:        "repo@id",
			expectedRepo: "repo",
			expectedId:   "id",
			expectError:  false,
		},
	}

	for name, test := range tests {
		repo, id, err := ParseImageStreamImageName(test.input)
		didError := err != nil
		if e, a := test.expectError, didError; e != a {
			t.Errorf("%s: expected error=%t, got=%t: %s", name, e, a, err)
			continue
		}
		if test.expectError {
			continue
		}
		if e, a := test.expectedRepo, repo; e != a {
			t.Errorf("%s: repo: expected %q, got %q", name, e, a)
			continue
		}
		if e, a := test.expectedId, id; e != a {
			t.Errorf("%s: id: expected %q, got %q", name, e, a)
			continue
		}
	}
}

func TestParseImageStreamTagName(t *testing.T) {
	tests := map[string]struct {
		id           string
		expectedName string
		expectedTag  string
		expectError  bool
	}{
		"empty id": {
			id:          "",
			expectError: true,
		},
		"missing semicolon": {
			id:          "hello",
			expectError: true,
		},
		"too many semicolons": {
			id:          "a:b:c",
			expectError: true,
		},
		"empty name": {
			id:          ":tag",
			expectError: true,
		},
		"empty tag": {
			id:          "name",
			expectError: true,
		},
		"happy path": {
			id:           "name:tag",
			expectError:  false,
			expectedName: "name",
			expectedTag:  "tag",
		},
	}

	for description, testCase := range tests {
		name, tag, err := ParseImageStreamTagName(testCase.id)
		gotError := err != nil
		if e, a := testCase.expectError, gotError; e != a {
			t.Fatalf("%s: expected err: %t, got: %t: %s", description, e, a, err)
		}
		if err != nil {
			continue
		}
		if e, a := testCase.expectedName, name; e != a {
			t.Errorf("%s: name: expected %q, got %q", description, e, a)
		}
		if e, a := testCase.expectedTag, tag; e != a {
			t.Errorf("%s: tag: expected %q, got %q", description, e, a)
		}
	}
}

func TestDockerImageReferenceAsRepository(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Namespace: "bar",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "bar/foo",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.AsRepository().String()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}

}

func TestDockerImageReferenceDaemonMinimal(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Namespace: "library",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "library/foo:tag",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
		{
			Registry:  "localhost:5000",
			Namespace: "library",
			Name:      "bar",
			Tag:       "latest",
			Expected:  "localhost:5000/library/bar",
		},
		{
			Registry:  "index.docker.io",
			Namespace: "foo",
			Name:      "bar",
			Tag:       "latest",
			Expected:  "docker.io/foo/bar",
		},
		{
			Registry:  "registry-1.docker.io",
			Namespace: "library",
			Name:      "foo",
			Tag:       "bar",
			Expected:  "docker.io/foo:bar",
		},
		{
			Registry:  "docker.io",
			Namespace: "foo",
			Name:      "library",
			Expected:  "docker.io/foo/library",
		},
		{
			Registry: "registry-1.docker.io",
			Name:     "library",
			Tag:      "foo",
			Expected: "docker.io/library:foo",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.DaemonMinimal().Exact()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestDockerImageReferenceString(t *testing.T) {
	testCases := []struct {
		Registry, Namespace, Name, Tag, ID string
		Expected                           string
	}{
		{
			Name:     "foo",
			Expected: "foo",
		},
		{
			Name:     "foo",
			Tag:      "tag",
			Expected: "foo:tag",
		},
		{
			Name:     "foo",
			ID:       "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Name:     "foo",
			ID:       "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected: "foo:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			Expected:  "bar/foo",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			Tag:       "tag",
			Expected:  "bar/foo:tag",
		},
		{
			Namespace: "bar",
			Name:      "foo",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar/foo/baz",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar/foo/baz:tag",
		},
		{
			Registry:  "bar",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Expected:  "bar:5000/foo/baz",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar:5000/foo/baz:tag",
		},
		{
			Registry:  "bar:5000",
			Namespace: "library",
			Name:      "baz",
			Tag:       "tag",
			Expected:  "bar:5000/library/baz:tag",
		},
		{
			Registry:  "bar:5000",
			Namespace: "foo",
			Name:      "baz",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "bar:5000/foo/baz@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
		{
			Registry:  "docker.io",
			Namespace: "user",
			Name:      "app",
			Expected:  "docker.io/user/app",
		},
		{
			Registry: "index.docker.io",
			Name:     "foo",
			Expected: "index.docker.io/library/foo",
		},
		{
			Registry:  "index.docker.io",
			Namespace: "library",
			Name:      "bar",
			ID:        "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			Expected:  "index.docker.io/library/bar@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
		},
	}

	for i, testCase := range testCases {
		ref := DockerImageReference{
			Registry:  testCase.Registry,
			Namespace: testCase.Namespace,
			Name:      testCase.Name,
			Tag:       testCase.Tag,
			ID:        testCase.ID,
		}
		actual := ref.String()
		if e, a := testCase.Expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestLatestTaggedImage(t *testing.T) {
	tests := []struct {
		tag            string
		tags           map[string]TagEventList
		expected       string
		expectNotFound bool
	}{
		{
			tag:            "foo",
			tags:           map[string]TagEventList{},
			expectNotFound: true,
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expectNotFound: true,
		},
		{
			tag: "",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
				"foo": {
					Items: []TagEvent{
						{DockerImageReference: "foo-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "foo-ref",
		},
	}

	for i, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags

		actual := LatestTaggedImage(stream, test.tag)
		if actual == nil {
			if !test.expectNotFound {
				t.Errorf("%d: unexpected nil result", i)
			}
			continue
		}
		if e, a := test.expected, actual.DockerImageReference; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestResolveLatestTaggedImage(t *testing.T) {
	tests := []struct {
		tag            string
		statusRef      string
		refs           map[string]TagReference
		tags           map[string]TagEventList
		expected       string
		expectNotFound bool
	}{
		{
			tag:            "foo",
			tags:           map[string]TagEventList{},
			expectNotFound: true,
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expectNotFound: true,
		},
		{
			tag: "",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},
		{
			tag: "foo",
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
				"foo": {
					Items: []TagEvent{
						{DockerImageReference: "foo-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "foo-ref",
		},

		// the default reference policy does nothing
		{
			refs: map[string]TagReference{
				"latest": {
					ReferencePolicy: TagReferencePolicy{Type: SourceTagReferencePolicy},
				},
			},
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy does nothing unless reference is set
		{
			refs: map[string]TagReference{
				"latest": {
					ReferencePolicy: TagReferencePolicy{Type: LocalTagReferencePolicy},
				},
			},
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy does nothing unless the image id is set
		{
			statusRef: "test.server/a/b",
			refs: map[string]TagReference{
				"latest": {
					ReferencePolicy: TagReferencePolicy{Type: LocalTagReferencePolicy},
				},
			},
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "latest-ref",
		},

		// the local reference policy uses the output status reference and the image id
		// and returns a pullthrough spec
		{
			statusRef: "test.server/a/b",
			refs: map[string]TagReference{
				"latest": {
					ReferencePolicy: TagReferencePolicy{Type: LocalTagReferencePolicy},
				},
			},
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{DockerImageReference: "latest-ref", Image: "sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246"},
						{DockerImageReference: "older"},
					},
				},
			},
			expected: "test.server/a/b@sha256:4ab15c48b859c2920dd5224f92aabcd39a52794c5b3cf088fb3bbb438756c246",
		},
	}

	for i, test := range tests {
		stream := &ImageStream{}
		stream.Status.DockerImageRepository = test.statusRef
		stream.Status.Tags = test.tags
		stream.Spec.Tags = test.refs

		actual, ok := ResolveLatestTaggedImage(stream, test.tag)
		if !ok {
			if !test.expectNotFound {
				t.Errorf("%d: unexpected nil result", i)
			}
			continue
		}
		if e, a := test.expected, actual; e != a {
			t.Errorf("%d: expected %q, got %q", i, e, a)
		}
	}
}

func TestAddTagEventToImageStream(t *testing.T) {
	tests := map[string]struct {
		tags           map[string]TagEventList
		nextRef        string
		nextImage      string
		expectedTags   map[string]TagEventList
		expectedUpdate bool
	}{
		"nil entry for tag": {
			tags:      map[string]TagEventList{},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"empty items for tag": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{},
				},
			},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"same ref and image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "ref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: false,
		},
		"same ref, different image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "ref",
			nextImage: "newimage",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "newimage",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"different ref, same image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "newref",
			nextImage: "image",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "newref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
		"different ref, different image": {
			tags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			nextRef:   "newref",
			nextImage: "newimage",
			expectedTags: map[string]TagEventList{
				"latest": {
					Items: []TagEvent{
						{
							DockerImageReference: "newref",
							Image:                "newimage",
						},
						{
							DockerImageReference: "ref",
							Image:                "image",
						},
					},
				},
			},
			expectedUpdate: true,
		},
	}

	for name, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags
		updated := AddTagEventToImageStream(stream, "latest", TagEvent{DockerImageReference: test.nextRef, Image: test.nextImage})
		if e, a := test.expectedUpdate, updated; e != a {
			t.Errorf("%s: expected updated=%t, got %t", name, e, a)
		}
		if e, a := test.expectedTags, stream.Status.Tags; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: expected\ntags=%#v\ngot=%#v", name, e, a)
		}
	}
}

func TestUpdateTrackingTags(t *testing.T) {
	tests := map[string]struct {
		fromNil               bool
		fromKind              string
		fromNamespace         string
		fromName              string
		trackingTags          []string
		nonTrackingTags       []string
		statusTags            []string
		updatedImageReference string
		updatedImage          string
		expectedUpdates       []string
	}{
		"nil from": {
			fromNil: true,
		},
		"from kind not ImageStreamTag": {
			fromKind: "ImageStreamImage",
		},
		"from namespace different": {
			fromNamespace: "other",
		},
		"from name different": {
			trackingTags: []string{"otherstream:2.0"},
		},
		"no tracking": {
			trackingTags: []string{},
			statusTags:   []string{"2.0", "3.0"},
		},
		"stream name in from name": {
			trackingTags:    []string{"latest"},
			fromName:        "ruby:2.0",
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest"},
		},
		"1 tracking, 1 not": {
			trackingTags:    []string{"latest"},
			nonTrackingTags: []string{"other"},
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest"},
		},
		"multiple tracking, multiple not": {
			trackingTags:    []string{"latest1", "latest2"},
			nonTrackingTags: []string{"other1", "other2"},
			statusTags:      []string{"2.0", "3.0"},
			expectedUpdates: []string{"latest1", "latest2"},
		},
		"no change to tracked tag": {
			trackingTags:          []string{"latest"},
			statusTags:            []string{"2.0", "3.0"},
			updatedImageReference: "ns/ruby@id",
			updatedImage:          "id",
		},
	}

	for name, test := range tests {
		stream := &ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "ns",
				Name:      "ruby",
			},
			Spec: ImageStreamSpec{
				Tags: map[string]TagReference{},
			},
			Status: ImageStreamStatus{
				Tags: map[string]TagEventList{},
			},
		}

		if len(test.fromNamespace) > 0 {
			stream.Namespace = test.fromNamespace
		}

		fromName := test.fromName
		if len(fromName) == 0 {
			fromName = "2.0"
		}

		for _, tag := range test.trackingTags {
			stream.Spec.Tags[tag] = TagReference{
				From: &kapi.ObjectReference{
					Kind: "ImageStreamTag",
					Name: fromName,
				},
			}
		}

		for _, tag := range test.nonTrackingTags {
			stream.Spec.Tags[tag] = TagReference{}
		}

		for _, tag := range test.statusTags {
			stream.Status.Tags[tag] = TagEventList{
				Items: []TagEvent{
					{
						DockerImageReference: "ns/ruby@id",
						Image:                "id",
					},
				},
			}
		}

		if test.fromNil {
			stream.Spec.Tags = map[string]TagReference{
				"latest": {},
			}
		}

		if len(test.fromKind) > 0 {
			stream.Spec.Tags = map[string]TagReference{
				"latest": {
					From: &kapi.ObjectReference{
						Kind: test.fromKind,
						Name: "asdf",
					},
				},
			}
		}

		updatedImageReference := test.updatedImageReference
		if len(updatedImageReference) == 0 {
			updatedImageReference = "ns/ruby@newid"
		}

		updatedImage := test.updatedImage
		if len(updatedImage) == 0 {
			updatedImage = "newid"
		}

		newTagEvent := TagEvent{
			DockerImageReference: updatedImageReference,
			Image:                updatedImage,
		}

		UpdateTrackingTags(stream, "2.0", newTagEvent)
		for _, tag := range test.expectedUpdates {
			tagEventList, ok := stream.Status.Tags[tag]
			if !ok {
				t.Errorf("%s: expected update for tag %q", name, tag)
				continue
			}
			if e, a := updatedImageReference, tagEventList.Items[0].DockerImageReference; e != a {
				t.Errorf("%s: dockerImageReference: expected %q, got %q", name, e, a)
			}
			if e, a := updatedImage, tagEventList.Items[0].Image; e != a {
				t.Errorf("%s: image: expected %q, got %q", name, e, a)
			}
		}
	}
}

func TestJoinImageStreamTag(t *testing.T) {
	if e, a := "foo:bar", JoinImageStreamTag("foo", "bar"); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
	if e, a := "foo:"+DefaultImageTag, JoinImageStreamTag("foo", ""); e != a {
		t.Errorf("Unexpected value: %s", a)
	}
}

func TestResolveImageID(t *testing.T) {
	tests := map[string]struct {
		tags     map[string]TagEventList
		imageID  string
		expErr   string
		expEvent TagEvent
	}{
		"single tag, match ID prefix": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
							Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
						},
					},
				},
			},
			imageID: "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:  "",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
				Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			},
		},
		"single tag, match string prefix": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo:mytag",
							Image:                "mytag",
						},
					},
				},
			},
			imageID: "mytag",
			expErr:  "",
			expEvent: TagEvent{
				DockerImageReference: "repo:mytag",
				Image:                "mytag",
			},
		},
		"single tag, ID error": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b2",
							Image:                "sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b2",
						},
					},
				},
			},
			imageID:  "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:   "not found",
			expEvent: TagEvent{},
		},
		"no tag": {
			tags:     map[string]TagEventList{},
			imageID:  "3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
			expErr:   "not found",
			expEvent: TagEvent{},
		},
		"multiple match": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@mytag",
							Image:                "mytag",
						},
						{
							DockerImageReference: "repo@mytag",
							Image:                "mytag2",
						},
					},
				},
			},
			imageID:  "mytag",
			expErr:   "multiple images match the prefix",
			expEvent: TagEvent{},
		},
		"find match out of multiple tags in first position": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000002",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000002",
						},
					},
				},
				"tag2": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000003",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000003",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
						},
					},
				},
			},
			imageID: "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
				Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
			},
		},
		"find match out of multiple tags in last position": {
			tags: map[string]TagEventList{
				"tag1": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000001",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000001",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000002",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000002",
						},
					},
				},
				"tag2": {
					Items: []TagEvent{
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000003",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000003",
						},
						{
							DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
							Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
						},
					},
				},
			},
			imageID: "sha256:0000000000000000000000000000000000000000000000000000000000000004",
			expEvent: TagEvent{
				DockerImageReference: "repo@sha256:0000000000000000000000000000000000000000000000000000000000000004",
				Image:                "sha256:0000000000000000000000000000000000000000000000000000000000000004",
			},
		},
	}

	for name, test := range tests {
		stream := &ImageStream{}
		stream.Status.Tags = test.tags
		event, err := ResolveImageID(stream, test.imageID)
		if len(test.expErr) > 0 {
			if err == nil || !strings.Contains(err.Error(), test.expErr) {
				t.Errorf("%s: unexpected error, expected %v, got %v", name, test.expErr, err)
			}
			continue
		} else if err != nil {
			t.Errorf("%s: unexpected error, got %v", name, err)
			continue
		}
		if test.expEvent.Image != event.Image || test.expEvent.DockerImageReference != event.DockerImageReference {
			t.Errorf("%s: unexpected tag, expected %#v, got %#v", name, test.expEvent, event)
		}
	}
}

func TestDockerImageReferenceEquality(t *testing.T) {
	equalityTests := []struct {
		a, b    DockerImageReference
		isEqual bool
	}{
		{
			a:       DockerImageReference{},
			b:       DockerImageReference{},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Name: "openshift",
			},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Name: "openshift3",
			},
			isEqual: false,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       DefaultImageTag,
			},
			isEqual: true,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       "v1.0",
			},
			isEqual: false,
		},
		{
			a: DockerImageReference{
				Name: "openshift",
			},
			b: DockerImageReference{
				Registry:  DockerDefaultRegistry,
				Namespace: DockerDefaultNamespace,
				Name:      "openshift",
				Tag:       DefaultImageTag,
				ID:        "d0a28ab59a",
			},
			isEqual: false,
		},
	}
	for i, test := range equalityTests {
		if isEqual := test.a.Equal(test.b); isEqual != test.isEqual {
			t.Errorf("test %d: %#v.Equal(%#v) = %t; want %t",
				i, test.a, test.b, isEqual, test.isEqual)
		}
		// commutativeness sanity check
		if x, y := test.a.Equal(test.b), test.b.Equal(test.a); x != y {
			t.Errorf("test %[1]d: %[2]q.Equal(%[3]q) = %[4]t != %[3]q.Equal(%[2]q) = %[5]t",
				i, test.a, test.b, x, y)
		}
	}
}

func TestPrioritizeTags(t *testing.T) {
	tests := []struct {
		tags     []string
		expected []string
	}{
		{
			tags:     []string{"other", "latest", "v5.5", "5.2.3", "5.5", "v5.3.6-bother", "5.3.6-abba", "5.6"},
			expected: []string{"latest", "5.6", "5.5", "v5.5", "v5.3.6-bother", "5.3.6-abba", "5.2.3", "other"},
		},
		{
			tags:     []string{"1.1-beta1", "1.2-rc1", "1.1-rc1", "1.1-beta2", "1.2-beta1", "1.2-alpha1", "1.2-beta4", "latest"},
			expected: []string{"latest", "1.2-rc1", "1.2-beta4", "1.2-beta1", "1.2-alpha1", "1.1-rc1", "1.1-beta2", "1.1-beta1"},
		},
		{
			tags:     []string{"7.1", "v7.1", "7.1.0"},
			expected: []string{"7.1", "v7.1", "7.1.0"},
		},
		{
			tags:     []string{"7.1.0", "v7.1", "7.1"},
			expected: []string{"7.1", "v7.1", "7.1.0"},
		},
	}

	for _, tc := range tests {
		t.Log("sorting", tc.tags)
		PrioritizeTags(tc.tags)
		if !reflect.DeepEqual(tc.tags, tc.expected) {
			t.Errorf("got %v, want %v", tc.tags, tc.expected)
		}
	}
}

func TestTagsChanged(t *testing.T) {
	tests := map[string]struct {
		new     []TagEvent
		old     []TagEvent
		changed bool
		deleted bool
	}{
		"both empty": {
			new:     []TagEvent{},
			old:     []TagEvent{},
			changed: false,
			deleted: false,
		},
		"new image": {
			new:     []TagEvent{{Image: "newimage"}},
			old:     []TagEvent{},
			changed: true,
			deleted: false,
		},
		"image deleted": {
			new:     []TagEvent{},
			old:     []TagEvent{{Image: "oldimage"}},
			changed: true,
			deleted: true,
		},
		"image changed": {
			new:     []TagEvent{{Image: "newimage"}},
			old:     []TagEvent{{Image: "oldImage"}},
			changed: true,
			deleted: false,
		},
	}
	for name, test := range tests {
		changed, deleted := tagsChanged(test.new, test.old)
		if changed != test.changed || deleted != test.deleted {
			t.Errorf("%s: unexpected tagsChanged, expected (%v, %v) got (%v, %v)",
				name, test.changed, test.deleted, changed, deleted)
		}
	}
}

func TestIndexOfImageSignature(t *testing.T) {
	for _, tc := range []struct {
		name          string
		signatures    []ImageSignature
		matchType     string
		matchContent  []byte
		expectedIndex int
	}{
		{
			name:          "empty",
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "not present",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: -1,
		},

		{
			name: "first and only",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("binary"),
			expectedIndex: 0,
		},

		{
			name: "last",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 2,
		},

		{
			name: "many matches",
			signatures: []ImageSignature{
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob2"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    "custom",
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("blob"),
				},
				{
					Type:    ImageSignatureTypeAtomicImageV1,
					Content: []byte("binary"),
				},
			},
			matchType:     ImageSignatureTypeAtomicImageV1,
			matchContent:  []byte("blob"),
			expectedIndex: 1,
		},
	} {

		im := Image{
			Signatures: make([]ImageSignature, len(tc.signatures)),
		}
		for i, signature := range tc.signatures {
			signature.Name = fmt.Sprintf("%s:%s", signature.Type, signature.Content)
			im.Signatures[i] = signature
		}

		matchName := fmt.Sprintf("%s:%s", tc.matchType, tc.matchContent)

		index := IndexOfImageSignatureByName(im.Signatures, matchName)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}

		index = IndexOfImageSignature(im.Signatures, tc.matchType, tc.matchContent)
		if index != tc.expectedIndex {
			t.Errorf("[%s] got unexpected index: %d != %d", tc.name, index, tc.expectedIndex)
		}
	}
}

func mockImageStream(policy TagReferencePolicyType) *ImageStream {
	now := metav1.Now()
	stream := &ImageStream{}
	stream.Status = ImageStreamStatus{}
	stream.Spec = ImageStreamSpec{}
	stream.Spec.Tags = map[string]TagReference{}
	stream.Spec.Tags["latest"] = TagReference{
		ReferencePolicy: TagReferencePolicy{
			Type: policy,
		},
	}
	stream.Status.DockerImageRepository = "registry:5000/test/foo"
	stream.Status.Tags = map[string]TagEventList{}
	stream.Status.Tags["latest"] = TagEventList{Items: []TagEvent{
		{
			Image:                "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fa",
			DockerImageReference: "test/bar@sha256:bar",
			Created:              now,
			Generation:           3,
		},
		{
			Image:                "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb",
			DockerImageReference: "test/foo@sha256:bar",
			Created:              now,
			Generation:           2,
		},
		{
			Image:                "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb",
			DockerImageReference: "test/foo@sha256:oldbar",
			Created:              metav1.Time{Time: now.Add(-5 * time.Second)},
			Generation:           1,
		},
	}}
	return stream
}

func TestLatestImageTagEvent(t *testing.T) {
	tag, event := LatestImageTagEvent(mockImageStream(SourceTagReferencePolicy), "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb")
	if tag != "latest" {
		t.Errorf("expected tag 'latest', got %q", tag)
	}
	if event == nil {
		t.Fatalf("expected event to not be nil")
	}
	if event.Generation != 2 {
		t.Errorf("expected second generation, got %d", event.Generation)
	}
}

func TestDockerImageReferenceForImage(t *testing.T) {
	reference, ok := DockerImageReferenceForImage(mockImageStream(SourceTagReferencePolicy), "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb")
	if !ok {
		t.Fatalf("expected success for source tag policy")
	}
	if reference != "test/foo@sha256:bar" {
		t.Errorf("expected source reference to be 'test/foo@sha256:bar', got %q", reference)
	}

	reference, ok = DockerImageReferenceForImage(mockImageStream(SourceTagReferencePolicy), "c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb")
	if !ok {
		t.Fatalf("expected success for source tag policy")
	}
	if reference != "test/foo@sha256:bar" {
		t.Errorf("expected source reference to be 'test/foo@sha256:bar', got %q", reference)
	}

	reference, ok = DockerImageReferenceForImage(mockImageStream(LocalTagReferencePolicy), "sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb")
	if !ok {
		t.Fatalf("expected success for local reference policy")
	}
	if reference != "registry:5000/test/foo@sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb" {
		t.Errorf("expected local reference to be 'registry:5000/test/foo@sha256:c3d8a3642ebfa6bd1fd50c2b8b90e99d3e29af1eac88637678f982cde90993fb', got %q", reference)
	}

	reference, ok = DockerImageReferenceForImage(mockImageStream(LocalTagReferencePolicy), "sha256:unknown")
	if ok {
		t.Errorf("expected failure for unknown image")
	}
}

func TestValidateRegistryURL(t *testing.T) {
	for _, tc := range []struct {
		input               string
		expectedError       bool
		expectedErrorString string
	}{
		{input: "172.30.30.30:5000"},
		{input: ":5000"},
		{input: "[fd12:3456:789a:1::1]:80/"},
		{input: "[fd12:3456:789a:1::1]:80"},
		{input: "http://172.30.30.30:5000"},
		{input: "http://[fd12:3456:789a:1::1]:5000/"},
		{input: "http://[fd12:3456:789a:1::1]:5000"},
		{input: "http://registry.org:5000"},
		{input: "https://172.30.30.30:5000"},
		{input: "https://:80/"},
		{input: "https://[fd12:3456:789a:1::1]/"},
		{input: "https://[fd12:3456:789a:1::1]"},
		{input: "https://[fd12:3456:789a:1::1]:5000/"},
		{input: "https://[fd12:3456:789a:1::1]:5000"},
		{input: "https://registry.org/"},
		{input: "https://registry.org"},
		{input: "localhost/"},
		{input: "localhost"},
		{input: "localhost:80"},
		{input: "registry.org/"},
		{input: "registry.org"},
		{input: "registry.org:5000"},

		{
			input:               "httpss://registry.org",
			expectedErrorString: "unsupported scheme: httpss",
		},
		{
			input:               "ftp://registry.org",
			expectedErrorString: "unsupported scheme: ftp",
		},
		{
			input:               "http://registry.org://",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:               "http://registry.org/path",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:         "[fd12:3456:789a:1::1",
			expectedError: true,
		},
		{
			input:         "bad url",
			expectedError: true,
		},
		{
			input:               "/registry.org",
			expectedErrorString: errNoRegistryURLPathAllowed.Error(),
		},
		{
			input:               "https:///",
			expectedErrorString: errRegistryURLHostEmpty.Error(),
		},
		{
			input:               "http://registry.org?parm=arg",
			expectedErrorString: errNoRegistryURLQueryAllowed.Error(),
		},
	} {

		err := ValidateRegistryURL(tc.input)
		if err != nil {
			if len(tc.expectedErrorString) > 0 && err.Error() != tc.expectedErrorString {
				t.Errorf("[%s] unexpected error string: %q != %q", tc.input, err.Error(), tc.expectedErrorString)
			} else if len(tc.expectedErrorString) == 0 && !tc.expectedError {
				t.Errorf("[%s] unexpected error: %q", tc.input, err.Error())
			}
		} else if len(tc.expectedErrorString) > 0 {
			t.Errorf("[%s] got non-error while expecting %q", tc.input, tc.expectedErrorString)
		} else if tc.expectedError {
			t.Errorf("[%s] got unexpected non-error", tc.input)
		}
	}
}

func TestFollowTagReference(t *testing.T) {
	tests := map[string]struct {
		stream      *ImageStream
		tag         string
		expFinalTag string
		expRef      *TagReference
		expMultiple bool
		expErr      error
	}{
		"follow tag reference": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag":   {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "sometag"}},
						"sometag": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "sometag",
			expRef: &TagReference{
				From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"},
			},
			expMultiple: true,
			expErr:      nil,
		},
		"follow tag reference with istag:mytag format": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag":   {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "testis:sometag"}},
						"sometag": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "sometag",
			expRef: &TagReference{
				From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:sometag"},
			},
			expMultiple: true,
			expErr:      nil,
		},
		"no tag reference error": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag":    {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "correcttag"}},
						"wrongtag": {From: &kapi.ObjectReference{Kind: "DockerImage", Name: "repo.com/somens/someimage:mytag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "correcttag",
			expRef:      nil,
			expMultiple: true,
			expErr:      ErrNotFoundReference,
		},
		"crosss image tag reference error": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "another:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: false,
			expErr:      ErrCrossImageStreamReference,
		},
		"crosss namespace tag reference error": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "thisns",
					Name:      "thisis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Namespace: "anotherns", Name: "thisis:sometag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: false,
			expErr:      ErrCrossImageStreamReference,
		},
		"circular tag reference error": {
			stream: &ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "testis",
				},
				Spec: ImageStreamSpec{
					Tags: map[string]TagReference{
						"mytag":   {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "sometag"}},
						"sometag": {From: &kapi.ObjectReference{Kind: "ImageStreamTag", Name: "mytag"}},
					},
				},
			},
			tag:         "mytag",
			expFinalTag: "mytag",
			expRef:      nil,
			expMultiple: true,
			expErr:      ErrCircularReference,
		},
	}

	for name, tc := range tests {
		finalTag, ref, multiple, err := FollowTagReference(tc.stream, tc.tag)
		if !reflect.DeepEqual(finalTag, tc.expFinalTag) {
			t.Errorf("%s: got %v, want %v", name, finalTag, tc.expFinalTag)
		}
		if !reflect.DeepEqual(ref, tc.expRef) {
			t.Errorf("%s: got %#v, want %#v", name, ref, tc.expRef)
		}
		if !reflect.DeepEqual(multiple, tc.expMultiple) {
			t.Errorf("%s: got %v, want %v", name, multiple, tc.expMultiple)
		}
		if err != tc.expErr {
			t.Errorf("%s: got %v, want %v", name, err, tc.expErr)
		}
	}
}
