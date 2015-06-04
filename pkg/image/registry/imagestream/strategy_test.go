package imagestream

import (
	"errors"
	"fmt"
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/api"
)

type fakeDefaultRegistry struct {
	registry string
}

func (f *fakeDefaultRegistry) DefaultRegistry() (string, bool) {
	return f.registry, len(f.registry) > 0
}

type fakeSubjectAccessReviewRegistry struct {
	err              error
	allow            bool
	request          *authorizationapi.SubjectAccessReview
	requestNamespace string
}

var _ subjectaccessreview.Registry = &fakeSubjectAccessReviewRegistry{}

func (f *fakeSubjectAccessReviewRegistry) CreateSubjectAccessReview(ctx kapi.Context, subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error) {
	f.request = subjectAccessReview
	f.requestNamespace = kapi.NamespaceValue(ctx)
	return &authorizationapi.SubjectAccessReviewResponse{Allowed: f.allow}, f.err
}

func TestDockerImageRepository(t *testing.T) {
	tests := map[string]struct {
		stream          *api.ImageStream
		expected        string
		defaultRegistry string
	}{
		"DockerImageRepository set on stream": {
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name: "somerepo",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			expected: "a/b",
		},
		"default namespace": {
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name: "somerepo",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/default/somerepo",
		},
		"nondefault namespace": {
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/somens/somerepo",
		},
		"missing default registry": {
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "",
			expected:        "",
		},
	}

	for testName, test := range tests {
		strategy := NewStrategy(&fakeDefaultRegistry{test.defaultRegistry}, &fakeSubjectAccessReviewRegistry{})
		value := strategy.dockerImageRepository(test.stream)
		if e, a := test.expected, value; e != a {
			t.Errorf("%s: expected %q, got %q", testName, e, a)
		}
	}
}

func TestTagVerifier(t *testing.T) {
	tests := map[string]struct {
		oldTags    map[string]api.TagReference
		newTags    map[string]api.TagReference
		sarError   error
		sarAllowed bool
		expectSar  bool
		expected   fielderrors.ValidationErrorList
	}{
		"old nil, no tags": {},
		"old nil, all tags are new": {
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			expectSar:  true,
			sarAllowed: true,
		},
		"nil from": {
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					DockerImageReference: "registry/old/stream:latest",
				},
			},
			expectSar: false,
		},
		"same namespace": {
			newTags: map[string]api.TagReference{
				"other": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "namespace",
						Name:      "otherstream:latest",
					},
				},
			},
		},
		"ref unchanged": {
			oldTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			expectSar: false,
		},
		"invalid from name": {
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "a:b:c",
					},
				},
			},
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("spec.tags[latest].from.name", "a:b:c", "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"),
			},
		},
		"sar error": {
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			expectSar: true,
			sarError:  errors.New("foo"),
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldForbidden("spec.tags[latest].from", "otherns/otherstream"),
			},
		},
		"sar denied": {
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			expectSar:  true,
			sarAllowed: false,
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldForbidden("spec.tags[latest].from", "otherns/otherstream"),
			},
		},
		"ref changed": {
			oldTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:foo",
					},
				},
			},
			newTags: map[string]api.TagReference{
				api.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			expectSar:  true,
			sarAllowed: true,
		},
	}

	for name, test := range tests {
		sar := &fakeSubjectAccessReviewRegistry{
			err:   test.sarError,
			allow: test.sarAllowed,
		}

		old := &api.ImageStream{
			Spec: api.ImageStreamSpec{
				Tags: test.oldTags,
			},
		}

		stream := &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "namespace",
				Name:      "stream",
			},
			Spec: api.ImageStreamSpec{
				Tags: test.newTags,
			},
		}

		tagVerifier := &TagVerifier{sar}
		errs := tagVerifier.Verify(old, stream, "user")

		sarCalled := sar.request != nil
		if e, a := test.expectSar, sarCalled; e != a {
			t.Errorf("%s: expected SAR request=%t, got %t", name, e, a)
		}
		if test.expectSar {
			if e, a := "otherns", sar.requestNamespace; e != a {
				t.Errorf("%s: sar namespace: expected %v, got %v", name, e, a)
			}
			expectedSar := &authorizationapi.SubjectAccessReview{
				Verb:         "get",
				Resource:     "imageStream",
				User:         "user",
				ResourceName: "otherstream",
			}
			if e, a := expectedSar, sar.request; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: unexpected SAR request: %s", name, util.ObjectDiff(e, a))
			}
		}

		if e, a := test.expected, errs; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", name, util.ObjectDiff(e, a))
		}
	}
}

