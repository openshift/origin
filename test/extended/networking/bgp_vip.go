package networking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	bgpVIPRouterDeployment = "router-default"
	// bgpVIPIngressProbeBudget is the number of failed probe requests per
	// ingress VIP tolerated while a router pod moves away from its node.
	bgpVIPIngressProbeBudget = 3
	// test 5 body budget: the suite timeout (20m) minus the failure dump
	// (bgpVIPDumpTimeout, 2m), the cleanup budget (5m) and a 2m margin.
	bgpVIPIngressMoveBodyTimeout = 11 * time.Minute
	bgpVIPIngressMoveCleanup     = 5 * time.Minute
)

var _ = g.Describe(bgpVIPTags+" BGP VIP management", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("bgp-vip", admissionapi.LevelPrivileged)

	var (
		infra *configv1.Infrastructure
		h     *bgpVIPHypervisor
	)
	g.BeforeEach(func(ctx context.Context) {
		infra = skipUnlessBGPVIP(ctx, oc)
		h = getBGPVIPHypervisor()
	})
	registerBGPVIPDump(oc, func() *bgpVIPHypervisor { return h })

	g.It("should report BGPVIPConfig rendered and sessions configured for the current generation", func(ctx context.Context) {
		err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see BGPVIPConfig/cluster Rendered and SessionsConfigured for the current generation", func(ctx context.Context) error {
			cr, err := getBGPVIPConfig(ctx, oc)
			if apierrors.IsNotFound(err) {
				return fmt.Errorf("BGPVIPConfig/cluster not found: the payload must ship the CRD and the installer must create the CR: %w", err)
			}
			if err != nil {
				return err
			}
			return checkBGPVIPConditions(cr)
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should run frr-k8s on every control plane node instead of keepalived", func(ctx context.Context) {
		masters := bgpVIPMasters(ctx, oc)
		err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see a Ready frr-k8s static pod on every control plane node", func(ctx context.Context) error {
			var problems []string
			for _, node := range masters {
				pod, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPFRRNamespace).Get(ctx, "frr-k8s-"+node.Name, metav1.GetOptions{})
				if err != nil {
					problems = append(problems, err.Error())
					continue
				}
				if !podReady(pod) {
					problems = append(problems, fmt.Sprintf("pod %s is not Ready", pod.Name))
				}
			}
			if len(problems) > 0 {
				return fmt.Errorf("%s", strings.Join(problems, "; "))
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		pods, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPInfraNamespace).List(ctx, metav1.ListOptions{LabelSelector: "app=" + bgpVIPKeepalivedLabel})
		o.Expect(err).NotTo(o.HaveOccurred())
		var names []string
		for _, pod := range pods.Items {
			names = append(names, pod.Name)
		}
		o.Expect(names).To(o.BeEmpty(), "keepalived must not run when the VIPs are managed by BGP")
	})

	g.It("should establish a BGP session from every control plane node to every configured peer", func(ctx context.Context) {
		peers := bgpVIPDefaultPeers(ctx, oc)
		masters := bgpVIPMasters(ctx, oc)
		logged := sets.New[string]()
		err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see every control plane node's session to every peer Established with the configured timers and BFD", func(ctx context.Context) error {
			var problems []string
			for _, node := range masters {
				for _, peer := range peers {
					n, err := nodeNeighbor(ctx, oc, node.Name, peer.Address)
					if err != nil {
						problems = append(problems, err.Error())
						continue
					}
					problems = append(problems, checkBGPVIPNeighbor(node.Name, peer, n, true)...)
					key := node.Name + "/" + peer.Address
					if n.BGPState == "Established" && !logged.Has(key) {
						logged.Insert(key)
						e2e.Logf("%s -> %s: Established, hold %dms (configured %dms), keepalive %dms (configured %dms), BFD %q",
							node.Name, peer.Address, n.NegotiatedHoldTimeMsecs, n.ConfiguredHoldTimeMsecs, n.NegotiatedKeepAliveMsecs, n.ConfiguredKeepAliveMsecs, n.bfdStatus())
					}
				}
			}
			if len(problems) > 0 {
				return fmt.Errorf("%s", strings.Join(problems, "; "))
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should advertise the API and ingress VIPs of every peered address family", func(ctx context.Context) {
		peers := bgpVIPDefaultPeers(ctx, oc)
		bm := infra.Status.PlatformStatus.BareMetal
		apiVIPs := vipsWithPeerFamily(bm.APIServerInternalIPs, peers)
		ingressVIPs := vipsWithPeerFamily(bm.IngressIPs, peers)
		o.Expect(apiVIPs).NotTo(o.BeEmpty(), "no API VIP has a BGP peer of its address family")
		o.Expect(ingressVIPs).NotTo(o.BeEmpty(), "no ingress VIP has a BGP peer of its address family")
		masters := bgpVIPMasters(ctx, oc)

		err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see every control plane node advertise every API VIP to every peer of its family", func(ctx context.Context) error {
			var problems []string
			for _, vip := range apiVIPs {
				for _, peer := range peersOfFamily(peers, isIPv6(vip)) {
					for _, node := range masters {
						ok, err := nodeAdvertises(ctx, oc, node.Name, peer.Address, hostPrefix(vip))
						if err != nil {
							problems = append(problems, err.Error())
						} else if !ok {
							problems = append(problems, fmt.Sprintf("%s does not advertise API VIP %s to %s", node.Name, hostPrefix(vip), peer.Address))
						}
					}
				}
			}
			if len(problems) > 0 {
				return fmt.Errorf("%s", strings.Join(problems, "; "))
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		err = bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see exactly the nodes with a Ready router pod advertise every ingress VIP", func(ctx context.Context) error {
			return checkIngressAdvertisers(ctx, oc, peers, ingressVIPs)
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.It("should move the ingress VIP advertisement with the router pods [Serial][Requires:HypervisorSSHConfig]", func(ctx context.Context) {
		h := requireBGPVIPHypervisor()
		bodyCtx, cancel := context.WithTimeout(ctx, bgpVIPIngressMoveBodyTimeout)
		defer cancel()

		peers := bgpVIPDefaultPeers(bodyCtx, oc)
		ingressVIPs := vipsWithPeerFamily(infra.Status.PlatformStatus.BareMetal.IngressIPs, peers)
		o.Expect(ingressVIPs).NotTo(o.BeEmpty(), "no ingress VIP has a BGP peer of its address family")

		// Skip only when the router is not meant to run on 2 nodes; a router
		// that wants 2 replicas but has fewer Ready is waited for.
		var routerNodes sets.Set[string]
		var routerReplicas int32
		err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "find the nodes with Ready router pods", func(ctx context.Context) error {
			d, err := oc.AdminKubeClient().AppsV1().Deployments(bgpVIPRouterNamespace).Get(ctx, bgpVIPRouterDeployment, metav1.GetOptions{})
			if err != nil {
				return err
			}
			routerReplicas = 1
			if d.Spec.Replicas != nil {
				routerReplicas = *d.Spec.Replicas
			}
			routerNodes, err = bgpVIPRouterNodes(ctx, oc)
			if err != nil {
				return err
			}
			if routerReplicas >= 2 && routerNodes.Len() < 2 {
				return fmt.Errorf("%s wants %d replicas but only nodes %v have a Ready router pod", bgpVIPRouterDeployment, routerReplicas, sets.List(routerNodes))
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		if routerNodes.Len() < 2 {
			e2eskipper.Skipf("need router pods on at least 2 nodes, %s wants %d replicas", bgpVIPRouterDeployment, routerReplicas)
		}
		nodeIPs := bgpVIPNodeIPs(bodyCtx, oc)

		g.By("checking that the hypervisor reaches every ingress VIP over a BGP route via a router node")
		for _, vip := range ingressVIPs {
			err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "see a BGP route to ingress VIP "+vip+" on the hypervisor", func(ctx context.Context) error {
				return checkHypervisorBGPRoute(ctx, h, vip, routerNodes, nodeIPs)
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		probeURL := bgpVIPIngressProbeURL(bodyCtx, oc)
		g.By("probing " + probeURL + " through every ingress VIP from the hypervisor")
		probe, err := startBGPVIPProbe(bodyCtx, h, "ingress", probeURL, ingressVIPs, 1)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_, err := probe.stop(ctx)
			o.Expect(err).NotTo(o.HaveOccurred(), "stopping the ingress probe on the hypervisor")
		})

		victim := sets.List(routerNodes)[0]
		var victimNode *corev1.Node
		err = bgpVIPEventually(bodyCtx, time.Minute, "read node "+victim, func(ctx context.Context) error {
			var err error
			victimNode, err = oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, victim, metav1.GetOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		wasUnschedulable := victimNode.Spec.Unschedulable
		g.By("cordoning " + victim + " and deleting its router pod")
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), bgpVIPIngressMoveCleanup)
			defer cancel()
			o.Expect(restoreRouterNode(ctx, oc, victim, wasUnschedulable, peers, ingressVIPs)).To(o.Succeed())
		})
		o.Expect(setNodeUnschedulable(bodyCtx, oc, victim, true)).To(o.Succeed())
		pods, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPRouterNamespace).List(bodyCtx, metav1.ListOptions{LabelSelector: bgpVIPRouterSelector, FieldSelector: "spec.nodeName=" + victim})
		o.Expect(err).NotTo(o.HaveOccurred())
		grace := int64(30)
		for _, pod := range pods.Items {
			o.Expect(oc.AdminKubeClient().CoreV1().Pods(bgpVIPRouterNamespace).Delete(bodyCtx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace})).To(o.Succeed())
		}

		// The replacement router pod may land on a node outside routerNodes,
		// so the expected advertisers are recomputed on every poll.
		for _, vip := range ingressVIPs {
			err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, fmt.Sprintf("see the ToR's paths to %s from the Ready router nodes other than %s only", vip, victim), func(ctx context.Context) error {
				want, err := bgpVIPRouterNodes(ctx, oc)
				if err != nil {
					return err
				}
				want.Delete(victim)
				if want.Len() == 0 {
					return fmt.Errorf("no Ready router pod outside %s", victim)
				}
				return checkTorPaths(ctx, h, vip, want, nodeIPs)
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
		g.By("letting the probe run with the VIP served by the remaining router nodes")
		time.Sleep(30 * time.Second)

		samples, err := probe.stop(bodyCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		// The probe's log only holds this probe's samples, stamped with the
		// hypervisor's clock, so all of them are counted.
		stats := summarizeBGPVIPProbe(samples, 0, math.Inf(1))
		for _, vip := range ingressVIPs {
			st := stats[vip]
			e2e.Logf("ingress probe via %s: %d requests, %d failed", vip, st.Total, st.Failed)
			o.Expect(st.Total).To(o.BeNumerically(">", 10), "the probe via %s barely ran", vip)
			o.Expect(st.Failed).To(o.BeNumerically("<=", bgpVIPIngressProbeBudget), "ingress traffic via %s failed %d times while its router pod moved", vip, st.Failed)
		}

		g.By("restoring the schedulability of " + victim)
		o.Expect(restoreRouterNode(bodyCtx, oc, victim, wasUnschedulable, peers, ingressVIPs)).To(o.Succeed())
	})

	g.It("should serve API and ingress traffic through the VIPs of every peered address family [Requires:HypervisorSSHConfig]", func(ctx context.Context) {
		h := requireBGPVIPHypervisor()
		peers := bgpVIPDefaultPeers(ctx, oc)
		bm := infra.Status.PlatformStatus.BareMetal
		apiURL := strings.TrimSuffix(infra.Status.APIServerURL, "/") + "/readyz"
		ingressURL := bgpVIPIngressProbeURL(ctx, oc)
		for _, check := range []struct {
			url  string
			vips []string
		}{
			{url: apiURL, vips: vipsWithPeerFamily(bm.APIServerInternalIPs, peers)},
			{url: ingressURL, vips: vipsWithPeerFamily(bm.IngressIPs, peers)},
		} {
			o.Expect(check.vips).NotTo(o.BeEmpty(), "no VIP for %s has a BGP peer of its address family", check.url)
			for _, vip := range check.vips {
				err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, fmt.Sprintf("get HTTP 200 from %s via %s on the hypervisor", check.url, vip), func(ctx context.Context) error {
					return hypervisorGet(ctx, h, check.url, vip)
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}
	})
})

func podReady(pod *corev1.Pod) bool {
	for _, c := range pod.Status.Conditions {
		if c.Type == corev1.PodReady {
			return c.Status == corev1.ConditionTrue
		}
	}
	return false
}

func bgpVIPMasters(ctx context.Context, oc *exutil.CLI) []corev1.Node {
	var nodes *corev1.NodeList
	err := bgpVIPEventually(ctx, time.Minute, "list control plane nodes", func(ctx context.Context) error {
		var err error
		nodes, err = oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master"})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(nodes.Items).NotTo(o.BeEmpty())
	sort.Slice(nodes.Items, func(i, j int) bool { return nodes.Items[i].Name < nodes.Items[j].Name })
	return nodes.Items
}

func bgpVIPDefaultPeers(ctx context.Context, oc *exutil.CLI) []bgpVIPPeer {
	var peers []bgpVIPPeer
	err := bgpVIPEventually(ctx, time.Minute, "read BGPVIPConfig/cluster", func(ctx context.Context) error {
		cr, err := getBGPVIPConfig(ctx, oc)
		if err != nil {
			return err
		}
		peers, err = bgpVIPPeersFromConfig(cr)
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(peers).NotTo(o.BeEmpty(), "BGPVIPConfig/cluster declares no default peer")
	return peers
}

// checkBGPVIPNeighbor compares a node's session to peer with the CR. FRR
// negotiates the lower of both sides' hold times, so only the configured
// timers must equal the CR. With checkNegotiated, the negotiated hold time
// must not exceed it either; FRR only renegotiates when a session restarts,
// so that does not hold right after a timer change.
func checkBGPVIPNeighbor(node string, peer bgpVIPPeer, n frrNeighbor, checkNegotiated bool) []string {
	var problems []string
	if n.BGPState != "Established" {
		return []string{fmt.Sprintf("%s -> %s: state %q, want Established", node, peer.Address, n.BGPState)}
	}
	if peer.HoldTimeSeconds != 0 {
		want := peer.HoldTimeSeconds * 1000
		if n.ConfiguredHoldTimeMsecs != want {
			problems = append(problems, fmt.Sprintf("%s -> %s: configured hold time %dms, want %dms", node, peer.Address, n.ConfiguredHoldTimeMsecs, want))
		}
		if checkNegotiated && n.NegotiatedHoldTimeMsecs > want {
			problems = append(problems, fmt.Sprintf("%s -> %s: negotiated hold time %dms exceeds the configured %dms", node, peer.Address, n.NegotiatedHoldTimeMsecs, want))
		}
	}
	if peer.KeepaliveTimeSeconds != 0 {
		if want := peer.KeepaliveTimeSeconds * 1000; n.ConfiguredKeepAliveMsecs != want {
			problems = append(problems, fmt.Sprintf("%s -> %s: configured keepalive %dms, want %dms", node, peer.Address, n.ConfiguredKeepAliveMsecs, want))
		}
	}
	wantBFD := peer.FailureDetection == "BFD"
	if gotBFD := n.bfdStatus() == "Up"; gotBFD != wantBFD {
		problems = append(problems, fmt.Sprintf("%s -> %s: BFD status %q with failureDetection %q", node, peer.Address, n.bfdStatus(), peer.FailureDetection))
	}
	return problems
}

func bgpVIPRouterNodes(ctx context.Context, oc *exutil.CLI) (sets.Set[string], error) {
	pods, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPRouterNamespace).List(ctx, metav1.ListOptions{LabelSelector: bgpVIPRouterSelector})
	if err != nil {
		return nil, err
	}
	nodes := sets.New[string]()
	for i := range pods.Items {
		pod := &pods.Items[i]
		if pod.DeletionTimestamp == nil && pod.Spec.NodeName != "" && podReady(pod) {
			nodes.Insert(pod.Spec.NodeName)
		}
	}
	return nodes, nil
}

// checkIngressAdvertisers checks that every ingress VIP is advertised to every
// peer of its family by exactly the nodes with a Ready default router pod.
func checkIngressAdvertisers(ctx context.Context, oc *exutil.CLI, peers []bgpVIPPeer, ingressVIPs []string) error {
	want, err := bgpVIPRouterNodes(ctx, oc)
	if err != nil {
		return err
	}
	if want.Len() == 0 {
		return fmt.Errorf("no node hosts a Ready default router pod")
	}
	var problems []string
	for _, vip := range ingressVIPs {
		for _, peer := range peersOfFamily(peers, isIPv6(vip)) {
			got, err := nodesAdvertising(ctx, oc, peer.Address, hostPrefix(vip))
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			if !got.Equal(want) {
				problems = append(problems, fmt.Sprintf("ingress VIP %s is advertised to %s by %v, want the router nodes %v", vip, peer.Address, sets.List(got), sets.List(want)))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("%s", strings.Join(problems, "; "))
	}
	return nil
}

// nodesAdvertising returns the nodes running frr-k8s that advertise prefix
// to peer.
func nodesAdvertising(ctx context.Context, oc *exutil.CLI, peer, prefix string) (sets.Set[string], error) {
	nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := sets.New[string]()
	for _, node := range nodes.Items {
		if _, err := frrPodOnNode(ctx, oc, node.Name); err != nil {
			if errors.Is(err, errNoFRRPod) {
				continue
			}
			return nil, err
		}
		ok, err := nodeAdvertises(ctx, oc, node.Name, peer, prefix)
		if err != nil {
			// a node with no session to this peer advertises nothing to it
			if strings.Contains(err.Error(), "is not JSON") || strings.Contains(err.Error(), "Unknown neighbor") {
				continue
			}
			return nil, err
		}
		if ok {
			out.Insert(node.Name)
		}
	}
	return out, nil
}

// bgpVIPNodeIPs maps every node InternalIP to its node name.
func bgpVIPNodeIPs(ctx context.Context, oc *exutil.CLI) map[string]string {
	out := map[string]string{}
	err := bgpVIPEventually(ctx, time.Minute, "list nodes", func(ctx context.Context) error {
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for i := range nodes.Items {
			for _, ip := range nodeInternalIPs(&nodes.Items[i]) {
				out[ip] = nodes.Items[i].Name
			}
		}
		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return out
}

// checkTorPaths checks that the ToR's paths to vip come from exactly the
// given nodes.
func checkTorPaths(ctx context.Context, h *bgpVIPHypervisor, vip string, want sets.Set[string], nodeIPs map[string]string) error {
	peers, err := h.torPathPeers(ctx, vip)
	if err != nil {
		return err
	}
	got := sets.New[string]()
	for _, peer := range peers {
		name, ok := nodeIPs[peer]
		if !ok {
			name = peer
		}
		got.Insert(name)
	}
	if !got.Equal(want) || len(peers) != want.Len() {
		return fmt.Errorf("the ToR has paths to %s from %v (%v), want %v", vip, sets.List(got), peers, sets.List(want))
	}
	return nil
}

// checkHypervisorBGPRoute checks that the hypervisor kernel routes vip over
// the host route the ToR's zebra installed from BGP, via router nodes.
// Without it, traffic to the in-subnet VIP would go over L2 and bypass BGP.
func checkHypervisorBGPRoute(ctx context.Context, h *bgpVIPHypervisor, vip string, via sets.Set[string], nodeIPs map[string]string) error {
	ipFlag := "-4"
	if isIPv6(vip) {
		ipFlag = "-6"
	}
	out, err := h.run(ctx, time.Minute, fmt.Sprintf("ip %s -j route show exact %s", ipFlag, hostPrefix(vip)))
	if err != nil {
		return err
	}
	var routes []struct {
		Gateway  string `json:"gateway"`
		Nexthops []struct {
			Gateway string `json:"gateway"`
		} `json:"nexthops"`
	}
	if err := json.Unmarshal([]byte(out), &routes); err != nil {
		return fmt.Errorf("cannot parse the hypervisor's route to %s: %v: %s", vip, err, out)
	}
	if len(routes) == 0 {
		return fmt.Errorf("the hypervisor has no host route to %s", hostPrefix(vip))
	}
	var gateways []string
	for _, r := range routes {
		if r.Gateway != "" {
			gateways = append(gateways, r.Gateway)
		}
		for _, nh := range r.Nexthops {
			gateways = append(gateways, nh.Gateway)
		}
	}
	if len(gateways) == 0 {
		return fmt.Errorf("the hypervisor's route to %s has no gateway: %s", hostPrefix(vip), out)
	}
	for _, gw := range gateways {
		if !via.Has(nodeIPs[gw]) {
			return fmt.Errorf("the hypervisor routes %s via %s (%s), not via a router node %v", vip, gw, nodeIPs[gw], sets.List(via))
		}
	}
	return nil
}

// bgpVIPIngressProbeURL returns the URL of the ingress canary route, which is
// served by the default router on every cluster.
func bgpVIPIngressProbeURL(ctx context.Context, oc *exutil.CLI) string {
	var host string
	err := bgpVIPEventually(ctx, time.Minute, "read the ingress canary route", func(ctx context.Context) error {
		route, err := oc.AdminRouteClient().RouteV1().Routes("openshift-ingress-canary").Get(ctx, "canary", metav1.GetOptions{})
		if err != nil {
			return err
		}
		host = route.Spec.Host
		if host == "" && len(route.Status.Ingress) > 0 {
			host = route.Status.Ingress[0].Host
		}
		if host == "" {
			return fmt.Errorf("route openshift-ingress-canary/canary has no host")
		}
		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return "https://" + host + "/"
}

// hypervisorGet requests rawURL from the hypervisor with its host resolved to
// vip, as an external client reaching the VIP over the BGP route would.
func hypervisorGet(ctx context.Context, h *bgpVIPHypervisor, rawURL, vip string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return err
	}
	port := u.Port()
	if port == "" {
		port = "443"
	}
	out, err := h.run(ctx, time.Minute, fmt.Sprintf("curl -sk -o /dev/null -w '%%{http_code}' --max-time 20 --noproxy '*' --resolve %s %s || true",
		shellQuote(u.Hostname()+":"+port+":"+curlResolveAddr(vip)), shellQuote(rawURL)))
	if err != nil {
		return err
	}
	if code := strings.TrimSpace(out); code != "200" {
		return fmt.Errorf("%s via %s returned HTTP %q", rawURL, vip, code)
	}
	return nil
}

func setNodeUnschedulable(ctx context.Context, oc *exutil.CLI, node string, unschedulable bool) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"unschedulable":%t}}`, unschedulable))
	return bgpVIPEventually(ctx, time.Minute, fmt.Sprintf("set node %s unschedulable=%t", node, unschedulable), func(ctx context.Context) error {
		_, err := oc.AdminKubeClient().CoreV1().Nodes().Patch(ctx, node, types.MergePatchType, patch, metav1.PatchOptions{})
		return err
	})
}

// restoreRouterNode restores node's original schedulability and waits until
// the default router is fully available again and every ingress VIP is
// advertised by exactly the nodes with a Ready router pod.
func restoreRouterNode(ctx context.Context, oc *exutil.CLI, node string, unschedulable bool, peers []bgpVIPPeer, ingressVIPs []string) error {
	if err := setNodeUnschedulable(ctx, oc, node, unschedulable); err != nil {
		return err
	}
	if err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see the default router fully available", func(ctx context.Context) error {
		d, err := oc.AdminKubeClient().AppsV1().Deployments(bgpVIPRouterNamespace).Get(ctx, bgpVIPRouterDeployment, metav1.GetOptions{})
		if err != nil {
			return err
		}
		want := int32(1)
		if d.Spec.Replicas != nil {
			want = *d.Spec.Replicas
		}
		if d.Status.UpdatedReplicas != want || d.Status.AvailableReplicas != want || d.Status.Replicas != want {
			return fmt.Errorf("router-default has %d/%d/%d updated/available/total replicas, want %d", d.Status.UpdatedReplicas, d.Status.AvailableReplicas, d.Status.Replicas, want)
		}
		return nil
	}); err != nil {
		return err
	}
	return bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see every ingress VIP advertised by exactly the router nodes", func(ctx context.Context) error {
		return checkIngressAdvertisers(ctx, oc, peers, ingressVIPs)
	})
}
