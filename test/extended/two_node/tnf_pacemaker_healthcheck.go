package two_node

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	v1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	"github.com/openshift/origin/test/extended/two_node/utils"
	"github.com/openshift/origin/test/extended/two_node/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	// healthCheckUpdatedTimeout is the time to wait for the Pacemaker health check condition to update (degraded or healthy).
	healthCheckUpdatedTimeout  = 2 * time.Minute
	healthCheckDegradedTimeout = healthCheckUpdatedTimeout
	healthCheckHealthyTimeout  = healthCheckUpdatedTimeout
	// Longer timeouts for tests that trigger a fencing event (ungraceful shutdown, cold-boot, network disruption):
	// API server can be slow to recover, so we wait up to 5 minutes before asserting PacemakerHealthCheckDegraded/Healthy.
	healthCheckDegradedTimeoutAfterFencing = 5 * time.Minute
	healthCheckHealthyTimeoutAfterFencing  = 5 * time.Minute
	// StatusUnknownDegradedThreshold and StatusStalenessThreshold in CEO are 5 minutes; we must block for at least this long before asserting degraded.
	staleMinBlockDuration = 5 * time.Minute
	// After blocking, allow time for healthcheck controller (30s resync) to observe degraded.
	staleCRDegradedTimeout        = 2 * time.Minute
	staleTimestampDegradedTimeout = 2 * time.Minute
	// Interval for background delete loops: delete as soon as resources appear (match aggressive manual watch cadence).
	staleTestDeleteInterval = 2 * time.Second
	pacemakerClusterCRName  = "cluster"
	statusCollectorLabel    = "app.kubernetes.io/name=pacemaker-status-collector"
	etcdNamespaceFencing    = "openshift-etcd"
)

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][Suite:openshift/two-node][Serial][Disruptive] Pacemaker health check disruptive scenarios", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("tnf-pacemaker-healthcheck").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
		peerNode          corev1.Node
		targetNode        corev1.Node
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)

		nodes, err := utils.GetNodes(oc, utils.AllNodes)
		o.Expect(err).ShouldNot(o.HaveOccurred(), "Expected to retrieve nodes without error")
		randomIndex := rand.Intn(len(nodes.Items))
		peerNode = nodes.Items[randomIndex]
		targetNode = nodes.Items[(randomIndex+1)%len(nodes.Items)]
	})

	g.It("should report degraded when a node is in standby then healthy after unstandby", func() {
		g.By(fmt.Sprintf("Putting %s in standby from %s", targetNode.Name, peerNode.Name))
		o.Expect(utils.PcsNodeStandby(oc, peerNode.Name, targetNode.Name)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition reports target node in standby")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, "standby", healthCheckDegradedTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
		o.Expect(services.AssertPacemakerHealthCheckContains(oc, []string{targetNode.Name, "standby"})).To(o.Succeed())

		g.By(fmt.Sprintf("Bringing %s out of standby", targetNode.Name))
		o.Expect(utils.PcsNodeUnstandby(oc, peerNode.Name, targetNode.Name)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

	g.It("should report degraded when cluster is in maintenance mode then healthy after clearing", func() {
		g.By("Setting cluster maintenance mode")
		o.Expect(utils.PcsPropertySetMaintenanceMode(oc, peerNode.Name, true)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition reports maintenance")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, "maintenance", healthCheckDegradedTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())

		g.By("Clearing cluster maintenance mode")
		o.Expect(utils.PcsPropertySetMaintenanceMode(oc, peerNode.Name, false)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

	g.It("should report degraded when a node is in maintenance mode then healthy after unmaintenance", func() {
		g.By(fmt.Sprintf("Putting %s in node maintenance from %s", targetNode.Name, peerNode.Name))
		o.Expect(utils.PcsNodeMaintenance(oc, peerNode.Name, targetNode.Name)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition reports target node in maintenance")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, "maintenance", healthCheckDegradedTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
		o.Expect(services.AssertPacemakerHealthCheckContains(oc, []string{targetNode.Name, "maintenance"})).To(o.Succeed())

		g.By(fmt.Sprintf("Bringing %s out of node maintenance", targetNode.Name))
		o.Expect(utils.PcsNodeUnmaintenance(oc, peerNode.Name, targetNode.Name)).To(o.Succeed())

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

})

var _ = g.Describe("[sig-etcd][apigroup:config.openshift.io][Suite:openshift/two-node][Serial] Pacemaker health check stale status scenarios", func() {
	defer g.GinkgoRecover()

	var (
		oc                = exutil.NewCLIWithoutNamespace("tnf-pacemaker-stale").AsAdmin()
		etcdClientFactory *helpers.EtcdClientFactoryImpl
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, v1.DualReplicaTopologyMode)
		etcdClientFactory = helpers.NewEtcdClientFactory(oc.KubeClient())
		utils.SkipIfClusterIsNotHealthy(oc, etcdClientFactory)
	})

	g.It("should report degraded when PacemakerCluster CR is repeatedly deleted then healthy after CR is allowed to exist", func() {
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(staleTestDeleteInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					out, err := oc.AsAdmin().Run("delete").Args("pacemakercluster", pacemakerClusterCRName, "--ignore-not-found").Output()
					if err != nil {
						e2e.Logf("Staleness CR delete loop: delete pacemakercluster/%s failed: %v (output: %q)", pacemakerClusterCRName, err, string(out))
					} else if strings.TrimSpace(string(out)) != "" {
						e2e.Logf("Staleness CR delete loop: %s", string(out))
					}
				}
			}
		}()

		g.By("Deleting PacemakerCluster CR for 5 minutes so operator exceeds StatusUnknownDegradedThreshold")
		time.Sleep(staleMinBlockDuration)

		g.By("Waiting for PacemakerHealthCheckDegraded (CR not found)")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, "not found", staleCRDegradedTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())

		// Only stop the delete loop after asserting degraded; otherwise the operator could recreate the CR before we observe not found.
		g.By("Stopping CR delete loop and allowing operator to recreate CR")
		cancel()
		wg.Wait()

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
	})

	g.It("should report degraded when status collector jobs are repeatedly deleted then healthy after jobs can run", func() {
		ctx, cancel := context.WithCancel(context.Background())
		var wg sync.WaitGroup
		wg.Add(1)
		go func() {
			defer wg.Done()
			ticker := time.NewTicker(staleTestDeleteInterval)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					out, err := oc.AsAdmin().Run("delete").Args("jobs", "-n", etcdNamespaceFencing, "-l", statusCollectorLabel, "--ignore-not-found").Output()
					if err != nil {
						e2e.Logf("Staleness job delete loop: delete jobs -l %s -n %s failed: %v (output: %q)", statusCollectorLabel, etcdNamespaceFencing, err, string(out))
					} else if strings.TrimSpace(string(out)) != "" {
						e2e.Logf("Staleness job delete loop: %s", string(out))
					}
				}
			}
		}()

		g.By("Blocking status collector for 5 minutes so CR lastUpdated exceeds StatusStalenessThreshold")
		time.Sleep(staleMinBlockDuration)

		g.By("Waiting for PacemakerHealthCheckDegraded (stale status)")
		o.Expect(services.WaitForPacemakerHealthCheckDegraded(oc, "stale", staleTimestampDegradedTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())

		// Only stop the delete loop after asserting degraded; otherwise a job could complete and update the CR before we observe stale.
		g.By("Stopping job delete loop and allowing cronjob to run")
		cancel()
		wg.Wait()

		g.By("Verifying PacemakerHealthCheckDegraded condition clears")
		o.Expect(services.WaitForPacemakerHealthCheckHealthy(oc, healthCheckHealthyTimeout, utils.FiveSecondPollInterval)).To(o.Succeed())
	})
})
