package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/external"
)

const (
	manifestEnvVar = "TEST_CSI_DRIVER_FILES"
)

// Initialize openshift/csi suite, i.e. define CSI tests from TEST_CSI_DRIVER_FILES.
func initCSITests(dryRun bool) error {
	manifestList := os.Getenv(manifestEnvVar)
	if manifestList != "" {
		manifests := strings.Split(manifestList, ",")
		for _, manifest := range manifests {
			if err := external.AddDriverDefinition(manifest); err != nil {
				return fmt.Errorf("failed to load manifest from %q: %s", manifest, err)
			}
			// Register the base dir of the manifest file as a file source.
			// With this we can reference the CSI driver's storageClass
			// in the manifest file (FromFile field).
			testfiles.AddFileSource(testfiles.RootFileSource{
				Root: filepath.Dir(manifest),
			})
		}
	}

	return nil
}
