package router

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
)

const (
	replicaDiagnosticsTimeout = 20 * time.Second
	replicaRequestTimeout     = 5 * time.Second
	replicaMaxSlices          = 16
	replicaMaxEndpoints       = 64
	replicaMaxContainers      = 4
	replicaMaxOutput          = 4096
)

type replicaFailure int

const (
	replicaOther replicaFailure = iota
	replicaTimeout
	replicaCanceled
	replicaExec
	replicaParse
	replicaForbidden
	replicaNotFound
	replicaHTTPZero
	replicaHTTP503
	replicaHTTP4xx
	replicaHTTP5xx
	replicaHTTPOther
	replicaFailureCount
)

var replicaFailureNames = [replicaFailureCount]string{
	"other", "timeout", "canceled", "exec", "parse", "forbidden", "not_found",
	"http_zero", "http_503", "http_4xx", "http_5xx", "http_other",
}

func classifyReplicaError(err error) replicaFailure {
	var timeout net.Error
	var parse *strconv.NumError
	var exit interface{ ExitStatus() int }
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return replicaTimeout
	case apierrors.IsTimeout(err), apierrors.IsServerTimeout(err):
		return replicaTimeout
	case errors.Is(err, context.Canceled):
		return replicaCanceled
	case errors.As(err, &timeout) && timeout.Timeout():
		return replicaTimeout
	case errors.As(err, &exit):
		return replicaExec
	case errors.As(err, &parse):
		return replicaParse
	case apierrors.IsForbidden(err), apierrors.IsUnauthorized(err):
		return replicaForbidden
	case apierrors.IsNotFound(err):
		return replicaNotFound
	default:
		return replicaOther
	}
}

type replicaRequestStats struct {
	iteration, expected, observed, attempts, successes, unexpected int
	failures                                                       [replicaFailureCount]int
}

func (stats *replicaRequestStats) observeBackend(output string, expected, observed sets.Set[string]) (bool, error) {
	if !expected.Has(output) {
		stats.unexpected++
		return false, fmt.Errorf("unexpected backend %q", output)
	}
	observed.Insert(output)
	stats.observed = observed.Len()
	return observed.Equal(expected), nil
}

func (stats *replicaRequestStats) record(code int, err error) {
	stats.attempts++
	if err != nil {
		stats.failures[classifyReplicaError(err)]++
		return
	}
	switch {
	case code == 0:
		stats.failures[replicaHTTPZero]++
	case code == 503:
		stats.failures[replicaHTTP503]++
	case code >= 400 && code < 500:
		stats.failures[replicaHTTP4xx]++
	case code >= 500 && code < 600:
		stats.failures[replicaHTTP5xx]++
	case code >= 600:
		stats.failures[replicaHTTPOther]++
	default:
		stats.successes++
	}
}

type replicaEndpointCounts struct {
	slices, endpoints, ready, notReady, unknown, serving, terminating int
	incomplete, truncated                                             bool
	failure                                                           replicaFailure
	failed                                                            bool
}

type replicaPodCounts struct {
	present, ready, readyKnown, deleting, incomplete, truncated bool
	pending, running, succeeded, failed, unknown                int
	containers, containersReady, restarts                       int
	containerRunning, waiting, terminated, unobserved           int
	failure                                                     replicaFailure
	lookupFailed                                                bool
}

func collectReplicaEndpoints(ctx context.Context, client kubernetes.Interface, namespace, service string) replicaEndpointCounts {
	counts := replicaEndpointCounts{}
	requestCtx, cancel := context.WithTimeout(ctx, replicaRequestTimeout)
	defer cancel()
	slices, err := client.DiscoveryV1().EndpointSlices(namespace).List(requestCtx, metav1.ListOptions{
		LabelSelector: labels.Set{discoveryv1.LabelServiceName: service}.String(),
		Limit:         replicaMaxSlices,
	})
	if err != nil {
		counts.incomplete, counts.failed, counts.failure = true, true, classifyReplicaError(err)
		return counts
	}
	counts.truncated = slices.Continue != "" || (slices.RemainingItemCount != nil && *slices.RemainingItemCount > 0)
	for sliceIndex, slice := range slices.Items {
		if sliceIndex >= replicaMaxSlices {
			counts.truncated = true
			break
		}
		if slice.Namespace != namespace || slice.Labels[discoveryv1.LabelServiceName] != service {
			counts.incomplete = true
			continue
		}
		counts.slices++
		for _, endpoint := range slice.Endpoints {
			if counts.endpoints >= replicaMaxEndpoints {
				counts.truncated = true
				break
			}
			counts.endpoints++
			switch {
			case endpoint.Conditions.Ready == nil:
				counts.unknown++
			case *endpoint.Conditions.Ready:
				counts.ready++
			default:
				counts.notReady++
			}
			if endpoint.Conditions.Serving != nil && *endpoint.Conditions.Serving {
				counts.serving++
			}
			if endpoint.Conditions.Terminating != nil && *endpoint.Conditions.Terminating {
				counts.terminating++
			}
		}
	}
	counts.incomplete = counts.incomplete || counts.truncated || counts.slices == 0 || counts.unknown > 0
	return counts
}

