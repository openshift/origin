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
	"k8s.io/client-go/rest"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	replicaDiagnosticsTimeout = 20 * time.Second
	replicaRequestTimeout     = 5 * time.Second
	replicaExecTimeout        = 8 * time.Second
	replicaMaxSlices          = 16
	replicaMaxEndpoints       = 64
	replicaMaxContainers      = 4
	replicaMaxOutput          = 4096
	replicaMaxStateBytes      = 32768
	replicaMaxStateRows       = 256
	replicaMaxServers         = 16

	// replicaHAProxySocketPath is the router container's HAProxy admin socket. The same
	// path and socat access form are already relied on by the dynamic configuration
	// manager coverage in config_manager_ingress.go.
	replicaHAProxySocketPath = "/var/lib/haproxy/run/haproxy.sock"
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

	// names holds the expected backend pod names in the order the test adds them, so a
	// pod is only ever referred to by its synthetic ordinal (its index here). responses
	// counts how many replies each ordinal served, which separates "server is absent or
	// unhealthy" from "server exists and is healthy but never receives traffic".
	names     []string
	responses []int
}

// ordinalOf maps a backend response to the synthetic ordinal of the expected pod that
// produced it, returning -1 when the response matches no expected pod.
func (stats *replicaRequestStats) ordinalOf(name string) int {
	for ordinal, expected := range stats.names {
		if expected == name {
			return ordinal
		}
	}
	return -1
}

func (stats *replicaRequestStats) observeBackend(output string, expected, observed sets.Set[string]) (bool, error) {
	if !expected.Has(output) {
		stats.unexpected++
		return false, fmt.Errorf("unexpected backend %q", output)
	}
	if ordinal := stats.ordinalOf(output); ordinal >= 0 {
		if len(stats.responses) != len(stats.names) {
			stats.responses = make([]int, len(stats.names))
		}
		stats.responses[ordinal]++
	}
	observed.Insert(output)
	stats.observed = observed.Len()
	return observed.Equal(expected), nil
}

