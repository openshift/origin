package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/kubernetes/test/e2e/framework/testfiles"
	"k8s.io/kubernetes/test/e2e/storage/external"

	"github.com/openshift/origin/test/extended/csi"
)

const (
	manifestEnvVar       = "TEST_CSI_DRIVER_FILES"
	installDriversEnvVar = "TEST_INSTALL_CSI_DRIVERS"
)

// Initialize openshift/csi suite, i.e. define CSI tests from TEST_CSI_DRIVER_FILES.
func initCSITests(dryRun bool) error {
	driverList := os.Getenv(installDriversEnvVar)
	if driverList != "" {
		drivers := strings.Split(driverList, ",")
		for _, driver := range drivers {
			manifestFile, err := csi.InstallCSIDriver(driver, dryRun)
			if err != nil {
				return fmt.Errorf("failed to install CSI driver from %q: %s", driver, err)
			}
			// Children processes need to see the newly introduced manifest,
			// store it in TEST_CSI_DRIVER_FILES env. var for them.
			manifestList := os.Getenv(manifestEnvVar)
			if len(manifestList) > 0 {
				manifestList += ","
			}
			manifestList += manifestFile
			os.Setenv(manifestEnvVar, manifestList)
		}
	}

	// Clear TEST_INSTALL_CSI_DRIVERS, we don't want the driver installed by children too.
	os.Setenv(installDriversEnvVar, "")

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