func collectReplicaPod(ctx context.Context, client kubernetes.Interface, namespace, name string) replicaPodCounts {
	counts := replicaPodCounts{}
	requestCtx, cancel := context.WithTimeout(ctx, replicaRequestTimeout)
	defer cancel()
	pod, err := client.CoreV1().Pods(namespace).Get(requestCtx, name, metav1.GetOptions{})
	if err != nil {
		counts.incomplete, counts.lookupFailed, counts.failure = true, true, classifyReplicaError(err)
		return counts
	}
	if pod.Namespace != namespace || pod.Name != name {
		counts.incomplete = true
		return counts
	}
	counts.present, counts.deleting = true, pod.DeletionTimestamp != nil
	switch pod.Status.Phase {
	case corev1.PodPending:
		counts.pending++
	case corev1.PodRunning:
		counts.running++
	case corev1.PodSucceeded:
		counts.succeeded++
	case corev1.PodFailed:
		counts.failed++
	default:
		counts.unknown++
	}
	for _, condition := range pod.Status.Conditions {
		if condition.Type == corev1.PodReady {
			counts.readyKnown = condition.Status == corev1.ConditionTrue || condition.Status == corev1.ConditionFalse
			counts.ready = condition.Status == corev1.ConditionTrue
		}
	}
	for _, statuses := range [][]corev1.ContainerStatus{pod.Status.InitContainerStatuses, pod.Status.ContainerStatuses} {
		for _, status := range statuses {
			if counts.containers >= replicaMaxContainers {
				counts.truncated = true
				break
			}
			counts.containers++
			if status.Ready {
				counts.containersReady++
			}
			counts.restarts += int(status.RestartCount)
			switch {
			case status.State.Running != nil:
				counts.containerRunning++
			case status.State.Waiting != nil:
				counts.waiting++
			case status.State.Terminated != nil:
				counts.terminated++
			default:
				counts.unobserved++
			}
		}
	}
	counts.incomplete = counts.truncated || !counts.readyKnown || counts.unknown > 0 || counts.unobserved > 0 || counts.containers < len(pod.Spec.Containers)+len(pod.Spec.InitContainers)
	return counts
}

func reportReplicaFailure(ctx context.Context, failure error, stats replicaRequestStats, client kubernetes.Interface, namespace, service, execName string, emit func(string)) {
	if failure == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, replicaDiagnosticsTimeout)
	defer cancel()
	endpoints := collectReplicaEndpoints(ctx, client, namespace, service)
	router := collectReplicaPod(ctx, client, namespace, configManagerRouterPodName)
	execPod := collectReplicaPod(ctx, client, namespace, execName)
	var output strings.Builder
	fmt.Fprintf(&output, "Replica diagnostics iteration=%d expected=%d observed=%d attempts=%d successes=%d unexpected=%d failure=%s\n", stats.iteration, stats.expected, stats.observed, stats.attempts, stats.successes, stats.unexpected, replicaFailureNames[classifyReplicaError(failure)])
	for category, count := range stats.failures {
		fmt.Fprintf(&output, "request_%s=%d\n", replicaFailureNames[category], count)
	}
	fmt.Fprintf(&output, "endpoints slices=%d total=%d ready=%d not_ready=%d unknown=%d serving=%d terminating=%d incomplete=%t truncated=%t lookup_failed=%t error=%s\n", endpoints.slices, endpoints.endpoints, endpoints.ready, endpoints.notReady, endpoints.unknown, endpoints.serving, endpoints.terminating, endpoints.incomplete, endpoints.truncated, endpoints.failed, replicaFailureNames[endpoints.failure])
	for _, snapshot := range []struct {
		role string
		pod  replicaPodCounts
	}{{"router", router}, {"exec", execPod}} {
		pod := snapshot.pod
		fmt.Fprintf(&output, "pod role=%s present=%t ready=%t ready_known=%t deleting=%t pending=%d running=%d succeeded=%d failed=%d unknown=%d containers=%d containers_ready=%d restarts=%d container_running=%d waiting=%d terminated=%d unobserved=%d incomplete=%t truncated=%t lookup_failed=%t error=%s\n", snapshot.role, pod.present, pod.ready, pod.readyKnown, pod.deleting, pod.pending, pod.running, pod.succeeded, pod.failed, pod.unknown, pod.containers, pod.containersReady, pod.restarts, pod.containerRunning, pod.waiting, pod.terminated, pod.unobserved, pod.incomplete, pod.truncated, pod.lookupFailed, replicaFailureNames[pod.failure])
	}
	fmt.Fprintf(&output, "incomplete=%t output_truncated=false\n", endpoints.incomplete || router.incomplete || execPod.incomplete || ctx.Err() != nil)
	emit(boundReplicaOutput(output.String()))
}

func boundReplicaOutput(output string) string {
	if len(output) <= replicaMaxOutput {
		return output
	}
	const suffix = "\nincomplete=true output_truncated=true\n"
	return output[:replicaMaxOutput-len(suffix)] + suffix
}
