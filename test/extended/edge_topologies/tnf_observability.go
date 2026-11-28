package edge_topologies

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	etcdv1 "github.com/openshift/api/etcd/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	"github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

var _ = g.Describe("[sig-etcd][apigroup:etcd.openshift.io][OCPFeatureGate:DualReplica][OCPFeatureGate:TNFObservability][Suite:openshift/two-node][TNFObservability][Serial][Disruptive] TNF alert-agent observability", func() {
	var (
		oc    = exutil.NewCLIWithoutNamespace("tnf-observability").AsAdmin()
		pcs   *tnfPCSRunner
		nodes []string
	)

	g.BeforeEach(func() {
		utils.SkipIfNotTopology(oc, configv1.DualReplicaTopologyMode)
		gateCtx, cancelGate := context.WithTimeout(context.Background(), tnfCommandTimeout)
		enabled, err := tnfObservabilityEnabled(gateCtx, oc)
		cancelGate()
		o.Expect(err).NotTo(o.HaveOccurred())
		if !enabled {
			g.Skip("TNFObservability is disabled for the collector release")
		}
		pcs, nodes = setupTNFObservability(oc)
		o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, time.Now())).To(o.Succeed(), "establish fresh, healthy alert-agent status")
	})

	g.It("reports both healthy alert agents on both nodes", func() {
		output, err := pcs.output(context.Background(), nodes[0], "alert", "config", "--output-format", "json")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(expectTNFAlertRegistrations(output)).To(o.Succeed())
		pc, err := apis.GetPacemakerCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, pc.Status.LastUpdated.Time)).To(o.Succeed(), "remain healthy across another collection")
		o.Expect(apis.ExpectPacemakerHealthCheckNotDegraded(oc)).To(o.Succeed())
	})

	g.It("reports and repairs missing and misconfigured alert registrations", func() {
		ctx := context.Background()
		g.DeferCleanup(func() {
			o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, time.Now())).To(o.Succeed(), "recover after restoring registrations")
			o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
		})
		for _, id := range []string{services.TaintAlertID, services.UntaintAlertID} {
			backupTNFAlertRegistration(ctx, pcs, nodes[0], id)
		}

		for _, fault := range []struct {
			description string
			agent       etcdv1.PacemakerClusterAlertAgentName
			reason      string
			command     []string
		}{
			{
				description: "remove the taint registration",
				agent:       etcdv1.PacemakerClusterAlertAgentNameTaint,
				reason:      etcdv1.AlertAgentConfiguredReasonNotRegistered,
				command:     []string{"pcs", "alert", "delete", services.TaintAlertID},
			},
			{
				description: "change the untaint script path",
				agent:       etcdv1.PacemakerClusterAlertAgentNameUntaint,
				reason:      etcdv1.AlertAgentConfiguredReasonMisconfigured,
				command:     []string{"pcs", "alert", "update", services.UntaintAlertID, "path=/var/lib/pacemaker/alerts/tnf-e2e-invalid.sh"},
			},
			{
				description: "change only the taint event selector",
				agent:       etcdv1.PacemakerClusterAlertAgentNameTaint,
				reason:      etcdv1.AlertAgentConfiguredReasonMisconfigured,
				command:     []string{"cibadmin", "--replace", "--xpath", tnfAlertXPath(services.TaintAlertID) + "/select", "--xml-text", "<select><select_nodes/></select>"},
			},
		} {
			g.By(fault.description)
			failedAt := observeTNFRegistrationFault(ctx, oc, pcs, nodes, fault.command, tnfAlertFailure{
				agent: fault.agent, condition: etcdv1.AlertAgentConfiguredConditionType, reason: fault.reason,
			})
			g.By("waiting for automatic repair and a subsequent healthy collection")
			o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, failedAt)).To(o.Succeed())
			output, err := pcs.output(ctx, nodes[0], "alert", "config", "--output-format", "json")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(expectTNFAlertRegistrations(output)).To(o.Succeed())
			o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
		}
	})

	g.It("retains alert observations and defers repairs when the collector release is unresolved", func() {
		testTNFUnresolvedCollectorRelease(oc, pcs, nodes)
	})

	g.It("reports node-local alert script drift and recovers health after restoration", func() {
		ctx := context.Background()
		target, path := nodes[0], services.TaintAlertScriptPath
		backup := "/var/tmp/tnf-alert-" + string(uuid.NewUUID())
		restore := func(ctx context.Context) error {
			_, err := pcs.hostOutput(ctx, target, "bash", "-ceu", `if test -f "$2"; then cp -a -- "$2" "$1"; fi`, "--", path, backup)
			return err
		}
		g.DeferCleanup(func() {
			o.Expect(retryTNFOperation(context.Background(), tnfPollInterval, tnfCommandTimeout, restore)).To(o.Succeed(), "restore the MCO-managed script")
			o.Expect(waitForTNFMCOState(context.Background(), oc, target, "Done", "")).To(o.Succeed())
			o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, time.Now())).To(o.Succeed())
			o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
			_, err := pcs.hostOutput(context.Background(), target, "rm", "-f", "--", backup)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.By("changing script content without changing its executable behavior")
		_, err := pcs.hostOutput(ctx, target, "bash", "-ceu", `test -x "$1"
test ! -e "$2"
cp -a -- "$1" "$2"
printf '\n# TNF observability drift test\n' >> "$1"`, "--", path, backup)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(waitForTNFMCOState(ctx, oc, target, "Degraded", path)).To(o.Succeed(), "MCO must identify the alert script drift")
		o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, time.Now(), tnfAlertFailure{
			node: target, agent: etcdv1.PacemakerClusterAlertAgentNameTaint,
			condition: etcdv1.AlertAgentAvailableConditionType, reason: etcdv1.AlertAgentAvailableReasonUnavailable,
		})).To(o.Succeed(), "only the affected node's taint agent should be unavailable")
		o.Expect(apis.WaitForPacemakerHealthCheckDegraded(oc, string(etcdv1.PacemakerClusterAlertAgentNameTaint), tnfObservabilityTimeout)).To(o.Succeed())

		g.By("restoring the script and waiting for MCO and Pacemaker health to recover")
		o.Expect(restore(ctx)).To(o.Succeed())
		o.Expect(waitForTNFMCOState(ctx, oc, target, "Done", "")).To(o.Succeed())
		o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, time.Now())).To(o.Succeed())
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
	})
})

