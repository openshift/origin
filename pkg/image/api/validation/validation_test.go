package validation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util/diff"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/image/api"
)

func TestValidateImageOK(t *testing.T) {
	errs := ValidateImage(&api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateImageMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		I api.Image
		T field.ErrorType
		F string
	}{
		"missing Name": {
			api.Image{DockerImageReference: "ref"},
			field.ErrorTypeRequired,
			"metadata.name",
		},
		"no slash in Name": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo/bar"}},
			field.ErrorTypeInvalid,
			"metadata.name",
		},
		"no percent in Name": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo%%bar"}},
			field.ErrorTypeInvalid,
			"metadata.name",
		},
		"missing DockerImageReference": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			field.ErrorTypeRequired,
			"dockerImageReference",
		},
	}

	for k, v := range errorCases {
		errs := ValidateImage(&v.I)
		if len(errs) == 0 {
			t.Errorf("Expected failure for %s", k)
			continue
		}
		match := false
		for i := range errs {
			if errs[i].Type == v.T && errs[i].Field == v.F {
				match = true
				break
			}
		}
		if !match {
			t.Errorf("%s: expected errors to have field %s and type %s: %v", k, v.F, v.T, errs)
		}
	}
}

func TestValidateImageSignature(t *testing.T) {
	for _, tc := range []struct {
		name      string
		signature api.ImageSignature
		expected  field.ErrorList
	}{
		{
			name: "valid",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{
					Name: "imgname@valid",
				},
				Type:    "valid",
				Content: []byte("blob"),
			},
			expected: field.ErrorList{},
		},

		{
			name: "valid trusted",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{
					Name: "imgname@trusted",
				},
				Type:    "valid",
				Content: []byte("blob"),
				Conditions: []api.SignatureCondition{
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
					{
						Type:   api.SignatureForImage,
						Status: kapi.ConditionTrue,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{},
		},

		{
			name: "valid untrusted",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{
					Name: "imgname@untrusted",
				},
				Type:    "valid",
				Content: []byte("blob"),
				Conditions: []api.SignatureCondition{
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
					{
						Type:   api.SignatureForImage,
						Status: kapi.ConditionFalse,
					},
					// compare the latest condition
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionFalse,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{},
		},

		{
			name: "invalid name and missing type",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{Name: "notype"},
				Content:    []byte("blob"),
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata").Child("name"), "notype", "name must be of format <imageName>@<signatureName>"),
				field.Required(field.NewPath("type"), ""),
			},
		},

		{
			name: "missing content",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{Name: "img@nocontent"},
				Type:       "invalid",
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("content"), ""),
			},
		},

		{
			name: "missing ForImage condition",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{Name: "img@noforimage"},
				Type:       "invalid",
				Content:    []byte("blob"),
				Conditions: []api.SignatureCondition{
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{field.Invalid(field.NewPath("conditions"),
				[]api.SignatureCondition{
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
				},
				fmt.Sprintf("missing %q condition type", api.SignatureForImage))},
		},

		{
			name: "adding labels and anotations",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{
					Name:        "img@annotated",
					Annotations: map[string]string{"key": "value"},
					Labels:      map[string]string{"label": "value"},
				},
				Type:    "valid",
				Content: []byte("blob"),
			},
			expected: field.ErrorList{
				field.Forbidden(field.NewPath("metadata").Child("labels"), "signature labels cannot be set"),
				field.Forbidden(field.NewPath("metadata").Child("annotations"), "signature annotations cannot be set"),
			},
		},

		{
			name: "filled metadata for unknown signature state",
			signature: api.ImageSignature{
				ObjectMeta: kapi.ObjectMeta{Name: "img@metadatafilled"},
				Type:       "invalid",
				Content:    []byte("blob"),
				Conditions: []api.SignatureCondition{
					{
						Type:   api.SignatureTrusted,
						Status: kapi.ConditionUnknown,
					},
					{
						Type:   api.SignatureForImage,
						Status: kapi.ConditionUnknown,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
				SignedClaims:  map[string]string{"claim": "value"},
				IssuedBy: &api.SignatureIssuer{
					SignatureGenericEntity: api.SignatureGenericEntity{Organization: "org"},
				},
				IssuedTo: &api.SignatureSubject{PublicKeyID: "id"},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("imageIdentity"), "registry.company.ltd/app/core:v1.2", "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("signedClaims"), map[string]string{"claim": "value"}, "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("issuedBy"), &api.SignatureIssuer{
					SignatureGenericEntity: api.SignatureGenericEntity{Organization: "org"},
				}, "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("issuedTo"), &api.SignatureSubject{PublicKeyID: "id"}, "must be unset for unknown signature state"),
			},
		},
	} {
		errs := validateImageSignature(&tc.signature, nil)
		if e, a := tc.expected, errs; !reflect.DeepEqual(a, e) {
			t.Errorf("[%s] unexpected errors: %s", tc.name, diff.ObjectDiff(e, a))
		}
	}

}

func TestValidateImageStreamMappingNotOK(t *testing.T) {
	errorCases := map[string]struct {
		I api.ImageStreamMapping
		T field.ErrorType
		F string
	}{
		"missing DockerImageRepository": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				Tag: api.DefaultImageTag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeRequired,
			"dockerImageRepository",
		},
		"missing Name": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				Tag: api.DefaultImageTag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeRequired,
			"name",
		},
		"missing Tag": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "openshift/ruby-19-centos",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeRequired,
			"tag",
		},
		"missing image name": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "openshift/ruby-19-centos",
				Tag: api.DefaultImageTag,
				Image: api.Image{
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeRequired,
			"image.metadata.name",
		},
		"invalid repository pull spec": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "registry/extra/openshift//ruby-19-centos",
				Tag: api.DefaultImageTag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeInvalid,
			"dockerImageRepository",
		},
	}

	for k, v := range errorCases {
		errs := ValidateImageStreamMapping(&v.I)
		if len(errs) == 0 {
			t.Errorf("Expected failure for %s", k)
			continue
		}
		match := false
		for i := range errs {
			if errs[i].Type == v.T && errs[i].Field == v.F {
				match = true
				break
			}
		}
		if !match {
			t.Errorf("%s: expected errors to have field %s and type %s: %v", k, v.F, v.T, errs)
		}
	}
}

