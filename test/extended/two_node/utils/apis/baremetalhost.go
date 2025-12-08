// Package apis provides BareMetalHost utilities: status checks, provisioning state monitoring, and Metal3 operations.
package apis

import (
	"context"
	"fmt"
	"strings"

	metal3v1alpha1 "github.com/metal3-io/baremetal-operator/apis/metal3.io/v1alpha1"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8srand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/kubernetes"
)

const (
	BMCSecretNamespace = "openshift-machine-api"
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

// RotateNodeBMCPassword discovers the BMC Secret for the given node,
// rotates its "password" key and returns (namespace, secretName, originalPassword).
func RotateNodeBMCPassword(kubeClient kubernetes.Interface, node *corev1.Node) (string, string, []byte, error) {
	ctx := context.Background()
	secretClient := kubeClient.CoreV1().Secrets(BMCSecretNamespace)

	secrets, err := secretClient.List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", "", nil, fmt.Errorf("failed to list secrets: %w", err)
	}

	var target *corev1.Secret
	for _, s := range secrets.Items {
		if strings.Contains(s.Name, node.Name) &&
			strings.Contains(s.Name, "bmc") {
			target = &s
			break
		}
	}

	if target == nil {
		return "", "", nil, fmt.Errorf("no BMC secret found for node %s", node.Name)
	}

	// Save original password
	original := target.Data["password"]

	// Rotate password
	newPass := k8srand.String(32)
	updated := target.DeepCopy()
	updated.Data["password"] = []byte(newPass)

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return "", "", nil, fmt.Errorf("failed to update secret %s/%s: %w",
			BMCSecretNamespace, target.Name, err)
	}

	return BMCSecretNamespace, target.Name, original, nil
}

// RestoreBMCPassword restores the password key on the given BMC Secret.
func RestoreBMCPassword(kubeClient kubernetes.Interface, namespace, name string, originalPassword []byte) error {
	if originalPassword == nil {
		return nil
	}

	ctx := context.Background()
	secretClient := kubeClient.CoreV1().Secrets(namespace)

	secret, err := secretClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to re-fetch BMC secret %s/%s: %w", namespace, name, err)
	}

	updated := secret.DeepCopy()
	updated.Data["password"] = originalPassword

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to restore password for %s/%s: %w", namespace, name, err)
	}

	return nil
}
