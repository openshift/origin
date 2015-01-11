package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/openshift/origin/pkg/image/api"
)

func TestValidateImageOK(t *testing.T) {
	errs := ValidateImage(&api.Image{
		ObjectMeta:           kapi.ObjectMeta{Name: "foo", Namespace: "default"},
		DockerImageReference: "openshift/ruby-19-centos",
	})
	if len(errs) > 0 {
		t.Errorf("Unexpected non-empty error list: %#v", errs)
	}
}

func TestValidateImageMissingFields(t *testing.T) {
	errorCases := map[string]struct {
		I api.Image
		T errors.ValidationErrorType
		F string
	}{
		"missing Name": {
			api.Image{DockerImageReference: "ref"},
			errors.ValidationErrorTypeRequired,
			"name",
		},
		"missing DockerImageReference": {
			api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}},
			errors.ValidationErrorTypeRequired,
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
			if errs[i].(*errors.ValidationError).Type == v.T && errs[i].(*errors.ValidationError).Field == v.F {
				match = true
				break
			}
		}
		if !match {
			t.Errorf("%s: expected errors to have field %s and type %s: %v", k, v.F, v.T, errs)
		}
	}
}

func TestValidateImageRepositoryMappingNotOK(t *testing.T) {
	errorCases := map[string]struct {
		I api.ImageRepositoryMapping
		T errors.ValidationErrorType
		F string
	}{
		"missing DockerImageRepository": {
			api.ImageRepositoryMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"dockerImageRepository",
		},
		"missing Name": {
			api.ImageRepositoryMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"name",
		},
		"missing Tag": {
			api.ImageRepositoryMapping{
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
			errors.ValidationErrorTypeRequired,
			"tag",
		},
		"missing image name": {
			api.ImageRepositoryMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "openshift/ruby-19-centos",
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"image.name",
		},
		"invalid repository pull spec": {
			api.ImageRepositoryMapping{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "default",
				},
				DockerImageRepository: "registry/extra/openshift/ruby-19-centos",
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:      "foo",
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeInvalid,
			"dockerImageRepository",
		},
	}

	for k, v := range errorCases {
		errs := ValidateImageRepositoryMapping(&v.I)
		if len(errs) == 0 {
			t.Errorf("Expected failure for %s", k)
			continue
		}
		match := false
		for i := range errs {
			if errs[i].(*errors.ValidationError).Type == v.T && errs[i].(*errors.ValidationError).Field == v.F {
				match = true
				break
			}
		}
		if !match {
			t.Errorf("%s: expected errors to have field %s and type %s: %v", k, v.F, v.T, errs)
		}
	}
}
