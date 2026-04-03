// Package apis provides BareMetalHost utilities: status checks, provisioning state monitoring, and Metal3 operations.
package apis

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
)

const (
	BMCSecretNamespace     = "openshift-machine-api"
	secretsDataPasswordKey = "password"
)

// BMHGVR is the GroupVersionResource for BareMetalHost (metal3.io/v1alpha1). Use for API-based get/delete/patch.
var BMHGVR = schema.GroupVersionResource{
	Group: "metal3.io", Version: "v1alpha1", Resource: "baremetalhosts",
}

// getBMHDynamic fetches a BareMetalHost via the dynamic client and converts to typed.
func getBMHDynamic(oc *exutil.CLI, bmhName, namespace string) (*metal3v1alpha1.BareMetalHost, error) {
	ctx := context.Background()
	u, err := oc.AdminDynamicClient().Resource(BMHGVR).Namespace(namespace).Get(ctx, bmhName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	var bmh metal3v1alpha1.BareMetalHost
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &bmh); err != nil {
		return nil, core.WrapError("convert BareMetalHost", bmhName, err)
	}
	return &bmh, nil
}

// GetBMHProvisioningState retrieves the current provisioning state of a BareMetalHost (via API).
func GetBMHProvisioningState(oc *exutil.CLI, bmhName, namespace string) (metal3v1alpha1.ProvisioningState, error) {
	bmh, err := getBMHDynamic(oc, bmhName, namespace)
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}
	return bmh.Status.Provisioning.State, nil
}

// GetBMHErrorMessage retrieves the error message from a BareMetalHost's status (via API).
func GetBMHErrorMessage(oc *exutil.CLI, bmhName, namespace string) (string, error) {
	bmh, err := getBMHDynamic(oc, bmhName, namespace)
	if err != nil {
		return "", core.WrapError("get BareMetalHost", bmhName, err)
	}
	return bmh.Status.ErrorMessage, nil
}

// GetBMH retrieves a BareMetalHost via the cluster API (preferred over oc get).
func GetBMH(oc *exutil.CLI, bmhName, namespace string) (*metal3v1alpha1.BareMetalHost, error) {
	return getBMHDynamic(oc, bmhName, namespace)
}

// BareMetalHostExists returns true if the BareMetalHost exists in the namespace.
func BareMetalHostExists(oc *exutil.CLI, bmhName, namespace string) (bool, error) {
	_, err := getBMHDynamic(oc, bmhName, namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// FindBMHByNodeName finds a BareMetalHost name matching *-{shortName} by listing via API.
func FindBMHByNodeName(oc *exutil.CLI, namespace, nodeName string) (string, error) {
	shortName := strings.Split(nodeName, ".")[0]
	pattern := regexp.MustCompile(fmt.Sprintf(`.*-%s$`, regexp.QuoteMeta(shortName)))
	ctx := context.Background()
	list, err := oc.AdminDynamicClient().Resource(BMHGVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list BareMetalHosts: %w", err)
	}
	for _, item := range list.Items {
		name := item.GetName()
		if pattern.MatchString(name) {
			return name, nil
		}
	}
	return "", fmt.Errorf("no BareMetalHost found matching pattern %s", pattern.String())
}

// FindBMCSecretByNodeName finds a BMC secret name matching *-{shortName}-bmc-secret by listing via API.
func FindBMCSecretByNodeName(oc *exutil.CLI, namespace, nodeName string) (string, error) {
	shortName := strings.Split(nodeName, ".")[0]
	pattern := regexp.MustCompile(fmt.Sprintf(`.*-%s-bmc-secret$`, regexp.QuoteMeta(shortName)))
	ctx := context.Background()
	list, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", fmt.Errorf("list Secrets: %w", err)
	}
	for _, secret := range list.Items {
		if pattern.MatchString(secret.Name) {
			return secret.Name, nil
		}
	}
	return "", fmt.Errorf("no Secret found matching pattern %s", pattern.String())
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

	// Rotate password using oc patch
	newPass := k8srand.String(32)
	updated := secret.DeepCopy()
	updated.Data[secretsDataPasswordKey] = []byte(newPass)

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return "", "", nil, fmt.Errorf("failed to update secret %s/%s: %w",
			BMCSecretNamespace, secret.Name, err)
	}

	return BMCSecretNamespace, secret.Name, original, nil
}

// RestoreBMCPassword restores the password key on the given BMC Secret in namespace (must match
// where the secret lives; BMC secrets for control-plane nodes are in BMCSecretNamespace).
func RestoreBMCPassword(oc *exutil.CLI, namespace, name string, originalPassword []byte) error {
	if originalPassword == nil {
		return nil
	}

	ctx := context.Background()
	secretClient := oc.AdminKubeClient().CoreV1().Secrets(namespace)
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
