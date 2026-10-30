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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
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
	if err == nil && !supportsTarget(target) {
		err = fmt.Errorf("no supported target matches control-plane=%s infrastructure=%s node counts=%+v", target.Topology.ControlPlaneTopology, target.Topology.InfrastructureTopology, target.nodeCounts)
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
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster Infrastructure")

	definition, err := selectTransitionDefinition(infra, target)
	o.Expect(err).NotTo(o.HaveOccurred(), "source and target must match one supported transition definition")

	return definition, requireSource(ctx, oc, definition)
}

func requireSource(ctx context.Context, oc *exutil.CLI, definition *transitionDefinition) *configv1.Infrastructure {
	infra, err := getInfrastructure(ctx, oc)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster Infrastructure")
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
	}).WithContext(ctx).WithTimeout(preconditionWaitTimeout).WithPolling(15*time.Second).Should(o.Succeed(), "transition evaluation did not reach the expected current preflight results")
}

func registerPreflightTest(target transitionTarget) {
	g.Describe(suiteTags+"[Suite:openshift/topology-transitions/preflight] Topology transition preflight", func() {
		oc := exutil.NewCLIWithoutNamespace("topology-transitions-preflight").AsAdmin()

		g.It("reports the expected preflight checks [Timeout:30m][apigroup:config.openshift.io][apigroup:operator.openshift.io]", func(ctx context.Context) {
			definition, before := requireTransitionSource(ctx, oc, target)

			g.By("checking the source node counts for " + definition.Name)
			o.Expect(checkNodeCounts(ctx, oc, definition.SourceNodes)).To(o.Succeed(), "source node counts do not match the transition definition")
			factory := etcdhelpers.NewEtcdClientFactory(oc.AdminKubeClient())
			o.Expect(etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), factory, oc.AdminKubeClient(), definition.SourceNodes.ControlPlaneNodes+definition.SourceNodes.ArbiterNodes)).To(o.Succeed(), "source etcd voting member count does not match the transition definition")

			g.By("waiting for the expected preflight results for " + definition.Name)
			waitForEvaluation(ctx, oc, definition, true)

			after := requireSource(ctx, oc, definition)
			o.Expect(equality.Semantic.DeepEqual(after.Spec, before.Spec)).To(o.BeTrue(), "preflight must leave the topology request unchanged")
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
			o.Expect(createBaselineWorkload(ctx, oc)).To(o.Succeed(), "failed to create the baseline workload")
			o.Eventually(func() error {
				return checkBaselineWorkloadReady(ctx, oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()))
			}).WithContext(ctx).WithTimeout(5*time.Minute).WithPolling(2*time.Second).Should(o.Succeed(), "baseline deployment did not become ready before the transition")

			before := requireSource(ctx, oc, definition)
			g.By("requesting the topology transition through oc adm")
			// Output logs raw stdout and stderr on failure; keep them out of suite logs.
			command, _, _, err := oc.Run("adm", "transition", "topology").Args(
				"--control-plane="+string(target.Topology.ControlPlaneTopology),
				"--infrastructure="+string(target.Topology.InfrastructureTopology),
				"--confirm",
			).EnvVar("OC_ENABLE_CMD_TRANSITION_TOPOLOGY", "true").Background()
			if err == nil {
				stop := context.AfterFunc(ctx, func() {
					_ = command.Process.Kill()
				})

				err = command.Wait()
				stop()
			}
			o.Expect(err == nil).To(o.BeTrue(), "oc adm transition topology failed")

			requested, err := getInfrastructure(ctx, oc)
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster Infrastructure")
			o.Expect(requested.Spec.ControlPlaneTopology).To(o.Equal(target.Topology.ControlPlaneTopology), "requested control-plane topology was not recorded")
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
			}).WithContext(ctx).WithTimeout(90*time.Minute).WithPolling(15*time.Second).Should(o.Succeed(), "topology transition did not complete with both target topologies for the current request")

			g.By("waiting for operators and exact node counts to settle")
			o.Eventually(func() error {
				return checkOperatorsSettled(ctx, oc.AdminConfigClient().ConfigV1())
			}).WithContext(ctx).WithTimeout(20*time.Minute).WithPolling(10*time.Second).Should(o.Succeed(), "operators did not settle after the transition")
			o.Eventually(func() error {
				return checkNodeCounts(ctx, oc, target.nodeCounts)
			}).WithContext(ctx).WithTimeout(10*time.Minute).WithPolling(15*time.Second).Should(o.Succeed(), "target node counts did not settle after the transition")

			g.By("checking etcd voting members and health")
			factory := etcdhelpers.NewEtcdClientFactory(oc.AdminKubeClient())
			o.Expect(etcdhelpers.EnsureVotingMembersCount(ctx, g.GinkgoT(), factory, oc.AdminKubeClient(), target.ControlPlaneNodes+target.ArbiterNodes)).To(o.Succeed(), "etcd voting member count did not reach the target after the transition")
			o.Eventually(func() error {
				return checkEtcdHealthy(ctx, oc)
			}).WithContext(ctx).WithTimeout(10*time.Minute).WithPolling(15*time.Second).Should(o.Succeed(), "etcd did not become healthy after the transition")

			g.By("checking the baseline workload after the transition")
			o.Eventually(func() error {
				return checkBaselineWorkloadReady(ctx, oc.AdminKubeClient().AppsV1().Deployments(oc.Namespace()))
			}).WithContext(ctx).WithTimeout(2*time.Minute).WithPolling(2*time.Second).Should(o.Succeed(), "baseline deployment did not become ready after the transition")
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
	if err != nil {
		return fmt.Errorf("failed to create baseline workload")
	}

	return nil
}