func setupTNFObservability(oc *exutil.CLI) (*tnfPCSRunner, []string) {
	utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))
	ctx, cancel := context.WithTimeout(context.Background(), tnfCommandTimeout)
	defer cancel()
	core := oc.AdminKubeClient().CoreV1()
	pcs := &tnfPCSRunner{oc: oc, namespaces: core.Namespaces(), serviceAccounts: core.ServiceAccounts}
	g.DeferCleanup(func() {
		o.Expect(pcs.cleanup(context.Background())).To(o.Succeed())
	})
	masters, err := core.Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(masters.Items).To(o.HaveLen(2))
	var nodes []string
	for _, node := range masters.Items {
		nodes = append(nodes, node.Name)
		o.Expect(node.Annotations["machineconfiguration.openshift.io/state"]).To(o.Equal("Done"), "MCO must be healthy before fault injection")
	}
	sort.Strings(nodes)
	return pcs, nodes
}

func observeTNFRegistrationFault(ctx context.Context, oc *exutil.CLI, pcs *tnfPCSRunner, nodes, command []string, failure tnfAlertFailure) time.Time {
	pc, err := apis.GetPacemakerCluster(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	watchCtx, cancel := context.WithTimeout(ctx, tnfObservabilityTimeout)
	defer cancel()

	// Resume from the last event so a disconnect cannot hide the brief pre-repair status.
	resource := oc.AdminDynamicClient().Resource(apis.PacemakerClusterGVR)
	stream, err := watchtools.NewRetryWatcherWithContext(watchCtx, pc.ResourceVersion, &cache.ListWatch{
		WatchFuncWithContext: func(ctx context.Context, options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = "metadata.name=cluster"
			stream, err := resource.Watch(ctx, options)
			if apierrors.IsResourceExpired(err) || apierrors.IsGone(err) {
				// RetryWatcher only treats expiration as terminal when it arrives as an event.
				events := make(chan watch.Event, 1)
				events <- watch.Event{Type: watch.Error, Object: &apierrors.NewResourceExpired(err.Error()).ErrStatus}
				close(events)
				return watch.NewProxyWatcher(events), nil
			}
			return stream, err
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())
	defer stream.Stop()

	_, err = pcs.hostOutput(ctx, nodes[0], command...)
	o.Expect(err).NotTo(o.HaveOccurred())

	var failedAt time.Time
	lastObservation := "no new collection observed"
	_, err = watchtools.UntilWithoutRetry(watchCtx, stream, func(event watch.Event) (bool, error) {
		if event.Type == watch.Error {
			return false, apierrors.FromObject(event.Object)
		}
		if event.Type != watch.Modified {
			return false, nil
		}
		obj, ok := event.Object.(*unstructured.Unstructured)
		if !ok {
			return false, fmt.Errorf("unexpected PacemakerCluster watch object %T", event.Object)
		}
		var observed etcdv1.PacemakerCluster
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &observed); err != nil {
			return false, err
		}
		if !observed.Status.LastUpdated.After(pc.Status.LastUpdated.Time) {
			return false, nil
		}
		if err := expectTNFAlertStatus(&observed, nodes, failure); err != nil {
			lastObservation = err.Error()
			return false, nil
		}
		failedAt = observed.Status.LastUpdated.Time
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "capture the pre-repair failure status: %s", lastObservation)
	return failedAt
}

func waitForTNFMCOState(ctx context.Context, oc *exutil.CLI, node, state, reason string) error {
	return retryTNFOperation(ctx, 5*time.Second, tnfObservabilityTimeout, func(ctx context.Context) error {
		n, err := oc.AdminKubeClient().CoreV1().Nodes().Get(ctx, node, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if n.Annotations["machineconfiguration.openshift.io/state"] != state || !strings.Contains(n.Annotations["machineconfiguration.openshift.io/reason"], reason) {
			return fmt.Errorf("waiting for MCO state %s and reason containing %q", state, reason)
		}
		return nil
	})
}
