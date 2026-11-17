package networking

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"net/url"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// bgpVIPTags is the frozen tag set of every BGP VIP management test.
	// Renaming any part of it resets the test history in Sippy.
	bgpVIPTags = `[sig-network][Jira:"Networking / On-Prem Load Balancer"][OCPFeatureGate:BGPBasedVIPManagement][Feature:BGPVIPManagement][apigroup:config.openshift.io][apigroup:machineconfiguration.openshift.io]`

	bgpVIPFRRNamespace     = "openshift-frr-k8s"
	bgpVIPFRRContainer     = "frr"
	bgpVIPFRRConfiguration = "bgp-vip"
	bgpVIPKeepalivedLabel  = "kni-infra-vrrp"
	bgpVIPInfraNamespace   = "openshift-kni-infra"
	bgpVIPPeersFilePath    = "/etc/kubernetes/static-pod-resources/frr-k8s/frr-peers.json"
	bgpVIPTorContainer     = "bgp-tor"
	bgpVIPRouterNamespace  = "openshift-ingress"
	bgpVIPRouterSelector   = "ingresscontroller.operator.openshift.io/deployment-ingresscontroller=default"

	bgpVIPPollInterval    = 10 * time.Second
	bgpVIPConvergeTimeout = 5 * time.Minute
	// bgpVIPDumpTimeout bounds the state dump of a failed test. It runs
	// between the body and the cleanup, so every test that bounds its body
	// budgets for it.
	bgpVIPDumpTimeout = 2 * time.Minute
)

var (
	bgpVIPConfigGVR     = schema.GroupVersionResource{Group: "machineconfiguration.openshift.io", Version: "v1alpha1", Resource: "bgpvipconfigs"}
	frrConfigurationGVR = schema.GroupVersionResource{Group: "frrk8s.metallb.io", Version: "v1beta1", Resource: "frrconfigurations"}
	machineConfigGVR    = schema.GroupVersionResource{Group: "machineconfiguration.openshift.io", Version: "v1", Resource: "machineconfigs"}
)

// skipUnlessBGPVIP skips the test unless the cluster is bare metal with
// BGP-based VIP management, and returns the Infrastructure object.
func skipUnlessBGPVIP(ctx context.Context, oc *exutil.CLI) *configv1.Infrastructure {
	var infra *configv1.Infrastructure
	err := bgpVIPEventually(ctx, time.Minute, "read infrastructures/cluster", func(ctx context.Context) error {
		var err error
		infra, err = oc.AdminConfigClient().ConfigV1().Infrastructures().Get(ctx, "cluster", metav1.GetOptions{})
		return err
	})
	if err != nil {
		e2e.Failf("%v", err)
	}
	ps := infra.Status.PlatformStatus
	if ps == nil || ps.Type != configv1.BareMetalPlatformType || ps.BareMetal == nil {
		e2eskipper.Skipf("BGP VIP management is only supported on the BareMetal platform")
	}
	if ps.BareMetal.VIPManagement != configv1.VIPManagementTypeBGP {
		e2eskipper.Skipf("the cluster does not use BGP-based VIP management (vipManagement=%q)", ps.BareMetal.VIPManagement)
	}
	return infra
}

// bgpVIPEventually polls fn until it returns nil. A non-nil error means "not
// yet" and is kept as the reason reported on timeout. API errors are retried
// the same way: the test client may reach the API server through the BGP VIP
// under test.
func bgpVIPEventually(ctx context.Context, timeout time.Duration, what string, fn func(ctx context.Context) error) error {
	return bgpVIPEventuallyEvery(ctx, bgpVIPPollInterval, timeout, what, fn)
}

// bgpVIPEventuallyEvery is bgpVIPEventually polling every interval, for the
// few waits that time an event rather than wait for a steady state.
func bgpVIPEventuallyEvery(ctx context.Context, interval, timeout time.Duration, what string, fn func(ctx context.Context) error) error {
	var last error
	start := time.Now()
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
		callCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		if err := fn(callCtx); err != nil {
			if last == nil || last.Error() != err.Error() {
				e2e.Logf("waiting to %s: %v", what, err)
			}
			last = err
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("timed out after %s waiting to %s: %v", time.Since(start).Round(time.Second), what, last)
	}
	e2e.Logf("%s: done after %s", what, time.Since(start).Round(time.Second))
	return nil
}

// bgpVIPPeer is one BGPVIPConfig spec.defaultPeers entry.
type bgpVIPPeer struct {
	Address              string
	ASN                  int64
	FailureDetection     string
	HoldTimeSeconds      int64
	KeepaliveTimeSeconds int64
}

func (p bgpVIPPeer) IPv6() bool { return isIPv6(p.Address) }

func getBGPVIPConfig(ctx context.Context, oc *exutil.CLI) (*unstructured.Unstructured, error) {
	return oc.AdminDynamicClient().Resource(bgpVIPConfigGVR).Get(ctx, "cluster", metav1.GetOptions{})
}

func bgpVIPPeersFromConfig(u *unstructured.Unstructured) ([]bgpVIPPeer, error) {
	raw, _, err := unstructured.NestedSlice(u.Object, "spec", "defaultPeers")
	if err != nil {
		return nil, err
	}
	var peers []bgpVIPPeer
	for i, item := range raw {
		m, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("spec.defaultPeers[%d] is not an object", i)
		}
		peer := bgpVIPPeer{
			Address:          stringField(m, "peerAddress"),
			FailureDetection: stringField(m, "failureDetection"),
		}
		for _, f := range []struct {
			key string
			out *int64
		}{{"peerASN", &peer.ASN}, {"holdTimeSeconds", &peer.HoldTimeSeconds}, {"keepaliveTimeSeconds", &peer.KeepaliveTimeSeconds}} {
			v, err := intField(m, f.key)
			if err != nil {
				return nil, fmt.Errorf("spec.defaultPeers[%d]: %w", i, err)
			}
			*f.out = v
		}
		peers = append(peers, peer)
	}
	return peers, nil
}

func stringField(m map[string]interface{}, key string) string {
	s, _ := m[key].(string)
	return s
}

