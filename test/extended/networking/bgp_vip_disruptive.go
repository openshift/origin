package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
	admissionapi "k8s.io/pod-security-admission/api"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	// Disruptive suite: 30m test timeout = 16m body + 2m failure dump
	// (bgpVIPDumpTimeout) + 10m cleanup + 2m margin.
	bgpVIPDisruptiveBodyTimeout    = 16 * time.Minute
	bgpVIPDisruptiveCleanupTimeout = 10 * time.Minute

	// bgpVIPBFDDeadline bounds how long the ToR may keep a path from a master
	// whose BGP and BFD traffic is cut. BFD detects the failure in about a
	// second; the hold timer (90s in dev-scripts) is the slow fallback.
	bgpVIPBFDDeadline = 45 * time.Second
	// bgpVIPFailoverBudget bounds how long API requests through the API VIP
	// keep failing after a master loses its peering: BFD detection, the
	// withdrawal reaching the hypervisor's kernel, and a margin.
	bgpVIPFailoverBudget = 20 * time.Second
	// bgpVIPAPIProbeRate is the number of new connections per API VIP and
	// second; with port-based multipath hashing they spread over the masters.
	bgpVIPAPIProbeRate = 10
)

// bgpVIPDay2Timers are the hold/keepalive pairs the day-2 test may set, in
// order of preference; all are below the ToR's own timers in dev-scripts.
var bgpVIPDay2Timers = [][2]int64{{30, 10}, {24, 8}, {36, 12}}

var bgpVIPNftCounter = regexp.MustCompile(`packets (\d+)`)

