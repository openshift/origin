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
	e2e "k8s.io/kubernetes/test/e2e/framework"
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
	// EtcdNamespace is the OpenShift namespace for etcd static pods and related Secrets/Jobs.
	EtcdNamespace = "openshift-etcd"

	// Label selector for TNF update-setup jobs (CEO uses app.kubernetes.io/name=tnf-update-setup-job).
	// Job names include a hash suffix (e.g. tnf-update-setup-job-master-0-637363be), so tests discover
	// the actual name by listing with this label and matching Spec.Template.Spec.NodeName.
	tnfUpdateSetupJobLabelSelector = "app.kubernetes.io/name=tnf-update-setup-job"

	// dumpJobPodLogsTimeout caps List (and avoids hanging if the API is wedged during log capture).
	dumpJobPodLogsTimeout = 30 * time.Second
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
		e2e.Logf("Detected etcd quorum error - may need operator intervention: %v", err)
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

	e2e.Logf("Waiting for CEO to create %s on node %s (timeout: %v)", revisionFile, targetNodeIP, timeout)

	return core.PollUntil(func() (bool, error) {
		// Check if the revision.json file exists using a remote SSH command
		checkCmd := fmt.Sprintf("test -f %s && echo 'exists' || echo 'not found'", revisionFile)
		output, _, err := core.ExecuteRemoteSSHCommand(targetNodeIP, checkCmd, hypervisorConfig, hypervisorKnownHostsPath, targetNodeKnownHostsPath)

		if err != nil {
			e2e.Logf("Error checking for %s on %s: %v", revisionFile, targetNodeIP, err)
			return false, nil // Continue polling on SSH errors
		}

		// Check if the file exists based on command output
		if strings.TrimSpace(output) == "exists" {
			e2e.Logf("File %s now exists on node %s", revisionFile, targetNodeIP)
			return true, nil
		}

		e2e.Logf("File %s does not exist yet on node %s", revisionFile, targetNodeIP)
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
	e2e.Logf("Deleted job %s in namespace %s", jobName, namespace)
	return nil
}

// DeleteAuthJob deletes the TNF authentication job for a node from openshift-etcd namespace.
//
//	err := DeleteAuthJob("tnf-auth-job-master-0", oc)
func DeleteAuthJob(authJobName string, oc *exutil.CLI) error {
	return DeleteJob(authJobName, EtcdNamespace, oc)
}

// DeleteAfterSetupJob deletes the TNF after-setup job for a node from openshift-etcd namespace.
//
//	err := DeleteAfterSetupJob("tnf-after-setup-job-master-0", oc)
func DeleteAfterSetupJob(afterSetupJobName string, oc *exutil.CLI) error {
	return DeleteJob(afterSetupJobName, EtcdNamespace, oc)
}

// DumpJobPodLogs lists pods for the given job (label job-name=jobName), then dumps each pod's
// container logs into the test output. Call this when a job completes so logs are captured even
// if the node is later unavailable (e.g. must-gather cannot collect from a dead node).
func DumpJobPodLogs(jobName, namespace string, oc *exutil.CLI) {
	ctx, cancel := context.WithTimeout(context.Background(), dumpJobPodLogsTimeout)
	defer cancel()
	selector := "job-name=" + jobName
	podList, err := oc.AdminKubeClient().CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		e2e.Logf("Failed to list pods for job %s: %v", jobName, err)
		return
	}
	if len(podList.Items) == 0 {
		e2e.Logf("No pods found for job %s (selector %q)", jobName, selector)
		return
	}
	e2e.Logf("Capturing container logs for job %s (%d pod(s))", jobName, len(podList.Items))
	exutil.DumpPodLogs(podList.Items, oc)
}

// WaitForJobCompletion waits for a Kubernetes job to complete by polling status until Complete or Failed.
// When the job finishes (success or failure), container logs of the job's pods are dumped into the
// test output so they are available even if the node is later unavailable.
//
//	err := WaitForJobCompletion("tnf-auth-job-master-0", "openshift-etcd", 5*time.Minute, 10*time.Second, oc)
func WaitForJobCompletion(jobName, namespace string, timeout, pollInterval time.Duration, oc *exutil.CLI) error {
	e2e.Logf("Waiting for job %s in namespace %s to complete (timeout: %v)", jobName, namespace, timeout)

	err := core.PollUntil(func() (bool, error) {
		// Get job status using client API
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(context.Background(), jobName, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Job %s not found yet, waiting...", jobName)
			return false, nil // Job doesn't exist yet, continue polling
		}

		// Check if job has conditions
		if len(job.Status.Conditions) == 0 {
			e2e.Logf("Job %s has no conditions yet, waiting...", jobName)
			return false, nil // No conditions yet, continue polling
		}

		// Check each condition
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				e2e.Logf("Job %s completed successfully", jobName)
				return true, nil // Job completed, stop polling
			}

			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				// Return error to stop polling - job failed permanently
				return false, core.NewError(fmt.Sprintf("job %s", jobName), fmt.Sprintf("failed: %s - %s", cond.Reason, cond.Message))
			}
		}

		e2e.Logf("Job %s still running...", jobName)
		return false, nil // Job still running, continue polling
	}, timeout, pollInterval, fmt.Sprintf("job %s completion", jobName))

	// Capture job pod logs as soon as the job has finished (success or failure), so they are
	// in the test output even if the node later dies or must-gather cannot collect from it.
	DumpJobPodLogs(jobName, namespace, oc)
	return err
}

