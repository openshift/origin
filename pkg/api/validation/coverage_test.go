package validation

import (
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// KnownValidationExceptions is the list of API types that do NOT have corresponding validation
// If you add something to this list, explain why it doesn't need validation.  waaaa is not a valid
// reason.
var KnownValidationExceptions = []reflect.Type{
	reflect.TypeOf(&imageapi.ImageStreamImage{}),                      // this object is only returned, never accepted
	reflect.TypeOf(&imageapi.ImageStreamTag{}),                        // this object is only returned, never accepted
	reflect.TypeOf(&authorizationapi.IsPersonalSubjectAccessReview{}), // only an api type for runtime.EmbeddedObject, never accepted
	reflect.TypeOf(&authorizationapi.SubjectAccessReviewResponse{}),   // this object is only returned, never accepted
	reflect.TypeOf(&authorizationapi.ResourceAccessReviewResponse{}),  // this object is only returned, never accepted
}

// MissingValidationExceptions is the list of types that were missing validation methods when I started
// You should never add to this list
var MissingValidationExceptions = []reflect.Type{
	reflect.TypeOf(&buildapi.BuildLogOptions{}),              // TODO, looks like this one should have validation
	reflect.TypeOf(&buildapi.BuildLog{}),                     // TODO, I have no idea what this is doing
	reflect.TypeOf(&buildapi.BuildRequest{}),                 // TODO, looks to be used internally, but needs review
	reflect.TypeOf(&imageapi.ImageStreamMapping{}),           // TODO, looks like this one should have validation
	reflect.TypeOf(&imageapi.DockerImage{}),                  // TODO, I think this type is ok to skip validation (internal), but needs review
	reflect.TypeOf(&authorizationapi.SubjectAccessReview{}),  // TODO, this one should have validation
	reflect.TypeOf(&authorizationapi.ResourceAccessReview{}), // TODO, this one should have validation
}

func TestCoverage(t *testing.T) {
	for kind, apiType := range kapi.Scheme.KnownTypes("") {
		if !strings.Contains(apiType.PkgPath(), "openshift/origin") {
			continue
		}
		if strings.HasSuffix(kind, "List") {
			continue
		}

		ptrType := reflect.PtrTo(apiType)

		if _, exists := Validator.typeToValidator[ptrType]; !exists {
			allowed := false
			for _, exception := range KnownValidationExceptions {
				if ptrType == exception {
					allowed = true
					break
				}
			}
			for _, exception := range MissingValidationExceptions {
				if ptrType == exception {
					allowed = true
				}
			}

			if !allowed {
				t.Errorf("%v is not registered.  Look in pkg/api/validation/register.go.", apiType)
			}
		}
	}
}