func TestValidateImageStream(t *testing.T) {

	namespace63Char := strings.Repeat("a", 63)
	name191Char := strings.Repeat("b", 191)
	name192Char := "x" + name191Char

	missingNameErr := field.Required(field.NewPath("metadata", "name"), "")
	missingNameErr.Detail = "name or generateName is required"

	tests := map[string]struct {
		namespace             string
		name                  string
		dockerImageRepository string
		specTags              map[string]api.TagReference
		statusTags            map[string]api.TagEventList
		expected              field.ErrorList
	}{
		"missing name": {
			namespace: "foo",
			name:      "",
			expected:  field.ErrorList{missingNameErr},
		},
		"no slash in Name": {
			namespace: "foo",
			name:      "foo/bar",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo/bar", `name may not contain "/"`),
			},
		},
		"no percent in Name": {
			namespace: "foo",
			name:      "foo%%bar",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo%%bar", `name may not contain "%"`),
			},
		},
		"other invalid name": {
			namespace: "foo",
			name:      "foo bar",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo bar", `must match "[a-z0-9]+(?:[._-][a-z0-9]+)*"`),
			},
		},
		"missing namespace": {
			namespace: "",
			name:      "foo",
			expected: field.ErrorList{
				field.Required(field.NewPath("metadata", "namespace"), ""),
			},
		},
		"invalid namespace": {
			namespace: "!$",
			name:      "foo",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "namespace"), "!$", `must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])? (e.g. 'my-name' or '123-abc')`),
			},
		},
		"invalid dockerImageRepository": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a-|///bbb",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "dockerImageRepository"), "a-|///bbb", "invalid reference format"),
			},
		},
		"invalid dockerImageRepository with tag": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a/b:tag",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "dockerImageRepository"), "a/b:tag", "the repository name may not contain a tag"),
			},
		},
		"invalid dockerImageRepository with ID": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a/b@sha256:something",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "dockerImageRepository"), "a/b@sha256:something", "invalid reference format"),
			},
		},
		"status tag missing dockerImageReference": {
			namespace: "namespace",
			name:      "foo",
			statusTags: map[string]api.TagEventList{
				"tag": {
					Items: []api.TagEvent{
						{DockerImageReference: ""},
						{DockerImageReference: "foo/bar:latest"},
						{DockerImageReference: ""},
					},
				},
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("status", "tags").Key("tag").Child("items").Index(0).Child("dockerImageReference"), ""),
				field.Required(field.NewPath("status", "tags").Key("tag").Child("items").Index(2).Child("dockerImageReference"), ""),
			},
		},
		"ImageStreamTags can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
					ImportPolicy: api.TagImportPolicy{Scheduled: true},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:latest",
					},
					ImportPolicy: api.TagImportPolicy{Scheduled: true},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("other").Child("importPolicy", "scheduled"), true, "only tags pointing to Docker repositories may be scheduled for background import"),
			},
		},
		"image IDs can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"badid": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc@badid",
					},
					ImportPolicy: api.TagImportPolicy{Scheduled: true},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("badid").Child("from", "name"), "abc@badid", "invalid reference format"),
			},
		},
		"ImageStreamImages can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"otherimage": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "other@latest",
					},
					ImportPolicy: api.TagImportPolicy{Scheduled: true},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("otherimage").Child("importPolicy", "scheduled"), true, "only tags pointing to Docker repositories may be scheduled for background import"),
			},
		},
		"valid": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:latest",
					},
				},
			},
			statusTags: map[string]api.TagEventList{
				"tag": {
					Items: []api.TagEvent{
						{DockerImageReference: "foo/bar:latest"},
					},
				},
			},
			expected: field.ErrorList{},
		},
		"shortest name components": {
			namespace: "f",
			name:      "g",
			expected:  field.ErrorList{},
		},
		"all possible characters used": {
			namespace: "abcdefghijklmnopqrstuvwxyz-1234567890",
			name:      "abcdefghijklmnopqrstuvwxyz-1234567890.dot_underscore-dash",
			expected:  field.ErrorList{},
		},
		"max name and namespace length met": {
			namespace: namespace63Char,
			name:      name191Char,
			expected:  field.ErrorList{},
		},
		"max name and namespace length exceeded": {
			namespace: namespace63Char,
			name:      name192Char,
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), name192Char, "'namespace/name' cannot be longer than 255 characters"),
			},
		},
	}

	for name, test := range tests {
		stream := api.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: test.namespace,
				Name:      test.name,
			},
			Spec: api.ImageStreamSpec{
				DockerImageRepository: test.dockerImageRepository,
				Tags: test.specTags,
			},
			Status: api.ImageStreamStatus{
				Tags: test.statusTags,
			},
		}

		errs := ValidateImageStream(&stream)
		if e, a := test.expected, errs; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected errors: %s", name, diff.ObjectDiff(e, a))
		}
	}
}

