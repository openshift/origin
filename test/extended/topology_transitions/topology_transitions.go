package topology_transitions

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	etcdhelpers "github.com/openshift/origin/test/extended/etcd/helpers"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
	coutil "github.com/openshift/origin/test/extended/util/operator"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	preconditionWaitTimeout = 20 * time.Minute
	baselineWorkloadName    = "topology-transitions-baseline"
	suiteTags               = "[sig-etcd][sig-node][OCPFeatureGate:MutableTopology][Serial]"
)

func init() {
	target, err := transitionTargetFromEnvironment(os.Getenv)
	if err == nil && !supportsTarget(target.Topology) {
		err = fmt.Errorf("no supported target matches control-plane=%s infrastructure=%s", target.Topology.ControlPlaneTopology, target.Topology.InfrastructureTopology)
	}
	if err != nil {
		registerTargetError("openshift/topology-transitions/preflight", "30m", "", err)
		registerTargetError("openshift/topology-transitions", "150m", "[Disruptive]", err)
		return
	}

	registerPreflightTest(target)
	registerTransitionTest(target)
}

func registerTargetError(suite, timeout, extraTags string, err error) {
	g.Describe(suiteTags+"[Suite:"+suite+"]"+extraTags+" transition target configuration", func() {
		g.It("requires valid configuration [Timeout:"+timeout+"]", func() {
			o.Expect(err).NotTo(o.HaveOccurred(), "transition target configuration is invalid: %v", err)
		})
	})
}

func requireTransitionSource(ctx context.Context, oc *exutil.CLI, target transitionTarget) (*transitionDefinition, *configv1.Infrastructure) {
	infra, err := getInfrastructure(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())

	definition, err := selectTransitionDefinition(infra, target.Topology)
	o.Expect(err).NotTo(o.HaveOccurred())

	return definition, requireSource(ctx, oc, definition)
}

func requireSource(ctx context.Context, oc *exutil.CLI, definition *transitionDefinition) *configv1.Infrastructure {
	infra, err := getInfrastructure(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred())
	o.Expect(definition.matchesSource(infra)).To(o.BeTrue(), "cluster must match the source for %s", definition.Name)

	// Do not run against a pending request: preflight must not admit a transition.
	o.Expect(infra.Spec.ControlPlaneTopology == "" || infra.Spec.ControlPlaneTopology == definition.Source.ControlPlaneTopology).To(o.BeTrue(), "a topology transition request is already pending")

	return infra
}

func waitForEvaluation(ctx context.Context, oc *exutil.CLI, definition *transitionDefinition, preflight bool) {
	o.Eventually(func() error {
		infra, err := getInfrastructure(ctx, oc)
		if err != nil {
			return err
		}

		return checkTransitionEvaluation(infra, definition, preflight)
	}).WithContext(ctx).WithTimeout(preconditionWaitTimeout).WithPolling(15 * time.Second).Should(o.Succeed())
}

func registerPreflightTest(target transitionTarget) {
	g.Describe(suiteTags+"[Suite:openshift/topology-transitions/preflight] Topology transition preflight", func() {
		oc := exutil.NewCLIWithoutNamespace("topology-transitions-preflight").AsAdmin()

		g.It("reports the expected preflight checks [Timeout:30m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
			definition, before := requireTransitionSource(ctx, oc, target)

			g.By("checking the source node counts for " + definition.Name)
			o.Expect(checkNodeCounts(ctx, oc, definition.SourceNodes)).To(o.Succeed())
			factory := etcdhelpers.NewEtcdClientFactory(oc.AdminKubeClient())
			o.Expect(etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), factory, oc.AdminKubeClient(), definition.SourceNodes.ControlPlaneNodes+definition.SourceNodes.ArbiterNodes)).To(o.Succeed())

			g.By("waiting for the expected preflight results for " + definition.Name)
			waitForEvaluation(ctx, oc, definition, true)

			after := requireSource(ctx, oc, definition)
			o.Expect(after.Spec).To(o.Equal(before.Spec), "preflight must leave the topology request unchanged")
		})
	})
}

