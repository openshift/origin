package validation

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
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
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldRequired("metadata.name"),
			},
		},
		"no slash in Name": {
			namespace: "foo",
			name:      "foo/bar",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", "foo/bar", `may not contain "/"`),
			},
		},
		"no percent in Name": {
			namespace: "foo",
			name:      "foo%%bar",
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("metadata.name", "foo%%bar", `may not contain "%"`),
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
				fielderrors.NewFieldInvalid("metadata.namespace", "!$", `must have at most 253 characters and match regex [a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*`),
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
		"both dockerImageReference and from set": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"tag": {
					DockerImageReference: "abc",
					From:                 &kapi.ObjectReference{},
				},
			},
			expected: fielderrors.ValidationErrorList{
				fielderrors.NewFieldInvalid("spec.tags[tag]", "", "only 1 of dockerImageReference or from may be set"),
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
				fielderrors.NewFieldRequired("status.tags[tag].Items[0].dockerImageReference"),
				fielderrors.NewFieldRequired("status.tags[tag].Items[2].dockerImageReference"),
			},
		},
		"valid": {
			namespace: "namespace",
			name:      "foo",
			specTags: map[string]api.TagReference{
				"tag": {
					DockerImageReference: "abc",
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