// intField returns the integer field key of m, or 0 when it is absent. The
// error names the field but never quotes its value.
func intField(m map[string]interface{}, key string) (int64, error) {
	switch v := m[key].(type) {
	case nil:
		return 0, nil
	case int64:
		return v, nil
	case int:
		return int64(v), nil
	case float64:
		if v != math.Trunc(v) {
			return 0, fmt.Errorf("field %s is not an integer", key)
		}
		return int64(v), nil
	case json.Number:
		n, err := v.Int64()
		if err != nil {
			return 0, fmt.Errorf("field %s is not an integer", key)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("field %s has type %T, want an integer", key, v)
	}
}

// bgpVIPCondition returns a BGPVIPConfig status condition. Each condition
// carries its own observedGeneration; the status has none.
func bgpVIPCondition(u *unstructured.Unstructured, condType string) (status, message string, observedGeneration int64, found bool, err error) {
	conditions, _, err := unstructured.NestedSlice(u.Object, "status", "conditions")
	if err != nil {
		return "", "", 0, false, fmt.Errorf("status.conditions: %w", err)
	}
	for _, c := range conditions {
		m, ok := c.(map[string]interface{})
		if !ok || stringField(m, "type") != condType {
			continue
		}
		observedGeneration, err := intField(m, "observedGeneration")
		if err != nil {
			return "", "", 0, false, fmt.Errorf("condition %s: %w", condType, err)
		}
		return stringField(m, "status"), stringField(m, "message"), observedGeneration, true, nil
	}
	return "", "", 0, false, nil
}

// checkBGPVIPConditions returns nil when Rendered and SessionsConfigured are
// True for the current generation.
func checkBGPVIPConditions(u *unstructured.Unstructured) error {
	var problems []string
	for _, condType := range []string{"Rendered", "SessionsConfigured"} {
		status, message, observed, found, err := bgpVIPCondition(u, condType)
		switch {
		case err != nil:
			problems = append(problems, err.Error())
		case !found:
			problems = append(problems, condType+" is not set")
		case status != "True":
			problems = append(problems, fmt.Sprintf("%s=%s: %s", condType, status, message))
		case observed != u.GetGeneration():
			problems = append(problems, fmt.Sprintf("%s observedGeneration %d, generation %d", condType, observed, u.GetGeneration()))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

func isIPv6(ip string) bool {
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.To4() == nil
}

func hostPrefix(ip string) string {
	if isIPv6(ip) {
		return ip + "/128"
	}
	return ip + "/32"
}

func addressFamily(ip string) string {
	if isIPv6(ip) {
		return "ipv6"
	}
	return "ipv4"
}

// curlResolveAddr formats an address for curl --resolve.
func curlResolveAddr(ip string) string {
	if isIPv6(ip) {
		return "[" + ip + "]"
	}
	return ip
}

func peersOfFamily(peers []bgpVIPPeer, ipv6 bool) []bgpVIPPeer {
	var out []bgpVIPPeer
	for _, p := range peers {
		if p.IPv6() == ipv6 {
			out = append(out, p)
		}
	}
	return out
}

// vipsWithPeerFamily keeps the VIPs that have a peer of the same family. A VIP
// family without a peer is a valid configuration; it is not advertised.
func vipsWithPeerFamily(vips []string, peers []bgpVIPPeer) []string {
	var out []string
	for _, vip := range vips {
		if len(peersOfFamily(peers, isIPv6(vip))) > 0 {
			out = append(out, vip)
		} else {
			e2e.Logf("skipping VIP %s: BGPVIPConfig has no %s peer", vip, addressFamily(vip))
		}
	}
	return out
}

func nodeAddressesOfFamily(node *corev1.Node, ipv6 bool) []string {
	var out []string
	for _, a := range node.Status.Addresses {
		if a.Type == corev1.NodeInternalIP && net.ParseIP(a.Address) != nil && isIPv6(a.Address) == ipv6 {
			out = append(out, a.Address)
		}
	}
	return out
}

func nodeInternalIPs(node *corev1.Node) []string {
	return append(nodeAddressesOfFamily(node, false), nodeAddressesOfFamily(node, true)...)
}

var (
	bgpVIPJSONPassword = regexp.MustCompile(`("password"\s*:\s*)"(?:[^"\\]|\\.)*"`)
	bgpVIPYAMLPassword = regexp.MustCompile(`(?m)^(\s*password:[ \t]*)\S.*$`)
	bgpVIPFRRPassword  = regexp.MustCompile(`(\spassword[ \t]+)\S+`)
)

// redactBGPVIPPasswords hides BGP peer passwords in FRR configuration, JSON
// and YAML dumps. CNO and MCO render the resolved password inline (OPNET-823).
func redactBGPVIPPasswords(s string) string {
	s = bgpVIPJSONPassword.ReplaceAllString(s, `$1"<redacted>"`)
	s = bgpVIPYAMLPassword.ReplaceAllString(s, `$1<redacted>`)
	return bgpVIPFRRPassword.ReplaceAllString(s, `$1<redacted>`)
}

// bgpVIPTimers are the timers rendered into frr-peers.json, as bare decimal
// seconds; empty when unset.
type bgpVIPTimers struct {
	Hold      string
	Keepalive string
}

func bgpVIPTimersFromPeer(p bgpVIPPeer) bgpVIPTimers {
	var t bgpVIPTimers
	if p.HoldTimeSeconds != 0 {
		t.Hold = strconv.FormatInt(p.HoldTimeSeconds, 10)
	}
	if p.KeepaliveTimeSeconds != 0 {
		t.Keepalive = strconv.FormatInt(p.KeepaliveTimeSeconds, 10)
	}
	return t
}

// peerTimersFromRenderedMachineConfig returns the default peers' timers from
// the frr-peers.json file in a rendered MachineConfig, keyed by peer address.
// The file also carries the peer password, so neither the file nor parse
// errors that could quote it are ever returned or logged.
func peerTimersFromRenderedMachineConfig(mc *unstructured.Unstructured) (map[string]bgpVIPTimers, bool, error) {
	files, _, err := unstructured.NestedSlice(mc.Object, "spec", "config", "storage", "files")
	if err != nil {
		return nil, false, fmt.Errorf("cannot read spec.config.storage.files of MachineConfig %s", mc.GetName())
	}
	for _, f := range files {
		file, ok := f.(map[string]interface{})
		if !ok || stringField(file, "path") != bgpVIPPeersFilePath {
			continue
		}
		contents, _ := file["contents"].(map[string]interface{})
		data, err := decodeIgnitionSource(stringField(contents, "source"), stringField(contents, "compression"))
		if err != nil {
			return nil, true, fmt.Errorf("cannot decode %s in MachineConfig %s", bgpVIPPeersFilePath, mc.GetName())
		}
		var peers struct {
			DefaultPeers []struct {
				PeerAddress   string `json:"peerAddress"`
				HoldTime      string `json:"holdTime"`
				KeepaliveTime string `json:"keepaliveTime"`
			} `json:"defaultPeers"`
		}
		if err := json.Unmarshal(data, &peers); err != nil {
			return nil, true, fmt.Errorf("cannot parse %s in MachineConfig %s as JSON", bgpVIPPeersFilePath, mc.GetName())
		}
		out := map[string]bgpVIPTimers{}
		for _, p := range peers.DefaultPeers {
			out[p.PeerAddress] = bgpVIPTimers{Hold: p.HoldTime, Keepalive: p.KeepaliveTime}
		}
		return out, true, nil
	}
	return nil, false, nil
}

func decodeIgnitionSource(source, compression string) ([]byte, error) {
	if !strings.HasPrefix(source, "data:") {
		return nil, fmt.Errorf("not a data URL")
	}
	meta, payload, ok := strings.Cut(strings.TrimPrefix(source, "data:"), ",")
	if !ok {
		return nil, fmt.Errorf("malformed data URL")
	}
	var data []byte
	var err error
	if strings.HasSuffix(meta, ";base64") {
		data, err = base64.StdEncoding.DecodeString(payload)
	} else {
		var s string
		s, err = url.PathUnescape(payload)
		data = []byte(s)
	}
	if err != nil {
		return nil, err
	}
	if compression == "gzip" {
		r, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(r)
		if closeErr := r.Close(); err == nil {
			err = closeErr
		}
		return data, err
	}
	return data, nil
}

// errNoFRRPod is returned by frrPodOnNode when node runs no frr-k8s pod.
var errNoFRRPod = errors.New("no frr-k8s pod")

// frrPodOnNode returns the frr-k8s pod running on node: the MCO static pod
// frr-k8s-<node> on control plane nodes, the CNO DaemonSet pod elsewhere.
func frrPodOnNode(ctx context.Context, oc *exutil.CLI, node string) (string, error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPFRRNamespace).List(ctx, metav1.ListOptions{FieldSelector: "spec.nodeName=" + node})
	if err != nil {
		return "", err
	}
	var candidates []string
	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if c.Name == bgpVIPFRRContainer {
				if pod.Name == "frr-k8s-"+node {
					return pod.Name, nil
				}
				candidates = append(candidates, pod.Name)
			}
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("%w: no running pod with an %q container in %s on node %s", errNoFRRPod, bgpVIPFRRContainer, bgpVIPFRRNamespace, node)
	}
	sort.Strings(candidates)
	return candidates[0], nil
}

// bgpVIPExec runs a command in a container, bounded by a timeout.
func bgpVIPExec(ctx context.Context, oc *exutil.CLI, namespace, pod, container string, command ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()
	req := oc.AdminKubeClient().CoreV1().RESTClient().Post().Resource("pods").Namespace(namespace).Name(pod).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{Container: container, Command: command, Stdout: true, Stderr: true}, scheme.ParameterCodec)
	executor, err := remotecommand.NewSPDYExecutor(oc.AdminConfig(), "POST", req.URL())
	if err != nil {
		return "", err
	}
	var stdout, stderr bytes.Buffer
	if err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{Stdout: &stdout, Stderr: &stderr}); err != nil {
		return stdout.String(), fmt.Errorf("exec %v in %s/%s: %v: %s", command, namespace, pod, err, truncateBGPVIPOutput(stderr.String()))
	}
	return stdout.String(), nil
}

