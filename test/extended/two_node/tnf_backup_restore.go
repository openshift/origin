// TNF node replacement: restore Kubernetes Secrets from on-disk backup YAML (shared by BMC + etcd recovery).
package two_node

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/core"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// isRetryableSecretWriteError reports whether a Secret Create error should be retried.
func isRetryableSecretWriteError(err error) bool {
	if err == nil {
		return false
	}
	if apierrors.IsAlreadyExists(err) {
		return false
	}
	return services.IsRetryableEtcdError(err) || utils.IsTransientKubernetesAPIError(err)
}

// verifySecretReadable polls until Get succeeds for the Secret, or timeout.
func verifySecretReadable(oc *exutil.CLI, namespace, secretName string) error {
	return core.PollUntil(func() (bool, error) {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		defer cancel()
		_, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		if err == nil {
			e2e.Logf("[backup restore] verified Secret %s/%s is readable", namespace, secretName)
			return true, nil
		}
		if apierrors.IsNotFound(err) {
			e2e.Logf("[backup restore] Secret %s/%s not visible yet after create: %v", namespace, secretName, err)
			return false, nil
		}
		if utils.IsTransientKubernetesAPIError(err) {
			e2e.Logf("[backup restore] transient get Secret %s/%s: %v", namespace, secretName, err)
			return false, nil
		}
		return false, err
	}, backupSecretVerifyTimeout, backupSecretVerifyPollInterval, fmt.Sprintf("verify Secret %s/%s readable", namespace, secretName))
}

// createSecretFromBackupIfNeeded ensures Secret `secretName` exists in `namespace`, restoring from
// `filepath.Join(backupDir, secretName+".yaml")` when absent. Create is retried on etcd/transient errors;
// after a successful write (including AlreadyExists), Get is polled until the Secret is readable.
func createSecretFromBackupIfNeeded(oc *exutil.CLI, namespace, backupDir, secretName string) error {
	if backupDir == "" {
		return fmt.Errorf("backup directory is empty")
	}
	secretPath := filepath.Join(backupDir, secretName+".yaml")

	// Fast path: Secret already present (tolerate transient Get flakes during disruption).
	deadline := time.Now().Add(backupSecretInitialGetTimeout)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		_, err := oc.AdminKubeClient().CoreV1().Secrets(namespace).Get(ctx, secretName, metav1.GetOptions{})
		cancel()
		if err == nil {
			e2e.Logf("[backup restore] Secret %s/%s already exists, skipping restore from %s", namespace, secretName, secretPath)
			return nil
		}
		if apierrors.IsNotFound(err) {
			break
		}
		if utils.IsTransientKubernetesAPIError(err) {
			e2e.Logf("[backup restore] transient get Secret %s/%s: %v; retrying", namespace, secretName, err)
			time.Sleep(backupSecretVerifyPollInterval)
			continue
		}
		return fmt.Errorf("get Secret %s/%s: %w", namespace, secretName, err)
	}

	secretBytes, readErr := os.ReadFile(secretPath)
	if readErr != nil {
		return fmt.Errorf("read backup file %s: %w", secretPath, readErr)
	}
	var secret corev1.Secret
	if err := utils.DecodeObject(string(secretBytes), &secret); err != nil {
		return fmt.Errorf("decode Secret from %s: %w", secretPath, err)
	}
	// Backup YAML may carry a different name/namespace than this restore path; pin to the requested object.
	secret.Name = secretName
	secret.Namespace = namespace
	secret.ResourceVersion = ""
	secret.UID = ""

	err := core.RetryWithOptions(func() error {
		createCtx, cancel := context.WithTimeout(context.Background(), shortK8sClientTimeout)
		defer cancel()
		_, createErr := oc.AdminKubeClient().CoreV1().Secrets(namespace).Create(createCtx, &secret, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(createErr) {
			return nil
		}
		return createErr
	}, core.RetryOptions{
		Timeout:      etcdThreeMinutePollTimeout,
		PollInterval: utils.ThirtySecondPollInterval,
		MaxRetries:   10,
		ShouldRetry:  isRetryableSecretWriteError,
	}, fmt.Sprintf("create Secret %s/%s from backup", namespace, secretName))
	if err != nil {
		return fmt.Errorf("create Secret %s/%s after retries: %w", namespace, secretName, err)
	}

	if verifyErr := verifySecretReadable(oc, namespace, secretName); verifyErr != nil {
		return fmt.Errorf("verify Secret %s/%s after create: %w", namespace, secretName, verifyErr)
	}
	e2e.Logf("[backup restore] restored Secret %s/%s from %s", namespace, secretName, secretPath)
	return nil
}
