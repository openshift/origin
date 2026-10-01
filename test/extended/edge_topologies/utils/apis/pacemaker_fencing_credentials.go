// Package apis provides utilities for interacting with OpenShift API resources.
//
// This file manages Pacemaker fencing secrets in openshift-etcd (fencing-credentials-*).
// Note: BareMetal Host BMC credentials (in openshift-machine-api) are separate and
// managed in baremetalhost.go for Baremetal Operator node provisioning.
package apis

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// EtcdNamespace is the OpenShift namespace for etcd static pods and related
	// Secrets/CronJobs, including fencing-credentials secrets.
	EtcdNamespace = "openshift-etcd"

	// PacemakerFencingSecretPrefix is the prefix for openshift-etcd fencing-credentials secrets.
	PacemakerFencingSecretPrefix = "fencing-credentials-"

	secretsDataPasswordKey = "password"

	// PacemakerFencingValidationJobName is the static name CEO gives the TNF fencing validation job
	// in openshift-etcd. CEO hashes the fencing-credentials-<node> Secret resourceVersions into this
	// job's spec, so a fencing-credentials change makes CEO delete and recreate the job to validate
	// the new credentials via fence_redfish.
	PacemakerFencingValidationJobName = "tnf-fencing-job"

	// fencingJobAPITimeout bounds each Jobs.Get request so a stalled API call cannot hang past a single
	// poll iteration (the surrounding core.PollUntil has its own overall timeout).
	fencingJobAPITimeout = 15 * time.Second
)

// PacemakerFencingCredentials holds the fields from a fencing-credentials secret in openshift-etcd.
type PacemakerFencingCredentials struct {
	SecretName              string
	Address                 string
	Username                string
	Password                string
	CertificateVerification string
}

// FindPacemakerFencingCredentialsByNodeName discovers the fencing-credentials secret for a node
// by listing secrets in openshift-etcd and matching against the node's short name.
func FindPacemakerFencingCredentialsByNodeName(oc *exutil.CLI, nodeName string) (*PacemakerFencingCredentials, error) {
	shortName := strings.Split(nodeName, ".")[0]

	ctx := context.Background()
	list, err := oc.AdminKubeClient().CoreV1().Secrets(EtcdNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list secrets in %s: %w", EtcdNamespace, err)
	}

	expected := map[string]struct{}{
		PacemakerFencingSecretPrefix + shortName: {},
		PacemakerFencingSecretPrefix + nodeName:  {},
	}

	for _, secret := range list.Items {
		if _, ok := expected[secret.Name]; ok {
			getRequired := func(key string) (string, error) {
				v, exists := secret.Data[key]
				if !exists || len(v) == 0 {
					return "", fmt.Errorf("secret %s missing required key %q", secret.Name, key)
				}
				return string(v), nil
			}
			address, err := getRequired("address")
			if err != nil {
				return nil, err
			}
			username, err := getRequired("username")
			if err != nil {
				return nil, err
			}
			password, err := getRequired("password")
			if err != nil {
				return nil, err
			}
			return &PacemakerFencingCredentials{
				SecretName:              secret.Name,
				Address:                 address,
				Username:                username,
				Password:                password,
				CertificateVerification: string(secret.Data["certificateVerification"]),
			}, nil
		}
	}

	return nil, fmt.Errorf("no fencing-credentials secret found matching node %q (prefix: %s, contains: %s) in %s",
		nodeName, PacemakerFencingSecretPrefix, shortName, EtcdNamespace)
}

// UpdatePacemakerFencingPassword sets the password key on the fencing-credentials
// secret namespace/name to newPassword.
func UpdatePacemakerFencingPassword(oc *exutil.CLI, namespace, name string, newPassword []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()

	secretClient := oc.AdminKubeClient().CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get fencing credentials secret %s/%s: %w", namespace, name, err)
	}

	updated := secret.DeepCopy()
	updated.Data[secretsDataPasswordKey] = newPassword

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to update password for fencing credentials secret %s/%s: %w", namespace, name, err)
	}

	return nil
}

// RestorePacemakerFencingPassword restores the password key on the given fencing-credentials
// secret in namespace (must match where the secret lives).
func RestorePacemakerFencingPassword(oc *exutil.CLI, namespace, name string, originalPassword []byte) error {
	if originalPassword == nil {
		return nil
	}

	ctx := context.Background()
	secretClient := oc.AdminKubeClient().CoreV1().Secrets(namespace)
	secret, err := secretClient.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to re-fetch fencing credentials secret %s/%s: %w", namespace, name, err)
	}

	updated := secret.DeepCopy()
	updated.Data[secretsDataPasswordKey] = originalPassword

	if _, err := secretClient.Update(ctx, updated, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("failed to restore password for %s/%s: %w", namespace, name, err)
	}

	return nil
}

// WaitForPacemakerFencingRejection waits for the cluster to reject an invalid fencing-credentials update.
// CEO hashes the fencing-credentials-<node> Secret resourceVersions into the tnf-fencing-job spec, so a
// Secret change makes CEO delete and recreate tnf-fencing-job to validate the new credentials via
// fence_redfish. Rejection is observed as the recreated job reaching JobFailed=True, which leaves the
// live stonith config untouched. A job that instead completes successfully means invalid credentials
// were accepted — a test failure. The job name is static (tnf-fencing-job); the CreationTimestamp
// filter ensures we observe the post-update recreation rather than the pre-existing job.
func WaitForPacemakerFencingRejection(oc *exutil.CLI, namespace string, minCreationTime time.Time, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for fencing job (recreated after %v) to fail (timeout: %v)", minCreationTime.UTC(), timeout)

	err := core.PollUntil(func() (bool, error) {
		getCtx, getCancel := context.WithTimeout(context.Background(), fencingJobAPITimeout)
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(getCtx, PacemakerFencingValidationJobName, metav1.GetOptions{})
		getCancel()
		if err != nil {
			e2e.Logf("Fencing job %s not found yet, waiting...", PacemakerFencingValidationJobName)
			return false, nil
		}
		if !job.CreationTimestamp.Time.After(minCreationTime) {
			e2e.Logf("Fencing job %s not yet recreated after the credentials update, waiting...", PacemakerFencingValidationJobName)
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				e2e.Logf("Fencing job %s failed as expected (reason=%s, message=%q)", PacemakerFencingValidationJobName, cond.Reason, cond.Message)
				return true, nil
			}
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				return false, core.NewError(fmt.Sprintf("fencing job %s", PacemakerFencingValidationJobName),
					"completed successfully but invalid fencing credentials should have been rejected")
			}
		}
		e2e.Logf("Fencing job %s still running...", PacemakerFencingValidationJobName)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("fencing job failure (recreated after %v)", minCreationTime.UTC()))

	services.DumpJobPodLogs(PacemakerFencingValidationJobName, namespace, oc)
	return err
}