// nodeVtysh runs a vtysh command in the frr container on node.
func nodeVtysh(ctx context.Context, oc *exutil.CLI, node, command string) (string, error) {
	pod, err := frrPodOnNode(ctx, oc, node)
	if err != nil {
		return "", err
	}
	return bgpVIPExec(ctx, oc, bgpVIPFRRNamespace, pod, bgpVIPFRRContainer, "vtysh", "-c", command)
}

func nodeVtyshJSON(ctx context.Context, oc *exutil.CLI, node, command string, out interface{}) error {
	raw, err := nodeVtysh(ctx, oc, node, command+" json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("node %s: %q is not JSON: %v: %s", node, command, err, truncateBGPVIPOutput(raw))
	}
	return nil
}

// frrNeighbor is the subset of "show bgp neighbors <peer> json" the tests use.
type frrNeighbor struct {
	BGPState                 string `json:"bgpState"`
	ConfiguredHoldTimeMsecs  int64  `json:"bgpTimerConfiguredHoldTimeMsecs"`
	ConfiguredKeepAliveMsecs int64  `json:"bgpTimerConfiguredKeepAliveIntervalMsecs"`
	NegotiatedHoldTimeMsecs  int64  `json:"bgpTimerHoldTimeMsecs"`
	NegotiatedKeepAliveMsecs int64  `json:"bgpTimerKeepAliveIntervalMsecs"`
	PeerBFDInfo              *struct {
		Status string `json:"status"`
	} `json:"peerBfdInfo"`
}

func (n frrNeighbor) bfdStatus() string {
	if n.PeerBFDInfo == nil {
		return ""
	}
	return n.PeerBFDInfo.Status
}

func nodeNeighbor(ctx context.Context, oc *exutil.CLI, node, peer string) (frrNeighbor, error) {
	neighbors := map[string]frrNeighbor{}
	if err := nodeVtyshJSON(ctx, oc, node, "show bgp neighbors "+peer, &neighbors); err != nil {
		return frrNeighbor{}, err
	}
	n, ok := neighbors[peer]
	if !ok {
		return frrNeighbor{}, fmt.Errorf("node %s has no BGP neighbor %s", node, peer)
	}
	return n, nil
}

// nodeAdvertises reports whether node advertises prefix to peer.
func nodeAdvertises(ctx context.Context, oc *exutil.CLI, node, peer, prefix string) (bool, error) {
	var routes struct {
		AdvertisedRoutes map[string]json.RawMessage `json:"advertisedRoutes"`
	}
	cmd := fmt.Sprintf("show bgp %s unicast neighbors %s advertised-routes", addressFamily(peer), peer)
	if err := nodeVtyshJSON(ctx, oc, node, cmd, &routes); err != nil {
		return false, err
	}
	_, ok := routes.AdvertisedRoutes[prefix]
	return ok, nil
}

// bgpVIPHypervisor runs commands on the dev-scripts hypervisor, which also
// hosts the bgp-tor FRR container.
type bgpVIPHypervisor struct {
	ip, user, key string
}

func getBGPVIPHypervisor() *bgpVIPHypervisor {
	cfg := exutil.GetHypervisorConfig()
	if cfg == nil || cfg.HypervisorIP == "" {
		return nil
	}
	return &bgpVIPHypervisor{ip: cfg.HypervisorIP, user: cfg.SSHUser, key: cfg.PrivateKeyPath}
}

func requireBGPVIPHypervisor() *bgpVIPHypervisor {
	h := getBGPVIPHypervisor()
	if h == nil {
		e2e.Failf("this test needs the hypervisor SSH configuration (--with-hypervisor-json)")
	}
	return h
}

// run executes script with bash on the hypervisor. Unlike the shared SSH
// helper, every call is bounded: a connect timeout, keepalives and a deadline.
func (h *bgpVIPHypervisor) run(ctx context.Context, timeout time.Duration, script string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	args := []string{
		"-o", "StrictHostKeyChecking=no",
		"-o", "UserKnownHostsFile=/dev/null",
		"-o", "LogLevel=ERROR",
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
		"-o", "ServerAliveInterval=5",
		"-o", "ServerAliveCountMax=3",
	}
	if h.key != "" {
		args = append(args, "-i", h.key)
	}
	args = append(args, fmt.Sprintf("%s@%s", h.user, h.ip), "bash -s")
	cmd := exec.CommandContext(ctx, "ssh", args...)
	cmd.Stdin = strings.NewReader("set -o errexit -o nounset -o pipefail\n" + script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("hypervisor command failed: %v: %s", err, truncateBGPVIPOutput(stderr.String()))
	}
	return stdout.String(), nil
}

// torVtysh runs a vtysh command in the ToR FRR container on the hypervisor.
func (h *bgpVIPHypervisor) torVtysh(ctx context.Context, command string) (string, error) {
	return h.run(ctx, time.Minute, fmt.Sprintf("podman exec %s vtysh -c %s", bgpVIPTorContainer, shellQuote(command)))
}

func (h *bgpVIPHypervisor) torVtyshJSON(ctx context.Context, command string, out interface{}) error {
	raw, err := h.torVtysh(ctx, command+" json")
	if err != nil {
		return err
	}
	if err := json.Unmarshal([]byte(raw), out); err != nil {
		return fmt.Errorf("ToR: %q is not JSON: %v: %s", command, err, truncateBGPVIPOutput(raw))
	}
	return nil
}

// torPathPeers returns the BGP peers the ToR has a path to vip from.
func (h *bgpVIPHypervisor) torPathPeers(ctx context.Context, vip string) ([]string, error) {
	var route struct {
		Paths []struct {
			Peer struct {
				PeerID string `json:"peerId"`
			} `json:"peer"`
		} `json:"paths"`
	}
	if err := h.torVtyshJSON(ctx, fmt.Sprintf("show bgp %s unicast %s", addressFamily(vip), hostPrefix(vip)), &route); err != nil {
		return nil, err
	}
	var peers []string
	for _, p := range route.Paths {
		peers = append(peers, p.Peer.PeerID)
	}
	sort.Strings(peers)
	return peers, nil
}

