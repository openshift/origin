package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
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
		T errors.ValidationErrorType
		F string
	}{
		"missing Name":                 {api.Image{DockerImageReference: "ref"}, errors.ValidationErrorTypeRequired, "Name"},
		"missing DockerImageReference": {api.Image{ObjectMeta: kapi.ObjectMeta{Name: "foo"}}, errors.ValidationErrorTypeRequired, "DockerImageReference"},
	}

	for k, v := range errorCases {
		errs := ValidateImage(&v.I)
		if len(errs) == 0 {
			t.Errorf("Expected failure for %s", k)
			continue
		}
		for i := range errs {
			if errs[i].(errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
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
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name: "foo",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"DockerImageRepository",
		},
		"missing Tag": {
			api.ImageRepositoryMapping{
				DockerImageRepository: "openshift/ruby-19-centos",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name: "foo",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"Tag",
		},
		"missing image attributes": {
			api.ImageRepositoryMapping{
				Tag: "latest",
				DockerImageRepository: "openshift/ruby-19-centos",
				Image: api.Image{
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			errors.ValidationErrorTypeRequired,
			"image.Name",
		},
	}

	for k, v := range errorCases {
		errs := ValidateImageRepositoryMapping(&v.I)
		if len(errs) == 0 {
			t.Errorf("Expected failure for %s", k)
			continue
		}
		for i := range errs {
			if errs[i].(errors.ValidationError).Type != v.T {
				t.Errorf("%s: expected errors to have type %s: %v", k, v.T, errs[i])
			}
			if errs[i].(errors.ValidationError).Field != v.F {
				t.Errorf("%s: expected errors to have field %s: %v", k, v.F, errs[i])
			}
		}
	}
}
