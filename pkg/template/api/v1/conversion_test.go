package v1_test

import (
	"testing"

	"github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util/api"

	// install all APIs
	_ "github.com/openshift/origin/pkg/api/install"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Template",
		// Ensure all currently returned labels are supported
		api.TemplateToSelectableFields(&api.Template{}),
	)
}
