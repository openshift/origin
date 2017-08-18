package docker

import (
	"testing"

	"github.com/openshift/origin/pkg/oc/bootstrap"
)

// TestBootstrapFiles ensures that the files that are used for
// Bootstrapping a cluster are available.
func TestBootstrapFiles(t *testing.T) {
	templateMaps := []map[string]string{
		imageStreams,
		templateLocations,
	}
	for _, templateMap := range templateMaps {
		for _, location := range templateMap {
			_, err := bootstrap.Asset(location)
			if err != nil {
				t.Errorf("Error getting asset at: %s", location)
			}
		}
	}
}