// torNeighborStates returns the ToR's BGP neighbors and their states. The ToR
// uses dynamic neighbors, so a neighbor whose session is down disappears.
func (h *bgpVIPHypervisor) torNeighborStates(ctx context.Context) (map[string]string, error) {
	neighbors := map[string]struct {
		BGPState string `json:"bgpState"`
	}{}
	if err := h.torVtyshJSON(ctx, "show bgp neighbors", &neighbors); err != nil {
		return nil, err
	}
	out := map[string]string{}
	for addr, n := range neighbors {
		if net.ParseIP(addr) != nil {
			out[addr] = n.BGPState
		}
	}
	return out, nil
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func truncateBGPVIPOutput(s string) string {
	const max = 4096
	if len(s) > max {
		return s[:max] + fmt.Sprintf("... (%d bytes)", len(s))
	}
	return s
}

// bgpVIPProbe is a continuous HTTP probe on the hypervisor: every second it
// opens n new connections per VIP to url (whose host is resolved to the VIP)
// and logs "<start time> <vip> <http code>" per request.
type bgpVIPProbe struct {
	h    *bgpVIPHypervisor
	name string
}

func (p *bgpVIPProbe) dir() string { return "/tmp/bgp-vip-e2e-" + p.name }

// startBGPVIPProbe requests rawURL through every VIP n times a second from
// the hypervisor until stopped. With ports, every request binds to its own
// block of local ports and its log line carries its request number.
func startBGPVIPProbe(ctx context.Context, h *bgpVIPHypervisor, name, rawURL string, vips []string, n int, ports *bgpVIPProbePorts) (*bgpVIPProbe, error) {
	p := newBGPVIPProbe(h, name)
	if err := p.start(ctx, rawURL, vips, n, ports); err != nil {
		return nil, err
	}
	return p, nil
}

// newBGPVIPProbe returns a probe that has not started yet, so that a test can
// record it before start creates its files on the hypervisor.
func newBGPVIPProbe(h *bgpVIPHypervisor, name string) *bgpVIPProbe {
	return &bgpVIPProbe{h: h, name: name}
}

func (p *bgpVIPProbe) start(ctx context.Context, rawURL string, vips []string, n int, ports *bgpVIPProbePorts) error {
	loop, err := bgpVIPProbeLoop(rawURL, vips, n, p.dir()+"/log", ports)
	if err != nil {
		return err
	}
	script := fmt.Sprintf(`rm -rf %[1]s; mkdir -p %[1]s
cat > %[1]s/loop.sh <<'EOF'
%[2]sEOF
setsid bash %[1]s/loop.sh > /dev/null 2>&1 < /dev/null &
echo $! > %[1]s/pid
`, p.dir(), loop)
	_, err = p.h.run(ctx, time.Minute, script)
	return err
}

// bgpVIPProbeLoop returns the probe's shell loop. Each request appends
// "<start time> <vip> <HTTP code>" to logFile, followed by its request
// number when ports is set.
func bgpVIPProbeLoop(rawURL string, vips []string, n int, logFile string, ports *bgpVIPProbePorts) (string, error) {
	u, port, err := parseBGPVIPURL(rawURL)
	if err != nil {
		return "", err
	}
	var loop strings.Builder
	loop.WriteString("r=0\nwhile :; do\n")
	for _, vip := range vips {
		resolve := shellQuote(u.Hostname() + ":" + port + ":" + curlResolveAddr(vip))
		if ports == nil {
			fmt.Fprintf(&loop, "  for i in $(seq %d); do ( t=$(date +%%s.%%N); c=$(curl -sk -o /dev/null -w '%%{http_code}' --max-time 3 --noproxy '*' --resolve %s %s || true); echo \"$t %s ${c:-000}\" >> %s ) & done\n",
				n, resolve, shellQuote(rawURL), vip, logFile)
			continue
		}
		fmt.Fprintf(&loop, "  for i in $(seq %d); do lp=$((%d + r %% %d * %d)); ( t=$(date +%%s.%%N); c=$(curl -sk -o /dev/null -w '%%{http_code}' --max-time 3 --noproxy '*' --local-port \"$lp-$((lp + %d))\" --resolve %s %s || true); echo \"$t %s ${c:-000} $r\" >> %s ) & r=$((r + 1)); done\n",
			n, ports.Base, ports.Blocks, ports.Block, ports.Block-1, resolve, shellQuote(rawURL), vip, logFile)
	}
	loop.WriteString("  sleep 1\ndone\n")
	return loop.String(), nil
}

// stop stops the probe and returns its samples. Safe to call more than once.
func (p *bgpVIPProbe) stop(ctx context.Context) ([]bgpVIPProbeSample, error) {
	out, err := p.h.run(ctx, time.Minute, bgpVIPProbeStopScript(p.dir(), false))
	if err != nil {
		return nil, err
	}
	return parseBGPVIPProbeLog(out), nil
}

// stopAndRemove stops the probe and removes its files from the hypervisor.
// It returns the samples read before the removal, also when it fails.
func (p *bgpVIPProbe) stopAndRemove(ctx context.Context, timeout time.Duration) ([]bgpVIPProbeSample, error) {
	out, err := p.h.run(ctx, timeout, bgpVIPProbeStopScript(p.dir(), true))
	return parseBGPVIPProbeLog(out), err
}

// bgpVIPProbeStopScript stops the loop first so that it launches no more
// requests, gives the requests in flight (curl --max-time 3) time to log
// their result, kills whatever is left of the process group and prints the
// log; with remove, it then deletes the probe's files.
func bgpVIPProbeStopScript(dir string, remove bool) string {
	script := fmt.Sprintf(`if [ -f %[1]s/pid ]; then pid=$(cat %[1]s/pid); kill -TERM "$pid" 2>/dev/null || true; sleep 4; kill -TERM -- "-$pid" 2>/dev/null || true; rm -f %[1]s/pid; fi
cat %[1]s/log 2>/dev/null || true`, dir)
	if remove {
		script += "\nrm -rf " + dir + "\n"
	}
	return script
}

// describeBGPVIPProbeSamples summarizes the samples per VIP with the start
// times of up to maxFailures failed requests, for a test that fails before
// it analyzes them.
func describeBGPVIPProbeSamples(samples []bgpVIPProbeSample, maxFailures int) []string {
	stats := summarizeBGPVIPProbe(samples, math.Inf(-1), math.Inf(1))
	failures := map[string][]string{}
	for _, s := range samples {
		if !s.OK && len(failures[s.VIP]) < maxFailures {
			failures[s.VIP] = append(failures[s.VIP], fmt.Sprintf("%.3f", s.Time))
		}
	}
	var out []string
	for _, vip := range sets.List(sets.KeySet(stats)) {
		st := stats[vip]
		line := fmt.Sprintf("ingress probe via %s (%s): %d requests, %d failed", vip, addressFamily(vip), st.Total, st.Failed)
		if st.Failed > 0 {
			line += ", failed requests started at " + strings.Join(failures[vip], ", ")
			if st.Failed > len(failures[vip]) {
				line += fmt.Sprintf(" and %d more", st.Failed-len(failures[vip]))
			}
			line += " (hypervisor clock)"
		}
		out = append(out, line)
	}
	return out
}

type bgpVIPProbeSample struct {
	Time float64
	VIP  string
	OK   bool
	// Request is the request number of a probe with ports, -1 otherwise.
	Request int
}

func parseBGPVIPProbeLog(log string) []bgpVIPProbeSample {
	var samples []bgpVIPProbeSample
	for _, line := range strings.Split(log, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 && len(fields) != 4 {
			continue
		}
		t, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		sample := bgpVIPProbeSample{Time: t, VIP: fields[1], OK: fields[2] == "200", Request: -1}
		if len(fields) == 4 {
			if sample.Request, err = strconv.Atoi(fields[3]); err != nil || sample.Request < 0 {
				continue
			}
		}
		samples = append(samples, sample)
	}
	sort.SliceStable(samples, func(i, j int) bool { return samples[i].Time < samples[j].Time })
	return samples
}

type bgpVIPProbeStats struct {
	Total, Failed             int
	FirstFailure, LastFailure float64
}

// summarizeBGPVIPProbe counts the samples per VIP that started in [from, to).
func summarizeBGPVIPProbe(samples []bgpVIPProbeSample, from, to float64) map[string]bgpVIPProbeStats {
	stats := map[string]bgpVIPProbeStats{}
	for _, s := range samples {
		if s.Time < from || s.Time >= to {
			continue
		}
		st := stats[s.VIP]
		st.Total++
		if !s.OK {
			st.Failed++
			if st.FirstFailure == 0 {
				st.FirstFailure = s.Time
			}
			st.LastFailure = s.Time
		}
		stats[s.VIP] = st
	}
	return stats
}

// bgpVIPFamilies returns the sorted address families of ips.
func bgpVIPFamilies(ips []string) []string {
	families := sets.New[string]()
	for _, ip := range ips {
		families.Insert(addressFamily(ip))
	}
	return sets.List(families)
}

// bgpVIPProbePorts gives every request of a probe its own block of local
// ports. The hypervisor records the source port of each probe SYN per router
// node, so a port identifies the probe request that was sent to that node,
// apart from any other traffic of the hypervisor to the VIPs, and a
// retransmitted SYN is the same request rather than another one.
type bgpVIPProbePorts struct {
	// Base is the first port, Block the number of ports curl may pick from
	// for one request, and Blocks the number of requests before the ports
	// repeat.
	Base, Block, Blocks int
}

// bgpVIPIngressProbePorts lies below the default ephemeral port range
// (32768-60999), so the hypervisor's other connections do not pick these
// ports, and a few ports per request cover one that happens to be taken.
// 2500 requests outlast the ingress VIP move test: 2 VIPs at 1/s each for at
// most its body timeout.
var bgpVIPIngressProbePorts = bgpVIPProbePorts{Base: 20000, Block: 4, Blocks: 2500}

// ports returns the first and last local port of request r.
func (p bgpVIPProbePorts) ports(r int) (int, int) {
	first := p.Base + (r%p.Blocks)*p.Block
	return first, first + p.Block - 1
}

// block returns the block of local ports that port belongs to, or -1.
func (p bgpVIPProbePorts) block(port int) int {
	if port < p.Base || port >= p.Base+p.Block*p.Blocks {
		return -1
	}
	return (port - p.Base) / p.Block
}

// parseBGPVIPPortRange parses net.ipv4.ip_local_port_range: "<low> <high>".
func parseBGPVIPPortRange(out string) (int, int, error) {
	fields := strings.Fields(out)
	if len(fields) != 2 {
		return 0, 0, fmt.Errorf("want 2 ports, got %q", truncateBGPVIPOutput(out))
	}
	low, err := strconv.Atoi(fields[0])
	if err != nil {
		return 0, 0, err
	}
	high, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, err
	}
	if low < 1 || high > 65535 || low > high {
		return 0, 0, fmt.Errorf("invalid port range %d-%d", low, high)
	}
	return low, high, nil
}

