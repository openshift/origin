package v1

import (
	"testing"

	"github.com/openshift/origin/pkg/template/api"
	testutil "github.com/openshift/origin/test/util/api"
)

func TestFieldSelectorConversions(t *testing.T) {
	testutil.CheckFieldLabelConversions(t, "v1", "Template",
		// Ensure all currently returned labels are supported
		api.TemplateToSelectableFields(&api.Template{}),
	)
}
