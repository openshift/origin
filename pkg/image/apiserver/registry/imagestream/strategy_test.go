package imagestream

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	authorizationapi "k8s.io/api/authorization/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/authentication/user"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/api/image"
	oauthorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/validation/fake"
	admfake "github.com/openshift/origin/pkg/image/apiserver/admission/fake"
	"github.com/openshift/origin/pkg/image/apiserver/admission/limitrange"
	"github.com/openshift/origin/pkg/image/apiserver/registryhostname"
	"github.com/openshift/origin/pkg/image/util/testutil"
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
		oauthorizationapi.ScopesKey: {"a", "b"},
	}
}

type fakeDefaultRegistry struct {
	registry string
}

func (f *fakeDefaultRegistry) DefaultRegistry() (string, bool) {
	return f.registry, len(f.registry) > 0
}

type subjectAccessReviewRecord struct {
	err              error
	allow            bool
	request          *authorizationapi.SubjectAccessReview
	requestNamespace string
}

type fakeMultiSubjectAccessReviewRegistry struct {
	records map[identifier]subjectAccessReviewRecord
}

func (f *fakeMultiSubjectAccessReviewRegistry) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReview, error) {
	id := identifier{name: subjectAccessReview.Spec.ResourceAttributes.Name, namespace: subjectAccessReview.Spec.ResourceAttributes.Namespace}
	if record, exists := f.records[id]; exists {
		if record.request != nil {
			return nil, fmt.Errorf("SAR created more than once for %s/%s", id.namespace, id.name)
		}
		record.request = subjectAccessReview
		record.requestNamespace = subjectAccessReview.Spec.ResourceAttributes.Namespace
		f.records[id] = record
		return &authorizationapi.SubjectAccessReview{
			Status: authorizationapi.SubjectAccessReviewStatus{
				Allowed: record.allow,
			},
		}, record.err
	} else {
		return nil, fmt.Errorf("no such SAR recorded for %s/%s", id.namespace, id.name)
	}
}

type fakeSubjectAccessReviewRegistry struct {
	subjectAccessReviewRecord
}

func (f *fakeSubjectAccessReviewRegistry) Create(subjectAccessReview *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReview, error) {
	f.request = subjectAccessReview
	f.requestNamespace = subjectAccessReview.Spec.ResourceAttributes.Namespace
	return &authorizationapi.SubjectAccessReview{
		Status: authorizationapi.SubjectAccessReviewStatus{
			Allowed: f.allow,
		},
	}, f.err
}

func TestPublicDockerImageRepository(t *testing.T) {
	tests := map[string]struct {
		stream         *imageapi.ImageStream
		expected       string
		publicRegistry string
	}{
		"public registry is not set": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somerepo",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			publicRegistry: "",
			expected:       "",
		},
		"public registry is set": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somerepo",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			publicRegistry: "registry-default.external.url",
			expected:       "registry-default.external.url/somerepo",
		},
	}

	for testName, test := range tests {
		strategy := NewStrategy(registryhostname.TestingRegistryHostnameRetriever(nil, test.publicRegistry, ""), &fakeSubjectAccessReviewRegistry{}, &admfake.ImageStreamLimitVerifier{}, nil, nil)
		value := strategy.publicDockerImageRepository(test.stream)
		if e, a := test.expected, value; e != a {
			t.Errorf("%s: expected %q, got %q", testName, e, a)
		}
	}
}

func TestDockerImageRepository(t *testing.T) {
	tests := map[string]struct {
		stream          *imageapi.ImageStream
		expected        string
		defaultRegistry string
	}{
		"DockerImageRepository set on stream": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somerepo",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			expected: "a/b",
		},
		"DockerImageRepository set on stream with default registry": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "foo",
					Name:      "somerepo",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "a/b",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/foo/somerepo",
		},
		"default namespace": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name: "somerepo",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/default/somerepo",
		},
		"nondefault namespace": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "registry:5000",
			expected:        "registry:5000/somens/somerepo",
		},
		"missing default registry": {
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "somerepo",
					Namespace: "somens",
				},
			},
			defaultRegistry: "",
			expected:        "",
		},
	}

	for testName, test := range tests {
		fakeRegistry := &fakeDefaultRegistry{test.defaultRegistry}
		strategy := NewStrategy(registryhostname.TestingRegistryHostnameRetriever(fakeRegistry.DefaultRegistry, "", ""), &fakeSubjectAccessReviewRegistry{}, &admfake.ImageStreamLimitVerifier{}, nil, nil)
		value := strategy.dockerImageRepository(test.stream, true)
		if e, a := test.expected, value; e != a {
			t.Errorf("%s: expected %q, got %q", testName, e, a)
		}
	}
}

