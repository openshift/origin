package validation

import (
	"reflect"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/image/api"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"
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
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing Name": {
			api.Image{DockerImageReference: "ref"},
			fielderrors.ValidationErrorTypeRequired,
			"metadata.name",
		},
		"no slash in Name": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo/bar"}},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.name",
		},
		"no percent in Name": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo%%bar"}},
			fielderrors.ValidationErrorTypeInvalid,
			"metadata.name",
		},
		"missing DockerImageReference": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			fielderrors.ValidationErrorTypeRequired,
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
			if errs[i].(*fielderrors.ValidationError).Type == v.T && errs[i].(*fielderrors.ValidationError).Field == v.F {
				match = true
				break
			}
		}
		if !match {
			t.Errorf("%s: expected errors to have field %s and type %s: %v", k, v.F, v.T, errs)
		}
	}
}

func TestValidateImageStreamMappingNotOK(t *testing.T) {
	errorCases := map[string]struct {
		I api.ImageStreamMapping
		T fielderrors.ValidationErrorType
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
			fielderrors.ValidationErrorTypeRequired,
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
			fielderrors.ValidationErrorTypeRequired,
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
			fielderrors.ValidationErrorTypeRequired,
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
			fielderrors.ValidationErrorTypeRequired,
			"image.metadata.name",
		},
		"invalid repository pull spec": {
			api.ImageStreamMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "registry/extra/openshift/ruby-19-centos",
				Tag: api.DefaultImageTag,
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			fielderrors.ValidationErrorTypeInvalid,
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
			if errs[i].(*fielderrors.ValidationError).Type == v.T && errs[i].(*fielderrors.ValidationError).Field == v.F {
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

	missingNameErr := fielderrors.NewFieldRequired("metadata.name")
	missingNameErr.Detail = "name or generateName is required"

	tests := map[string]struct {
		namespace             string
		name                  string
		dockerImageRepository string
		specTags              map[string]api.TagReference
		statusTags            map[string]api.TagEventList
		expected              fielderrors.ValidationErrorList
	}{
		"missing name": {
			namespace: "foo",
			name:      "",
			expected:  fielderrors.ValidationErrorList{missingNameErr},
		},
		"no slash in Name": {
			namespace: "foo",
			name:      "foo/bar",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", "foo/bar", `name may not contain "/"`),
			},
		},
		"no percent in Name": {
			namespace: "foo",
			name:      "foo%%bar",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", "foo%%bar", `name may not contain "%"`),
			},
		},
		"other invalid name": {
			namespace: "foo",
			name:      "foo bar",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", "foo bar", `must match "[a-z0-9]+(?:[._-][a-z0-9]+)*"`),
			},
		},
		"missing namespace": {
			namespace: "",
			name:      "foo",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldRequired("metadata.namespace"),
			},
		},
		"invalid namespace": {
			namespace: "!$",
			name:      "foo",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.namespace", "!$", `must be a DNS label (at most 63 characters, matching regex [a-z0-9]([-a-z0-9]*[a-z0-9])?): e.g. "my-name"`),
			},
		},
		"short namespace": {
			namespace: "f",
			name:      "foo",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.namespace", "f", `must be at least 2 characters long`),
			},
		},
		"invalid dockerImageRepository": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a-|///bbb",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("spec.dockerImageRepository", "a-|///bbb", "the docker pull spec \"a-|///bbb\" must be two or three segments separated by slashes"),
			},
		},
		"invalid dockerImageRepository with tag": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a/b:tag",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("spec.dockerImageRepository", "a/b:tag", "the repository name may not contain a tag"),
			},
		},
		"invalid dockerImageRepository with ID": {
			namespace: "namespace",
			name:      "foo",
			dockerImageRepository: "a/b@sha256:something",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("spec.dockerImageRepository", "a/b@sha256:something", "the repository name may not contain an ID"),
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
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldRequired("status.tags[tag].items[0].dockerImageReference"),
				fielderrors.NewFieldRequired("status.tags[tag].items[2].dockerImageReference"),
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
			expected: fielderrors.ValidationErrorList{},
		},
		"all possible characters used": {
			namespace: "abcdefghijklmnopqrstuvwxyz-1234567890",
			name:      "abcdefghijklmnopqrstuvwxyz-1234567890.dot_underscore-dash",
			expected:  fielderrors.ValidationErrorList{},
		},
		"max name and namespace length met": {
			namespace: namespace63Char,
			name:      name191Char,
			expected:  fielderrors.ValidationErrorList{},
		},
		"max name and namespace length exceeded": {
			namespace: namespace63Char,
			name:      name192Char,
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", name192Char, "'namespace/name' cannot be longer than 255 characters"),
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
			t.Errorf("%s: unexpected errors: %s", name, util.ObjectDiff(e, a))
		}
	}
}
