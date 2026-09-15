// Package services: fencingcredentials.go manages the openshift-etcd fencing-credentials
// secrets used for pacemaker-driven fencing.
//
// A TNF cluster keeps BMC credentials in two different secrets, owned by different
// operators and used for different purposes:
//   - openshift-machine-api/<host>-bmc-secret is owned by the baremetal operator and used
//     to provision and manage the physical hosts. It plays no part in fencing.
//   - openshift-etcd/fencing-credentials-<node> is owned by cluster-etcd-operator and drives
//     pacemaker fencing: its values feed the fence_redfish stonith devices that power-cycle a
//     peer during recovery.
//
// This file only touches the openshift-etcd secrets; the machine-api BMC secret helpers live
// in the apis package (baremetalhost.go).
package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/edge_topologies/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	fencingCredentialsPrefix = "fencing-credentials-"
	secretsDataPasswordKey   = "password"

	// tnfFencingJobName is the static name CEO gives the TNF fencing job in openshift-etcd. CEO hashes
	// the fencing-credentials-<node> Secret resourceVersions into this job's spec, so a fencing-credentials
	// change makes CEO delete and recreate tnf-fencing-job to validate the new credentials via fence_redfish.
	tnfFencingJobName = "tnf-fencing-job"

	// fencingJobAPITimeout bounds each Jobs.Get request so a stalled API call cannot hang past a single
	// poll iteration (the surrounding core.PollUntil has its own overall timeout).
	fencingJobAPITimeout = 15 * time.Second
)

// FencingCredentials holds the fields from a fencing-credentials secret in openshift-etcd.
type FencingCredentials struct {
	SecretName              string
	Address                 string
	Username                string
	Password                string
	CertificateVerification string
}

// FindFencingCredentialsByNodeName discovers the fencing-credentials secret for a node
// by listing secrets in openshift-etcd and matching against the node's short name.
func FindFencingCredentialsByNodeName(oc *exutil.CLI, nodeName string) (*FencingCredentials, error) {
	shortName := strings.Split(nodeName, ".")[0]

	ctx := context.Background()
	list, err := oc.AdminKubeClient().CoreV1().Secrets(EtcdNamespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list secrets in %s: %w", EtcdNamespace, err)
	}

	expected := map[string]struct{}{
		fencingCredentialsPrefix + shortName: {},
		fencingCredentialsPrefix + nodeName:  {},
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
			return &FencingCredentials{
				SecretName:              secret.Name,
				Address:                 address,
				Username:                username,
				Password:                password,
				CertificateVerification: string(secret.Data["certificateVerification"]),
			}, nil
		}
	}

	return nil, fmt.Errorf("no fencing-credentials secret found matching node %q (prefix: %s, contains: %s) in %s",
		nodeName, fencingCredentialsPrefix, shortName, EtcdNamespace)
}

// UpdateFencingCredentialsPassword sets the password key on the fencing-credentials
// secret namespace/name to newPassword.
func UpdateFencingCredentialsPassword(oc *exutil.CLI, namespace, name string, newPassword []byte) error {
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

// RestoreFencingCredentialsPassword restores the password key on the given fencing-credentials
// secret in namespace (must match where the secret lives).
func RestoreFencingCredentialsPassword(oc *exutil.CLI, namespace, name string, originalPassword []byte) error {
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

// WaitForFencingRejection waits for the cluster to reject an invalid fencing-credentials update.
// CEO hashes the fencing-credentials-<node> Secret resourceVersions into the tnf-fencing-job spec, so a
// Secret change makes CEO delete and recreate tnf-fencing-job to validate the new credentials via
// fence_redfish. Rejection is observed as the recreated job reaching JobFailed=True, which leaves the
// live stonith config untouched. A job that instead completes successfully means invalid credentials
// were accepted — a test failure. The job name is static (tnf-fencing-job); the CreationTimestamp
// filter ensures we observe the post-update recreation rather than the pre-existing job.
func WaitForFencingRejection(oc *exutil.CLI, namespace string, minCreationTime time.Time, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for fencing job (recreated after %v) to fail (timeout: %v)", minCreationTime.UTC(), timeout)

	err := core.PollUntil(func() (bool, error) {
		getCtx, getCancel := context.WithTimeout(context.Background(), fencingJobAPITimeout)
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(getCtx, tnfFencingJobName, metav1.GetOptions{})
		getCancel()
		if err != nil {
			e2e.Logf("Fencing job %s not found yet, waiting...", tnfFencingJobName)
			return false, nil
		}
		if !job.CreationTimestamp.Time.After(minCreationTime) {
			e2e.Logf("Fencing job %s not yet recreated after the credentials update, waiting...", tnfFencingJobName)
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				e2e.Logf("Fencing job %s failed as expected (reason=%s, message=%q)", tnfFencingJobName, cond.Reason, cond.Message)
				return true, nil
			}
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				return false, core.NewError(fmt.Sprintf("fencing job %s", tnfFencingJobName),
					"completed successfully but invalid fencing credentials should have been rejected")
			}
		}
		e2e.Logf("Fencing job %s still running...", tnfFencingJobName)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("fencing job failure (recreated after %v)", minCreationTime.UTC()))

	DumpJobPodLogs(tnfFencingJobName, namespace, oc)
	return err
}
