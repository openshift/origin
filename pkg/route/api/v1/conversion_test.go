package v1

import (
	"testing"

	"github.com/openshift/origin/pkg/route/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Route",
		// Ensure all currently returned labels are supported
		api.RouteToSelectableFields(&api.Route{}),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		"spec.host", "spec.path", "spec.to.name",
	)
}
