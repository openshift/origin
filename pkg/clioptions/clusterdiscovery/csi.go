package clusterdiscovery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/external"
	"sigs.k8s.io/yaml"

	"github.com/openshift/origin/test/extended/storage/csi"
)

const (
	CSIManifestEnvVar = "TEST_CSI_DRIVER_FILES"
	OCPManifestEnvVar = "TEST_OCP_CSI_DRIVER_FILES"
)

// InitCSITests initializes the openshift/csi suite, i.e. define CSI tests from TEST_CSI_DRIVER_FILES.
func InitCSITests() error {
	ocpDrivers := sets.New[string]()
	upstreamDrivers := sets.New[string]()

	// Load OCP specific tests first, because AddOpenShiftCSITests() modifies global list of
	// testsuites.CSISuites used by AddDriverDefinition() below.
	ocpManifestList := os.Getenv(OCPManifestEnvVar)
	if ocpManifestList != "" {
		manifests := strings.Split(ocpManifestList, ",")
		for _, manifest := range manifests {
			fmt.Printf("Loading OCP test manifest from %q\n", manifest)
			csiDriver, err := csi.AddOpenShiftCSITests(manifest)
			if err != nil {
				return fmt.Errorf("failed to load OCP manifest from %q: %s", manifest, err)
			}
			ocpDrivers.Insert(csiDriver)
		}
	}

	upstreamManifestList := os.Getenv(CSIManifestEnvVar)
	if upstreamManifestList != "" {
		manifests := strings.Split(upstreamManifestList, ",")
		for _, manifest := range manifests {
			if err := external.AddDriverDefinition(manifest); err != nil {
				return fmt.Errorf("failed to load manifest from %q: %s", manifest, err)
			}
			csiDriver, err := parseDriverName(manifest)
			if err != nil {
				return fmt.Errorf("failed to parse CSI driver name from manifest %q: %s", manifest, err)
			}
			upstreamDrivers.Insert(csiDriver)

			// Register the base dir of the manifest file as a file source.
			// With this we can reference the CSI driver's storageClass
			// in the manifest file (FromFile field).
			testfiles.AddFileSource(testfiles.RootFileSource{
				Root: filepath.Dir(manifest),
			})
		}
	}

	// We allow missing OCP specific manifest for CI jobs that do not have it defined yet,
	// but all OCP specific manifest must have a corresponding upstream manifest.
	if ocpDrivers.Difference(upstreamDrivers).Len() > 0 {
		return fmt.Errorf("env. var %s must describe the same CSI drivers as %s: %v vs. %v", OCPManifestEnvVar, CSIManifestEnvVar, ocpDrivers.UnsortedList(), upstreamDrivers.UnsortedList())
	}

	return nil
}

func parseDriverName(filename string) (string, error) {
	bytes, err := os.ReadFile(filename)
	if err != nil {
		return "", err
	}

	// Minimal chunk of the upstream CSI driver manifest to extract the driver name.
	// See vendor/k8s.io/kubernetes/test/e2e/storage/external/external.go for the full definition.
	// It's private in that file, so we can't import it here.
	type upstreamManifest struct {
		DriverInfo struct {
			Name string
		}
	}
	manifest := &upstreamManifest{}
	err = yaml.Unmarshal(bytes, manifest)
	if err != nil {
		return "", err
	}
	if manifest.DriverInfo.Name == "" {
		return "", fmt.Errorf("missing driver name in manifest %q", filename)
	}
	return manifest.DriverInfo.Name, nil

}