type fakeImageStreamGetter struct {
	stream *api.ImageStream
}

func (f *fakeImageStreamGetter) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	return f.stream, nil
}

func TestTagsChanged(t *testing.T) {
	tests := map[string]struct {
		tags               map[string]api.TagReference
		previous           map[string]api.TagReference
		existingTagHistory map[string]api.TagEventList
		expectedTagHistory map[string]api.TagEventList
		stream             string
		otherStream        *api.ImageStream
	}{
		"no tags, no history": {
			stream:             "registry:5000/ns/stream",
			tags:               make(map[string]api.TagReference),
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: make(map[string]api.TagEventList),
		},
		"single tag update, preserves history": {
			stream:   "registry:5000/ns/stream",
			previous: map[string]api.TagReference{},
			tags:     map[string]api.TagReference{"t1": {DockerImageReference: "registry:5000/ns/stream:t1"}},
			existingTagHistory: map[string]api.TagEventList{
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"empty tag ignored on create": {
			stream:             "registry:5000/ns/stream",
			tags:               map[string]api.TagReference{"t1": {}},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{},
		},
		"tag to missing ignored on create": {
			stream:             "registry:5000/ns/stream",
			tags:               map[string]api.TagReference{"t1": {DockerImageReference: "t2"}},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{},
		},
		"new tags, no history": {
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {DockerImageReference: "registry:5000/ns/stream:t1"},
				"t2": {DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"no-op": {
			stream: "registry:5000/ns/stream",
			previous: map[string]api.TagReference{
				"t1": {DockerImageReference: "v1image1"},
				"t2": {DockerImageReference: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			},
			tags: map[string]api.TagReference{
				"t1": {DockerImageReference: "v1image1"},
				"t2": {DockerImageReference: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			},
			existingTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"new tag copies existing history": {
			stream: "registry:5000/ns/stream",
			previous: map[string]api.TagReference{
				"t1": {DockerImageReference: "t1"},
				"t3": {DockerImageReference: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			},
			tags: map[string]api.TagReference{
				"t1": {DockerImageReference: "registry:5000/ns/stream:v1image1"},
				"t2": {DockerImageReference: "registry:5000/ns/stream:v1image1"},
				"t3": {DockerImageReference: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},
			},
			existingTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t3": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
					},
				}},
				// tag copies existing history
				"t2": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
					},
				}},
				"t3": {Items: []api.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"object reference to image stream tag in same stream": {
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "stream:other",
					},
				},
			},
			existingTagHistory: map[string]api.TagEventList{
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
		},
		"object reference to image stream image in same stream": {
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
			},
			existingTagHistory: map[string]api.TagEventList{
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
		},
		"object reference to image stream tag in different stream": {
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:other",
					},
				},
			},
			existingTagHistory: map[string]api.TagEventList{},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			otherStream: &api.ImageStream{
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"other": {
							Items: []api.TagEvent{
								{
									DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
									Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
								},
							},
						},
					},
				},
			},
		},
	}

	for testName, test := range tests {
		fmt.Println(testName)
		stream := &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: "stream",
			},
			Spec: api.ImageStreamSpec{
				Tags: test.tags,
			},
			Status: api.ImageStreamStatus{
				DockerImageRepository: test.stream,
				Tags: test.existingTagHistory,
			},
		}
		previousStream := &api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name: "stream",
			},
			Spec: api.ImageStreamSpec{
				Tags: test.previous,
			},
			Status: api.ImageStreamStatus{
				DockerImageRepository: test.stream,
				Tags: test.existingTagHistory,
			},
		}
		if test.previous == nil {
			previousStream = nil
		}

		s := &Strategy{}
		s.ImageStreamGetter = &fakeImageStreamGetter{test.otherStream}
		s.tagsChanged(previousStream, stream)

		if !reflect.DeepEqual(test.tags, stream.Spec.Tags) {
			t.Errorf("%s: stream.Tags was unexpectedly updated: %#v", testName, stream.Spec.Tags)
			continue
		}

		for expectedTag, expectedTagHistory := range test.expectedTagHistory {
			updatedTagHistory, ok := stream.Status.Tags[expectedTag]
			if !ok {
				t.Errorf("%s: missing history for tag %q", testName, expectedTag)
				continue
			}
			if e, a := len(expectedTagHistory.Items), len(updatedTagHistory.Items); e != a {
				t.Errorf("%s: tag %q: expected %d in history, got %d: %#v", testName, expectedTag, e, a, updatedTagHistory)
				continue
			}
			for i, expectedTagEvent := range expectedTagHistory.Items {
				if e, a := expectedTagEvent.Image, updatedTagHistory.Items[i].Image; e != a {
					t.Errorf("%s: tag %q: docker image id: expected %q, got %q", testName, expectedTag, e, a)
					continue
				}
				if e, a := expectedTagEvent.DockerImageReference, updatedTagHistory.Items[i].DockerImageReference; e != a {
					t.Errorf("%s: tag %q: docker image reference: expected %q, got %q", testName, expectedTag, e, a)
				}
			}
		}
	}
}

