package default_imagestreams

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/clusterup/manifests"
)

// TestBootstrapFiles ensures that the files that are used for
// Bootstrapping a cluster are available.  These aren't tested every build because we don't normally use rhel
func TestBootstrapFiles(t *testing.T) {
	for _, location := range []string{rhelLocation, centosLocation} {
		_, err := manifests.Asset(location)
		if err != nil {
			t.Errorf("Error getting asset at: %s", location)
		}
	}
}