// checkBGPVIPProbePorts checks that the hypervisor's kernel never picks a
// probe port by itself, given the output of `sysctl -n
// net.ipv4.ip_local_port_range`. Linux takes the local port of a connect()
// or bind() without a port from that range, for IPv4 and IPv6 sockets
// alike, and a socket's IP_LOCAL_PORT_RANGE can only narrow it. A probe port
// then only carries traffic of a process that binds to it explicitly.
func checkBGPVIPProbePorts(rangeOut string, p bgpVIPProbePorts) error {
	low, high, err := parseBGPVIPPortRange(rangeOut)
	if err != nil {
		return fmt.Errorf("cannot parse net.ipv4.ip_local_port_range: %w", err)
	}
	first, _ := p.ports(0)
	_, last := p.ports(p.Blocks - 1)
	if first <= high && low <= last {
		return fmt.Errorf("the probe's ports %d-%d overlap the hypervisor's ephemeral ports %d-%d", first, last, low, high)
	}
	return nil
}

// bgpVIPIngressHashPolicyFile holds the original multipath hash policy of
// the families whose policy the ingress VIP move changed.
const bgpVIPIngressHashPolicyFile = "/tmp/bgp-vip-e2e-ingress-hash-policy"

func bgpVIPHashPolicyKey(family string) string {
	return "net." + family + ".fib_multipath_hash_policy"
}

// bgpVIPSetHashPolicyFamiliesScript hashes the multipath routes of families,
// and of no other family, by port. Before it changes a family it records the
// family's policy in file, unless file still holds the original from a run
// that could not restore it, so that a change that stops halfway is
// restored too. It stops at the first failure.
func bgpVIPSetHashPolicyFamiliesScript(families []string, file string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "f=%s\n", file)
	for _, f := range families {
		fmt.Fprintf(&b, "if ! grep -q '^%[1]s=' \"$f\" 2>/dev/null; then v=$(sysctl -n %[1]s); echo \"%[1]s=$v\" >> \"$f\"; fi\nsysctl -q -w %[1]s=1\n", bgpVIPHashPolicyKey(f))
	}
	return b.String()
}

// bgpVIPRestoreHashPolicyFamiliesScript restores every policy recorded in
// file, each even when another fails, and removes file only once all are
// restored. It sets rc=1 on failure rather than exiting, so that the cleanup
// steps after it still run.
func bgpVIPRestoreHashPolicyFamiliesScript(file string) string {
	return fmt.Sprintf(`f=%s
if [ -f "$f" ]; then
  restored=1
  while IFS= read -r kv; do
    [ -n "$kv" ] || continue
    sysctl -q -w "$kv" || { echo "cannot restore $kv" >&2; restored=0; rc=1; }
  done < "$f"
  [ "$restored" = 0 ] || rm -f "$f"
fi
`, file)
}

// bgpVIPIngressHostCleanupScript undoes what the ingress VIP move changed on
// the hypervisor besides its probe: the probe path records and, when it
// changed it, the multipath hash policy. Each part runs even when the other
// fails, and the script fails when either did.
func bgpVIPIngressHostCleanupScript(pathsRecorded, hashPolicySet bool) string {
	script := "rc=0\n"
	if pathsRecorded {
		script += `if nft list table bridge bgp_vip_e2e > /dev/null 2>&1; then nft delete table bridge bgp_vip_e2e || { echo "cannot delete nft table bridge bgp_vip_e2e" >&2; rc=1; }; fi` + "\n"
	}
	if hashPolicySet {
		script += bgpVIPRestoreHashPolicyFamiliesScript(bgpVIPIngressHashPolicyFile)
	}
	return script + "exit \"$rc\"\n"
}

// bgpVIPExposureRouter is a router node and the MAC addresses its InternalIPs
// resolve to on the hypervisor.
type bgpVIPExposureRouter struct {
	Node string
	MACs []string
}

