// Package services provides etcd utilities: error classification, retry detection, and learner state handling.
package services

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/two_node/utils/core"
	exutil "github.com/openshift/origin/test/extended/util"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// EtcdErrorType categorizes etcd errors for granular error handling.
type EtcdErrorType int

const (
	// EtcdErrorUnknown represents an unclassified error
	EtcdErrorUnknown EtcdErrorType = iota

	// EtcdErrorLearner represents errors related to etcd learner state transitions
	// These are typically transient and should be retried
	EtcdErrorLearner

	// EtcdErrorQuorum represents errors related to etcd cluster quorum issues
	// These may require operator intervention but can sometimes be retried
	EtcdErrorQuorum

	// EtcdErrorNetwork represents network connectivity errors
	// These are typically transient and should be retried
	EtcdErrorNetwork

	// EtcdErrorTimeout represents timeout errors
	// These are typically transient and should be retried
	EtcdErrorTimeout

	// EtcdErrorRateLimit represents rate limiting or overload errors
	// These should be retried with backoff
	EtcdErrorRateLimit
)

const (
	etcdNamespace = "openshift-etcd"
)

// String returns a human-readable name for the error type
func (e EtcdErrorType) String() string {
	switch e {
	case EtcdErrorLearner:
		return "Learner"
	case EtcdErrorQuorum:
		return "Quorum"
	case EtcdErrorNetwork:
		return "Network"
	case EtcdErrorTimeout:
		return "Timeout"
	case EtcdErrorRateLimit:
		return "RateLimit"
	default:
		return "Unknown"
	}
}

// ClassifyEtcdError categorizes an etcd error into a specific error type for intelligent retry strategies.
//
//	errorType := ClassifyEtcdError(err)
func ClassifyEtcdError(err error) EtcdErrorType {
	if err == nil {
		return EtcdErrorUnknown
	}

	errStr := strings.ToLower(err.Error())

	// Check for learner-specific errors first (most specific)
	learnerPatterns := []string{
		"learner",
		"not a voter",
		"member is not started",
	}
	for _, pattern := range learnerPatterns {
		if strings.Contains(errStr, pattern) {
			return EtcdErrorLearner
		}
	}

	// Check for quorum errors
	quorumPatterns := []string{
		"raft: not leader",
		"no quorum",
		"etcdserver: no leader",
		"cluster id mismatch",
	}
	for _, pattern := range quorumPatterns {
		if strings.Contains(errStr, pattern) {
			return EtcdErrorQuorum
		}
	}

	// Check for rate limiting errors
	rateLimitPatterns := []string{
		"etcdserver: too many requests",
		"rate limit exceeded",
		"too many operations",
	}
	for _, pattern := range rateLimitPatterns {
		if strings.Contains(errStr, pattern) {
			return EtcdErrorRateLimit
		}
	}

	// Check for timeout errors
	timeoutPatterns := []string{
		"context deadline exceeded",
		"request timed out",
		"etcdserver: request timed out",
		"deadline exceeded",
		"i/o timeout",
	}
	for _, pattern := range timeoutPatterns {
		if strings.Contains(errStr, pattern) {
			return EtcdErrorTimeout
		}
	}

	// Check for network errors
	networkPatterns := []string{
		"connection refused",
		"rpc error: code = unavailable",
		"transport is closing",
		"network is unreachable",
		"no route to host",
		"connection reset",
		"broken pipe",
	}
	for _, pattern := range networkPatterns {
		if strings.Contains(errStr, pattern) {
			return EtcdErrorNetwork
		}
	}

	return EtcdErrorUnknown
}

// IsRetryableEtcdError checks if an etcd error should be retried (learner, network, timeout, rate limit).
//
//	err := RetryWithOptions(func() error { return etcdOp() }, RetryOptions{ShouldRetry: IsRetryableEtcdError}, "etcd operation")
func IsRetryableEtcdError(err error) bool {
	errType := ClassifyEtcdError(err)

	switch errType {
	case EtcdErrorLearner, EtcdErrorNetwork, EtcdErrorTimeout, EtcdErrorRateLimit:
		return true
	case EtcdErrorQuorum:
		// Quorum errors might be retryable in some contexts, but generally
		// require operator intervention. Log a warning.
		klog.V(2).Infof("Detected etcd quorum error - may need operator intervention: %v", err)
		return false
	default:
		return false
	}
}

// IsEtcdLearnerError checks if an error is related to transient etcd learner state issues.
//
//	err := RetryWithOptions(func() error { return createEtcdSecret(oc, secretFile) }, RetryOptions{ShouldRetry: IsEtcdLearnerError}, "create etcd secret")
func IsEtcdLearnerError(err error) bool {
	return ClassifyEtcdError(err) == EtcdErrorLearner
}

