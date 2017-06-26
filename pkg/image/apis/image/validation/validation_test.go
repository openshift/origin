package validation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/validation/field"
	kapi "k8s.io/kubernetes/pkg/api"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

func TestValidateImageOK(t *testing.T) {
	errs := ValidateImage(&imageapi.Image{
		ObjectMeta:           metav1.ObjectMeta{Name: "foo"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateImageMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		I imageapi.Image
		T field.ErrorType
		F string
	}{
		"missing Name": {
			imageapi.Image{DockerImageReference: "ref"},
			field.ErrorTypeRequired,
			"metadata.name",
		},
		"no slash in Name": {
			imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "foo/bar"}},
			field.ErrorTypeInvalid,
			"metadata.name",
		},
		"no percent in Name": {
			imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "foo%%bar"}},
			field.ErrorTypeInvalid,
			"metadata.name",
		},
		"missing DockerImageReference": {
			imageapi.Image{ObjectMeta: metav1.ObjectMeta{Name: "foo"}},
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
		signature imageapi.ImageSignature
		expected  field.ErrorList
	}{
		{
			name: "valid",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "imgname@valid",
				},
				Type:    "valid",
				Content: []byte("blob"),
			},
			expected: field.ErrorList{},
		},

		{
			name: "valid trusted",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "imgname@trusted",
				},
				Type:    "valid",
				Content: []byte("blob"),
				Conditions: []imageapi.SignatureCondition{
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
					{
						Type:   imageapi.SignatureForImage,
						Status: kapi.ConditionTrue,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{},
		},

		{
			name: "valid untrusted",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{
					Name: "imgname@untrusted",
				},
				Type:    "valid",
				Content: []byte("blob"),
				Conditions: []imageapi.SignatureCondition{
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
					{
						Type:   imageapi.SignatureForImage,
						Status: kapi.ConditionFalse,
					},
					// compare the latest condition
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionFalse,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{},
		},

		{
			name: "invalid name and missing type",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{Name: "notype"},
				Content:    []byte("blob"),
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata").Child("name"), "notype", "name must be of format <imageName>@<signatureName>"),
				field.Required(field.NewPath("type"), ""),
			},
		},

		{
			name: "missing content",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{Name: "img@nocontent"},
				Type:       "invalid",
			},
			expected: field.ErrorList{
				field.Required(field.NewPath("content"), ""),
			},
		},

		{
			name: "missing ForImage condition",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{Name: "img@noforimage"},
				Type:       "invalid",
				Content:    []byte("blob"),
				Conditions: []imageapi.SignatureCondition{
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
			},
			expected: field.ErrorList{field.Invalid(field.NewPath("conditions"),
				[]imageapi.SignatureCondition{
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionTrue,
					},
				},
				fmt.Sprintf("missing %q condition type", imageapi.SignatureForImage))},
		},

		{
			name: "adding labels and anotations",
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{
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
			signature: imageapi.ImageSignature{
				ObjectMeta: metav1.ObjectMeta{Name: "img@metadatafilled"},
				Type:       "invalid",
				Content:    []byte("blob"),
				Conditions: []imageapi.SignatureCondition{
					{
						Type:   imageapi.SignatureTrusted,
						Status: kapi.ConditionUnknown,
					},
					{
						Type:   imageapi.SignatureForImage,
						Status: kapi.ConditionUnknown,
					},
				},
				ImageIdentity: "registry.company.ltd/app/core:v1.2",
				SignedClaims:  map[string]string{"claim": "value"},
				IssuedBy: &imageapi.SignatureIssuer{
					SignatureGenericEntity: imageapi.SignatureGenericEntity{Organization: "org"},
				},
				IssuedTo: &imageapi.SignatureSubject{PublicKeyID: "id"},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("imageIdentity"), "registry.company.ltd/app/core:v1.2", "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("signedClaims"), map[string]string{"claim": "value"}, "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("issuedBy"), &imageapi.SignatureIssuer{
					SignatureGenericEntity: imageapi.SignatureGenericEntity{Organization: "org"},
				}, "must be unset for unknown signature state"),
				field.Invalid(field.NewPath("issuedTo"), &imageapi.SignatureSubject{PublicKeyID: "id"}, "must be unset for unknown signature state"),
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
		I imageapi.ImageStreamMapping
		T field.ErrorType
		F string
	}{
		"missing DockerImageRepository": {
			imageapi.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Tag: imageapi.DefaultImageTag,
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
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
			imageapi.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				Tag: imageapi.DefaultImageTag,
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
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
			imageapi.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "openshift/ruby-19-centos",
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
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
			imageapi.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "openshift/ruby-19-centos",
				Tag: imageapi.DefaultImageTag,
				Image: imageapi.Image{
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			field.ErrorTypeRequired,
			"image.metadata.name",
		},
		"invalid repository pull spec": {
			imageapi.ImageStreamMapping{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "registry/extra/openshift//ruby-19-centos",
				Tag: imageapi.DefaultImageTag,
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
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
		specTags              map[string]imageapi.TagReference
		statusTags            map[string]imageapi.TagEventList
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
				field.Invalid(field.NewPath("metadata", "name"), "foo/bar", `may not contain '/'`),
			},
		},
		"no percent in Name": {
			namespace: "foo",
			name:      "foo%%bar",
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo%%bar", `may not contain '%'`),
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
				field.Invalid(field.NewPath("metadata", "namespace"), "!$", `a DNS-1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`),
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
			statusTags: map[string]imageapi.TagEventList{
				"tag": {
					Items: []imageapi.TagEvent{
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
		"referencePolicy.type must be valid": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]imageapi.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.TagReferencePolicyType("Other")},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("tag").Child("referencePolicy", "type"), imageapi.TagReferencePolicyType("Other"), "valid values are \"Source\", \"Local\""),
			},
		},
		"ImageStreamTags can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]imageapi.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
					ImportPolicy:    imageapi.TagImportPolicy{Scheduled: true},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:latest",
					},
					ImportPolicy:    imageapi.TagImportPolicy{Scheduled: true},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("other").Child("importPolicy", "scheduled"), true, "only tags pointing to Docker repositories may be scheduled for background import"),
			},
		},
		"image IDs can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]imageapi.TagReference{
				"badid": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc@badid",
					},
					ImportPolicy:    imageapi.TagImportPolicy{Scheduled: true},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("badid").Child("from", "name"), "abc@badid", "invalid reference format"),
			},
		},
		"ImageStreamImages can't be scheduled": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]imageapi.TagReference{
				"otherimage": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamImage",
						Name: "other@latest",
					},
					ImportPolicy:    imageapi.TagImportPolicy{Scheduled: true},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "tags").Key("otherimage").Child("importPolicy", "scheduled"), true, "only tags pointing to Docker repositories may be scheduled for background import"),
			},
		},
		"valid": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]imageapi.TagReference{
				"tag": {
					From: &kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "abc",
					},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
				"other": {
					From: &kapi.ObjectReference{
						Kind: "ImageStreamTag",
						Name: "other:latest",
					},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			statusTags: map[string]imageapi.TagEventList{
				"tag": {
					Items: []imageapi.TagEvent{
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
		stream := imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: test.namespace,
				Name:      test.name,
			},
			Spec: imageapi.ImageStreamSpec{
				DockerImageRepository: test.dockerImageRepository,
				Tags: test.specTags,
			},
			Status: imageapi.ImageStreamStatus{
				Tags: test.statusTags,
			},
		}

		errs := ValidateImageStream(&stream)
		if e, a := test.expected, errs; !reflect.DeepEqual(e, a) {
			t.Errorf("%s: unexpected errors: %s", name, diff.ObjectReflectDiff(e, a))
		}
	}
}

func TestValidateISTUpdate(t *testing.T) {
	old := &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
		Tag: &imageapi.TagReference{
			From: &kapi.ObjectReference{Kind: "DockerImage", Name: "some/other:system"},
		},
	}

	errs := ValidateImageStreamTagUpdate(
		&imageapi.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two", "three": "four"}},
		},
		old,
	)
	if len(errs) != 0 {
		t.Errorf("expected success: %v", errs)
	}

	errorCases := map[string]struct {
		A imageapi.ImageStreamTag
		T field.ErrorType
		F string
	}{
		"changedLabel": {
			A: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}, Labels: map[string]string{"a": "b"}},
			},
			T: field.ErrorTypeInvalid,
			F: "metadata",
		},
		"mismatchedAnnotations": {
			A: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &imageapi.TagReference{
					From:            &kapi.ObjectReference{Kind: "DockerImage", Name: "some/other:system"},
					Annotations:     map[string]string{"one": "three"},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			T: field.ErrorTypeInvalid,
			F: "tag.annotations",
		},
		"tagToNameRequired": {
			A: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &imageapi.TagReference{
					From:            &kapi.ObjectReference{Kind: "DockerImage", Name: ""},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
				},
			},
			T: field.ErrorTypeRequired,
			F: "tag.from.name",
		},
		"tagToKindRequired": {
			A: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{Namespace: metav1.NamespaceDefault, Name: "foo:bar", ResourceVersion: "1", Annotations: map[string]string{"one": "two"}},
				Tag: &imageapi.TagReference{
					From:            &kapi.ObjectReference{Kind: "", Name: "foo/bar:biz"},
					ReferencePolicy: imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy},
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

	validMeta := metav1.ObjectMeta{Namespace: "foo", Name: "foo"}
	validSpec := imageapi.ImageStreamImportSpec{Repository: &imageapi.RepositoryImportSpec{From: kapi.ObjectReference{Kind: "DockerImage", Name: "redis"}}}
	repoFn := func(spec string) imageapi.ImageStreamImportSpec {
		return imageapi.ImageStreamImportSpec{Repository: &imageapi.RepositoryImportSpec{From: kapi.ObjectReference{Kind: "DockerImage", Name: spec}}}
	}

	tests := map[string]struct {
		isi      *imageapi.ImageStreamImport
		expected field.ErrorList

		namespace             string
		name                  string
		dockerImageRepository string
		specTags              map[string]imageapi.TagReference
		statusTags            map[string]imageapi.TagEventList
	}{
		"missing name": {
			isi:      &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Namespace: "foo"}, Spec: validSpec},
			expected: field.ErrorList{missingNameErr},
		},
		"no slash in Name": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "foo/bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo/bar", `may not contain '/'`),
			},
		},
		"no percent in Name": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "foo%%bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo%%bar", `may not contain '%'`),
			},
		},
		"other invalid name": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Namespace: "foo", Name: "foo bar"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "name"), "foo bar", `must match "[a-z0-9]+(?:[._-][a-z0-9]+)*"`),
			},
		},
		"missing namespace": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Name: "foo"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Required(field.NewPath("metadata", "namespace"), ""),
			},
		},
		"invalid namespace": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: metav1.ObjectMeta{Namespace: "!$", Name: "foo"}, Spec: validSpec},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("metadata", "namespace"), "!$", `a DNS-1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name',  or '123-abc', regex used for validation is '[a-z0-9]([-a-z0-9]*[a-z0-9])?')`),
			},
		},
		"invalid dockerImageRepository": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a-|///bbb")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a-|///bbb", "invalid reference format"),
			},
		},
		"invalid dockerImageRepository with tag": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a/b:tag")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a/b:tag", "you must specify an image repository, not a tag or ID"),
			},
		},
		"invalid dockerImageRepository with ID": {
			isi: &imageapi.ImageStreamImport{ObjectMeta: validMeta, Spec: repoFn("a/b@sha256:something")},
			expected: field.ErrorList{
				field.Invalid(field.NewPath("spec", "repository", "from", "name"), "a/b@sha256:something", "invalid reference format"),
			},
		},
		"only DockerImage tags can be scheduled": {
			isi: &imageapi.ImageStreamImport{
				ObjectMeta: validMeta, Spec: imageapi.ImageStreamImportSpec{
					Images: []imageapi.ImageImportSpec{
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc",
							},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc@badid",
							},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "abc@sha256:3c87c572822935df60f0f5d3665bd376841a7fcfeb806b5f212de6a00e9a7b25",
							},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "ImageStreamTag",
								Name: "other:latest",
							},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
						},
						{
							From: kapi.ObjectReference{
								Kind: "ImageStreamImage",
								Name: "other@latest",
							},
							ImportPolicy: imageapi.TagImportPolicy{Scheduled: true},
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
			specTags: map[string]imageapi.TagReference{
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
			statusTags: map[string]imageapi.TagEventList{
				"tag": {
					Items: []imageapi.TagEvent{
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
