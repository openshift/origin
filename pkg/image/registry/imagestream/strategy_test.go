package imagestream

import (
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/auth/user"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/validation/field"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/authorization/registry/subjectaccessreview"
	"github.com/openshift/origin/pkg/image/admission"
	"github.com/openshift/origin/pkg/image/admission/testutil"
	"github.com/openshift/origin/pkg/image/api"
)

type fakeUser struct {
}

var _ user.Info = &fakeUser{}

func (u *fakeUser) GetName() string {
	return "user"
}

func (u *fakeUser) GetUID() string {
	return "uid"
}

func (u *fakeUser) GetGroups() []string {
	return []string{"group1"}
}

func (u *fakeUser) GetExtra() map[string][]string {
	return map[string][]string{
		authorizationapi.ScopesKey: {"a", "b"},
	}
}

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
		"DockerImageRepository set on stream with default registry": {
			stream: &api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "foo",
					Name:      "somerepo",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/foo/somerepo",
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
		strategy := NewStrategy(&fakeDefaultRegistry{test.defaultRegistry}, &fakeSubjectAccessReviewRegistry{}, &testutil.FakeImageStreamLimitVerifier{}, nil)
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
		expected   field.ErrorList
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
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry/old/stream:latest",
					},
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
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("latest").Child("from", "name"), "a:b:c", "must be of the form <tag>, <repo>:<tag>, <id>, or <repo>@<id>"),
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
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), "otherns/otherstream"),
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
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), "otherns/otherstream"),
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
		errs := tagVerifier.Verify(old, stream, &fakeUser{})

		sarCalled := sar.request != nil
		if e, a := test.expectSar, sarCalled; e != a {
			t.Errorf("%s: expected SAR request=%t, got %t", name, e, a)
		}
		if test.expectSar {
			if e, a := "otherns", sar.requestNamespace; e != a {
				t.Errorf("%s: sar namespace: expected %v, got %v", name, e, a)
			}
			expectedSar := &authorizationapi.SubjectAccessReview{
				Action: authorizationapi.Action{
					Verb:         "get",
					Resource:     "imagestreams/layers",
					ResourceName: "otherstream",
				},
				User:   "user",
				Groups: sets.NewString("group1"),
				Scopes: []string{"a", "b"},
			}
			if e, a := expectedSar, sar.request; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: unexpected SAR request: %s", name, diff.ObjectDiff(e, a))
			}
		}

		if e, a := test.expected, errs; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", name, diff.ObjectDiff(e, a))
		}
	}
}