// WaitForEtcdRevisionCreation polls until the /var/lib/etcd/revision.json file exists on the target node.
// This file is created by the Cluster Etcd Operator (CEO) after the node joins the cluster.
//
//	err := WaitForEtcdRevisionCreation(targetNodeIP, tenMinuteTimeout, thirtySecondPollInterval, &hypervisorConfig, hypervisorKnownHosts, targetNodeKnownHosts, oc)
func WaitForEtcdRevisionCreation(targetNodeIP string, timeout, pollInterval time.Duration, hypervisorConfig *core.SSHConfig, hypervisorKnownHostsPath, targetNodeKnownHostsPath string, oc *exutil.CLI) error {
	const revisionFile = "/var/lib/etcd/revision.json"

	klog.V(2).Infof("Waiting for CEO to create %s on node %s (timeout: %v)", revisionFile, targetNodeIP, timeout)

	return core.PollUntil(func() (bool, error) {
		// Check if the revision.json file exists using a remote SSH command
		checkCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'not found'", revisionFile)
		output, _, err := core.ExecuteRemoteSSHCommand(targetNodeIP, checkCmd, hypervisorConfig, hypervisorKnownHostsPath, targetNodeKnownHostsPath)

		if err != nil {
			klog.V(4).Infof("Error checking for %s on %s: %v", revisionFile, targetNodeIP, err)
			return false, nil // Continue polling on SSH errors
		}

		// Check if the file exists based on command output
		if strings.TrimSpace(output) == "exists" {
			klog.V(2).Infof("File %s now exists on node %s", revisionFile, targetNodeIP)
			return true, nil
		}

		klog.V(4).Infof("File %s does not exist yet on node %s", revisionFile, targetNodeIP)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("%s to be created on %s", revisionFile, targetNodeIP))
}

// DeleteJob deletes a Kubernetes job from a specified namespace.
//
//	err := DeleteJob("tnf-auth-job-master-0", "openshift-etcd", oc)
func DeleteJob(jobName, namespace string, oc *exutil.CLI) error {
	err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Delete(context.Background(), jobName, metav1.DeleteOptions{})
	if err != nil {
		return core.WrapError("delete job", fmt.Sprintf("%s in namespace %s", jobName, namespace), err)
	}
	klog.V(2).Infof("Deleted job %s in namespace %s", jobName, namespace)
	return nil
}

// DeleteAuthJob deletes the TNF authentication job for a node from openshift-etcd namespace.
//
//	err := DeleteAuthJob("tnf-auth-job-master-0", oc)
func DeleteAuthJob(authJobName string, oc *exutil.CLI) error {
	return DeleteJob(authJobName, etcdNamespace, oc)
}

// DeleteAfterSetupJob deletes the TNF after-setup job for a node from openshift-etcd namespace.
//
//	err := DeleteAfterSetupJob("tnf-after-setup-job-master-0", oc)
func DeleteAfterSetupJob(afterSetupJobName string, oc *exutil.CLI) error {
	return DeleteJob(afterSetupJobName, etcdNamespace, oc)
}

// WaitForJobCompletion waits for a Kubernetes job to complete by polling status until Complete or Failed.
//
//	err := WaitForJobCompletion("tnf-auth-job-master-0", "openshift-etcd", 5*time.Minute, 10*time.Second, oc)
func WaitForJobCompletion(jobName, namespace string, timeout, pollInterval time.Duration, oc *exutil.CLI) error {
	klog.V(2).Infof("Waiting for job %s in namespace %s to complete (timeout: %v)", jobName, namespace, timeout)

	return core.PollUntil(func() (bool, error) {
		// Get job status using client API
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			klog.V(4).Infof("Job %s not found yet, waiting...", jobName)
			return false, nil // Job doesn't exist yet, continue polling
		}

		// Check if job has conditions
		if len(job.Status.Conditions) == 0 {
			klog.V(4).Infof("Job %s has no conditions yet, waiting...", jobName)
			return false, nil // No conditions yet, continue polling
		}

		// Check each condition
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				klog.V(2).Infof("Job %s completed successfully", jobName)
				return true, nil // Job completed, stop polling
			}

			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				// Return error to stop polling - job failed permanently
				return false, core.NewError(fmt.Sprintf("job %s", jobName), fmt.Sprintf("failed: %s - %s", cond.Reason, cond.Message))
			}
		}

		klog.V(4).Infof("Job %s still running...", jobName)
		return false, nil // Job still running, continue polling
	}, timeout, pollInterval, fmt.Sprintf("job %s completion", jobName))
}
