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
		utils.SkipIfClusterIsNotHealthy(oc, helpers.NewEtcdClientFactory(oc.KubeClient()))
		ctx, cancel := context.WithTimeout(context.Background(), tnfCommandTimeout)
		defer cancel()
		core := oc.AdminKubeClient().CoreV1()
		pcs = &tnfPCSRunner{oc: oc, namespaces: core.Namespaces(), serviceAccounts: core.ServiceAccounts}
		g.DeferCleanup(func() {
			o.Expect(pcs.cleanup(context.Background())).To(o.Succeed())
		})
		masters, err := core.Nodes().List(ctx, metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/master="})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(masters.Items).To(o.HaveLen(2))
		nodes = nil
		for _, node := range masters.Items {
			nodes = append(nodes, node.Name)
			o.Expect(node.Annotations["machineconfiguration.openshift.io/state"]).To(o.Equal("Done"), "MCO must be healthy before fault injection")
		}
		sort.Strings(nodes)
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
		// Save only the affected entries, including their metadata and recipients.
		for _, id := range []string{services.TaintAlertID, services.UntaintAlertID} {
			xpath := fmt.Sprintf("/cib/configuration/alerts/alert[@id='%s']", id)
			backup := "/var/tmp/tnf-alert-" + string(uuid.NewUUID())
			g.DeferCleanup(func() {
				err := retryTNFOperation(context.Background(), tnfPollInterval, tnfCommandTimeout, func(ctx context.Context) error {
					_, err := pcs.hostOutput(ctx, nodes[0], "bash", "-ceu", `if test -s "$2"; then
	if cibadmin --query --xpath "$1" >/dev/null 2>&1; then
		cibadmin --replace --xpath "$1" --xml-file "$2"
	else
		cibadmin --modify --scope alerts --xml-file "$2"
	fi
fi
rm -f -- "$2"`, "--", xpath, backup)
					return err
				})
				o.Expect(err).To(o.Succeed(), "restore the original alert registration")
			})
			_, err := pcs.hostOutput(ctx, nodes[0], "bash", "-ceu", `umask 077
set -o noclobber
cibadmin --query --xpath "$1" > "$2"`, "--", xpath, backup)
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		pc, err := apis.GetPacemakerCluster(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		watchCtx, cancel := context.WithTimeout(ctx, tnfObservabilityTimeout)
		defer cancel()
		stream, err := oc.AdminDynamicClient().Resource(apis.PacemakerClusterGVR).Watch(watchCtx, metav1.ListOptions{
			FieldSelector: "metadata.name=cluster", ResourceVersion: pc.ResourceVersion,
		})
		o.Expect(err).NotTo(o.HaveOccurred())
		defer stream.Stop()

		g.By("removing the taint registration and changing the untaint script path")
		_, err = pcs.hostOutput(ctx, nodes[0], "bash", "-ceu", `pcs alert delete tnf-taint-alert
pcs alert update tnf-untaint-alert path=/var/lib/pacemaker/alerts/tnf-e2e-invalid.sh`)
		o.Expect(err).NotTo(o.HaveOccurred())

		// Watch from before the mutation: repairs happen immediately after the failed
		// observation is written, and polling could miss it before the next collection.
		g.By("observing registration failures and their node/cluster health aggregation")
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
			if err := expectTNFAlertStatus(&observed, nodes,
				tnfAlertFailure{agent: etcdv1.PacemakerClusterAlertAgentNameTaint, condition: etcdv1.AlertAgentConfiguredConditionType, reason: etcdv1.AlertAgentConfiguredReasonNotRegistered},
				tnfAlertFailure{agent: etcdv1.PacemakerClusterAlertAgentNameUntaint, condition: etcdv1.AlertAgentConfiguredConditionType, reason: etcdv1.AlertAgentConfiguredReasonMisconfigured},
			); err != nil {
				lastObservation = err.Error()
				return false, nil
			}
			failedAt = observed.Status.LastUpdated.Time
			return true, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "capture the pre-repair failure status: %s", lastObservation)

		g.By("waiting for automatic repair and a subsequent healthy collection")
		o.Expect(retryTNFOperation(ctx, tnfPollInterval, tnfObservabilityTimeout, func(ctx context.Context) error {
			output, err := pcs.output(ctx, nodes[0], "alert", "config", "--output-format", "json")
			if err != nil {
				return err
			}
			return expectTNFAlertRegistrations(output)
		})).To(o.Succeed())
		o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, failedAt)).To(o.Succeed())
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
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
