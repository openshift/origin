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
	var last error
	start := time.Now()
	err := wait.PollUntilContextTimeout(ctx, bgpVIPPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
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

func startBGPVIPProbe(ctx context.Context, h *bgpVIPHypervisor, name, rawURL string, vips []string, n int) (*bgpVIPProbe, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	p := &bgpVIPProbe{h: h, name: name}
	var loop strings.Builder
	loop.WriteString("while :; do\n")
	for _, vip := range vips {
		fmt.Fprintf(&loop, "  for i in $(seq %d); do ( t=$(date +%%s.%%N); c=$(curl -sk -o /dev/null -w '%%{http_code}' --max-time 3 --noproxy '*' --resolve %s %s || true); echo \"$t %s ${c:-000}\" >> %s/log ) & done\n",
			n, shellQuote(u.Hostname()+":"+port+":"+curlResolveAddr(vip)), shellQuote(rawURL), vip, p.dir())
	}
	loop.WriteString("  sleep 1\ndone\n")
	script := fmt.Sprintf(`rm -rf %[1]s; mkdir -p %[1]s
cat > %[1]s/loop.sh <<'EOF'
%[2]sEOF
setsid bash %[1]s/loop.sh > /dev/null 2>&1 < /dev/null &
echo $! > %[1]s/pid
`, p.dir(), loop.String())
	if _, err := h.run(ctx, time.Minute, script); err != nil {
		return nil, err
	}
	return p, nil
}

// stop stops the probe and returns its samples. Safe to call more than once.
func (p *bgpVIPProbe) stop(ctx context.Context) ([]bgpVIPProbeSample, error) {
	// Stop the loop first so that it launches no more requests, give the
	// requests in flight (curl --max-time 3) time to log their result, then
	// kill whatever is left of the process group.
	out, err := p.h.run(ctx, time.Minute, fmt.Sprintf(`if [ -f %[1]s/pid ]; then pid=$(cat %[1]s/pid); kill -TERM "$pid" 2>/dev/null || true; sleep 4; kill -TERM -- "-$pid" 2>/dev/null || true; rm -f %[1]s/pid; fi
cat %[1]s/log 2>/dev/null || true`, p.dir()))
	if err != nil {
		return nil, err
	}
	return parseBGPVIPProbeLog(out), nil
}

type bgpVIPProbeSample struct {
	Time float64
	VIP  string
	OK   bool
}

func parseBGPVIPProbeLog(log string) []bgpVIPProbeSample {
	var samples []bgpVIPProbeSample
	for _, line := range strings.Split(log, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 {
			continue
		}
		t, err := strconv.ParseFloat(fields[0], 64)
		if err != nil {
			continue
		}
		samples = append(samples, bgpVIPProbeSample{Time: t, VIP: fields[1], OK: fields[2] == "200"})
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