// bgpVIPPathSet names the nft set of the probe SYN source ports sent to
// routers[i] for family; bgpVIPSYNCounter names the counter of those SYNs.
func bgpVIPPathSet(i int, family string) string    { return fmt.Sprintf("p%d_%s", i, family) }
func bgpVIPSYNCounter(i int, family string) string { return fmt.Sprintf("s%d_%s", i, family) }

// bgpVIPProbePathNftScript records, per router node and address family, the
// source port of every probe SYN that the hypervisor sends to the node's MAC
// address, and counts those SYNs, retransmissions included. The probe runs on
// the hypervisor, whose kernel holds the BGP routes the ToR's zebra installs
// and transmits the routed probe packets through the bridge the nodes are
// attached to; frames the host itself sends through a bridge traverse the
// bridge family's output hook. Their destination MAC is the next hop that the
// kernel's multipath route picked for that connection, under whatever hash
// policy is in effect. It only records; nothing is dropped.
func bgpVIPProbePathNftScript(routers []bgpVIPExposureRouter, vips []string, dport string, ports bgpVIPProbePorts) string {
	byFamily := map[string][]string{}
	for _, vip := range vips {
		byFamily[addressFamily(vip)] = append(byFamily[addressFamily(vip)], vip)
	}
	families := bgpVIPFamilies(vips)
	first, _ := ports.ports(0)
	_, last := ports.ports(ports.Blocks - 1)
	var b strings.Builder
	b.WriteString("table bridge bgp_vip_e2e {\n")
	for i := range routers {
		for _, f := range families {
			fmt.Fprintf(&b, "\tset %s {\n\t\ttype inet_service\n\t\tsize 65535\n\t\tflags dynamic\n\t}\n", bgpVIPPathSet(i, f))
			fmt.Fprintf(&b, "\tcounter %s { }\n", bgpVIPSYNCounter(i, f))
		}
	}
	b.WriteString("\tchain output {\n\t\ttype filter hook output priority -200; policy accept;\n")
	for i, r := range routers {
		for _, f := range families {
			proto := "ip"
			if f == "ipv6" {
				proto = "ip6"
			}
			fmt.Fprintf(&b, "\t\tether daddr { %s } %s daddr { %s } tcp sport %d-%d tcp dport %s tcp flags & (syn | ack) == syn add @%s { tcp sport } counter name %s\n",
				strings.Join(r.MACs, ", "), proto, strings.Join(byFamily[f], ", "), first, last, dport, bgpVIPPathSet(i, f), bgpVIPSYNCounter(i, f))
		}
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

// bgpVIPObservationMarker separates the probe log from the nft table in the
// output of bgpVIPObservationScript.
const bgpVIPObservationMarker = "--- bgp-vip-e2e nft ---"

// bgpVIPObservationScript reads the probe log, then the recorded probe paths,
// then the hypervisor clock: every request in the log has sent its SYNs by
// the time the paths are read.
func bgpVIPObservationScript(logFile string) string {
	return fmt.Sprintf("cat %s 2>/dev/null || true\necho %s\nnft -j list table bridge bgp_vip_e2e\necho\ndate +%%s.%%N\n", logFile, shellQuote(bgpVIPObservationMarker))
}

// bgpVIPProbeObservation is one reading of the probe log and of what the
// hypervisor recorded of the probe's paths, with the hypervisor clock.
type bgpVIPProbeObservation struct {
	Time    float64
	Samples []bgpVIPProbeSample
	// Blocks holds, per router node and family, the port blocks of the
	// probe requests whose SYNs were sent to the node; SYNs counts those
	// SYNs, retransmissions included.
	Blocks map[string]map[string]sets.Set[int]
	SYNs   map[string]map[string]int64
}

// parseBGPVIPProbeObservation parses the output of bgpVIPObservationScript.
// Every set and counter of routers and families must be present.
func parseBGPVIPProbeObservation(out string, routers []bgpVIPExposureRouter, families []string, ports bgpVIPProbePorts) (bgpVIPProbeObservation, error) {
	log, rest, ok := strings.Cut(out, bgpVIPObservationMarker+"\n")
	if !ok {
		return bgpVIPProbeObservation{}, fmt.Errorf("no %q in %q", bgpVIPObservationMarker, truncateBGPVIPOutput(out))
	}
	t, err := parseHypervisorClock(rest)
	if err != nil {
		return bgpVIPProbeObservation{}, err
	}
	var doc struct {
		Nftables []struct {
			Set *struct {
				Name string            `json:"name"`
				Elem []json.RawMessage `json:"elem"`
			} `json:"set"`
			Counter *struct {
				Name    string `json:"name"`
				Packets int64  `json:"packets"`
			} `json:"counter"`
		} `json:"nftables"`
	}
	found := false
	for _, line := range strings.Split(rest, "\n") {
		if line = strings.TrimSpace(line); strings.HasPrefix(line, "{") {
			if err := json.Unmarshal([]byte(line), &doc); err != nil {
				return bgpVIPProbeObservation{}, fmt.Errorf("nft output is not JSON: %w: %s", err, truncateBGPVIPOutput(line))
			}
			found = true
			break
		}
	}
	if !found {
		return bgpVIPProbeObservation{}, fmt.Errorf("no nft JSON in %q", truncateBGPVIPOutput(rest))
	}
	setPorts := map[string][]int{}
	counters := map[string]int64{}
	for _, o := range doc.Nftables {
		switch {
		case o.Set != nil:
			setPorts[o.Set.Name] = []int{}
			for _, raw := range o.Set.Elem {
				port, err := parseBGPVIPNftPort(raw)
				if err != nil {
					return bgpVIPProbeObservation{}, fmt.Errorf("set %s: %w", o.Set.Name, err)
				}
				setPorts[o.Set.Name] = append(setPorts[o.Set.Name], port)
			}
		case o.Counter != nil:
			counters[o.Counter.Name] = o.Counter.Packets
		}
	}
	obs := bgpVIPProbeObservation{Time: t, Samples: parseBGPVIPProbeLog(log), Blocks: map[string]map[string]sets.Set[int]{}, SYNs: map[string]map[string]int64{}}
	for i, r := range routers {
		obs.Blocks[r.Node] = map[string]sets.Set[int]{}
		obs.SYNs[r.Node] = map[string]int64{}
		for _, f := range families {
			ps, ok := setPorts[bgpVIPPathSet(i, f)]
			if !ok {
				return bgpVIPProbeObservation{}, fmt.Errorf("nft set %s of %s is missing", bgpVIPPathSet(i, f), r.Node)
			}
			n, ok := counters[bgpVIPSYNCounter(i, f)]
			if !ok {
				return bgpVIPProbeObservation{}, fmt.Errorf("nft counter %s of %s is missing", bgpVIPSYNCounter(i, f), r.Node)
			}
			blocks := sets.New[int]()
			for _, port := range ps {
				b := ports.block(port)
				if b < 0 {
					return bgpVIPProbeObservation{}, fmt.Errorf("nft set %s holds port %d outside the probe's ports", bgpVIPPathSet(i, f), port)
				}
				blocks.Insert(b)
			}
			obs.Blocks[r.Node][f] = blocks
			obs.SYNs[r.Node][f] = n
		}
	}
	return obs, nil
}

// parseBGPVIPNftPort parses a set element of `nft -j`: a port, or an object
// holding it when the element carries more attributes.
func parseBGPVIPNftPort(raw json.RawMessage) (int, error) {
	var port int
	if err := json.Unmarshal(raw, &port); err == nil {
		return port, nil
	}
	var elem struct {
		Elem *struct {
			Val *int `json:"val"`
		} `json:"elem"`
	}
	if err := json.Unmarshal(raw, &elem); err == nil && elem.Elem != nil && elem.Elem.Val != nil {
		return *elem.Elem.Val, nil
	}
	return 0, fmt.Errorf("unexpected element %s", truncateBGPVIPOutput(string(raw)))
}

// bgpVIPAttributedSample is a probe sample with the router nodes its SYNs
// were sent to.
type bgpVIPAttributedSample struct {
	bgpVIPProbeSample
	Family string
	// Routers is sorted, and empty when no SYN of the request was recorded
	// or its port block does not identify it.
	Routers []string
}

// attributeBGPVIPProbe attributes every sample to the router nodes whose
// recorded blocks hold its port block. Samples without a request number,
// and samples whose port block another sample also used, stay unattributed.
func attributeBGPVIPProbe(samples []bgpVIPProbeSample, blocks map[string]map[string]sets.Set[int], ports bgpVIPProbePorts) []bgpVIPAttributedSample {
	uses := map[int]int{}
	for _, s := range samples {
		if s.Request >= 0 {
			uses[s.Request%ports.Blocks]++
		}
	}
	nodes := sets.List(sets.KeySet(blocks))
	out := make([]bgpVIPAttributedSample, 0, len(samples))
	for _, s := range samples {
		a := bgpVIPAttributedSample{bgpVIPProbeSample: s, Family: addressFamily(s.VIP)}
		if b := s.Request % ports.Blocks; s.Request >= 0 && uses[b] == 1 {
			for _, node := range nodes {
				if blocks[node][a.Family].Has(b) {
					a.Routers = append(a.Routers, node)
				}
			}
		}
		out = append(out, a)
	}
	return out
}

// bgpVIPExposure counts, per router node and family, the successful probe
// requests that started in [from, to) and whose SYNs went to that node only.
func bgpVIPExposure(samples []bgpVIPAttributedSample, from, to float64) map[string]map[string]int {
	out := map[string]map[string]int{}
	for _, s := range samples {
		if !s.OK || len(s.Routers) != 1 || s.Time < from || s.Time >= to {
			continue
		}
		if out[s.Routers[0]] == nil {
			out[s.Routers[0]] = map[string]int{}
		}
		out[s.Routers[0]][s.Family]++
	}
	return out
}

// selectBGPVIPExposedRouter returns the router node that carried at least
// atLeast probe requests of every family, preferring the node whose least
// exposed family carried the most, then the first by name.
func selectBGPVIPExposedRouter(exposure map[string]map[string]int, families []string, atLeast int) (string, bool) {
	best, bestLeast := "", -1
	for _, node := range sets.List(sets.KeySet(exposure)) {
		least := math.MaxInt
		for _, f := range families {
			least = min(least, exposure[node][f])
		}
		if len(families) > 0 && least >= atLeast && least > bestLeast {
			best, bestLeast = node, least
		}
	}
	return best, best != ""
}

// bgpVIPProbePhase is a window of a probe run on the hypervisor clock: the
// samples that started in [From, To) belong to it.
type bgpVIPProbePhase struct {
	Name     string
	From, To float64
}

// bgpVIPProbePhases splits a probe run into contiguous phases at the
// boundaries, boundaries[i] ending names[i]. The first phase starts and the
// last ends unbounded, so every sample falls into exactly one phase.
func bgpVIPProbePhases(names []string, boundaries []float64) ([]bgpVIPProbePhase, error) {
	if len(names) == 0 || len(boundaries) != len(names)-1 {
		return nil, fmt.Errorf("%d phases need %d boundaries, got %d", len(names), len(names)-1, len(boundaries))
	}
	phases := make([]bgpVIPProbePhase, 0, len(names))
	from := math.Inf(-1)
	for i, name := range names {
		to := math.Inf(1)
		if i < len(boundaries) {
			to = boundaries[i]
		}
		if to < from {
			return nil, fmt.Errorf("phase %s ends at %.3f before it starts at %.3f", name, to, from)
		}
		phases = append(phases, bgpVIPProbePhase{Name: name, From: from, To: to})
		from = to
	}
	return phases, nil
}

// bgpVIPFailure is a failed probe request: when it started on the hypervisor
// clock, in which phase, and the router nodes its SYNs were sent to.
type bgpVIPFailure struct {
	Time    float64
	Phase   string
	Routers []string
}

// bgpVIPPhaseStats summarizes the probe requests through one VIP.
type bgpVIPPhaseStats struct {
	Total, Failed int
	First, Last   *bgpVIPFailure
	// Requests counts the requests by the router nodes their SYNs were sent
	// to, joined by "+", with "none" for those not attributed to a node.
	Requests map[string]int
}

func (st *bgpVIPPhaseStats) add(s bgpVIPAttributedSample, phase string) {
	st.Total++
	routers := "none"
	if len(s.Routers) > 0 {
		routers = strings.Join(s.Routers, "+")
	}
	st.Requests[routers]++
	if s.OK {
		return
	}
	st.Failed++
	f := &bgpVIPFailure{Time: s.Time, Phase: phase, Routers: s.Routers}
	if st.First == nil || s.Time < st.First.Time {
		st.First = f
	}
	if st.Last == nil || s.Time >= st.Last.Time {
		st.Last = f
	}
}

// summarizeBGPVIPPhases summarizes the samples per phase and VIP, and per VIP
// over the whole run.
func summarizeBGPVIPPhases(phases []bgpVIPProbePhase, samples []bgpVIPAttributedSample) (map[string]map[string]*bgpVIPPhaseStats, map[string]*bgpVIPPhaseStats) {
	perPhase := map[string]map[string]*bgpVIPPhaseStats{}
	overall := map[string]*bgpVIPPhaseStats{}
	get := func(m map[string]*bgpVIPPhaseStats, vip string) *bgpVIPPhaseStats {
		if m[vip] == nil {
			m[vip] = &bgpVIPPhaseStats{Requests: map[string]int{}}
		}
		return m[vip]
	}
	for _, p := range phases {
		perPhase[p.Name] = map[string]*bgpVIPPhaseStats{}
	}
	for _, s := range samples {
		for _, p := range phases {
			if s.Time >= p.From && s.Time < p.To {
				get(perPhase[p.Name], s.VIP).add(s, p.Name)
				get(overall, s.VIP).add(s, p.Name)
				break
			}
		}
	}
	return perPhase, overall
}

// describeBGPVIPPhaseStats reports a VIP's probe requests in a phase, with
// the failure times relative to the disruption on the hypervisor clock.
func describeBGPVIPPhaseStats(phase, vip string, st *bgpVIPPhaseStats, disruptedAt float64) string {
	if st == nil {
		st = &bgpVIPPhaseStats{}
	}
	s := fmt.Sprintf("ingress probe via %s (%s), %s: %d requests, %d failed, requests by router node %v", vip, addressFamily(vip), phase, st.Total, st.Failed, st.Requests)
	describe := func(f *bgpVIPFailure) string {
		routers := "no recorded router node"
		if len(f.Routers) > 0 {
			routers = strings.Join(f.Routers, "+")
		}
		return fmt.Sprintf("%+.1fs in phase %s via %s", f.Time-disruptedAt, f.Phase, routers)
	}
	if st.First != nil {
		s += fmt.Sprintf("; first failure %s, last failure %s (hypervisor clock, relative to the disruption)", describe(st.First), describe(st.Last))
	}
	return s
}

// bgpVIPTorPathsVia reports whether the ToR paths peers include one from node.
func bgpVIPTorPathsVia(peers []string, node string, nodeIPs map[string]string) bool {
	for _, peer := range peers {
		if nodeIPs[peer] == node {
			return true
		}
	}
	return false
}

// bgpVIPCleanupStep is one step of a test's cleanup, bounded by its own
// timeout when it has one.
type bgpVIPCleanupStep struct {
	Name    string
	Timeout time.Duration
	Run     func(ctx context.Context) error
}

// runBGPVIPCleanup runs every step in order and returns all their errors: a
// failed or timed out step never skips the steps after it.
func runBGPVIPCleanup(ctx context.Context, steps []bgpVIPCleanupStep) error {
	var errs []error
	for _, step := range steps {
		err := func() error {
			stepCtx, cancel := ctx, context.CancelFunc(func() {})
			if step.Timeout > 0 {
				stepCtx, cancel = context.WithTimeout(ctx, step.Timeout)
			}
			defer cancel()
			return step.Run(stepCtx)
		}()
		if err != nil {
			e2e.Logf("cleanup: %s failed: %v", step.Name, err)
			errs = append(errs, fmt.Errorf("%s: %w", step.Name, err))
		}
	}
	return errors.Join(errs...)
}

// parseBGPVIPURL returns rawURL and the TCP port it is served on.
func parseBGPVIPURL(rawURL string) (*url.URL, string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, "", err
	}
	if u.Hostname() == "" {
		return nil, "", fmt.Errorf("URL %q has no host", rawURL)
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	return u, port, nil
}

// bgpVIPBlackholeNftScript drops the hypervisor's frames to one master's MAC
// that carry API traffic to an API VIP: the data-plane half of a cut, so
// connections still routed to that master fail until BGP withdraws it. The
// hypervisor's own SSH and direct-IP access to the master are untouched.
func bgpVIPBlackholeNftScript(mac string, apiVIPs []string) string {
	var v4, v6 []string
	for _, vip := range apiVIPs {
		if isIPv6(vip) {
			v6 = append(v6, vip)
		} else {
			v4 = append(v4, vip)
		}
	}
	var b strings.Builder
	b.WriteString("table bridge bgp_vip_e2e {\n\tcounter v4 { }\n\tcounter v6 { }\n\tchain output {\n\t\ttype filter hook output priority -200; policy accept;\n")
	if len(v4) > 0 {
		fmt.Fprintf(&b, "\t\tether daddr %s ip daddr { %s } tcp dport 6443 counter name v4 drop\n", mac, strings.Join(v4, ", "))
	}
	if len(v6) > 0 {
		fmt.Fprintf(&b, "\t\tether daddr %s ip6 daddr { %s } tcp dport 6443 counter name v6 drop\n", mac, strings.Join(v6, ", "))
	}
	b.WriteString("\t}\n}\n")
	return b.String()
}

// bgpVIPPeeringCutNftScript drops BGP and BFD (single hop and multihop)
// between the hypervisor, where the ToR runs with host networking, and one
// master.
func bgpVIPPeeringCutNftScript(masterIPs []string) string {
	var v4, v6 []string
	for _, ip := range masterIPs {
		if isIPv6(ip) {
			v6 = append(v6, ip)
		} else {
			v4 = append(v4, ip)
		}
	}
	rules := func(dir string) string {
		var b strings.Builder
		for _, fam := range []struct {
			proto string
			ips   []string
		}{{"ip", v4}, {"ip6", v6}} {
			if len(fam.ips) == 0 {
				continue
			}
			match := fmt.Sprintf("%s %s { %s }", fam.proto, dir, strings.Join(fam.ips, ", "))
			fmt.Fprintf(&b, "\t\t%s tcp dport 179 drop\n", match)
			fmt.Fprintf(&b, "\t\t%s tcp sport 179 drop\n", match)
			fmt.Fprintf(&b, "\t\t%s udp dport { 3784, 3785, 4784 } drop\n", match)
			fmt.Fprintf(&b, "\t\t%s udp sport { 3784, 3785, 4784 } drop\n", match)
		}
		return b.String()
	}
	return "table inet bgp_vip_e2e {\n" +
		"\tchain input {\n\t\ttype filter hook input priority -200; policy accept;\n" + rules("saddr") + "\t}\n" +
		"\tchain output {\n\t\ttype filter hook output priority -200; policy accept;\n" + rules("daddr") + "\t}\n" +
		"}\n"
}

// dumpBGPVIPState logs the BGP VIP state for a failed test, with passwords
// redacted.
func dumpBGPVIPState(ctx context.Context, oc *exutil.CLI, h *bgpVIPHypervisor) {
	ctx, cancel := context.WithTimeout(ctx, bgpVIPDumpTimeout)
	defer cancel()
	dump := func(what string, v interface{}) {
		raw, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			e2e.Logf("%s: %v", what, err)
			return
		}
		e2e.Logf("%s:\n%s", what, redactBGPVIPPasswords(string(raw)))
	}
	if cr, err := getBGPVIPConfig(ctx, oc); err == nil {
		dump("BGPVIPConfig/cluster", cr.Object)
	} else {
		e2e.Logf("BGPVIPConfig/cluster: %v", err)
	}
	if frr, err := oc.AdminDynamicClient().Resource(frrConfigurationGVR).Namespace(bgpVIPFRRNamespace).Get(ctx, bgpVIPFRRConfiguration, metav1.GetOptions{}); err == nil {
		unstructured.RemoveNestedField(frr.Object, "metadata", "managedFields")
		dump("FRRConfiguration/"+bgpVIPFRRConfiguration, frr.Object)
	} else {
		e2e.Logf("FRRConfiguration/%s: %v", bgpVIPFRRConfiguration, err)
	}
	if nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{}); err == nil {
		for _, node := range nodes.Items {
			for _, cmd := range []string{"show bgp summary", "show bfd peers brief"} {
				out, err := nodeVtysh(ctx, oc, node.Name, cmd)
				if err != nil {
					continue
				}
				e2e.Logf("node %s %q:\n%s", node.Name, cmd, redactBGPVIPPasswords(out))
			}
		}
	}
	if h != nil {
		for _, cmd := range []string{"show bgp summary", "show bgp ipv4 unicast", "show bgp ipv6 unicast", "show bfd peers brief"} {
			if out, err := h.torVtysh(ctx, cmd); err == nil {
				e2e.Logf("ToR %q:\n%s", cmd, redactBGPVIPPasswords(out))
			}
		}
		if out, err := h.run(ctx, time.Minute, "nft list table bridge bgp_vip_e2e 2>/dev/null || true; nft list table inet bgp_vip_e2e 2>/dev/null || true"); err == nil && out != "" {
			e2e.Logf("hypervisor nftables:\n%s", out)
		}
	}
}

// registerBGPVIPDump dumps the BGP VIP state when the current spec fails,
// before any DeferCleanup registered later restores the cluster.
func registerBGPVIPDump(oc *exutil.CLI, h func() *bgpVIPHypervisor) {
	g.AfterEach(func(ctx context.Context) {
		if g.CurrentSpecReport().Failed() {
			dumpBGPVIPState(context.Background(), oc, h())
		}
	})
}