var _ = g.Describe(bgpVIPTags+" BGP VIP management [Disruptive]", func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithPodSecurityLevel("bgp-vip-disruptive", admissionapi.LevelPrivileged)

	var (
		infra *configv1.Infrastructure
		h     *bgpVIPHypervisor
	)
	g.BeforeEach(func(ctx context.Context) {
		infra = skipUnlessBGPVIP(ctx, oc)
		h = getBGPVIPHypervisor()
	})
	registerBGPVIPDump(oc, func() *bgpVIPHypervisor { return h })

	g.It("should fail the API VIP over when a control plane node loses its BGP peering [Requires:HypervisorSSHConfig]", func(ctx context.Context) {
		h := requireBGPVIPHypervisor()
		bodyCtx, cancel := context.WithTimeout(ctx, bgpVIPDisruptiveBodyTimeout)
		defer cancel()

		peers := bgpVIPDefaultPeers(bodyCtx, oc)
		apiVIPs := vipsWithPeerFamily(infra.Status.PlatformStatus.BareMetal.APIServerInternalIPs, peers)
		o.Expect(apiVIPs).NotTo(o.BeEmpty(), "no API VIP has a BGP peer of its address family")
		masters := bgpVIPMasters(bodyCtx, oc)
		if len(masters) < 2 {
			e2eskipper.Skipf("API VIP failover needs at least 2 control plane nodes, found %d", len(masters))
		}
		nodeIPs := bgpVIPNodeIPs(bodyCtx, oc)
		allMasters := sets.New[string]()
		for _, m := range masters {
			allMasters.Insert(m.Name)
		}

		g.By("checking that the ToR has a path to every API VIP from every control plane node")
		for _, vip := range apiVIPs {
			err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "see the ToR's paths to "+vip+" from every control plane node", func(ctx context.Context) error {
				return checkTorPaths(ctx, h, vip, allMasters, nodeIPs)
			})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		victim := masters[0]
		victimIPs := nodeInternalIPs(&victim)
		o.Expect(victimIPs).NotTo(o.BeEmpty(), "node %s has no InternalIP", victim.Name)

		// Without BFD, only the hold timer detects the cut, so both deadlines
		// grow by the longest hold time the victim negotiated with a peer
		// that serves an API VIP family.
		withdrawDeadline, failoverBudget := bgpVIPBFDDeadline, bgpVIPFailoverBudget
		var holdOnly []bgpVIPPeer
		for _, p := range peers {
			if p.FailureDetection == "BFD" {
				continue
			}
			for _, vip := range apiVIPs {
				if isIPv6(vip) == p.IPv6() {
					holdOnly = append(holdOnly, p)
					break
				}
			}
		}
		if len(holdOnly) > 0 {
			var maxHold time.Duration
			err := bgpVIPEventually(bodyCtx, time.Minute, "read "+victim.Name+"'s negotiated hold times", func(ctx context.Context) error {
				maxHold = 0
				for _, p := range holdOnly {
					n, err := nodeNeighbor(ctx, oc, victim.Name, p.Address)
					if err != nil {
						return err
					}
					if n.NegotiatedHoldTimeMsecs <= 0 {
						return fmt.Errorf("%s -> %s negotiated no hold time, so without BFD nothing detects a cut", victim.Name, p.Address)
					}
					maxHold = max(maxHold, time.Duration(n.NegotiatedHoldTimeMsecs)*time.Millisecond)
				}
				return nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			withdrawDeadline += maxHold
			failoverBudget += maxHold
			e2e.Logf("peers without BFD serve the API VIPs; allowing for a %s hold time: withdrawal within %s, failover within %s", maxHold, withdrawDeadline, failoverBudget)
		}
		mac, err := hypervisorNeighborMAC(bodyCtx, h, victimIPs[0])
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("cutting %s (%v, MAC %s) off the ToR", victim.Name, victimIPs, mac)

		// Registered first so that it runs last, after the nftables tables are
		// gone: it restores the hash policy and waits for the peering to recover.
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), bgpVIPDisruptiveCleanupTimeout)
			defer cancel()
			_, err := h.run(ctx, time.Minute, bgpVIPRestoreHashPolicyScript)
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, vip := range apiVIPs {
				err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see the ToR's paths to "+vip+" from every control plane node again", func(ctx context.Context) error {
					return checkTorPaths(ctx, h, vip, allMasters, nodeIPs)
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
			o.Expect(waitBGPVIPAPIStable(ctx, oc)).To(o.Succeed())
		})
		g.By("hashing multipath routes by port on the hypervisor")
		_, err = h.run(bodyCtx, time.Minute, bgpVIPSetHashPolicyScript)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("probing the API through every API VIP from the hypervisor")
		apiURL := strings.TrimSuffix(infra.Status.APIServerURL, "/") + "/readyz"
		probe, err := startBGPVIPProbe(bodyCtx, h, "api", apiURL, apiVIPs, bgpVIPAPIProbeRate)
		o.Expect(err).NotTo(o.HaveOccurred())
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			_, err := probe.stop(ctx)
			o.Expect(err).NotTo(o.HaveOccurred(), "stopping the API probe on the hypervisor")
		})

		// Registered last so that it runs first.
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_, err := h.run(ctx, time.Minute, bgpVIPDeleteNftTablesScript)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("dropping API VIP traffic to " + victim.Name + " on the hypervisor")
		_, err = h.run(bodyCtx, time.Minute, bgpVIPDeleteNftTablesScript+"nft -f - <<'EOF'\n"+bgpVIPBlackholeNftScript(mac, apiVIPs)+"EOF\n")
		o.Expect(err).NotTo(o.HaveOccurred())
		families := sets.New[string]()
		for _, vip := range apiVIPs {
			if isIPv6(vip) {
				families.Insert("v6")
			} else {
				families.Insert("v4")
			}
		}
		err = bgpVIPEventually(bodyCtx, 2*time.Minute, fmt.Sprintf("see probe traffic to %s dropped in families %v", victim.Name, sets.List(families)), func(ctx context.Context) error {
			for _, family := range sets.List(families) {
				n, err := bgpVIPNftCounterPackets(ctx, h, family)
				if err != nil {
					return err
				}
				if n == 0 {
					return fmt.Errorf("the %s black hole counter is still 0", family)
				}
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("cutting BGP and BFD between the ToR and " + victim.Name)
		out, err := h.run(bodyCtx, time.Minute, "nft -f - <<'EOF'\n"+bgpVIPPeeringCutNftScript(victimIPs)+"EOF\ndate +%s.%N\n")
		o.Expect(err).NotTo(o.HaveOccurred())
		cutAt := time.Now()
		// The probe samples carry the hypervisor's clock, so the windows
		// compared with them start at the hypervisor's time of the cut.
		hvCutAt, err := parseHypervisorClock(out)
		o.Expect(err).NotTo(o.HaveOccurred())

		remaining := allMasters.Clone().Delete(victim.Name)
		err = bgpVIPEventually(bodyCtx, withdrawDeadline, fmt.Sprintf("see the ToR withdraw %s's paths to the API VIPs", victim.Name), func(ctx context.Context) error {
			states, err := h.torNeighborStates(ctx)
			if err != nil {
				return err
			}
			for _, ip := range victimIPs {
				if state, ok := states[ip]; ok && state == "Established" {
					return fmt.Errorf("the ToR's session to %s is still Established", ip)
				}
			}
			for _, vip := range apiVIPs {
				if err := checkTorPaths(ctx, h, vip, remaining, nodeIPs); err != nil {
					return err
				}
			}
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("the ToR withdrew %s's paths within %s of the cut", victim.Name, time.Since(cutAt).Round(time.Second))

		g.By("letting the probe run with the API VIPs served by the remaining control plane nodes")
		if wait := failoverBudget + 30*time.Second - time.Since(cutAt); wait > 0 {
			time.Sleep(wait)
		}
		samples, err := probe.stop(bodyCtx)
		o.Expect(err).NotTo(o.HaveOccurred())
		end := math.Inf(1)
		before := summarizeBGPVIPProbe(samples, 0, hvCutAt)
		after := summarizeBGPVIPProbe(samples, hvCutAt, end)
		settled := summarizeBGPVIPProbe(samples, hvCutAt+failoverBudget.Seconds(), end)
		for _, vip := range apiVIPs {
			failover := time.Duration(0)
			if after[vip].Failed > 0 {
				failover = time.Duration((after[vip].LastFailure - hvCutAt) * float64(time.Second))
			}
			e2e.Logf("API probe via %s: before the cut %d/%d failed (black hole); after the cut %d/%d failed, last failure %s after the cut",
				vip, before[vip].Failed, before[vip].Total, after[vip].Failed, after[vip].Total, failover.Round(100*time.Millisecond))
			o.Expect(settled[vip].Total).To(o.BeNumerically(">", 10*bgpVIPAPIProbeRate), "the probe via %s barely ran", vip)
			o.Expect(settled[vip].Failed).To(o.BeZero(), "API requests via %s still failed %s after %s lost its peering (last failure %s after the cut)",
				vip, failoverBudget, victim.Name, failover.Round(100*time.Millisecond))
		}
	})

	g.It("should apply a day-2 peer timer change without disrupting nodes", func(ctx context.Context) {
		bodyCtx, cancel := context.WithTimeout(ctx, bgpVIPDisruptiveBodyTimeout)
		defer cancel()
		mcClient := oc.MachineConfigurationClient().MachineconfigurationV1()

		g.By("checking that every MachineConfigPool is settled")
		var pools []mcfgv1.MachineConfigPool
		err := bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "see every MachineConfigPool settled", func(ctx context.Context) error {
			list, err := mcClient.MachineConfigPools().List(ctx, metav1.ListOptions{})
			if err != nil {
				return err
			}
			for i := range list.Items {
				if err := checkPoolSettled(&list.Items[i], ""); err != nil {
					return err
				}
			}
			pools = list.Items
			return nil
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		savedTargets := map[string]string{}
		for _, pool := range pools {
			savedTargets[pool.Name] = pool.Spec.Configuration.Name
		}
		o.Expect(savedTargets).To(o.HaveKey("master"))

		var saved *unstructured.Unstructured
		err = bgpVIPEventually(bodyCtx, time.Minute, "read BGPVIPConfig/cluster", func(ctx context.Context) error {
			var err error
			saved, err = getBGPVIPConfig(ctx, oc)
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		savedSpec, found, err := unstructured.NestedMap(saved.Object, "spec")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(found).To(o.BeTrue(), "BGPVIPConfig/cluster has no spec")
		savedPeers, err := bgpVIPPeersFromConfig(saved)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(savedPeers).NotTo(o.BeEmpty())
		masters := bgpVIPMasters(bodyCtx, oc)
		bootIDs := bgpVIPBootIDs(bodyCtx, oc)

		cordons := watchBGPVIPCordons(bodyCtx, oc)
		// Only stops the poller if the body fails early; the result is
		// checked at the end of the body.
		defer func() { _, _ = cordons.stop() }()

		var sessionReset bool
		g.DeferCleanup(func() {
			ctx, cancel := context.WithTimeout(context.Background(), bgpVIPDisruptiveCleanupTimeout)
			defer cancel()
			g.By("restoring the original BGPVIPConfig spec")
			cleanupPrior := snapshotBGPVIPSessions(ctx, oc, masters, savedPeers)
			// Retries conflicts and transient API errors alike: the API VIP's
			// next hops may just have changed under the client.
			err := bgpVIPEventually(ctx, 2*time.Minute, "restore the BGPVIPConfig/cluster spec", func(ctx context.Context) error {
				cr, err := getBGPVIPConfig(ctx, oc)
				if err != nil {
					return err
				}
				if err := unstructured.SetNestedMap(cr.Object, savedSpec, "spec"); err != nil {
					return err
				}
				_, err = oc.AdminDynamicClient().Resource(bgpVIPConfigGVR).Update(ctx, cr, metav1.UpdateOptions{})
				return err
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(waitBGPVIPTimersRendered(ctx, oc, savedPeers, savedTargets["worker"], 7*time.Minute)).To(o.Succeed())
			// The restore decides on its own whether the sessions need a reset.
			_, err = waitBGPVIPNodeTimers(ctx, oc, masters, savedPeers, cleanupPrior, true)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		day2Hold, day2Keepalive, ok := pickBGPVIPDay2Timers(savedPeers)
		o.Expect(ok).To(o.BeTrue(), "every candidate day-2 timer pair is already set on a default peer")
		priorSessions := snapshotBGPVIPSessions(bodyCtx, oc, masters, savedPeers)
		g.By(fmt.Sprintf("setting every default peer's timers to %ds hold / %ds keepalive", day2Hold, day2Keepalive))
		var generation int64
		err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
			cr, err := getBGPVIPConfig(bodyCtx, oc)
			if err != nil {
				return err
			}
			defaultPeers, _, err := unstructured.NestedSlice(cr.Object, "spec", "defaultPeers")
			if err != nil {
				return err
			}
			for i, p := range defaultPeers {
				peer, ok := p.(map[string]interface{})
				if !ok {
					return fmt.Errorf("spec.defaultPeers[%d] is not an object", i)
				}
				peer["holdTimeSeconds"] = day2Hold
				peer["keepaliveTimeSeconds"] = day2Keepalive
			}
			if err := unstructured.SetNestedSlice(cr.Object, defaultPeers, "spec", "defaultPeers"); err != nil {
				return err
			}
			updated, err := oc.AdminDynamicClient().Resource(bgpVIPConfigGVR).Update(bodyCtx, cr, metav1.UpdateOptions{})
			if err == nil {
				generation = updated.GetGeneration()
			}
			return err
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		newPeers := make([]bgpVIPPeer, len(savedPeers))
		for i, p := range savedPeers {
			p.HoldTimeSeconds, p.KeepaliveTimeSeconds = day2Hold, day2Keepalive
			newPeers[i] = p
		}

		err = bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, fmt.Sprintf("see BGPVIPConfig/cluster Rendered and SessionsConfigured for generation %d", generation), func(ctx context.Context) error {
			cr, err := getBGPVIPConfig(ctx, oc)
			if err != nil {
				return err
			}
			if cr.GetGeneration() != generation {
				return fmt.Errorf("generation is %d, want %d", cr.GetGeneration(), generation)
			}
			return checkBGPVIPConditions(cr)
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		err = bgpVIPEventually(bodyCtx, bgpVIPConvergeTimeout, "see the new timers in FRRConfiguration/"+bgpVIPFRRConfiguration, func(ctx context.Context) error {
			return checkFRRConfigurationTimers(ctx, oc, newPeers)
		})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("waiting for the master pool to apply the new timers")
		o.Expect(waitBGPVIPTimersRendered(bodyCtx, oc, newPeers, savedTargets["worker"], 8*time.Minute)).To(o.Succeed())

		g.By("checking the timers in every control plane node's BGP sessions")
		sessionReset, err = waitBGPVIPNodeTimers(bodyCtx, oc, masters, newPeers, priorSessions, true)
		o.Expect(err).NotTo(o.HaveOccurred())
		if sessionReset {
			e2e.Logf("the frr-k8s reload did not restart the sessions; they were reset one control plane node at a time")
		}

		g.By("checking that no node rebooted or was cordoned")
		o.Expect(bgpVIPBootIDs(bodyCtx, oc)).To(o.Equal(bootIDs), "a node rebooted for a BGP peer timer change")
		cordoned, err := cordons.stop()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(cordoned).To(o.BeEmpty(), "nodes were cordoned for a BGP peer timer change")
	})
})

const (
	bgpVIPHashPolicyFile = "/tmp/bgp-vip-e2e-hash-policy"

	bgpVIPSetHashPolicyScript = `f=` + bgpVIPHashPolicyFile + `
if [ ! -f "$f" ]; then
  { sysctl -n net.ipv4.fib_multipath_hash_policy; sysctl -n net.ipv6.fib_multipath_hash_policy; } > "$f.tmp"
  mv "$f.tmp" "$f"
fi
sysctl -q -w net.ipv4.fib_multipath_hash_policy=1 net.ipv6.fib_multipath_hash_policy=1
`
	bgpVIPRestoreHashPolicyScript = `f=` + bgpVIPHashPolicyFile + `
if [ -f "$f" ]; then
  { read -r v4; read -r v6; } < "$f"
  sysctl -q -w net.ipv4.fib_multipath_hash_policy="$v4" net.ipv6.fib_multipath_hash_policy="$v6"
  rm -f "$f"
fi
`
	bgpVIPDeleteNftTablesScript = `nft delete table bridge bgp_vip_e2e 2>/dev/null || true
nft delete table inet bgp_vip_e2e 2>/dev/null || true
`
)

// hypervisorNeighborMAC returns the MAC address of ip in the hypervisor's
// neighbor table, refreshing the entry first.
func hypervisorNeighborMAC(ctx context.Context, h *bgpVIPHypervisor, ip string) (string, error) {
	if net.ParseIP(ip) == nil {
		return "", fmt.Errorf("%q is not an IP address", ip)
	}
	var mac string
	err := bgpVIPEventually(ctx, time.Minute, "find the MAC address of "+ip+" on the hypervisor", func(ctx context.Context) error {
		out, err := h.run(ctx, time.Minute, fmt.Sprintf("ping -c 1 -W 1 %s > /dev/null 2>&1 || true\nip -j neigh show %s", shellQuote(ip), shellQuote(ip)))
		if err != nil {
			return err
		}
		var neighbors []struct {
			LLAddr string `json:"lladdr"`
		}
		if err := json.Unmarshal([]byte(out), &neighbors); err != nil {
			return fmt.Errorf("cannot parse the neighbor entry of %s: %v: %s", ip, err, out)
		}
		for _, n := range neighbors {
			if n.LLAddr != "" {
				mac = n.LLAddr
				return nil
			}
		}
		return fmt.Errorf("the hypervisor has no neighbor entry for %s", ip)
	})
	return mac, err
}

// parseHypervisorClock parses the last line of out, the output of
// `date +%s.%N` on the hypervisor, into Unix seconds.
func parseHypervisorClock(out string) (float64, error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := strings.TrimSpace(lines[len(lines)-1])
	t, err := strconv.ParseFloat(last, 64)
	if err != nil {
		return 0, fmt.Errorf("cannot parse the hypervisor clock from %q: %v", out, err)
	}
	return t, nil
}

func bgpVIPNftCounterPackets(ctx context.Context, h *bgpVIPHypervisor, counter string) (int64, error) {
	out, err := h.run(ctx, time.Minute, "nft list counter bridge bgp_vip_e2e "+counter)
	if err != nil {
		return 0, err
	}
	m := bgpVIPNftCounter.FindStringSubmatch(out)
	if m == nil {
		return 0, fmt.Errorf("cannot parse nft counter %s: %s", counter, out)
	}
	return strconv.ParseInt(m[1], 10, 64)
}

// checkPoolSettled returns nil when pool runs its target configuration (and,
// if target is set, that target is the given one) with every machine updated.
func checkPoolSettled(pool *mcfgv1.MachineConfigPool, target string) error {
	if target != "" && pool.Spec.Configuration.Name != target {
		return fmt.Errorf("pool %s targets %s, want %s", pool.Name, pool.Spec.Configuration.Name, target)
	}
	if pool.Status.ObservedGeneration != pool.Generation {
		return fmt.Errorf("pool %s observedGeneration %d, generation %d", pool.Name, pool.Status.ObservedGeneration, pool.Generation)
	}
	if pool.Status.Configuration.Name != pool.Spec.Configuration.Name {
		return fmt.Errorf("pool %s runs %s, targets %s", pool.Name, pool.Status.Configuration.Name, pool.Spec.Configuration.Name)
	}
	conditions := map[mcfgv1.MachineConfigPoolConditionType]corev1.ConditionStatus{}
	for _, c := range pool.Status.Conditions {
		conditions[c.Type] = c.Status
	}
	if conditions[mcfgv1.MachineConfigPoolUpdated] != corev1.ConditionTrue || conditions[mcfgv1.MachineConfigPoolUpdating] == corev1.ConditionTrue || conditions[mcfgv1.MachineConfigPoolDegraded] == corev1.ConditionTrue {
		return fmt.Errorf("pool %s is Updated=%s Updating=%s Degraded=%s", pool.Name, conditions[mcfgv1.MachineConfigPoolUpdated], conditions[mcfgv1.MachineConfigPoolUpdating], conditions[mcfgv1.MachineConfigPoolDegraded])
	}
	if pool.Status.DegradedMachineCount != 0 || pool.Status.UpdatedMachineCount != pool.Status.MachineCount {
		return fmt.Errorf("pool %s has %d/%d machines updated, %d degraded", pool.Name, pool.Status.UpdatedMachineCount, pool.Status.MachineCount, pool.Status.DegradedMachineCount)
	}
	return nil
}

// waitBGPVIPTimersRendered waits until the master pool targets a rendered
// MachineConfig whose frr-peers.json carries the peers' timers, then until the
// pool has applied it everywhere. The worker pool must keep its target: the
// file is rendered for control plane nodes only.
func waitBGPVIPTimersRendered(ctx context.Context, oc *exutil.CLI, peers []bgpVIPPeer, workerTarget string, applyTimeout time.Duration) error {
	mcClient := oc.MachineConfigurationClient().MachineconfigurationV1()
	var target string
	if err := bgpVIPEventually(ctx, bgpVIPConvergeTimeout, "see the master pool target a rendered MachineConfig with the expected peer timers", func(ctx context.Context) error {
		pool, err := mcClient.MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
		if err != nil {
			return err
		}
		mc, err := oc.AdminDynamicClient().Resource(machineConfigGVR).Get(ctx, pool.Spec.Configuration.Name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		timers, found, err := peerTimersFromRenderedMachineConfig(mc)
		if err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("MachineConfig %s has no %s", mc.GetName(), bgpVIPPeersFilePath)
		}
		for _, p := range peers {
			got, ok := timers[p.Address]
			if !ok {
				return fmt.Errorf("MachineConfig %s renders no peer %s", mc.GetName(), p.Address)
			}
			if want := bgpVIPTimersFromPeer(p); got != want {
				return fmt.Errorf("MachineConfig %s renders peer %s with timers %+v, want %+v", mc.GetName(), p.Address, got, want)
			}
		}
		target = mc.GetName()
		return nil
	}); err != nil {
		return err
	}
	return bgpVIPEventually(ctx, applyTimeout, "see the master pool apply "+target, func(ctx context.Context) error {
		pool, err := mcClient.MachineConfigPools().Get(ctx, "master", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := checkPoolSettled(pool, target); err != nil {
			return err
		}
		if workerTarget == "" {
			return nil
		}
		worker, err := mcClient.MachineConfigPools().Get(ctx, "worker", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if worker.Spec.Configuration.Name != workerTarget {
			return fmt.Errorf("worker pool target changed from %s to %s", workerTarget, worker.Spec.Configuration.Name)
		}
		return nil
	})
}

// checkFRRConfigurationTimers checks that every neighbor of the bgp-vip
// FRRConfiguration that is a default peer carries that peer's timers.
func checkFRRConfigurationTimers(ctx context.Context, oc *exutil.CLI, peers []bgpVIPPeer) error {
	frr, err := oc.AdminDynamicClient().Resource(frrConfigurationGVR).Namespace(bgpVIPFRRNamespace).Get(ctx, bgpVIPFRRConfiguration, metav1.GetOptions{})
	if err != nil {
		return err
	}
	want := map[string]bgpVIPPeer{}
	for _, p := range peers {
		want[p.Address] = p
	}
	seen := sets.New[string]()
	routers, _, err := unstructured.NestedSlice(frr.Object, "spec", "bgp", "routers")
	if err != nil {
		return fmt.Errorf("FRRConfiguration/%s spec.bgp.routers: %w", bgpVIPFRRConfiguration, err)
	}
	for i, r := range routers {
		router, ok := r.(map[string]interface{})
		if !ok {
			return fmt.Errorf("FRRConfiguration/%s spec.bgp.routers[%d] is not an object", bgpVIPFRRConfiguration, i)
		}
		neighbors, _, err := unstructured.NestedSlice(router, "neighbors")
		if err != nil {
			return fmt.Errorf("FRRConfiguration/%s spec.bgp.routers[%d].neighbors: %w", bgpVIPFRRConfiguration, i, err)
		}
		for _, n := range neighbors {
			neighbor, _ := n.(map[string]interface{})
			p, ok := want[stringField(neighbor, "address")]
			if !ok {
				continue
			}
			seen.Insert(p.Address)
			for _, timer := range []struct {
				field string
				want  int64
			}{{"holdTime", p.HoldTimeSeconds}, {"keepaliveTime", p.KeepaliveTimeSeconds}} {
				value := stringField(neighbor, timer.field)
				if timer.want == 0 && value == "" {
					continue
				}
				got, err := time.ParseDuration(value)
				if err != nil || got != time.Duration(timer.want)*time.Second {
					return fmt.Errorf("FRRConfiguration/%s neighbor %s has %s %q, want %ds", bgpVIPFRRConfiguration, p.Address, timer.field, value, timer.want)
				}
			}
		}
	}
	for addr := range want {
		if !seen.Has(addr) {
			return fmt.Errorf("FRRConfiguration/%s has no neighbor %s", bgpVIPFRRConfiguration, addr)
		}
	}
	return nil
}

// waitBGPVIPNodeTimers waits until every control plane node's session to every
// peer is Established with the peer's configured timers and with negotiated
// timers that follow from them (see checkBGPVIPNegotiatedTimers), given the
// sessions in prior, recorded before the change. FRR only renegotiates timers
// when a session restarts; if the sessions keep the old negotiated timers and
// allowReset is set, they are reset with "clear bgp *" one node at a time. It
// reports whether a reset was needed.
func waitBGPVIPNodeTimers(ctx context.Context, oc *exutil.CLI, masters []corev1.Node, peers []bgpVIPPeer, prior map[string]frrNeighbor, allowReset bool) (bool, error) {
	check := func(ctx context.Context, node string, needNegotiated bool) error {
		var problems []string
		for _, peer := range peers {
			n, err := nodeNeighbor(ctx, oc, node, peer.Address)
			if err != nil {
				problems = append(problems, err.Error())
				continue
			}
			problems = append(problems, checkBGPVIPNeighbor(node, peer, n, needNegotiated)...)
			if needNegotiated && n.BGPState == "Established" {
				p, ok := prior[bgpVIPSessionKey(node, peer.Address)]
				problems = append(problems, checkBGPVIPNegotiatedTimers(node, peer.Address, n, p, ok)...)
			}
		}
		if len(problems) > 0 {
			return fmt.Errorf("%s", strings.Join(problems, "; "))
		}
		return nil
	}
	all := func(needNegotiated bool) func(ctx context.Context) error {
		return func(ctx context.Context) error {
			for _, m := range masters {
				if err := check(ctx, m.Name, needNegotiated); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if err := bgpVIPEventually(ctx, 3*time.Minute, "see every control plane node configured with the expected timers", all(false)); err != nil {
		return false, err
	}
	err := bgpVIPEventually(ctx, time.Minute, "see every control plane node negotiate the expected timers", all(true))
	if err == nil || !allowReset {
		return false, err
	}
	e2e.Logf("the sessions kept their negotiated timers (%v); resetting them one control plane node at a time", err)
	for _, m := range masters {
		// Resetting a node's sessions withdraws its VIP routes, which can move
		// the test client's own API connection to another node and lose the
		// exec reply; the check below tells whether the reset happened.
		if _, err := nodeVtysh(ctx, oc, m.Name, "clear bgp *"); err != nil {
			e2e.Logf("resetting %s's BGP sessions: %v", m.Name, err)
		}
		node := m.Name
		if err := bgpVIPEventually(ctx, 2*time.Minute, "see "+node+"'s sessions Established with the expected timers", func(ctx context.Context) error {
			return check(ctx, node, true)
		}); err != nil {
			return true, err
		}
	}
	return true, waitBGPVIPAPIStable(ctx, oc)
}

// waitBGPVIPAPIStable waits until the API answers several requests in a row.
// A change in the API VIP's next hops on the client's side moves existing
// connections to another node, which resets them; this drops the broken
// pooled connections before the framework's own cleanup uses them.
func waitBGPVIPAPIStable(ctx context.Context, oc *exutil.CLI) error {
	time.Sleep(5 * time.Second)
	clients := []kubernetes.Interface{oc.AdminKubeClient(), oc.KubeFramework().ClientSet}
	ok := 0
	return bgpVIPEventually(ctx, 2*time.Minute, "see the API answer 3 requests in a row", func(ctx context.Context) error {
		for _, c := range clients {
			if c == nil {
				continue
			}
			if _, err := c.CoreV1().Namespaces().Get(ctx, "default", metav1.GetOptions{}); err != nil {
				ok = 0
				return err
			}
		}
		if ok++; ok < 3 {
			return fmt.Errorf("%d of 3 requests answered", ok)
		}
		return nil
	})
}

func bgpVIPBootIDs(ctx context.Context, oc *exutil.CLI) map[string]string {
	out := map[string]string{}
	err := bgpVIPEventually(ctx, time.Minute, "read the nodes' boot IDs", func(ctx context.Context) error {
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, node := range nodes.Items {
			out[node.Name] = node.Status.NodeInfo.BootID
		}
		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	return out
}

// bgpVIPCordonWatch records the nodes that become unschedulable while it
// runs. It polls, rather than watches, because the API server may be briefly
// unreachable through the VIP; an MCO drain keeps a node cordoned for far
// longer than the poll interval. Failed polls are tolerated, but a gap of
// more than bgpVIPCordonMaxGap without a successful one fails stop.
type bgpVIPCordonWatch struct {
	cancel context.CancelFunc
	done   chan struct{}
	once   sync.Once

	mu       sync.Mutex
	cordoned sets.Set[string]
	lastOK   time.Time
	maxGap   time.Duration
	lastErr  error
}

const bgpVIPCordonMaxGap = 2 * time.Minute

func watchBGPVIPCordons(parent context.Context, oc *exutil.CLI) *bgpVIPCordonWatch {
	ctx, cancel := context.WithCancel(parent)
	w := &bgpVIPCordonWatch{cancel: cancel, done: make(chan struct{}), cordoned: sets.New[string]()}
	baseline := sets.New[string]()
	err := bgpVIPEventually(ctx, time.Minute, "read the nodes' schedulability", func(ctx context.Context) error {
		nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return err
		}
		for _, node := range nodes.Items {
			if node.Spec.Unschedulable {
				baseline.Insert(node.Name)
			}
		}
		return nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	w.lastOK = time.Now()
	go func() {
		defer g.GinkgoRecover()
		defer close(w.done)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			nodes, err := oc.AdminKubeClient().CoreV1().Nodes().List(ctx, metav1.ListOptions{})
			w.mu.Lock()
			// A list that succeeds late only shows the nodes at its end, so
			// the gap runs from the previous success to its completion.
			w.maxGap = max(w.maxGap, time.Since(w.lastOK))
			if err == nil {
				w.lastOK = time.Now()
				for _, node := range nodes.Items {
					if node.Spec.Unschedulable && !baseline.Has(node.Name) {
						if !w.cordoned.Has(node.Name) {
							e2e.Logf("node %s became unschedulable", node.Name)
						}
						w.cordoned.Insert(node.Name)
					}
				}
			} else if ctx.Err() == nil {
				w.lastErr = err
			}
			w.mu.Unlock()
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
	return w
}

// stop stops the watch and returns the nodes that became unschedulable. It
// fails if the nodes went unobserved for longer than bgpVIPCordonMaxGap.
func (w *bgpVIPCordonWatch) stop() ([]string, error) {
	w.once.Do(func() {
		w.cancel()
		<-w.done
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	w.maxGap = max(w.maxGap, time.Since(w.lastOK))
	if w.maxGap > bgpVIPCordonMaxGap {
		return sets.List(w.cordoned), fmt.Errorf("could not list the nodes for %s (last error: %v), so a cordon may have gone unseen", w.maxGap.Round(time.Second), w.lastErr)
	}
	return sets.List(w.cordoned), nil
}

// pickBGPVIPDay2Timers returns the first candidate hold/keepalive pair that no
// default peer already uses, so that the day-2 update changes every peer.
func pickBGPVIPDay2Timers(peers []bgpVIPPeer) (hold, keepalive int64, ok bool) {
candidates:
	for _, c := range bgpVIPDay2Timers {
		for _, p := range peers {
			if p.HoldTimeSeconds == c[0] || p.KeepaliveTimeSeconds == c[1] {
				continue candidates
			}
		}
		return c[0], c[1], true
	}
	return 0, 0, false
}

func bgpVIPSessionKey(node, peer string) string { return node + "/" + peer }

// snapshotBGPVIPSessions records every control plane node's Established
// session to every peer. Sessions that cannot be read are logged and left
// out; checkBGPVIPNegotiatedTimers then checks them more loosely.
func snapshotBGPVIPSessions(ctx context.Context, oc *exutil.CLI, masters []corev1.Node, peers []bgpVIPPeer) map[string]frrNeighbor {
	out := map[string]frrNeighbor{}
	for _, m := range masters {
		for _, p := range peers {
			n, err := nodeNeighbor(ctx, oc, m.Name, p.Address)
			if err != nil {
				e2e.Logf("recording %s's session to %s: %v", m.Name, p.Address, err)
				continue
			}
			if n.BGPState == "Established" {
				out[bgpVIPSessionKey(m.Name, p.Address)] = n
			}
		}
	}
	return out
}

// bgpVIPNegotiatedHoldRange returns the hold times, in ms, that a session may
// negotiate once its configured hold time is cfg. BGP negotiates the lower of
// both sides' proposals. The peer's proposal is learned from the session
// before the change: a negotiated hold time below the configured one was the
// peer's proposal; otherwise the peer proposed at least the configured one,
// and a higher cfg leaves the result between the two.
func bgpVIPNegotiatedHoldRange(cfg int64, prior frrNeighbor, havePrior bool) (lo, hi int64) {
	switch {
	case !havePrior || prior.ConfiguredHoldTimeMsecs <= 0 || prior.NegotiatedHoldTimeMsecs <= 0:
		return 0, cfg
	case prior.NegotiatedHoldTimeMsecs < prior.ConfiguredHoldTimeMsecs:
		e := min(cfg, prior.NegotiatedHoldTimeMsecs)
		return e, e
	case cfg <= prior.ConfiguredHoldTimeMsecs:
		return cfg, cfg
	default:
		return prior.ConfiguredHoldTimeMsecs, cfg
	}
}

// bgpVIPNegotiatedKeepalive returns the keepalive interval, in ms, that FRR
// derives from a negotiated hold time: a third of it in whole seconds, or the
// configured keepalive when that is lower.
func bgpVIPNegotiatedKeepalive(configured, negotiatedHold int64) int64 {
	ka := negotiatedHold / 1000 / 3 * 1000
	if configured > 0 && configured < ka {
		ka = configured
	}
	return ka
}

// checkBGPVIPNegotiatedTimers checks an Established session's negotiated
// timers against its configured ones and the peer's proposal, as far as the
// session before the change (prior) reveals it.
func checkBGPVIPNegotiatedTimers(node, peer string, n, prior frrNeighbor, havePrior bool) []string {
	var problems []string
	lo, hi := bgpVIPNegotiatedHoldRange(n.ConfiguredHoldTimeMsecs, prior, havePrior)
	if n.NegotiatedHoldTimeMsecs < lo || n.NegotiatedHoldTimeMsecs > hi {
		problems = append(problems, fmt.Sprintf("%s -> %s: negotiated hold time %dms, want %d-%dms", node, peer, n.NegotiatedHoldTimeMsecs, lo, hi))
	}
	if want := bgpVIPNegotiatedKeepalive(n.ConfiguredKeepAliveMsecs, n.NegotiatedHoldTimeMsecs); n.NegotiatedKeepAliveMsecs != want {
		problems = append(problems, fmt.Sprintf("%s -> %s: negotiated keepalive %dms, want %dms", node, peer, n.NegotiatedKeepAliveMsecs, want))
	}
	return problems
}