func TestTrackingTagUpdate(t *testing.T) {
	original := api.ImageStream{
		Spec: api.ImageStreamSpec{
			Tags: map[string]api.TagReference{
				"tracking": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "2.0",
					},
				},
			},
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"tracking": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
				"2.0": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
			},
		},
	}

	updated := api.ImageStream{
		Spec: api.ImageStreamSpec{
			Tags: map[string]api.TagReference{
				"tracking": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "2.0",
					},
				},
			},
		},
		Status: api.ImageStreamStatus{
			Tags: map[string]api.TagEventList{
				"tracking": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
				"2.0": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "foo/bar@sha256:5678",
							Image:                "5678",
						},
						{
							DockerImageReference: "foo/bar@sha256:1234",
							Image:                "1234",
						},
					},
				},
			},
		},
	}

	s := &Strategy{}
	errs := s.tagsChanged(&original, &updated)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}

	latestEvent, ok := updated.Status.Tags["tracking"]
	if !ok {
		t.Fatal("unable to find status.tags[tracking]")
	}

	revisions := latestEvent.Items
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %#v", revisions)
	}

	if e, a := "foo/bar@sha256:5678", revisions[0].DockerImageReference; e != a {
		t.Errorf("DockerImageReference: expected %s, got %s", e, a)
	}

	if e, a := "5678", revisions[0].Image; e != a {
		t.Errorf("Image: expected %s, got %s", e, a)
	}
}

func TestTagRefChanged(t *testing.T) {
	tests := map[string]struct {
		old, next api.TagReference
		expected  bool
	}{
		"no ref, no from": {
			old:      api.TagReference{},
			next:     api.TagReference{},
			expected: false,
		},
		"same ref": {
			old:      api.TagReference{DockerImageReference: "foo"},
			next:     api.TagReference{DockerImageReference: "foo"},
			expected: false,
		},
		"different ref": {
			old:      api.TagReference{DockerImageReference: "foo"},
			next:     api.TagReference{DockerImageReference: "bar"},
			expected: true,
		},
		"no kind, no name": {
			old: api.TagReference{},
			next: api.TagReference{
				From: &kapi.ObjectReference{},
			},
			expected: false,
		},
		"old from nil": {
			old: api.TagReference{},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - old implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - old explicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "something",
					Name:      "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - next implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "something",
					Name:      "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			expected: true,
		},
		"different name - old namespace implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:other",
				},
			},
			expected: true,
		},
		"different name - old namespace explicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:other",
				},
			},
			expected: true,
		},
		"different name - next namespace implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:other",
				},
			},
			expected: true,
		},
		"same name - old namespace implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			expected: false,
		},
		"same name - old namespace explicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			expected: false,
		},
		"same name - both namespaces implicit": {
			old: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: api.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			expected: false,
		},
	}

	for name, test := range tests {
		actual := tagRefChanged(test.old, test.next, "streamnamespace")
		if test.expected != actual {
			t.Errorf("%s: expected %t, got %t", name, test.expected, actual)
		}
	}
}
