package validation

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
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
		T fielderrors.ValidationErrorType
		F string
	}{
		"missing Name": {
			api.Image{DockerImageReference: "ref"},
			fielderrors.ValidationErrorTypeRequired,
			"name",
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
				Tag: "latest",
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
				Tag: "latest",
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
				Tag: "latest",
				Image: api.Image{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "default",
					},
					DockerImageReference: "openshift/ruby-19-centos",
				},
			},
			fielderrors.ValidationErrorTypeRequired,
			"image.name",
		},
		"invalid repository pull spec": {
			api.ImageStreamMapping{
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
