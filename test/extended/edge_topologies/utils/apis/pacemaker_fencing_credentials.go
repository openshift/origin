// Package apis provides utilities for interacting with OpenShift API resources.
//
// This file manages Pacemaker fencing secrets in openshift-etcd (fencing-credentials-*).
// Note: BareMetal Host BMC credentials (in openshift-machine-api) are separate and
// managed in baremetalhost.go for Baremetal Operator node provisioning.
package apis

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
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

	// fencingCredentialRejectionLogSignalFmt is the node-scoped substring the TNF fencing validation pod
	// logs when fence_redfish cannot authenticate for a node. cluster-etcd-operator's tnf-setup-runner
	// emits "failed to verify fencing credentials for node <node>" (main.go) just before it exits
	// non-zero. Matching it for the SPECIFIC target node proves the job refused this node's invalid
	// credentials before touching the live stonith config, and guards against the wrong node being
	// validated. The "%s" is the node's short name.
	//
	// Confirmed against a TNF 5.0 run; WaitForPacemakerFencingResult treats a pod that completes
	// successfully without this substring as "credentials accepted" (a test failure) and anything else as
	// inconclusive, so a wording mismatch surfaces as a timeout, never a false pass.
	fencingCredentialRejectionLogSignalFmt = "failed to verify fencing credentials for node %s"
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

// FencingValidationResult is the expected outcome of a fencing credential validation.
type FencingValidationResult string

const (
	// FencingRejected means the credentials should be rejected (invalid credentials).
	FencingRejected FencingValidationResult = "rejected"
	// FencingAccepted means the credentials should be accepted (valid credentials).
	FencingAccepted FencingValidationResult = "accepted"
)

// fencingOutcome is how a single fencing-job pod is classified during validation.
type fencingOutcome int

const (
	// fencingInconclusive: the pod proves neither rejection nor acceptance; keep waiting for a retry pod.
	fencingInconclusive fencingOutcome = iota
	// fencingRejected: the pod refused the credentials.
	fencingRejected
	// fencingAccepted: the pod completed successfully, i.e. the credentials were accepted.
	fencingAccepted
)

// WaitForPacemakerFencingResult confirms the cluster validates a fencing-credentials update by
// following the validation pod's logs in real time instead of sampling the Job status on an interval.
//
// desiredResult specifies the expected outcome:
//   - FencingRejected: credentials should be rejected (invalid credentials, live stonith untouched)
//   - FencingAccepted: credentials should be accepted (valid credentials, live stonith updated)

func WaitForPacemakerFencingResult(oc *exutil.CLI, namespace, nodeName string, desiredResult FencingValidationResult, minCreationTime time.Time, timeout time.Duration) error {
	if desiredResult != FencingRejected && desiredResult != FencingAccepted {
		return fmt.Errorf("unsupported FencingValidationResult: %s (expected %s or %s)", desiredResult, FencingRejected, FencingAccepted)
	}

	rejectionSignal := fmt.Sprintf(fencingCredentialRejectionLogSignalFmt, strings.Split(nodeName, ".")[0])
	expectedDesc := "accepted"
	if desiredResult == FencingRejected {
		expectedDesc = "rejected"
	}
	e2e.Logf("Following fencing-job pods (created after %v) to confirm node %s's credentials are %s (signal: %q, timeout: %v)", minCreationTime.UTC(), nodeName, expectedDesc, rejectionSignal, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer services.DumpJobPodLogs(PacemakerFencingValidationJobName, namespace, oc)

	podClient := oc.AdminKubeClient().CoreV1().Pods(namespace)
	selector := "job-name=" + PacemakerFencingValidationJobName

	//Step 1: Get an initial list of pods. The resourceVersion from this list is the "starting point" for the watch. 
	// Any pods created after this point will be observed by the watch.
	list, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("list fencing-job pods in %s: %w", namespace, err)
	}

	inspected := map[string]bool{}
	var lastInconclusive error

	handle := func(pod *corev1.Pod) (bool, error) {
		switch outcome, reason := classifyFencingPodForResult(ctx, podClient, pod, minCreationTime, rejectionSignal, inspected); outcome {
		case fencingRejected:
			if desiredResult == FencingRejected {
				return true, nil // desired outcome
			}
			return true, fmt.Errorf("expected credentials accepted but they were rejected for node %s", nodeName)
		case fencingAccepted:
			if desiredResult == FencingAccepted {
				return true, nil // desired outcome
			}
			return true, reason 
		default:
			if reason != nil {
				lastInconclusive = reason
				e2e.Logf("%v; waiting for a retry pod", reason)
			}
			return false, nil
		}
	}

	// Step 2: Check all existing pods first. If any of them show the desired result, we're done.
	for i := range list.Items {
		if stop, err := handle(&list.Items[i]); stop {
			return err
		}
	}

	// Step 3: Watch for new pods and follow their logs until we see the desired result or timeout.
	watcherClient := &cache.ListWatch{
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.LabelSelector = selector
			return podClient.Watch(ctx, options)
		},
	}
	rw, err := watchtools.NewRetryWatcherWithContext(ctx, list.ResourceVersion, cache.ToWatcherWithContext(watcherClient))
	if err != nil {
		return fmt.Errorf("start fencing-job pod watch from resourceVersion %s: %w", list.ResourceVersion, err)
	}
	defer rw.Stop()

	// Step 4: Wait for new/updated pods and check each one until we find the desired result or timeout.
	for {
		select {
		case <-ctx.Done():
			return fencingValidationTimeout(expectedDesc, lastInconclusive, ctx.Err())
		case event, ok := <-rw.ResultChan():
			if !ok {
				return fencingValidationTimeout(expectedDesc, lastInconclusive, ctx.Err())
			}
			
			if event.Type != watch.Added && event.Type != watch.Modified {
				continue
			}
			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}
			if stop, err := handle(pod); stop {
				return err
			}
		}
	}
}

