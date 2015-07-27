package describe

import (
	"io/ioutil"
	"reflect"
	"strings"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	buildapi "github.com/openshift/origin/pkg/build/api"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

// PrinterCoverageExceptions is the list of API types that do NOT have corresponding printers
// If you add something to this list, explain why it doesn't need validation.  waaaa is not a valid
// reason.
var PrinterCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&imageapi.DockerImage{}), // not a top level resource
	reflect.TypeOf(&buildapi.BuildLog{}),    // just a marker type
}

// MissingPrinterCoverageExceptions is the list of types that were missing printer methods when I started
// You should never add to this list
// TODO printers should be added for these types
var MissingPrinterCoverageExceptions = []reflect.Type{
	reflect.TypeOf(&authorizationapi.SubjectAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReviewResponse{}),
	reflect.TypeOf(&authorizationapi.SubjectAccessReview{}),
	reflect.TypeOf(&authorizationapi.ResourceAccessReview{}),
	reflect.TypeOf(&deployapi.DeploymentConfigRollback{}),
	reflect.TypeOf(&imageapi.ImageStreamMapping{}),
	reflect.TypeOf(&buildapi.BuildLogOptions{}),
	reflect.TypeOf(&buildapi.BuildRequest{}),
	reflect.TypeOf(&projectapi.ProjectRequest{}),
}

func TestPrinterCoverage(t *testing.T) {
	printer := NewHumanReadablePrinter(false, false, false, []string{})

main:
	for _, apiType := range kapi.Scheme.KnownTypes("") {
		if !strings.Contains(apiType.PkgPath(), "openshift/origin") {
			continue
		}

		ptrType := reflect.PtrTo(apiType)
		for _, exception := range PrinterCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}
		for _, exception := range MissingPrinterCoverageExceptions {
			if ptrType == exception {
				continue main
			}
		}

		newStructValue := reflect.New(apiType)
		newStruct := newStructValue.Interface()

		if err := printer.PrintObj(newStruct.(runtime.Object), ioutil.Discard); (err != nil) && strings.Contains(err.Error(), "error: unknown type ") {
			t.Errorf("missing printer for %v.  Check pkg/cmd/cli/describe/printer.go", apiType)
		}
	}
}