// getUpdateSetupJobNameForNode returns the name of the TNF update-setup job that targets the given node,
// or "" if no such job exists. CEO creates these jobs with a hash suffix in the name, so the test discovers
// the actual name by listing jobs with the update-setup label and matching by node.
// When multiple jobs match (e.g. after node replacement), returns the newest by CreationTimestamp so we
// wait for the job created after the node was recreated.
// If minCreationTime is non-zero, only jobs with CreationTimestamp.After(minCreationTime) are considered,
// ensuring we wait for a job created after the node replacement event (e.g. node Ready time).
func getUpdateSetupJobNameForNode(oc *exutil.CLI, namespace, nodeName string, minCreationTime time.Time) (string, error) {
	list, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).List(context.Background(), metav1.ListOptions{
		LabelSelector: tnfUpdateSetupJobLabelSelector,
	})
	if err != nil {
		return "", err
	}
	var newest *batchv1.Job
	for i := range list.Items {
		job := &list.Items[i]
		if job.Spec.Template.Spec.NodeName != nodeName {
			continue
		}
		if !minCreationTime.IsZero() && !job.CreationTimestamp.Time.After(minCreationTime) {
			continue // require job created after node replacement event
		}
		if newest == nil || job.CreationTimestamp.After(newest.CreationTimestamp.Time) {
			newest = job
		}
	}
	if newest == nil {
		return "", nil
	}
	return newest.Name, nil
}

// WaitForUpdateSetupJobCompletionByNode waits for the TNF update-setup job targeting the given node to complete.
// It discovers the job by label and node name so it works regardless of CEO's job naming (e.g. hash suffix).
// If minJobCreationTime is non-zero, only a job created after that time is considered (ensures we wait for the
// job created after the node was recreated, not a stale pre-replacement job).
func WaitForUpdateSetupJobCompletionByNode(oc *exutil.CLI, namespace, nodeName string, minJobCreationTime time.Time, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for update-setup job for node %s in namespace %s to complete (timeout: %v)", nodeName, namespace, timeout)

	var resolvedJobName string
	err := core.PollUntil(func() (bool, error) {
		name, err := getUpdateSetupJobNameForNode(oc, namespace, nodeName, minJobCreationTime)
		if err != nil {
			return false, err
		}
		if name == "" {
			e2e.Logf("Update-setup job for node %s not found yet, waiting...", nodeName)
			return false, nil
		}
		resolvedJobName = name
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Job %s not found, waiting...", name)
			return false, nil
		}
		if len(job.Status.Conditions) == 0 {
			e2e.Logf("Job %s has no conditions yet, waiting...", name)
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				e2e.Logf("Update-setup job %s for node %s completed successfully", name, nodeName)
				return true, nil
			}
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				return false, core.NewError(fmt.Sprintf("job %s", name), fmt.Sprintf("failed: %s - %s", cond.Reason, cond.Message))
			}
		}
		e2e.Logf("Job %s still running...", name)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("update-setup job for node %s completion", nodeName))

	if resolvedJobName != "" {
		DumpJobPodLogs(resolvedJobName, namespace, oc)
	}
	return err
}

// WaitForSurvivorUpdateSetupJobCompletionByNode waits for the survivor's TNF update-setup job to complete in a
// run that started after minPodCreationTime. It discovers the job by label and node name (see WaitForUpdateSetupJobCompletionByNode).
func WaitForSurvivorUpdateSetupJobCompletionByNode(oc *exutil.CLI, namespace, nodeName string, minPodCreationTime time.Time, timeout, pollInterval time.Duration) error {
	e2e.Logf("Waiting for survivor update-setup job for node %s (run after %v) to complete (timeout: %v)", nodeName, minPodCreationTime.UTC(), timeout)

	var resolvedJobName string
	err := core.PollUntil(func() (bool, error) {
		name, err := getUpdateSetupJobNameForNode(oc, namespace, nodeName, time.Time{}) // survivor: any job name, we filter by pod time below
		if err != nil {
			return false, err
		}
		if name == "" {
			e2e.Logf("Survivor update-setup job for node %s not found yet, waiting...", nodeName)
			return false, nil
		}
		resolvedJobName = name
		job, err := oc.AdminKubeClient().BatchV1().Jobs(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("Job %s not found, waiting...", name)
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobFailed && cond.Status == "True" {
				return false, core.NewError(fmt.Sprintf("job %s", name), fmt.Sprintf("failed: %s - %s", cond.Reason, cond.Message))
			}
		}
		if len(job.Status.Conditions) == 0 {
			e2e.Logf("Job %s has no conditions yet, waiting...", name)
			return false, nil
		}
		for _, cond := range job.Status.Conditions {
			if cond.Type == batchv1.JobComplete && cond.Status == "True" {
				selector := "job-name=" + name
				podList, listErr := oc.AdminKubeClient().CoreV1().Pods(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector})
				if listErr != nil {
					e2e.Logf("Job %s completed but could not list pods: %v", name, listErr)
					return false, nil
				}
				for i := range podList.Items {
					if !podList.Items[i].CreationTimestamp.Time.Before(minPodCreationTime) {
						e2e.Logf("Survivor job %s completed in a run after target Ready (pod %s created at %v)", name, podList.Items[i].Name, podList.Items[i].CreationTimestamp.UTC())
						return true, nil
					}
				}
				e2e.Logf("Job %s completed but run started before replacement node was Ready; waiting for a fresh run", name)
				return false, nil
			}
		}
		e2e.Logf("Job %s still running...", name)
		return false, nil
	}, timeout, pollInterval, fmt.Sprintf("survivor update-setup job for node %s completion (run after %v)", nodeName, minPodCreationTime.UTC()))

	if resolvedJobName != "" {
		DumpJobPodLogs(resolvedJobName, namespace, oc)
	}
	return err
}
