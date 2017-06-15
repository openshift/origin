package origin

import (
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/api/latest"

	"k8s.io/kubernetes/pkg/api"
)

func TestLegacyKinds(t *testing.T) {
	for gvk := range api.Scheme.AllKnownTypes() {
		if latest.OriginLegacyKind(gvk) && !OriginLegacyKinds.Has(gvk.Kind) &&
			!strings.HasPrefix(gvk.Kind, "SecurityContextConstraint") /* SCC is a special case that's allowed */ {
			t.Errorf("%s should not be registered into legacy Origin API", gvk.Kind)
		}
	}
}
