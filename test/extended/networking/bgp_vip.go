package networking

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
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
	// test 5 budget within the suite timeout (20m): the body (11m), the
	// failure dump (bgpVIPDumpTimeout, 2m), the cleanup (6m: two hypervisor
	// steps of bgpVIPIngressMoveHostStep and the node restoration of
	// bgpVIPIngressMoveRestoreBudget) and a 1m margin.
	bgpVIPIngressMoveBodyTimeout   = 11 * time.Minute
	bgpVIPIngressMoveHostStep      = 30 * time.Second
	bgpVIPIngressMoveRestoreBudget = 5 * time.Minute
	// bgpVIPIngressExposureMin is the number of successful probe requests of
	// every address family that the moved router node must have carried
	// before the disruption; bgpVIPIngressExposureWindow bounds the wait for
	// them.
	bgpVIPIngressExposureMin    = 5
	bgpVIPIngressExposureWindow = 30 * time.Second
	// bgpVIPIngressSampleInterval paces the exposure and ToR withdrawal
	// sampling, which time events against the 1/s probe.
	bgpVIPIngressSampleInterval = time.Second
	// bgpVIPIngressObservationWindow is how long the probe runs after the
	// ToR converged and after the restoration.
	bgpVIPIngressObservationWindow = 30 * time.Second
)

