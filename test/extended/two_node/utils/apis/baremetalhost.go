// Package apis provides BareMetalHost utilities: status checks, provisioning state monitoring, and Metal3 operations.
package apis

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

	ctx := context.Background()
	secretClient := oc.AdminKubeClient().CoreV1().Secrets(BMCSecretNamespace)
	secret, err := secretClient.Get(ctx, secretName, metav1.GetOptions{})
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to get BMC secret %s/%s: %w", BMCSecretNamespace, secretName, err)
	}

	// Save original password
	original := secret.Data[secretsDataPasswordKey]

	// Rotate password
	newPass := k8srand.String(32)
	updated := secret.DeepCopy()
	updated.Data[secretsDataPasswordKey] = []byte(newPass)

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return "", "", nil, fmt.Errorf("failed to update secret %s/%s: %w",
			BMCSecretNamespace, secret.Name, err)
	}

	return BMCSecretNamespace, secret.Name, original, nil
}

// RestoreBMCPassword restores the password key on the given BMC Secret.
func RestoreBMCPassword(oc *exutil.CLI, namespace, name string, originalPassword []byte) error {
	if originalPassword == nil {
		return nil
	}

	ctx := context.Background()
	secretClient := oc.AdminKubeClient().CoreV1().Secrets(BMCSecretNamespace)
	secret, err := secretClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to re-fetch BMC secret %s/%s: %w", namespace, name, err)
	}

	updated := secret.DeepCopy()
	updated.Data[secretsDataPasswordKey] = originalPassword

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to restore password for %s/%s: %w", namespace, name, err)
	}

	return nil
}