func TestLimitVerifier(t *testing.T) {
	makeISForbiddenError := func(isName string, exceeded []kapi.ResourceName) error {
		if len(exceeded) == 0 {
			return nil
		}

		exceededStrings := []string{}
		for _, r := range exceeded {
			exceededStrings = append(exceededStrings, string(r))
		}
		sort.Strings(exceededStrings)

		err := fmt.Errorf("exceeded %s", strings.Join(exceededStrings, ","))

		return kapierrors.NewForbidden(api.Resource("ImageStream"), isName, err)
	}

	makeISEvaluator := func(maxImages, maxImageTags int64) func(string, *api.ImageStream) error {
		return func(ns string, is *api.ImageStream) error {
			limit := kapi.ResourceList{
				api.ResourceImageStreamImages: *resource.NewQuantity(maxImages, resource.DecimalSI),
				api.ResourceImageStreamTags:   *resource.NewQuantity(maxImageTags, resource.DecimalSI),
			}
			usage := admission.GetImageStreamUsage(is)
			if less, exceeded := kquota.LessThanOrEqual(usage, limit); !less {
				return makeISForbiddenError(is.Name, exceeded)
			}
			return nil
		}
	}

	tests := []struct {
		name        string
		isEvaluator func(string, *api.ImageStream) error
		is          api.ImageStream
		expected    error
	}{
		{
			name: "no limit",
			is: api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "is", testutil.BaseImageWith1LayerDigest),
									Image:                testutil.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
		},

		{
			name: "below limit",
			is: api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "is", testutil.BaseImageWith1LayerDigest),
									Image:                testutil.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			isEvaluator: makeISEvaluator(1, 0),
		},

		{
			name: "exceed images",
			is: api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "is", testutil.BaseImageWith1LayerDigest),
									Image:                testutil.BaseImageWith1LayerDigest,
								},
							},
						},
						"oldest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "is", testutil.BaseImageWith2LayersDigest),
									Image:                testutil.BaseImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			isEvaluator: makeISEvaluator(1, 0),
			expected:    makeISForbiddenError("is", []kapi.ResourceName{api.ResourceImageStreamImages}),
		},

		{
			name: "exceed tags",
			is: api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: testutil.MakeDockerImageReference("test", "is", testutil.ChildImageWith2LayersDigest),
							},
						},
					},
				},
			},
			isEvaluator: makeISEvaluator(0, 0),
			expected:    makeISForbiddenError("is", []kapi.ResourceName{api.ResourceImageStreamTags}),
		},

		{
			name: "exceed images and tags",
			is: api.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: testutil.MakeDockerImageReference("test", "other", testutil.BaseImageWith1LayerDigest),
							},
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"latest": {
							Items: []api.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "other", testutil.BaseImageWith1LayerDigest),
									Image:                testutil.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			isEvaluator: makeISEvaluator(0, 0),
			expected:    makeISForbiddenError("is", []kapi.ResourceName{api.ResourceImageStreamImages, api.ResourceImageStreamTags}),
		},
	}

	for _, tc := range tests {
		sar := &fakeSubjectAccessReviewRegistry{
			allow: true,
		}
		tagVerifier := &TagVerifier{sar}

		s := &Strategy{
			tagVerifier: tagVerifier,
			limitVerifier: &testutil.FakeImageStreamLimitVerifier{
				ImageStreamEvaluator: tc.isEvaluator,
			},
			defaultRegistry: &fakeDefaultRegistry{},
		}

		ctx := kapi.WithUser(kapi.NewDefaultContext(), &fakeUser{})
		err := s.BeforeCreate(ctx, &tc.is)
		if e, a := tc.expected, err; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", tc.name, diff.ObjectReflectDiff(e, a))
		}

		// Update must fail the exact same way
		err = s.BeforeUpdate(ctx, &tc.is, &api.ImageStream{})
		if e, a := tc.expected, err; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", tc.name, diff.ObjectReflectDiff(e, a))
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
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream:t1",
					},
					Reference: true,
				},
			},
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
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "t2",
					},
				},
			},
			existingTagHistory: make(map[string]api.TagEventList),
			expectedTagHistory: map[string]api.TagEventList{},
		},
		"new tags, no history": {
			stream: "registry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream:t1",
					},
					Reference: true,
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
					Reference: true,
				},
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
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "v1image1",
					},
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
			},
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "v1image1",
					},
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
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
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "t1",
					},
				},
				"t3": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
			},
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream:v1image1",
					},
					Reference: true,
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream:v1image1",
					},
					Reference: true,
				},
				"t3": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
					Reference: true,
				},
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
		"tag changes and referenced tag should react": {
			stream: "registry:5000/ns/stream",
			previous: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "stream:other",
					},
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "stream:t1",
					},
				},
			},
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
				"t2": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "stream:t1",
					},
				},
			},
			existingTagHistory: map[string]api.TagEventList{
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
				"t2": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
				"t2": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
			},
		},
		"object reference to image stream image in same stream": {
			stream: "internalregistry:5000/ns/stream",
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
		"object reference to image stream image in same stream (bad digest)": {
			stream: "internalregistry:5000/ns/stream",
			tags: map[string]api.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "stream@12345",
					},
				},
			},
			existingTagHistory: map[string]api.TagEventList{
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream:12345",
							Image:                "12345",
						},
					},
				},
			},
			expectedTagHistory: map[string]api.TagEventList{
				"t1": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream:12345",
							Image:                "12345",
						},
					},
				},
				"other": {
					Items: []api.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream:12345",
							Image:                "12345",
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
		// we can't reuse the same map twice, it causes both to be modified during updates
		var previousTagHistory = test.existingTagHistory
		if previousTagHistory != nil {
			obj, _ := kapi.Scheme.DeepCopy(previousTagHistory)
			previousTagHistory, _ = obj.(map[string]api.TagEventList)
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
				Tags: previousTagHistory,
			},
		}
		if test.previous == nil {
			previousStream = nil
		}

		s := &Strategy{
			defaultRegistry:   &fakeDefaultRegistry{},
			imageStreamGetter: &fakeImageStreamGetter{test.otherStream},
		}
		err := s.tagsChanged(previousStream, stream)
		if len(err) > 0 {
			t.Errorf("%s: unable to process tags: %v", testName, err)
			continue
		}

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
			old:      api.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			next:     api.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			expected: false,
		},
		"different ref": {
			old:      api.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			next:     api.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "bar"}},
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
		"different name - new namespace implicit": {
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
