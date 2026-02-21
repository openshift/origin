// Package apis provides BareMetalHost utilities: status checks, provisioning state monitoring, and Metal3 operations.
package apis

import (
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	BMCSecretNamespace     = "openshift-machine-api"
	secretsDataPasswordKey = "password"
)

// GetBMHProvisioningState retrieves the current provisioning state of a BareMetalHost.
//
//	state, err := GetBMHProvisioningState(oc, "master-0", "openshift-machine-api")
func GetBMHProvisioningState(oc *exutil.CLI, bmhName, namespace string) (metal3v1alpha1.ProvisioningState, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return "", core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return bmh.Status.Provisioning.State, nil
}

// GetBMHErrorMessage retrieves the error message from a BareMetalHost's status.
//
//	errorMsg, err := GetBMHErrorMessage(oc, "master-0", "openshift-machine-api")
func GetBMHErrorMessage(oc *exutil.CLI, bmhName, namespace string) (string, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return "", core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return bmh.Status.ErrorMessage, nil
}

// GetBMH retrieves and parses a BareMetalHost resource.
//
//	bmh, err := GetBMH(oc, "master-0", "openshift-machine-api")
func GetBMH(oc *exutil.CLI, bmhName, namespace string) (*metal3v1alpha1.BareMetalHost, error) {
	bmhOutput, err := oc.AsAdmin().Run("get").Args("bmh", bmhName, "-n", namespace, "-o", "yaml").Output()
	if err != nil {
		return nil, core.WrapError("get BareMetalHost", bmhName, err)
	}

	var bmh metal3v1alpha1.BareMetalHost
	if err := utils.DecodeObject(bmhOutput, &bmh); err != nil {
		return nil, core.WrapError("decode BareMetalHost YAML", bmhName, err)
	}

	return &bmh, nil
}

// findObjectByPattern lists resources of the given type and returns the first name matching the regex.
func findObjectByPattern(oc *exutil.CLI, resourceType, namespace string, pattern *regexp.Regexp) (string, error) {
	output, err := oc.AsAdmin().Run("get").Args(resourceType, "-n", namespace, "-o", "name").Output()
	if err != nil {
		return "", fmt.Errorf("failed to list %s: %w", resourceType, err)
	}

	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if idx := strings.LastIndex(line, "/"); idx != -1 {
			name := line[idx+1:]
			if pattern.MatchString(name) {
				return name, nil
			}
		}
	}

	return "", fmt.Errorf("no %s found matching pattern %s", resourceType, pattern.String())
}

// FindBMHByNodeName finds a BareMetalHost name matching the pattern *-{shortName} for a given node.
// Handles both simple names (master-0) and FQDNs (master-0.ostest.test.metalkube.org).
func FindBMHByNodeName(oc *exutil.CLI, namespace, nodeName string) (string, error) {
	shortName := strings.Split(nodeName, ".")[0]
	pattern := regexp.MustCompile(fmt.Sprintf(`.*-%s$`, regexp.QuoteMeta(shortName)))
	return findObjectByPattern(oc, "bmh", namespace, pattern)
}

// FindBMCSecretByNodeName finds a BMC secret name matching the pattern *-{shortName}-bmc-secret.
// Handles both simple names (master-0) and FQDNs (master-0.ostest.test.metalkube.org).
func FindBMCSecretByNodeName(oc *exutil.CLI, namespace, nodeName string) (string, error) {
	shortName := strings.Split(nodeName, ".")[0]
	pattern := regexp.MustCompile(fmt.Sprintf(`.*-%s-bmc-secret$`, regexp.QuoteMeta(shortName)))
	return findObjectByPattern(oc, "secret", namespace, pattern)
}

// RotateNodeBMCPassword discovers the BMC Secret for the given node,
// rotates its "password" key and returns (namespace, secretName, originalPassword).
func RotateNodeBMCPassword(oc *exutil.CLI, node *corev1.Node) (string, string, []byte, error) {
	// Find the BMC secret name using pattern matching (handles FQDNs)
	secretName, err := FindBMCSecretByNodeName(oc, BMCSecretNamespace, node.Name)
	if err != nil {
		return "", "", nil, err
	}

	// Get the secret to read current password
	secretOutput, err := oc.AsAdmin().Run("get").Args("secret", secretName, "-n", BMCSecretNamespace, "-o", "yaml").Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get BMC secret %s: %w", secretName, err)
	}

	var secret corev1.Secret
	if err := utils.DecodeObject(secretOutput, &secret); err != nil {
		return "", "", nil, fmt.Errorf("failed to decode secret: %w", err)
	}

	// Save original password
	original := secret.Data[secretsDataPasswordKey]

	// Rotate password using oc patch
	newPass := k8srand.String(32)
	newPassB64 := base64.StdEncoding.EncodeToString([]byte(newPass))
	patch := fmt.Sprintf(`{"data":{"%s":"%s"}}`, secretsDataPasswordKey, newPassB64)

	_, err = oc.AsAdmin().Run("patch").Args("secret", secretName, "-n", BMCSecretNamespace, "-p", patch).Output()
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to update secret %s: %w", secretName, err)
	}

	return BMCSecretNamespace, secretName, original, nil
}

// RestoreBMCPassword restores the password key on the given BMC Secret.
func RestoreBMCPassword(oc *exutil.CLI, namespace, name string, originalPassword []byte) error {
	if originalPassword == nil {
		return nil
	}

	// Restore password using oc patch
	passB64 := base64.StdEncoding.EncodeToString(originalPassword)
	patch := fmt.Sprintf(`{"data":{"%s":"%s"}}`, secretsDataPasswordKey, passB64)

	_, err := oc.AsAdmin().Run("patch").Args("secret", name, "-n", namespace, "-p", patch).Output()
	if err != nil {
		return fmt.Errorf("failed to restore password for %s/%s: %w", namespace, name, err)
	}

	return nil
}