// bgpVIPIngressMovePhaseNames are the phases of the ingress VIP move, split at
// the disruption, the ToR's convergence on the remaining router nodes, and
// the start and end of the restoration.
var bgpVIPIngressMovePhaseNames = []string{"exposure", "withdrawal", "moved", "restoration", "post-restore"}

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
		families := bgpVIPFamilies(ingressVIPs)

		g.By("checking that the hypervisor reaches every ingress VIP over a BGP route via a router node")
		for _, vip := range ingressVIPs {
			err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "see a BGP route to ingress VIP "+vip+" on the hypervisor", func(ctx context.Context) error {
				return checkHypervisorBGPRoute(ctx, h, vip, routerNodes, nodeIPs)
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		// Registered before the first change to the hypervisor or the
		// cluster, it undoes whichever of them happened, on every path, each
		// step within its own budget.
		state := &bgpVIPIngressMoveState{}
		g.DeferCleanup(func() {
			o.Expect(runBGPVIPCleanup(context.Background(), state.cleanupSteps(bgpVIPIngressMoveActions{
				stopProbe: func(ctx context.Context) error {
					samples, err := state.probe.stopAndRemove(ctx, bgpVIPIngressMoveHostStep)
					for _, line := range describeBGPVIPProbeSamples(samples, 20) {
						e2e.Logf("cleanup: %s", line)
					}
					return err
				},
				restoreHost: func(ctx context.Context, pathsRecorded, hashPolicySet bool) error {
					_, err := h.run(ctx, bgpVIPIngressMoveHostStep, bgpVIPIngressHostCleanupScript(pathsRecorded, hashPolicySet))
					return err
				},
				restoreNode: func(ctx context.Context, node string, unschedulable bool) error {
					return restoreRouterNode(ctx, oc, node, unschedulable, peers, ingressVIPs)
				},
			}))).To(o.Succeed())
		})

		probeURL := bgpVIPIngressProbeURL(bodyCtx, oc)
		_, probePort, err := parseBGPVIPURL(probeURL)
		o.Expect(err).NotTo(o.HaveOccurred())
		var routers []bgpVIPExposureRouter
		for _, node := range sets.List(routerNodes) {
			macs := sets.New[string]()
			for ip, name := range nodeIPs {
				if name != node || !slices.Contains(families, addressFamily(ip)) {
					continue
				}
				mac, err := hypervisorNeighborMAC(bodyCtx, h, ip)
				o.Expect(err).NotTo(o.HaveOccurred())
				macs.Insert(mac)
			}
			o.Expect(macs.Len()).NotTo(o.BeZero(), "router node %s has no InternalIP of families %v", node, families)
			routers = append(routers, bgpVIPExposureRouter{Node: node, MACs: sets.List(macs)})
		}
		e2e.Logf("router nodes and their MAC addresses on the hypervisor: %+v", routers)
		var policy strings.Builder
		for _, f := range families {
			fmt.Fprintf(&policy, "sysctl net.%s.fib_multipath_hash_policy\n", f)
		}
		out, err := h.run(bodyCtx, time.Minute, policy.String())
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the hypervisor's own multipath hash policy: %s", strings.Join(strings.Fields(out), " "))
		// The probe's source ports tell its requests apart only if the
		// hypervisor never assigns them to another connection by itself.
		out, err = h.run(bodyCtx, time.Minute, "sysctl -n net.ipv4.ip_local_port_range")
		o.Expect(err).NotTo(o.HaveOccurred(), "coverage: cannot read the hypervisor's ephemeral port range, so the probe's source ports may not identify its requests")
		e2e.Logf("the hypervisor's ephemeral port range: %s", strings.Join(strings.Fields(out), "-"))
		o.Expect(checkBGPVIPProbePorts(out, bgpVIPIngressProbePorts)).To(o.Succeed(), "coverage: the probe's source ports may not identify its requests")

		g.By("recording on the hypervisor the router node that every probe SYN is sent to")
		state.pathsRecorded = true
		_, err = h.run(bodyCtx, time.Minute, bgpVIPDeleteNftTablesScript+"nft -f - <<'EOF'\n"+bgpVIPProbePathNftScript(routers, ingressVIPs, probePort, bgpVIPIngressProbePorts)+"EOF\n")
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("probing " + probeURL + " through every ingress VIP from the hypervisor")
		probe := newBGPVIPProbe(h, "ingress")
		state.probe = probe
		o.Expect(probe.start(bodyCtx, probeURL, ingressVIPs, 1, &bgpVIPIngressProbePorts)).To(o.Succeed())
		observe := func(ctx context.Context) (bgpVIPProbeObservation, error) {
			return h.observeProbePaths(ctx, probe.dir()+"/log", routers, families, bgpVIPIngressProbePorts)
		}

		// Move the router pod off the node that the probe requests of every
		// family actually reach under the hypervisor's own multipath hash
		// policy. Only when that policy sends the families to different
		// nodes, hash by port, as the API VIP failover test does, and
		// measure again.
		g.By("finding a router node that carries probe requests of every address family " + strings.Join(families, ", "))
		victim, exposureFrom, exposure, err := waitBGPVIPExposedRouter(bodyCtx, h, observe, families)
		if err != nil && exposure != nil {
			e2e.Logf("under the hypervisor's own multipath hash policy: %v; hashing the multipath routes of %v by port and measuring again", err, families)
			state.hashPolicySet = true
			_, err = h.run(bodyCtx, time.Minute, bgpVIPSetHashPolicyFamiliesScript(families, bgpVIPIngressHashPolicyFile))
			o.Expect(err).NotTo(o.HaveOccurred(), "coverage: cannot hash the hypervisor's multipath routes by port")
			victim, exposureFrom, _, err = waitBGPVIPExposedRouter(bodyCtx, h, observe, families)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "coverage: no router node is proven to carry probe requests of every ingress VIP family, so moving its router pod would not test traffic exposure")

		var victimNode *corev1.Node
		err = bgpVIPEventually(bodyCtx, time.Minute, "read node "+victim, func(ctx context.Context) error {
			var err error
			victimNode, err = oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, victim, metav1.GetOptions{})
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		state.victim, state.wasUnschedulable = victim, victimNode.Spec.Unschedulable

		disrupted, err := observe(bodyCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		preExposure := bgpVIPExposure(attributeBGPVIPProbe(disrupted.Samples, disrupted.Blocks, bgpVIPIngressProbePorts), exposureFrom, disrupted.Time)
		for _, f := range families {
			e2e.Logf("before the disruption, %s carried %d successful %s probe requests in the %.1fs measured (successful requests by router node: %v; probe SYNs sent by router node since the probe started, retransmissions included: %v)",
				victim, preExposure[victim][f], f, disrupted.Time-exposureFrom, preExposure, disrupted.SYNs)
			o.Expect(preExposure[victim][f]).To(o.BeNumerically(">=", bgpVIPIngressExposureMin), "coverage: %s carried too few %s probe requests before the disruption", victim, f)
		}

		g.By("cordoning " + victim + " and deleting its router pod")
		o.Expect(setNodeUnschedulable(bodyCtx, oc, victim, true)).To(o.Succeed())
		pods, err := oc.AdminKubeClient().CoreV1().Pods(bgpVIPRouterNamespace).List(bodyCtx, metav1.ListOptions{LabelSelector: bgpVIPRouterSelector, FieldSelector: "spec.nodeName=" + victim})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(pods.Items).NotTo(o.BeEmpty(), "coverage: no router pod to delete on %s", victim)
		grace := int64(30)
		for _, pod := range pods.Items {
			o.Expect(oc.AdminKubeClient().CoreV1().Pods(bgpVIPRouterNamespace).Delete(bodyCtx, pod.Name, metav1.DeleteOptions{GracePeriodSeconds: &grace})).To(o.Succeed())
		}

		// Sampled every second, between two readings of the hypervisor clock
		// that stamps the probe samples, so whatever a look at the ToR shows
		// happened before the reading after it. The replacement router pod
		// may land on a node outside routerNodes, so the expected advertisers
		// are recomputed on every poll.
		victimSeen, victimGone, convergedBy := map[string]float64{}, map[string]float64{}, map[string]float64{}
		var converged float64
		err = bgpVIPEventuallyEvery(bodyCtx, bgpVIPIngressSampleInterval, bgpVIPConvergeTimeout, fmt.Sprintf("see the ToR's paths to %v from the Ready router nodes other than %s only", ingressVIPs, victim), func(ctx context.Context) error {
			before, err := h.clock(ctx)
			if err != nil {
				return err
			}
			want, err := bgpVIPRouterNodes(ctx, oc)
			if err != nil {
				return err
			}
			want.Delete(victim)
			if want.Len() == 0 {
				return fmt.Errorf("no Ready router pod outside %s", victim)
			}
			var gone, done, pending []string
			for _, vip := range ingressVIPs {
				if _, ok := convergedBy[vip]; ok {
					continue
				}
				peers, err := h.torPathPeers(ctx, vip)
				if err != nil {
					return err
				}
				if _, ok := victimGone[vip]; !ok {
					if bgpVIPTorPathsVia(peers, victim, nodeIPs) {
						victimSeen[vip] = before
					} else {
						gone = append(gone, vip)
					}
				}
				if err := compareTorPaths(vip, peers, want, nodeIPs); err != nil {
					pending = append(pending, err.Error())
					continue
				}
				done = append(done, vip)
			}
			after, err := h.clock(ctx)
			if err != nil {
				return err
			}
			for _, vip := range gone {
				victimGone[vip] = after
			}
			for _, vip := range done {
				convergedBy[vip] = after
			}
			if len(pending) > 0 {
				return fmt.Errorf("%s", strings.Join(pending, "; "))
			}
			converged = after
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "convergence: the ToR did not converge on the ingress VIP paths of the router nodes other than %s", victim)
		for _, vip := range ingressVIPs {
			seen := "not seen after the disruption"
			if t, ok := victimSeen[vip]; ok {
				seen = fmt.Sprintf("last seen at %+.1fs", t-disrupted.Time)
			}
			e2e.Logf("the ToR's paths to %s (%s), hypervisor clock relative to the disruption: %s's path %s, gone by %+.1fs; only the remaining router nodes' paths by %+.1fs",
				vip, addressFamily(vip), victim, seen, victimGone[vip]-disrupted.Time, convergedBy[vip]-disrupted.Time)
		}

		g.By("letting the probe run with the VIP served by the remaining router nodes")
		time.Sleep(bgpVIPIngressObservationWindow)

		g.By("restoring the schedulability of " + victim + " with the probe running")
		restoring, err := h.clock(bodyCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(restoreRouterNode(bodyCtx, oc, victim, state.wasUnschedulable, peers, ingressVIPs)).To(o.Succeed(), "convergence: the router and its ingress VIP advertisers did not recover after %s was restored", victim)
		restored, err := h.clock(bodyCtx)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("letting the probe run with the restored ingress VIP advertisers")
		time.Sleep(bgpVIPIngressObservationWindow)
		samples, err := probe.stopAndRemove(bodyCtx, time.Minute)
		var final bgpVIPProbeObservation
		if err == nil {
			state.probe = nil
			final, err = observe(bodyCtx)
		}
		if err != nil {
			for _, line := range describeBGPVIPProbeSamples(samples, 20) {
				e2e.Logf("%s", line)
			}
		}
		o.Expect(err).NotTo(o.HaveOccurred())

		// Every sample, including those still in flight when the probe
		// stopped, falls into exactly one phase.
		phases, err := bgpVIPProbePhases(bgpVIPIngressMovePhaseNames, []float64{disrupted.Time, converged, restoring, restored})
		o.Expect(err).NotTo(o.HaveOccurred())
		perPhase, overall := summarizeBGPVIPPhases(phases, attributeBGPVIPProbe(samples, final.Blocks, bgpVIPIngressProbePorts))
		e2e.Logf("probe SYNs sent by router node over the whole run, retransmissions included: %v", final.SYNs)
		for _, p := range phases {
			for _, vip := range ingressVIPs {
				e2e.Logf("%s", describeBGPVIPPhaseStats("phase "+p.Name, vip, perPhase[p.Name][vip], disrupted.Time))
			}
		}
		for _, vip := range ingressVIPs {
			e2e.Logf("%s", describeBGPVIPPhaseStats("whole run", vip, overall[vip], disrupted.Time))
		}

		g.By("checking that " + victim + " has its original schedulability and every ingress VIP its healthy advertisers")
		err = bgpVIPEventually(bodyCtx, time.Minute, "see "+victim+" restored and exactly the nodes with a Ready router pod advertise every ingress VIP", func(ctx context.Context) error {
			node, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, victim, metav1.GetOptions{})
			if err != nil {
				return err
			}
			if node.Spec.Unschedulable != state.wasUnschedulable {
				return fmt.Errorf("node %s is unschedulable=%t, want %t", victim, node.Spec.Unschedulable, state.wasUnschedulable)
			}
			return checkIngressAdvertisers(ctx, oc, peers, ingressVIPs)
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "convergence: the router and its ingress VIP advertisers are not healthy at the end")
		state.restored = true

		for _, p := range []string{"moved", "post-restore"} {
			for _, vip := range ingressVIPs {
				o.Expect(perPhase[p][vip]).NotTo(o.BeNil(), "coverage: the probe via %s did not run in phase %s", vip, p)
				o.Expect(perPhase[p][vip].Total).To(o.BeNumerically(">", 10), "coverage: the probe via %s barely ran in phase %s", vip, p)
			}
		}
		// The probe's log only holds this probe's samples, stamped with the
		// hypervisor's clock, so all of them are counted, including those
		// sent while the router pod returned and the VIP was re-advertised.
		stats := summarizeBGPVIPProbe(samples, 0, math.Inf(1))
		for _, vip := range ingressVIPs {
			st := stats[vip]
			e2e.Logf("ingress probe via %s (%s): %d requests, %d failed", vip, addressFamily(vip), st.Total, st.Failed)
			o.Expect(st.Total).To(o.BeNumerically(">", 10), "the probe via %s barely ran", vip)
			o.Expect(st.Failed).To(o.BeNumerically("<=", bgpVIPIngressProbeBudget), "traffic: ingress traffic via %s (%s) failed %d times while the router pod moved off %s and back", vip, addressFamily(vip), st.Failed, victim)
		}
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
	return compareTorPaths(vip, peers, want, nodeIPs)
}

// compareTorPaths checks that the ToR paths peers to vip come from exactly
// the nodes want, one path each.
func compareTorPaths(vip string, peers []string, want sets.Set[string], nodeIPs map[string]string) error {
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
	u, port, err := parseBGPVIPURL(rawURL)
	if err != nil {
		return err
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

// bgpVIPIngressMoveState records what the ingress VIP move changed, so that
// its cleanup undoes exactly that. Each field is set before the change is
// attempted, so a change that fails halfway is undone too.
type bgpVIPIngressMoveState struct {
	// probe is set before the probe creates its files on the hypervisor and
	// cleared once the test has stopped it and removed them.
	probe         *bgpVIPProbe
	pathsRecorded bool
	hashPolicySet bool
	// victim is the node cordoned for the move, with its original
	// schedulability; restored is set once the test itself has seen the node
	// restored and the ingress VIP advertisers healthy at its end.
	victim           string
	wasUnschedulable bool
	restored         bool
}

type bgpVIPIngressMoveActions struct {
	stopProbe   func(ctx context.Context) error
	restoreHost func(ctx context.Context, pathsRecorded, hashPolicySet bool) error
	restoreNode func(ctx context.Context, node string, unschedulable bool) error
}

// cleanupSteps returns the steps that undo s, each with its own budget:
// first the quick hypervisor steps, then the node restoration, which waits
// for the router and the ingress VIP advertisers to recover.
func (s *bgpVIPIngressMoveState) cleanupSteps(a bgpVIPIngressMoveActions) []bgpVIPCleanupStep {
	var steps []bgpVIPCleanupStep
	if s.probe != nil {
		steps = append(steps, bgpVIPCleanupStep{Name: "stop the ingress probe and remove its files", Timeout: bgpVIPIngressMoveHostStep, Run: a.stopProbe})
	}
	if s.pathsRecorded || s.hashPolicySet {
		pathsRecorded, hashPolicySet := s.pathsRecorded, s.hashPolicySet
		var parts []string
		if pathsRecorded {
			parts = append(parts, "delete the probe path records")
		}
		if hashPolicySet {
			parts = append(parts, "restore the multipath hash policy")
		}
		steps = append(steps, bgpVIPCleanupStep{
			Name:    strings.Join(parts, " and "),
			Timeout: bgpVIPIngressMoveHostStep,
			Run:     func(ctx context.Context) error { return a.restoreHost(ctx, pathsRecorded, hashPolicySet) },
		})
	}
	if s.victim != "" && !s.restored {
		node, unschedulable := s.victim, s.wasUnschedulable
		steps = append(steps, bgpVIPCleanupStep{
			Name:    fmt.Sprintf("restore node %s (unschedulable=%t) and the ingress VIP advertisers", node, unschedulable),
			Timeout: bgpVIPIngressMoveRestoreBudget,
			Run:     func(ctx context.Context) error { return a.restoreNode(ctx, node, unschedulable) },
		})
	}
	return steps
}

// clock reads the hypervisor clock, which stamps the probe samples.
func (h *bgpVIPHypervisor) clock(ctx context.Context) (float64, error) {
	out, err := h.run(ctx, time.Minute, "date +%s.%N\n")
	if err != nil {
		return 0, err
	}
	return parseHypervisorClock(out)
}

// observeProbePaths reads the probe log and the recorded probe paths.
func (h *bgpVIPHypervisor) observeProbePaths(ctx context.Context, logFile string, routers []bgpVIPExposureRouter, families []string, ports bgpVIPProbePorts) (bgpVIPProbeObservation, error) {
	out, err := h.run(ctx, time.Minute, bgpVIPObservationScript(logFile))
	if err != nil {
		return bgpVIPProbeObservation{}, err
	}
	return parseBGPVIPProbeObservation(out, routers, families, ports)
}

// waitBGPVIPExposedRouter observes the probe every second for up to
// bgpVIPIngressExposureWindow and returns the router node that carried
// bgpVIPIngressExposureMin successful probe requests of every family since
// it started, with the hypervisor time it started at. On failure, the
// exposure it last measured is returned when the observations themselves
// worked, and nil otherwise.
func waitBGPVIPExposedRouter(ctx context.Context, h *bgpVIPHypervisor, observe func(context.Context) (bgpVIPProbeObservation, error), families []string) (string, float64, map[string]map[string]int, error) {
	from, err := h.clock(ctx)
	if err != nil {
		return "", 0, nil, err
	}
	var victim string
	var exposure map[string]map[string]int
	var syns map[string]map[string]int64
	what := fmt.Sprintf("see a router node carry %d successful probe requests of every family %v", bgpVIPIngressExposureMin, families)
	err = bgpVIPEventuallyEvery(ctx, bgpVIPIngressSampleInterval, bgpVIPIngressExposureWindow, what, func(ctx context.Context) error {
		obs, err := observe(ctx)
		if err != nil {
			exposure = nil
			return err
		}
		exposure = bgpVIPExposure(attributeBGPVIPProbe(obs.Samples, obs.Blocks, bgpVIPIngressProbePorts), from, math.Inf(1))
		syns = obs.SYNs
		var ok bool
		if victim, ok = selectBGPVIPExposedRouter(exposure, families, bgpVIPIngressExposureMin); !ok {
			return fmt.Errorf("successful probe requests by router node and family: %v", exposure)
		}
		return nil
	})
	if err != nil {
		if exposure == nil {
			return "", from, nil, fmt.Errorf("cannot observe the probe's paths: %w", err)
		}
		return "", from, exposure, fmt.Errorf("no router node carried %d successful probe requests of every family %v: %w (probe SYNs by router node, retransmissions included: %v)",
			bgpVIPIngressExposureMin, families, err, syns)
	}
	e2e.Logf("successful probe requests by router node and family: %v, probe SYNs by router node, retransmissions included: %v; moving the router pod off %s", exposure, syns, victim)
	return victim, from, exposure, nil
}
