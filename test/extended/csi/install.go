package csi

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/openshift/origin/test/extended/testdata"
	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	csiBasePath         = "test/extended/testdata/csi"
	defaultImageFormat  = "registry.svc.ci.openshift.org/origin/4.3:${component}"
	imageFormatVariable = "IMAGE_FORMAT"
)

// InstallCSIDriver installs a CSI driver and defines its tests.
// It applies "test/extended/csi/<driverName>/install-template.yaml" and
// returns path to test manifest "test/extended/csi/<driverName>/manifest.yaml"
func InstallCSIDriver(driverName string, dryRun bool) (string, error) {
	// The driver name comes from an user and we want a nice error message instead
	// of panic in FixturePath().
	templatePath := filepath.Join(csiBasePath, driverName, "install-template.yaml")
	if _, err := testdata.AssetInfo(templatePath); err != nil {
		return "", fmt.Errorf("failed to install CSI driver %q: %s", driverName, err)
	}

	manifestPath := filepath.Join(csiBasePath, driverName, "manifest.yaml")
	if _, err := testdata.AssetInfo(manifestPath); err != nil {
		return "", fmt.Errorf("failed to install CSI driver %q: %s", driverName, err)
	}

	// storageclass.yaml is optional, so we don't return and error if it's absent
	scPath := filepath.Join(csiBasePath, driverName, "storageclass.yaml")
	if _, err := testdata.AssetInfo(scPath); err == nil {
		scFixturePath := strings.Split(scPath, string(os.PathSeparator))[2:]
		exutil.FixturePath(scFixturePath...)
	}

	// Convert to array and cut "test/extended" for FixturePath()
	templateFixturePath := strings.Split(templatePath, string(os.PathSeparator))[2:]
	yamlPath, err := executeTemplate(exutil.FixturePath(templateFixturePath...))
	defer os.Remove(yamlPath)
	if err != nil {
		return "", err
	}

	if !dryRun {
		// Install the driver
		oc := exutil.NewCLIWithoutNamespace("csi-install")
		if err := oc.Run("apply").Args("-f", yamlPath).Execute(); err != nil {
			return "", fmt.Errorf("failed to apply %s: %s", yamlPath, err)
		}
	}

	// Cut "test/extended" for FixturePath()
	manifestFixturePath := strings.Split(manifestPath, string(os.PathSeparator))[2:]
	return exutil.FixturePath(manifestFixturePath...), nil
}

// ListCSIDrivers returns list of hardcoded CSI drivers, i.e. list of directories in "test/extended/csi".
func ListCSIDrivers() ([]string, error) {
	return testdata.AssetDir(csiBasePath)
}

// Executes given golang template file and returns path to resulting file.
func executeTemplate(templatePath string) (string, error) {
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return "", err
	}
	yamlFile, err := ioutil.TempFile("", "openshift-tests-csi-*")
	if err != nil {
		return "", err
	}
	yamlPath := yamlFile.Name()

	imageFormat := os.Getenv(imageFormatVariable)
	if imageFormat == "" {
		imageFormat = defaultImageFormat
	}

	variables := struct{ AttacherImage, ProvisionerImage, NodeDriverRegistrarImage, LivenessProbeImage, ImageFormat string }{
		AttacherImage:            strings.ReplaceAll(imageFormat, "${component}", "csi-external-attacher"),
		ProvisionerImage:         strings.ReplaceAll(imageFormat, "${component}", "csi-external-provisioner"),
		NodeDriverRegistrarImage: strings.ReplaceAll(imageFormat, "${component}", "csi-node-driver-registrar"),
		LivenessProbeImage:       strings.ReplaceAll(imageFormat, "${component}", "csi-livenessprobe"),
		ImageFormat:              imageFormat,
	}

	err = tmpl.Execute(yamlFile, variables)
	yamlFile.Close()
	if err != nil {
		os.Remove(yamlPath)
		return "", err
	}
	return yamlPath, nil
}
