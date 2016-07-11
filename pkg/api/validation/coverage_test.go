package validation

import (
	"reflect"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	quotaapi "github.com/openshift/origin/pkg/quota/api"
)

// KnownValidationExceptions is the list of API types that do NOT have corresponding validation
// If you add something to this list, explain why it doesn't need validation.  waaaa is not a valid
// reason.
var KnownValidationExceptions = []reflect.Type{
	reflect.TypeOf(&buildapi.BuildLog{}),                              // masks calls to a build subresource
	reflect.TypeOf(&deployapi.DeploymentLog{}),                        // masks calls to a deploymentConfig subresource
	reflect.TypeOf(&imageapi.ImageStreamImage{}),                      // this object is only returned, never accepted
	reflect.TypeOf(&imageapi.ImageStreamTag{}),                        // this object is only returned, never accepted
	reflect.TypeOf(&authorizationapi.IsPersonalSubjectAccessReview{}), // only an api type for runtime.EmbeddedObject, never accepted
	reflect.TypeOf(&authorizationapi.SubjectAccessReviewResponse{}),   // this object is only returned, never accepted
	reflect.TypeOf(&authorizationapi.ResourceAccessReviewResponse{}),  // this object is only returned, never accepted
	reflect.TypeOf(&quotaapi.AppliedClusterResourceQuota{}),           // this object is only returned, never accepted
}

// MissingValidationExceptions is the list of types that were missing validation methods when I started
// You should never add to this list
var MissingValidationExceptions = []reflect.Type{
	reflect.TypeOf(&buildapi.BuildLogOptions{}),           // TODO, looks like this one should have validation
	reflect.TypeOf(&buildapi.BinaryBuildRequestOptions{}), // TODO, looks like this one should have validation
	reflect.TypeOf(&imageapi.DockerImage{}),               // TODO, I think this type is ok to skip validation (internal), but needs review
}

func TestCoverage(t *testing.T) {
	for kind, apiType := range kapi.Scheme.KnownTypes(api.SchemeGroupVersion) {
		if strings.HasPrefix(apiType.PkgPath(), "github.com/openshift/origin/vendor/") {
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