type identifier struct {
	name, namespace string
}

func TestTagVerifier(t *testing.T) {
	tests := map[string]struct {
		oldTags  map[string]imageapi.TagReference
		newTags  map[string]imageapi.TagReference
		sars     map[identifier]subjectAccessReviewRecord
		expected field.ErrorList
	}{
		"old nil, no tags": {},
		"old nil, all tags are new": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{{name: "otherstream", namespace: "otherns"}: {allow: true}},
		},
		"nil from": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry/old/stream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{},
		},
		"same namespace": {
			newTags: map[string]imageapi.TagReference{
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
			oldTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{},
		},
		"invalid from name": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
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
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{{name: "otherstream", namespace: "otherns"}: {err: errors.New("foo")}},
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), `otherns/otherstream: "" ""- foo`),
			},
		},
		"sar error propagates to all tags sharing a SAR": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
				"third": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "secondns",
						Name:      "something:latest",
					},
				},
				"fourth": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "secondns",
						Name:      "something:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{
				{name: "otherstream", namespace: "otherns"}: {err: errors.New("foo")},
				{name: "something", namespace: "secondns"}:  {err: errors.New("bar")},
			},
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), `otherns/otherstream: "" ""- foo`),
				field.Forbidden(field.NewPath("spec", "tags").Key("other").Child("from"), `otherns/otherstream: "" ""- foo`),
				field.Forbidden(field.NewPath("spec", "tags").Key("third").Child("from"), `secondns/something: "" ""- bar`),
				field.Forbidden(field.NewPath("spec", "tags").Key("fourth").Child("from"), `secondns/something: "" ""- bar`),
			},
		},
		"sar error propagates only to tags sharing a SAR": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "thirdns",
						Name:      "something:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{
				{name: "otherstream", namespace: "otherns"}: {err: errors.New("foo")},
				{name: "something", namespace: "thirdns"}:   {allow: true},
			},
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), `otherns/otherstream: "" ""- foo`),
			},
		},
		"sar denied": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{{name: "otherstream", namespace: "otherns"}: {allow: false}},
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("spec", "tags").Key("latest").Child("from"), `otherns/otherstream: "" ""`),
			},
		},
		"ref changed": {
			oldTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:foo",
					},
				},
			},
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{{name: "otherstream", namespace: "otherns"}: {allow: true}},
		},
		"multiple sars to multiple namespaces": {
			newTags: map[string]imageapi.TagReference{
				imageapi.DefaultImageTag: {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "otherns",
						Name:      "otherstream:latest",
					},
				},
				"second": {
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "secondns",
						Name:      "otherstream:latest",
					},
				},
			},
			sars: map[identifier]subjectAccessReviewRecord{
				{name: "otherstream", namespace: "otherns"}:  {allow: true},
				{name: "otherstream", namespace: "secondns"}: {allow: true},
			},
		},
	}

	for name, test := range tests {
		sar := &fakeMultiSubjectAccessReviewRegistry{
			records: test.sars,
		}

		old := &imageapi.ImageStream{
			Spec: imageapi.ImageStreamSpec{
				Tags: test.oldTags,
			},
		}

		stream := &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "namespace",
				Name:      "stream",
			},
			Spec: imageapi.ImageStreamSpec{
				Tags: test.newTags,
			},
		}

		tagVerifier := &TagVerifier{sar}
		errs := tagVerifier.Verify(old, stream, &fakeUser{})

		for id, record := range test.sars {
			if e, a := id.namespace, record.requestNamespace; e != a {
				t.Errorf("%s: sar namespace: expected %v, got %v", name, e, a)
			}
			expectedSar := &authorizationapi.SubjectAccessReview{
				Spec: authorizationapi.SubjectAccessReviewSpec{
					ResourceAttributes: &authorizationapi.ResourceAttributes{
						Namespace:   id.namespace,
						Verb:        "get",
						Group:       "image.openshift.io",
						Resource:    "imagestreams",
						Subresource: "layers",
						Name:        id.name,
					},
					User:   "user",
					Groups: []string{"group1"},
					Extra:  map[string]authorizationapi.ExtraValue{oauthorizationapi.ScopesKey: {"a", "b"}},
				},
			}
			if e, a := expectedSar, record.request; !reflect.DeepEqual(e, a) {
				t.Errorf("%s: unexpected SAR request: %s", name, diff.ObjectDiff(e, a))
			}
		}

		sort.Slice(test.expected, func(i, j int) bool {
			return test.expected[i].Field < test.expected[j].Field
		})
		sort.Slice(errs, func(i, j int) bool {
			return errs[i].Field < errs[j].Field
		})
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

		return kapierrors.NewForbidden(image.Resource("ImageStream"), isName, err)
	}

	makeISEvaluator := func(maxImages, maxImageTags int64) func(string, *imageapi.ImageStream) error {
		return func(ns string, is *imageapi.ImageStream) error {
			limit := kapi.ResourceList{
				imageapi.ResourceImageStreamImages: *resource.NewQuantity(maxImages, resource.DecimalSI),
				imageapi.ResourceImageStreamTags:   *resource.NewQuantity(maxImageTags, resource.DecimalSI),
			}
			usage := limitrange.GetImageStreamUsage(is)
			if less, exceeded := kquota.LessThanOrEqual(usage, limit); !less {
				return makeISForbiddenError(is.Name, exceeded)
			}
			return nil
		}
	}

	tests := []struct {
		name        string
		isEvaluator func(string, *imageapi.ImageStream) error
		is          imageapi.ImageStream
		expected    field.ErrorList
	}{
		{
			name: "no limit",
			is: imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
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
			is: imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
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
			is: imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: testutil.MakeDockerImageReference("test", "is", testutil.BaseImageWith1LayerDigest),
									Image:                testutil.BaseImageWith1LayerDigest,
								},
							},
						},
						"oldest": {
							Items: []imageapi.TagEvent{
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
			expected:    field.ErrorList{field.InternalError(field.NewPath(""), makeISForbiddenError("is", []kapi.ResourceName{imageapi.ResourceImageStreamImages}))},
		},

		{
			name: "exceed tags",
			is: imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: testutil.MakeDockerImageReference("test", "is", testutil.ChildImageWith2LayersDigest),
							},
							ReferencePolicy: imageapi.TagReferencePolicy{
								Type: imageapi.SourceTagReferencePolicy,
							},
						},
					},
				},
			},
			isEvaluator: makeISEvaluator(0, 0),
			expected:    field.ErrorList{field.InternalError(field.NewPath(""), makeISForbiddenError("is", []kapi.ResourceName{imageapi.ResourceImageStreamTags}))},
		},

		{
			name: "exceed images and tags",
			is: imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: testutil.MakeDockerImageReference("test", "other", testutil.BaseImageWith1LayerDigest),
							},
							ReferencePolicy: imageapi.TagReferencePolicy{
								Type: imageapi.SourceTagReferencePolicy,
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
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
			expected:    field.ErrorList{field.InternalError(field.NewPath(""), makeISForbiddenError("is", []kapi.ResourceName{imageapi.ResourceImageStreamImages, imageapi.ResourceImageStreamTags}))},
		},
	}

	for _, tc := range tests {
		sar := &fakeSubjectAccessReviewRegistry{}
		sar.allow = true
		tagVerifier := &TagVerifier{sar}
		fakeRegistry := &fakeDefaultRegistry{}
		s := &Strategy{
			tagVerifier: tagVerifier,
			limitVerifier: &admfake.ImageStreamLimitVerifier{
				ImageStreamEvaluator: tc.isEvaluator,
			},
			registryWhitelister:       &fake.RegistryWhitelister{},
			registryHostnameRetriever: registryhostname.TestingRegistryHostnameRetriever(fakeRegistry.DefaultRegistry, "", ""),
		}

		ctx := apirequest.WithUser(apirequest.NewDefaultContext(), &fakeUser{})
		err := s.Validate(ctx, &tc.is)
		if e, a := tc.expected, err; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", tc.name, diff.ObjectReflectDiff(e, a))
		}

		// Update must fail the exact same way
		tc.is.ResourceVersion = "1"
		old := &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Namespace:       "test",
				Name:            "is",
				ResourceVersion: "1",
			},
		}
		err = s.ValidateUpdate(ctx, &tc.is, old)
		if e, a := tc.expected, err; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected validation errors: %s", tc.name, diff.ObjectReflectDiff(e, a))
		}
	}
}