func (stats *replicaRequestStats) responsesFor(ordinal int) int {
	if ordinal < 0 || ordinal >= len(stats.responses) {
		return 0
	}
	return stats.responses[ordinal]
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

	// addresses maps each endpoint address to the name of the pod backing it. It lets the
	// HAProxy collector recognize a server that identifies its endpoint only by address.
	// It is never emitted.
	addresses map[string]string
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
	counts := replicaEndpointCounts{addresses: map[string]string{}}
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
			if endpoint.TargetRef != nil && endpoint.TargetRef.Name != "" {
				for _, address := range endpoint.Addresses {
					counts.addresses[address] = endpoint.TargetRef.Name
				}
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

// Operational state of a backend server as reported by HAProxy's runtime API. Values are
// bucketed into fixed categories so that no runtime text reaches the diagnostic output.
type replicaServerOp int

const (
	replicaOpAbsent replicaServerOp = iota
	replicaOpDown
	replicaOpStarting
	replicaOpUp
	replicaOpStopping
	replicaOpUnknown
	replicaOpCount
)

var replicaServerOpNames = [replicaOpCount]string{"absent", "down", "starting", "up", "stopping", "unknown"}

// Administrative state of a backend server. Maintenance dominates drain because HAProxy
// reports a server held in both as unavailable rather than merely draining.
type replicaServerAdmin int

const (
	replicaAdminAbsent replicaServerAdmin = iota
	replicaAdminNone
	replicaAdminMaint
	replicaAdminDrain
	replicaAdminUnknown
	replicaAdminCount
)

var replicaServerAdminNames = [replicaAdminCount]string{"absent", "none", "maint", "drain", "unknown"}

// Result of the most recent health check for a backend server.
type replicaServerCheck int

const (
	replicaCheckUnknown replicaServerCheck = iota
	replicaCheckNeutral
	replicaCheckFailed
	replicaCheckPassed
	replicaCheckInvalid
	replicaCheckCount
)

var replicaServerCheckNames = [replicaCheckCount]string{"unknown", "neutral", "failed", "passed", "invalid"}

// HAProxy "show servers state" administrative state bits. Maintenance may be forced,
// inherited, configured, resolution-driven or set by health checks; drain may be forced
// or inherited.
const (
	replicaAdminMaintBits = 0x01 | 0x02 | 0x04 | 0x20 | 0x40
	replicaAdminDrainBits = 0x08 | 0x10
)

// Fields of "show servers state" this collector needs. They are located by name from the
// commented header row rather than by position, so a column order or arity change in
// HAProxy is reported as incomplete instead of silently misparsed.
var replicaStateFields = []string{"be_name", "srv_name", "srv_addr", "srv_op_state", "srv_admin_state", "srv_uweight", "srv_check_result"}

type replicaServerState struct {
	present, weightKnown, weightZero, duplicate bool
	op                                          replicaServerOp
	admin                                       replicaServerAdmin
	check                                       replicaServerCheck
	responses                                   int
}

type replicaBackendState struct {
	collected, backendFound, incomplete, truncated bool
	extraCapped, serversCapped, failed             bool
	servers                                        []replicaServerState
	extra, missing, lines                          int
	failure                                        replicaFailure
}

// replicaRouterExec reads the router container's HAProxy admin socket. It is a variable
// so that parser and bounding coverage can exercise the collector without a cluster.
//
// ExecInPodWithResult streams over SPDY into a buffer. The e2e framework's kubectl
// helpers are deliberately not used here: ExecWithFullOutput unconditionally logs the
// command and the whole of stdout and stderr through the same logger these diagnostics
// emit to, which would publish the raw server table this collector exists to summarize.
var replicaRouterExec = func(config *rest.Config, client kubernetes.Interface, namespace string) (string, error) {
	return exutil.ExecInPodWithResult(client.CoreV1(), config, namespace, configManagerRouterPodName, configManagerRouterContainerName, replicaServerStateCommand())
}

// replicaServerStateCommand reads HAProxy's runtime server table over the admin socket.
// The byte cap is applied in the router container so an unexpectedly large table cannot
// be streamed back in full, and stderr is discarded so no diagnostic text from the
// command can reach the log through the executor's error path.
func replicaServerStateCommand() []string {
	return []string{"/bin/sh", "-c", fmt.Sprintf("echo 'show servers state' | socat - %s 2>/dev/null | head -c %d", replicaHAProxySocketPath, replicaMaxStateBytes)}
}

// execReplicaServerState applies a per-exec deadline on top of the shared diagnostics
// budget. RunHostCmd has no context-aware form, so the deadline is enforced here and a
// late reply is discarded through the buffered channel.
func execReplicaServerState(ctx context.Context, config *rest.Config, client kubernetes.Interface, namespace string) (string, error) {
	execCtx, cancel := context.WithTimeout(ctx, replicaExecTimeout)
	defer cancel()
	type execResult struct {
		output string
		err    error
	}
	// Resolve the hook before starting the goroutine so a reply that arrives after the
	// deadline cannot observe a later value.
	runCommand := replicaRouterExec
	results := make(chan execResult, 1)
	go func() {
		output, err := runCommand(config, client, namespace)
		results <- execResult{output: output, err: err}
	}()
	select {
	case <-execCtx.Done():
		return "", execCtx.Err()
	case result := <-results:
		return result.output, result.err
	}
}

type replicaStateRow struct {
	backend string
	ordinal int
	server  replicaServerState
}

func replicaParseOp(value string) replicaServerOp {
	switch value {
	case "0":
		return replicaOpDown
	case "1":
		return replicaOpStarting
	case "2":
		return replicaOpUp
	case "3":
		return replicaOpStopping
	default:
		return replicaOpUnknown
	}
}

func replicaParseAdmin(value string) replicaServerAdmin {
	bits, err := strconv.Atoi(value)
	switch {
	case err != nil || bits < 0:
		return replicaAdminUnknown
	case bits&replicaAdminMaintBits != 0:
		return replicaAdminMaint
	case bits&replicaAdminDrainBits != 0:
		return replicaAdminDrain
	case bits == 0:
		return replicaAdminNone
	default:
		return replicaAdminUnknown
	}
}

func replicaParseCheck(value string) replicaServerCheck {
	switch value {
	case "1":
		return replicaCheckNeutral
	case "2":
		return replicaCheckFailed
	case "3", "4":
		return replicaCheckPassed
	case "0":
		return replicaCheckUnknown
	default:
		// A value HAProxy does not define must not read as its "unknown" result.
		return replicaCheckInvalid
	}
}

// replicaMatchOrdinal maps an HAProxy server to the synthetic ordinal of an expected pod.
//
// Server names are structured as colon-separated fields, so each field is compared whole
// rather than by substring, which would otherwise confuse ordinals that share a prefix.
// Name matching alone is not enough: a server occupying a pre-provisioned dynamic slot
// keeps the slot's name and identifies its endpoint only by address, so the endpoint
// addresses already gathered for the service are consulted as well. Addresses are used
// for matching only and never reach the emitted output.
func replicaMatchOrdinal(serverName, serverAddress string, names []string, addresses map[string]int) int {
	for _, field := range strings.Split(serverName, ":") {
		for ordinal, name := range names {
			if name != "" && field == name {
				return ordinal
			}
		}
	}
	if ordinal, matched := addresses[serverAddress]; matched {
		return ordinal
	}
	return -1
}

// parseReplicaServerState turns a bounded "show servers state" dump into per-ordinal
// categories. The target backend is identified as the one serving the expected pods, so
// the route's backend name never has to be reconstructed and unrelated backends are
// neither attributed nor reported.
func parseReplicaServerState(output string, names []string, addresses map[string]int) replicaBackendState {
	state := replicaBackendState{collected: true, servers: make([]replicaServerState, len(names))}
	state.truncated = len(output) >= replicaMaxStateBytes
	indexes := map[string]int{}
	rows := make([]replicaStateRow, 0, len(names))
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if state.lines >= replicaMaxStateRows {
			state.truncated = true
			break
		}
		state.lines++
		if strings.HasPrefix(line, "#") {
			if len(indexes) > 0 {
				// A second header would leave later rows reading the first header's
				// offsets, so stop rather than misattribute them.
				state.incomplete = true
				break
			}
			for index, field := range strings.Fields(strings.TrimPrefix(line, "#")) {
				indexes[field] = index
			}
			continue
		}
		if len(indexes) == 0 {
			// Rows ahead of the header (such as the version line) carry no field names.
			continue
		}
		fields := strings.Fields(line)
		required := true
		for _, field := range replicaStateFields {
			index, known := indexes[field]
			if !known || index >= len(fields) {
				required = false
				break
			}
		}
		if !required {
			// A missing column or a row cut short by the byte cap cannot be attributed.
			state.incomplete = true
			continue
		}
		weight, weightKnown := replicaParseWeight(fields[indexes["srv_uweight"]])
		rows = append(rows, replicaStateRow{
			backend: fields[indexes["be_name"]],
			ordinal: replicaMatchOrdinal(fields[indexes["srv_name"]], fields[indexes["srv_addr"]], names, addresses),
			server: replicaServerState{
				present:     true,
				op:          replicaParseOp(fields[indexes["srv_op_state"]]),
				admin:       replicaParseAdmin(fields[indexes["srv_admin_state"]]),
				check:       replicaParseCheck(fields[indexes["srv_check_result"]]),
				weightKnown: weightKnown,
				weightZero:  weightKnown && weight == 0,
			},
		})
	}
	if len(indexes) == 0 {
		state.incomplete = true
		state.missing = len(names)
		return state
	}
	// First pass: pick the backend under test. The same endpoint pod is routinely a server
	// in several backends, so the one covering the most expected ordinals is chosen rather
	// than the first match. Rows are visited in input order and the map only counts, so
	// the backend that first reaches the highest coverage wins deterministically.
	backend, matched := "", 0
	coverage := map[string]sets.Set[int]{}
	for _, row := range rows {
		if row.ordinal < 0 {
			continue
		}
		if coverage[row.backend] == nil {
			coverage[row.backend] = sets.New[int]()
		}
		coverage[row.backend].Insert(row.ordinal)
		if count := coverage[row.backend].Len(); count > matched {
			backend, matched = row.backend, count
		}
	}
	if backend == "" {
		state.incomplete = true
		state.missing = len(names)
		return state
	}
	state.backendFound = true
	// Second pass: attribute only rows belonging to that backend.
	for _, row := range rows {
		if row.backend != backend {
			continue
		}
		if row.ordinal < 0 {
			if state.extra < replicaMaxServers {
				state.extra++
			} else {
				state.extraCapped = true
			}
			continue
		}
		if state.servers[row.ordinal].present {
			// The conflicting row is dropped, so the retained one may not be the whole story.
			state.servers[row.ordinal].duplicate = true
			state.incomplete = true
			continue
		}
		state.servers[row.ordinal] = row.server
	}
	for _, server := range state.servers {
		if !server.present {
			state.missing++
		} else if !server.weightKnown || server.op == replicaOpUnknown || server.admin == replicaAdminUnknown || server.check == replicaCheckInvalid {
			state.incomplete = true
		}
	}
	state.incomplete = state.incomplete || state.truncated || state.extraCapped || state.missing > 0
	return state
}

// replicaParseWeight reads a server's user-assigned weight, reporting whether HAProxy
// supplied a value this collector understands.
func replicaParseWeight(value string) (int, bool) {
	weight, err := strconv.Atoi(value)
	if err != nil || weight < 0 {
		return 0, false
	}
	return weight, true
}

// replicaAddressOrdinals reduces the endpoint addresses to the synthetic ordinals the
// diagnostics report, discarding any address that does not back an expected pod.
func replicaAddressOrdinals(stats replicaRequestStats, endpoints replicaEndpointCounts) map[string]int {
	ordinals := map[string]int{}
	for address, pod := range endpoints.addresses {
		if ordinal := stats.ordinalOf(pod); ordinal >= 0 {
			ordinals[address] = ordinal
		}
	}
	return ordinals
}

// collectReplicaServerState reads HAProxy's runtime view of the backend under test. It is
// best effort: every failure is reduced to an allowlisted category and reported as
// uncollected rather than surfaced to the caller.
func collectReplicaServerState(ctx context.Context, config *rest.Config, client kubernetes.Interface, namespace string, stats replicaRequestStats, endpoints replicaEndpointCounts) replicaBackendState {
	if len(stats.names) == 0 || config == nil {
		return replicaBackendState{}
	}
	output, err := execReplicaServerState(ctx, config, client, namespace)
	if err != nil {
		return replicaBackendState{
			collected:  true,
			incomplete: true,
			failed:     true,
			failure:    classifyReplicaError(err),
			servers:    make([]replicaServerState, len(stats.names)),
			missing:    len(stats.names),
		}
	}
	if len(output) > replicaMaxStateBytes {
		// Defend the parser's input bound even if the in-container cap did not apply.
		output = output[:replicaMaxStateBytes]
	}
	state := parseReplicaServerState(output, stats.names, replicaAddressOrdinals(stats, endpoints))
	for ordinal := range state.servers {
		state.servers[ordinal].responses = stats.responsesFor(ordinal)
	}
	return state
}

func reportReplicaFailure(ctx context.Context, failure error, stats replicaRequestStats, client kubernetes.Interface, config *rest.Config, namespace, service, execName string, emit func(string)) {
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
	backend := collectReplicaServerState(ctx, config, client, namespace, stats, endpoints)
	if backend.collected {
		if len(backend.servers) > replicaMaxServers {
			backend.serversCapped, backend.incomplete = true, true
		}
		fmt.Fprintf(&output, "haproxy backend_found=%t servers=%d extra=%d missing=%d lines=%d incomplete=%t truncated=%t extra_capped=%t servers_capped=%t collect_failed=%t error=%s\n", backend.backendFound, len(backend.servers), backend.extra, backend.missing, backend.lines, backend.incomplete, backend.truncated, backend.extraCapped, backend.serversCapped, backend.failed, replicaErrorName(backend.failed, backend.failure))
		for ordinal, server := range backend.servers {
			if ordinal >= replicaMaxServers {
				break
			}
			fmt.Fprintf(&output, "haproxy_server ordinal=%d present=%t op=%s admin=%s check=%s weight_known=%t weight_zero=%t duplicate=%t responses=%d\n", ordinal, server.present, replicaServerOpNames[server.op], replicaServerAdminNames[server.admin], replicaServerCheckNames[server.check], server.weightKnown, server.weightZero, server.duplicate, server.responses)
		}
	}
	fmt.Fprintf(&output, "incomplete=%t output_truncated=false\n", endpoints.incomplete || router.incomplete || execPod.incomplete || backend.incomplete || ctx.Err() != nil)
	emit(boundReplicaOutput(output.String()))
}

// replicaErrorName keeps the zero-valued failure category out of a line that did not fail.
func replicaErrorName(failed bool, failure replicaFailure) string {
	if !failed {
		return "none"
	}
	return replicaFailureNames[failure]
}

func boundReplicaOutput(output string) string {
	if len(output) <= replicaMaxOutput {
		return output
	}
	const suffix = "\nincomplete=true output_truncated=true\n"
	return output[:replicaMaxOutput-len(suffix)] + suffix
}