func TestValidateISTUpdate(t *testing.T) {
	old := &api.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
		Tag: &api.TagReference{
			From: &kapi.ObjectReference{Kind: "DockerImage", Name: "some/other:system"},
		},
	}

	errs := ValidateImageStreamTagUpdate(
		&api.ImageStreamTag{
			ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two", "three": "four"}},
		},
		old,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A api.ImageStreamTag
		T field.ErrorType
		F string
	}{
		"changedLabel": {
			A: api.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}, Labels: map[string]string{"a": "b"}},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata",
		},
		"mismatchedAnnotations": {
			A: api.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &api.TagReference{
					From:        &kapi.ObjectReference{Kind: "DockerImage", Name: "some/other:system"},
					Annotations: map[string]string{"one": "three"},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "tag.annotations",
		},
		"tagToNameRequired": {
			A: api.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &api.TagReference{
					From: &kapi.ObjectReference{Kind: "DockerImage", Name: ""},
				},
			},
			T: field.ErrorTypeRequired,
			F: "tag.from.name",
		},
		"tagToKindRequired": {
			A: api.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{Namespace: kapi.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &api.TagReference{
					From: &kapi.ObjectReference{Kind: "", Name: "foo/bar:biz"},
				},
			},
			T: field.ErrorTypeRequired,
			F: "tag.from.kind",
		},
	}
	for k, v := range errorCases {
		errs := ValidateImageStreamTagUpdate(&v.A, old)
		if len(errs) == 0 {
			t.Errorf("expected failure %s for %v", k, v.A)
			continue
		}
		for i := range errs {
			if errs[i].Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}

func TestValidateImageStreamImport(t *testing.T) {
	namespace63Char := strings.Repeat("a", 63)
	name191Char := strings.Repeat("b", 191)
	name192Char := "x" + name191Char

	missingNameErr := field.Required(field.NewPath("metadata", "name"), "")
	missingNameErr.Detail = "name or generateName is required"

	validMeta := kapi.ObjectMeta{Namespace: "foo", Name: "foo"}
	validSpec := api.ImageStreamImportSpec{Repository: &api.RepositoryImportSpec{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis"}}}
	repoFn := func(spec string) api.ImageStreamImportSpec {
		return api.ImageStreamImportSpec{Repository: &api.RepositoryImportSpec{From: kapi.ObjectReference{Kind: "DockerImage", Name: spec}}}
	}

	tests := map[string]struct {
		isi      *api.ImageStreamImport
		expected field.ErrorList

		namespace             string
		name                  string
		dockerImageRepository string
		specTags              map[string]api.TagReference
		statusTags            map[string]api.TagEventList
	}{
		"missing name": {
			isi:      &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Namespace: "foo"}, Spec: validSpec},
			expected: field.ErrorList{missingNameErr},
		},
		"no slash in Name": {
			isi: &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "foo/bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo/bar", `name may not contain "/"`),
			},
		},
		"no percent in Name": {
			isi: &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "foo%%bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo%%bar", `name may not contain "%"`),
			},
		},
		"other invalid name": {
			isi: &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Namespace: "foo", Name: "foo bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo bar", `must match "[a-z0-9]+(?:[._-][a-z0-9]+)*"`),
			},
		},
		"missing namespace": {
			isi: &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Name: "foo"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Required(field.NewPath("metadata", "namespace"), ""),
			},
		},
		"invalid namespace": {
			isi: &api.ImageStreamImport{ObjectMeta: kapi.ObjectMeta{Namespace: "!$", Name: "foo"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "namespace"), "!$", `must match the regex [a-z0-9]([-a-z0-9]*[a-z0-9])? (e.g. 'my-name' or '123-abc')`),
			},
		},
		"invalid dockerImageRepository": {
			isi: &api.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a-|///bbb")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a-|///bbb", "invalid reference format"),
			},
		},
		"invalid dockerImageRepository with tag": {
			isi: &api.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a/b:tag")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a/b:tag", "you must specify an image repository, not a tag or ID"),
			},
		},
		"invalid dockerImageRepository with ID": {
			isi: &api.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a/b@sha256:something")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a/b@sha256:something", "invalid reference format"),
			},
		},
		"only DockerImage tags can be scheduled": {
			isi: &api.ImageStreamImport{
				ObjectMeta: validMeta, Spec: api.ImageStreamImportSpec{
					Images: []api.ImageImportSpec{
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc",
							},
							ImportPolicy: api.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc@badid",
							},
							ImportPolicy: api.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
							},
							ImportPolicy: api.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "other:latest",
							},
							ImportPolicy: api.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "ImageStreamImage",
								Name: "other@latest",
							},
							ImportPolicy: api.TagImportPolicy{Scheduled: true},
						},
					},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "images").Index(1).Child("from", "name"), "abc@badid", "invalid reference format"),
				field.Invalid(field.NewPath("spec", "images").Index(2).Child("from", "name"), "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25", "only tags can be scheduled for import"),
				field.Invalid(field.NewPath("spec", "images").Index(3).Child("from", "kind"), "ImageStreamTag", "only DockerImage is supported"),
				field.Invalid(field.NewPath("spec", "images").Index(4).Child("from", "kind"), "ImageStreamImage", "only DockerImage is supported"),
			},
		},
		"valid": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:latest",
					},
				},
			},
			statusTags: map[string]api.TagEventList{
				"tag": {
					Items: []api.TagEvent{
						{DockerImageReference: "foo/bar:latest"},
					},
				},
			},
			expected: field.ErrorList{},
		},
		"shortest name components": {
			namespace: "f",
			name:      "g",
			expected:  field.ErrorList{},
		},
		"all possible characters used": {
			namespace: "abcdefghijklmnopqrstuvwxyz-1234567890",
			name:      "abcdefghijklmnopqrstuvwxyz-1234567890.dot_underscore-dash",
			expected:  field.ErrorList{},
		},
		"max name and namespace length met": {
			namespace: namespace63Char,
			name:      name191Char,
			expected:  field.ErrorList{},
		},
		"max name and namespace length exceeded": {
			namespace: namespace63Char,
			name:      name192Char,
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), name192Char, "'namespace/name' cannot be longer than 255 characters"),
			},
		},
	}

	for name, test := range tests {
		if test.isi == nil {
			continue
		}
		errs := ValidateImageStreamImport(test.isi)
		if e, a := test.expected, errs; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected errors: %s", name, diff.ObjectDiff(e, a))
		}
	}
}