type fakeImageStreamGetter struct {
	stream *imageapi.ImageStream
}

func (f *fakeImageStreamGetter) Get(ctx context.Context, name string, opts *metav1.GetOptions) (runtime.Object, error) {
	return f.stream, nil
}

func TestTagsChanged(t *testing.T) {
	tests := map[string]struct {
		tags               map[string]imageapi.TagReference
		previous           map[string]imageapi.TagReference
		existingTagHistory map[string]imageapi.TagEventList
		expectedTagHistory map[string]imageapi.TagEventList
		stream             string
		otherStream        *imageapi.ImageStream
	}{
		"no tags, no history": {
			stream:             "registry:5000/ns/stream",
			tags:               make(map[string]imageapi.TagReference),
			existingTagHistory: make(map[string]imageapi.TagEventList),
			expectedTagHistory: make(map[string]imageapi.TagEventList),
		},
		"single tag update, preserves history": {
			stream:   "registry:5000/ns/stream",
			previous: map[string]imageapi.TagReference{},
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "registry:5000/ns/stream:t1",
					},
					Reference: true,
				},
			},
			existingTagHistory: map[string]imageapi.TagEventList{
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"empty tag ignored on create": {
			stream:             "registry:5000/ns/stream",
			tags:               map[string]imageapi.TagReference{"t1": {}},
			existingTagHistory: make(map[string]imageapi.TagEventList),
			expectedTagHistory: map[string]imageapi.TagEventList{},
		},
		"tag to missing ignored on create": {
			stream: "registry:5000/ns/stream",
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "t2",
					},
				},
			},
			existingTagHistory: make(map[string]imageapi.TagEventList),
			expectedTagHistory: map[string]imageapi.TagEventList{},
		},
		"new tags, no history": {
			stream: "registry:5000/ns/stream",
			tags: map[string]imageapi.TagReference{
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
			existingTagHistory: make(map[string]imageapi.TagEventList),
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:t1",
						Image:                "",
					},
				}},
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"no-op": {
			stream: "registry:5000/ns/stream",
			previous: map[string]imageapi.TagReference{
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
			tags: map[string]imageapi.TagReference{
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
			existingTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"new tag copies existing history": {
			stream: "registry:5000/ns/stream",
			previous: map[string]imageapi.TagReference{
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
			tags: map[string]imageapi.TagReference{
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
			existingTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
						Image:                "v1image1",
					},
				}},
				"t3": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
					},
				}},
				// tag copies existing history
				"t2": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "registry:5000/ns/stream:v1image1",
					},
				}},
				"t3": {Items: []imageapi.TagEvent{
					{
						DockerImageReference: "@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
					{
						DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				}},
			},
		},
		"object reference to image stream tag in same stream": {
			stream: "registry:5000/ns/stream",
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "stream:other",
					},
				},
			},
			existingTagHistory: map[string]imageapi.TagEventList{
				"other": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"other": {
					Items: []imageapi.TagEvent{
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
			previous: map[string]imageapi.TagReference{
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
			tags: map[string]imageapi.TagReference{
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
			existingTagHistory: map[string]imageapi.TagEventList{
				"other": {
					Items: []imageapi.TagEvent{
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
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
				"t2": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
							Image:                "sha256:293aa25bf219f3e47472281b7e68c09bb6f315c2adf7f86a7302b85bdaa63db3",
						},
					},
				},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"other": {
					Items: []imageapi.TagEvent{
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
					Items: []imageapi.TagEvent{
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
					Items: []imageapi.TagEvent{
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
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
					},
				},
			},
			existingTagHistory: map[string]imageapi.TagEventList{
				"other": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
				"other": {
					Items: []imageapi.TagEvent{
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
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "stream@12345",
					},
				},
			},
			existingTagHistory: map[string]imageapi.TagEventList{
				"other": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream:12345",
							Image:                "12345",
						},
					},
				},
			},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream:12345",
							Image:                "12345",
						},
					},
				},
				"other": {
					Items: []imageapi.TagEvent{
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
			tags: map[string]imageapi.TagReference{
				"t1": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:other",
					},
				},
			},
			existingTagHistory: map[string]imageapi.TagEventList{},
			expectedTagHistory: map[string]imageapi.TagEventList{
				"t1": {
					Items: []imageapi.TagEvent{
						{
							DockerImageReference: "registry:5000/ns/stream@sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
							Image:                "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
						},
					},
				},
			},
			otherStream: &imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"other": {
							Items: []imageapi.TagEvent{
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
		stream := &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stream",
			},
			Spec: imageapi.ImageStreamSpec{
				Tags: test.tags,
			},
			Status: imageapi.ImageStreamStatus{
				DockerImageRepository: test.stream,
				Tags: test.existingTagHistory,
			},
		}
		// we can't reuse the same map twice, it causes both to be modified during updates
		var previousTagHistory = test.existingTagHistory
		if previousTagHistory != nil {
			previousTagHistoryCopy := map[string]imageapi.TagEventList{}
			for k, v := range previousTagHistory {
				previousTagHistory[k] = *v.DeepCopy()
			}
			previousTagHistory = previousTagHistoryCopy
		}
		previousStream := &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Name: "stream",
			},
			Spec: imageapi.ImageStreamSpec{
				Tags: test.previous,
			},
			Status: imageapi.ImageStreamStatus{
				DockerImageRepository: test.stream,
				Tags: previousTagHistory,
			},
		}
		if test.previous == nil {
			previousStream = nil
		}

		fakeRegistry := &fakeDefaultRegistry{}
		s := &Strategy{
			registryHostnameRetriever: registryhostname.TestingRegistryHostnameRetriever(fakeRegistry.DefaultRegistry, "", ""),
			imageStreamGetter:         &fakeImageStreamGetter{test.otherStream},
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
		old, next imageapi.TagReference
		expected  bool
	}{
		"no ref, no from": {
			old:      imageapi.TagReference{},
			next:     imageapi.TagReference{},
			expected: false,
		},
		"same ref": {
			old:      imageapi.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			next:     imageapi.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			expected: false,
		},
		"different ref": {
			old:      imageapi.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "foo"}},
			next:     imageapi.TagReference{From: &kapi.ObjectReference{Kind: "DockerImage", Name: "bar"}},
			expected: true,
		},
		"no kind, no name": {
			old: imageapi.TagReference{},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{},
			},
			expected: false,
		},
		"old from nil": {
			old: imageapi.TagReference{},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - old implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - old explicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "something",
					Name:      "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "another",
					Name:      "other:latest",
				},
			},
			expected: true,
		},
		"different namespace - next implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "something",
					Name:      "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			expected: true,
		},
		"different name - old namespace implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:other",
				},
			},
			expected: true,
		},
		"different name - old namespace explicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:other",
				},
			},
			expected: true,
		},
		"different name - new namespace implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:other",
				},
			},
			expected: true,
		},
		"same name - old namespace implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			expected: false,
		},
		"same name - old namespace explicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			next: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Namespace: "streamnamespace",
					Name:      "other:latest",
				},
			},
			expected: false,
		},
		"same name - both namespaces implicit": {
			old: imageapi.TagReference{
				From: &kapi.ObjectReference{
					Name: "other:latest",
				},
			},
			next: imageapi.TagReference{
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