func registerTransitionTest(target transitionTarget) {
	g.Describe(suiteTags+"[Suite:openshift/topology-transitions][Disruptive] Topology transition", func() {
		oc := exutil.NewCLI("topology-transitions").AsAdmin()

		g.It("completes the configured topology transition [Timeout:150m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
			definition, _ := requireTransitionSource(ctx, oc, target)

			g.By("waiting for all transition preflight checks to pass for " + definition.Name)
			waitForEvaluation(ctx, oc, definition, false)

			g.By("creating and checking the baseline workload")
			o.Expect(createBaselineWorkload(ctx, oc)).To(o.Succeed())
			o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 5*time.Minute)).To(o.Succeed())

			before := requireSource(ctx, oc, definition)
			g.By("requesting the topology transition through oc adm")
			_, err := oc.Run("adm", "transition", "topology").Args(
				"--control-plane="+string(target.Topology.ControlPlaneTopology),
				"--infrastructure="+string(target.Topology.InfrastructureTopology),
				"--confirm",
			).EnvVar("OC_ENABLE_CMD_TRANSITION_TOPOLOGY", "true").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "oc adm transition topology failed")

			requested, err := getInfrastructure(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(requested.Spec.ControlPlaneTopology).To(o.Equal(target.Topology.ControlPlaneTopology))
			o.Expect(requested.Generation).To(o.BeNumerically(">", before.Generation), "the CLI must submit a new request")
			requestGeneration := requested.Generation

			g.By("waiting for this request to complete and reach both target topologies")
			o.Eventually(func() error {
				infra, err := getInfrastructure(ctx, oc)
				if err != nil {
					return err
				}
				completed := meta.FindStatusCondition(infra.Status.TopologyTransitionStatus.Conditions, configv1.TopologyTransitionCompletedConditionType)
				if infra.Generation != requestGeneration || infra.Spec.ControlPlaneTopology != target.Topology.ControlPlaneTopology {
					return fmt.Errorf("the topology request changed during the transition")
				}
				if completed == nil || completed.ObservedGeneration != requestGeneration || completed.Status != metav1.ConditionTrue {
					return fmt.Errorf("waiting for TopologyTransitionCompleted=True for generation %d", requestGeneration)
				}
				if infra.Status.ControlPlaneTopology != target.Topology.ControlPlaneTopology || infra.Status.InfrastructureTopology != target.Topology.InfrastructureTopology {
					return fmt.Errorf("topology status has not reached both targets")
				}

				return nil
			}).WithContext(ctx).WithTimeout(90 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())

			g.By("waiting for operators and exact node counts to settle")
			o.Expect(coutil.WaitForOperatorsToSettle(ctx, oc.AdminConfigClient(), 20)).To(o.Succeed())
			o.Eventually(func() error {
				return checkNodeCounts(ctx, oc, target.nodeCounts)
			}).WithContext(ctx).WithTimeout(10 * time.Minute).WithPolling(15 * time.Second).Should(o.Succeed())

			g.By("checking etcd voting members and health")
			factory := etcdhelpers.NewEtcdClientFactory(oc.AdminKubeClient())
			o.Expect(etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), factory, oc.AdminKubeClient(), target.ControlPlaneNodes+target.ArbiterNodes)).To(o.Succeed())
			o.Expect(checkEtcdHealthy(ctx, oc)).To(o.Succeed())

			g.By("checking the baseline workload after the transition")
			o.Expect(exutil.WaitForDeploymentReadyWithTimeout(oc, baselineWorkloadName, oc.Namespace(), -1, 2*time.Minute)).To(o.Succeed())
		})
	})
}

// The workload checks scheduling before and after the transition.
func createBaselineWorkload(ctx context.Context, oc *exutil.CLI) error {
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      baselineWorkloadName,
			Namespace: oc.Namespace(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: new(int32(2)),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": baselineWorkloadName},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": baselineWorkloadName},
				},
				Spec: corev1.PodSpec{
					// Soft (ScheduleAnyway), not required: this workload is
					// created before the transition, so a hard constraint
					// could leave a replica unschedulable if the starting
					// topology has too few eligible nodes. Once the target
					// topology is ready, this best-effort constraint encourages
					// the replicas to spread across available nodes
					// rather than staying co-located on the original node.
					TopologySpreadConstraints: []corev1.TopologySpreadConstraint{{
						MaxSkew:           1,
						TopologyKey:       "kubernetes.io/hostname",
						WhenUnsatisfiable: corev1.ScheduleAnyway,
						LabelSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": baselineWorkloadName},
						},
					}},
					Containers: []corev1.Container{{
						Name:    "workload",
						Image:   image.ShellImage(),
						Command: []string{"sleep", "infinity"},
					}},
				},
			},
		},
	}
	_, err := oc.KubeClient().AppsV1().Deployments(oc.Namespace()).Create(ctx, deployment, metav1.CreateOptions{})
	return err
}