// classifyFencingPodForResult follows a single post-update fencing-job pod's logs and classifies the
// outcome. It skips pods already inspected, created before the update, or not yet started (a later MODIFIED
// event re-delivers those). Callers interpret the outcome in context of their desired result.
func classifyFencingPodForResult(ctx context.Context, podClient typedcorev1.PodInterface, pod *corev1.Pod, minCreationTime time.Time, rejectionSignal string, inspected map[string]bool) (fencingOutcome, error) {
	if pod == nil || inspected[pod.Name] || !pod.CreationTimestamp.Time.After(minCreationTime) {
		return fencingInconclusive, nil
	}
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodUnknown {
		return fencingInconclusive, nil
	}
	inspected[pod.Name] = true
	e2e.Logf("Following logs for fencing-job pod %s (phase %s)", pod.Name, pod.Status.Phase)

	logOutput, err := followFencingPodLogs(ctx, podClient, pod.Name)
	if err != nil {
		return fencingInconclusive, fmt.Errorf("could not read fencing-job pod %s logs (likely GC'd by a job retry): %w", pod.Name, err)
	}
	if strings.Contains(logOutput, rejectionSignal) {
		e2e.Logf("Fencing-job pod %s logged the credential-rejection signal (logged %q)", pod.Name, rejectionSignal)
		return fencingRejected, nil
	}

	phase := pod.Status.Phase
	if final, getErr := podClient.Get(ctx, pod.Name, metav1.GetOptions{}); getErr == nil {
		phase = final.Status.Phase
	}
	if phase == corev1.PodSucceeded {
		return fencingAccepted, fmt.Errorf("fencing-job pod %s completed successfully without logging %q", pod.Name, rejectionSignal)
	}
	return fencingInconclusive, fmt.Errorf("fencing-job pod %s ended without the credential-rejection signal (phase %s)", pod.Name, phase)
}

// followFencingPodLogs streams the logs of a fencing-job pod until it completes or the context is canceled.
func followFencingPodLogs(ctx context.Context, podClient typedcorev1.PodInterface, podName string) (string, error) {
	stream, err := podClient.GetLogs(podName, &corev1.PodLogOptions{Follow: true}).Stream(ctx)
	if err != nil {
		return "", fmt.Errorf("following logs for fencing-job pod %s: %w", podName, err)
	}
	defer func() { _ = stream.Close() }()

	data, err := io.ReadAll(stream)
	if err != nil {
		return "", fmt.Errorf("reading logs for fencing-job pod %s: %w", podName, err)
	}
	return string(data), nil
}

func fencingValidationTimeout(expectedDesc string, lastInconclusive, ctxErr error) error {
	if lastInconclusive != nil {
		return fmt.Errorf("stopped waiting for a fencing-job pod to have credentials %s; the last observation was inconclusive: %w", expectedDesc, lastInconclusive)
	}
	return fmt.Errorf("timed out waiting for a fencing-job pod to have credentials %s: %w", expectedDesc, ctxErr)
}
