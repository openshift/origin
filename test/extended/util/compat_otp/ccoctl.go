package compat_otp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// ExtractCcotl extracts the specified version of the ccoctl binary from the given release image.
// It supports different versions of ccoctl for RHEL environments, including:
//   - "ccoctl"
//   - "ccoctl.rhel8" (RHEL 8 version)
//   - "ccoctl.rhel9" (RHEL 9 version)
//
// Usage example:
//
//	ccoctlTarget := "ccoctl.rhel8"
//	ccoctlPath := exutil.ExtractCcotl(oc, testOCPImage, ccoctlTarget, true)
//	defer os.Remove(filepath.Dir(ccoctlPath))
//
// Parameters:
//   - oc: exutil.CLI object to interact with OpenShift commands.
//   - releaseImage: The OpenShift release image from which to extract the ccoctl binary.
//   - ccoctlTarget: The target ccoctl version to extract (e.g., "ccoctl"(default), "ccoctl.rhel8", "ccoctl.rhel9").
//
// Returns:
//   - A string containing the file path of the extracted ccoctl binary.
func ExtractCcoctl(oc *exutil.CLI, releaseImage, ccoctlTarget string) string {
	e2e.Logf("Extracting ccoctl from release image %v ...", releaseImage)
	dirname := "/tmp/" + oc.Namespace() + "-ccoctl"
	err := os.MkdirAll(dirname, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("Extracting the pull secret file")
	pullSecretDirName := "/tmp/" + oc.Namespace() + "-auth"
	err = os.MkdirAll(pullSecretDirName, 0777)
	o.Expect(err).NotTo(o.HaveOccurred())
	defer os.Remove(pullSecretDirName)

	err = GetPullSec(oc, pullSecretDirName)
	pullSecretFile := filepath.Join(pullSecretDirName, ".dockerconfigjson")
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("Generated pullSecretFile: %s", pullSecretFile)

	e2e.Logf("Extracting CCO Image from release image")
	ccoImage, err := oc.AsAdmin().WithoutNamespace().Run("adm").Args("release", "info", "--registry-config", pullSecretFile, "--image-for=cloud-credential-operator", releaseImage).Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(ccoImage).NotTo(o.BeEmpty())
	e2e.Logf("CCO Image: %s", ccoImage)

	e2e.Logf("Extracting ccoctl binary from cco image")
	_, err = oc.AsAdmin().WithoutNamespace().Run("image").Args("extract", ccoImage, "--registry-config", pullSecretFile, "--path=/usr/bin/"+ccoctlTarget+":"+dirname, "--confirm").Output()
	o.Expect(err).NotTo(o.HaveOccurred())
	ccoctlPath := filepath.Join(dirname, ccoctlTarget)
	err = os.Chmod(ccoctlPath, 0775)
	o.Expect(err).NotTo(o.HaveOccurred())

	e2e.Logf("Making sure ccoctl is functional")
	outputBytes1, err := exec.Command("bash", "-c", fmt.Sprintf("%s --help", ccoctlPath)).CombinedOutput()
	e2e.Logf("ccoctl--help output: %s", string(outputBytes1))
	o.Expect(err).NotTo(o.HaveOccurred())
	e2e.Logf("ccoctl path %s", ccoctlPath)

	return ccoctlPath
}
