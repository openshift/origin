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
	// Confirmed against a TNF 5.0 run; WaitForPacemakerFencingRejection treats a pod that completes
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

// fencingOutcome is how a single fencing-job pod is classified by WaitForPacemakerFencingRejection.
type fencingOutcome int

const (
	// fencingInconclusive: the pod proves neither rejection nor acceptance; keep waiting for a retry pod.
	fencingInconclusive fencingOutcome = iota
	// fencingRejected: the pod refused the invalid credentials (the result we want).
	fencingRejected
	// fencingAccepted: the pod completed successfully, i.e. the invalid credentials were accepted (fail).
	fencingAccepted
)

// WaitForPacemakerFencingRejection confirms the cluster rejects an invalid fencing-credentials update by
// FOLLOWING the validation pod's logs in real time instead of sampling the Job status on an interval.
//
// CEO hashes the fencing-credentials-<node> Secret resourceVersions into the tnf-fencing-job spec, so a
// Secret change makes CEO recreate tnf-fencing-job to validate the new credentials via fence_redfish. The
// job retries on failure and its pods are short-lived, so a periodic Job-status check can land in the gap
// between a pod failing and the controller's next retry and miss the failure entirely. Following each
// post-update pod's logs closes that gap: a log containing fencingCredentialRejectionLogSignal proves the
// credentials were refused (the live stonith config is left untouched); a pod that instead completes
// successfully means the invalid credentials were accepted — a test failure. Pods are discovered with a
// list+watch so neither a pod that already exists when we start nor a later retry pod is missed.
func WaitForPacemakerFencingRejection(oc *exutil.CLI, namespace, nodeName string, minCreationTime time.Time, timeout time.Duration) error {
	rejectionSignal := fmt.Sprintf(fencingCredentialRejectionLogSignalFmt, strings.Split(nodeName, ".")[0])
	e2e.Logf("Following fencing-job pods (created after %v) to confirm node %s's invalid credentials are rejected (signal: %q, timeout: %v)", minCreationTime.UTC(), nodeName, rejectionSignal, timeout)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	defer services.DumpJobPodLogs(PacemakerFencingValidationJobName, namespace, oc)

	podClient := oc.AdminKubeClient().CoreV1().Pods(namespace)
	selector := "job-name=" + PacemakerFencingValidationJobName

	// List first to get a resourceVersion, then watch from it: this covers a pod created between the
	// Secret update and this call, as well as every retry pod that follows.
	list, err := podClient.List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return fmt.Errorf("list fencing-job pods in %s: %w", namespace, err)
	}

	inspected := map[string]bool{}
	var lastInconclusive error

	// handle classifies one pod and reports whether to stop. A non-nil error with stop=true is a hard
	// failure (credentials accepted); stop=false records the inconclusive reason and keeps waiting.
	handle := func(pod *corev1.Pod) (bool, error) {
		switch outcome, reason := classifyFencingPod(ctx, podClient, pod, minCreationTime, rejectionSignal, inspected); outcome {
		case fencingRejected:
			return true, nil
		case fencingAccepted:
			return true, reason
		default:
			if reason != nil {
				lastInconclusive = reason
				e2e.Logf("%v; waiting for a retry pod", reason)
			}
			return false, nil
		}
	}

	for i := range list.Items {
		if stop, err := handle(&list.Items[i]); stop {
			return err
		}
	}

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

	for {
		select {
		case <-ctx.Done():
			return fencingRejectionTimeout(lastInconclusive, ctx.Err())
		case event, ok := <-rw.ResultChan():
			if !ok {
				return fencingRejectionTimeout(lastInconclusive, ctx.Err())
			}
			// ADDED delivers new pods; MODIFIED re-delivers a pod once its container has started so we
			// can follow logs even if we first saw it while still Pending.
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

// classifyFencingPod follows a single post-update fencing-job pod's logs and decides what they prove. It
// skips pods already inspected, created before the update, or not yet started (a later MODIFIED event
// re-delivers those). It returns a reason for accepted/inconclusive outcomes and nil for a rejection.
func classifyFencingPod(ctx context.Context, podClient typedcorev1.PodInterface, pod *corev1.Pod, minCreationTime time.Time, rejectionSignal string, inspected map[string]bool) (fencingOutcome, error) {
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
		e2e.Logf("Fencing-job pod %s refused the invalid credentials (logged %q)", pod.Name, rejectionSignal)
		return fencingRejected, nil
	}

	// No rejection signal: a clean completion means the invalid credentials were accepted (a hard
	// failure); any other end is unrelated, so let the job retry.
	phase := pod.Status.Phase
	if final, getErr := podClient.Get(ctx, pod.Name, metav1.GetOptions{}); getErr == nil {
		phase = final.Status.Phase
	}
	if phase == corev1.PodSucceeded {
		return fencingAccepted, fmt.Errorf("fencing-job pod %s completed successfully without logging %q — the invalid fencing credentials were accepted", pod.Name, rejectionSignal)
	}
	return fencingInconclusive, fmt.Errorf("fencing-job pod %s ended without the credential-rejection signal (phase %s)", pod.Name, phase)
}

// followFencingPodLogs follows a pod's logs to EOF (the stream closes when the container exits) and
// returns the captured output. An error means the logs could not be read, which the caller treats as
// inconclusive rather than inferring a result from missing evidence.
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

// fencingRejectionTimeout builds the error returned when the watch ends without observing a rejection,
// preferring the last inconclusive observation (more actionable) over the bare context error.
func fencingRejectionTimeout(lastInconclusive, ctxErr error) error {
	if lastInconclusive != nil {
		return fmt.Errorf("stopped waiting for a fencing-job pod to reject the invalid credentials; the last observation was inconclusive: %w", lastInconclusive)
	}
	return fmt.Errorf("timed out waiting for a fencing-job pod to reject the invalid credentials: %w", ctxErr)
}
