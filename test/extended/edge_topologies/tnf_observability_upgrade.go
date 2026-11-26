package edge_topologies

import (
	"context"
	"fmt"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	etcdv1 "github.com/openshift/api/etcd/v1"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/apis"
	"github.com/openshift/origin/test/extended/edge_topologies/utils/services"
	exutil "github.com/openshift/origin/test/extended/util"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

func testTNFUnresolvedCollectorRelease(oc *exutil.CLI, pcs *tnfPCSRunner, nodes []string) {
	ctx := context.Background()
	id := services.UntaintAlertID
	g.DeferCleanup(func() {
		o.Expect(waitForTNFAlertStatus(context.Background(), oc, nodes, time.Now())).To(o.Succeed())
		o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
	})
	backupTNFAlertRegistration(ctx, pcs, nodes[0], id)

	g.By("running a collector with a release absent from FeatureGate status")
	// Run between scheduled collectors so a gate-enabled Job cannot repair the fault
	// or replace the retained status before the assertions.
	cron := waitForTNFCollectorWindow(ctx, oc)
	window, cancel := context.WithDeadline(ctx, cron.Status.LastScheduleTime.Add(time.Minute))
	defer cancel()
	before, err := apis.GetPacemakerCluster(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expectTNFAlertStatus(before, nodes)).To(o.Succeed())
	o.Expect(pcs.run(window, nodes[0], "alert", "delete", id)).To(o.Succeed())
	runTNFCollector(window, oc, cron, "0.0.0-tnf-e2e-unresolved")
	after, err := apis.GetPacemakerCluster(oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(after.Status.LastUpdated.After(before.Status.LastUpdated.Time)).To(o.BeTrue(), "collector must publish fresh base status")
	o.Expect(apis.ExpectClusterHealthy(after)).To(o.Succeed())
	o.Expect(after.Status.Nodes).NotTo(o.BeNil())
	o.Expect(*after.Status.Nodes).To(o.HaveLen(len(nodes)))
	previousNodes := make(map[string]etcdv1.PacemakerClusterNodeStatus, len(nodes))
	for _, node := range *before.Status.Nodes {
		previousNodes[node.NodeName] = node
	}
	for _, node := range *after.Status.Nodes {
		o.Expect(apis.ExpectNodeCondition(after, node.NodeName, etcdv1.NodeHealthyConditionType, metav1.ConditionTrue)).To(o.Succeed())
		previous := previousNodes[node.NodeName]
		observed := meta.FindStatusCondition(node.Conditions, alertAgentsObserved)
		o.Expect(observed).NotTo(o.BeNil())
		o.Expect(observed.Status).To(o.Equal(metav1.ConditionFalse))
		o.Expect(observed.Reason).To(o.Equal(featureGateUnresolved))
		o.Expect(node.AlertAgents).To(o.Equal(previous.AlertAgents))
		o.Expect(meta.FindStatusCondition(node.Conditions, etcdv1.NodeAlertAgentsHealthyConditionType)).
			To(o.Equal(meta.FindStatusCondition(previous.Conditions, etcdv1.NodeAlertAgentsHealthyConditionType)))
	}
	expectTNFAlertUnregistered(window, pcs, nodes[0], id)
	o.Expect(window.Err()).NotTo(o.HaveOccurred())

	g.By("letting the scheduled collector resolve its own release and repair the fault")
	o.Expect(waitForTNFAlertStatus(ctx, oc, nodes, after.Status.LastUpdated.Time)).To(o.Succeed())
	output, err := pcs.output(ctx, nodes[0], "alert", "config", "--output-format", "json")
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(expectTNFAlertRegistrations(output)).To(o.Succeed())
	o.Expect(apis.WaitForPacemakerHealthCheckCleared(oc, tnfObservabilityTimeout)).To(o.Succeed())
}

func waitForTNFCollectorWindow(ctx context.Context, oc *exutil.CLI) *batchv1.CronJob {
	var cron *batchv1.CronJob
	o.Expect(retryTNFOperation(ctx, time.Second, tnfObservabilityTimeout, func(ctx context.Context) error {
		var err error
		cron, err = oc.AdminKubeClient().BatchV1().CronJobs(tnfCollectorNamespace).Get(ctx, tnfCollectorName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		scheduled, completed := cron.Status.LastScheduleTime, cron.Status.LastSuccessfulTime
		if scheduled == nil || completed == nil || completed.Before(scheduled) || len(cron.Status.Active) != 0 {
			return fmt.Errorf("waiting for a scheduled collector to complete successfully")
		}
		if time.Until(scheduled.Add(time.Minute)) < 40*time.Second {
			return fmt.Errorf("too little time before the next scheduled collection")
		}
		return nil
	})).To(o.Succeed())
	return cron
}

func runTNFCollector(ctx context.Context, oc *exutil.CLI, cron *batchv1.CronJob, release string) {
	spec := cron.Spec.JobTemplate.Spec.DeepCopy()
	env := spec.Template.Spec.Containers[0].Env
	found := false
	for i := range env {
		if env[i].Name == tnfCollectorVersionEnv {
			env[i] = corev1.EnvVar{Name: tnfCollectorVersionEnv, Value: release}
			found = true
			break
		}
	}
	o.Expect(found).To(o.BeTrue(), "collector must be given its release version")

	jobs := oc.AdminKubeClient().BatchV1().Jobs(tnfCollectorNamespace)
	job, err := jobs.Create(ctx, &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{GenerateName: "tnf-observability-e2e-"},
		Spec:       *spec,
	}, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred())
	g.DeferCleanup(func(ctx context.Context) error {
		policy := metav1.DeletePropagationForeground
		err := jobs.Delete(ctx, job.Name, metav1.DeleteOptions{PropagationPolicy: &policy})
		if err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return wait.PollUntilContextTimeout(ctx, time.Second, tnfCommandTimeout, true, func(ctx context.Context) (bool, error) {
			_, err := jobs.Get(ctx, job.Name, metav1.GetOptions{})
			return apierrors.IsNotFound(err), nil
		})
	})
	o.Expect(wait.PollUntilContextTimeout(ctx, time.Second, tnfCommandTimeout, true, func(ctx context.Context) (bool, error) {
		current, err := jobs.Get(ctx, job.Name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		for _, condition := range current.Status.Conditions {
			if condition.Status != corev1.ConditionTrue {
				continue
			}
			switch condition.Type {
			case batchv1.JobComplete:
				return true, nil
			case batchv1.JobFailed:
				return false, fmt.Errorf("collector Job %s failed: %s", job.Name, condition.Message)
			}
		}
		return false, nil
	})).To(o.Succeed(), "collector Job %s must complete successfully", job.Name)
}
