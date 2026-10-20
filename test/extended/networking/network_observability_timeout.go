package networking

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
)

const netobservDeployedCondition = "NetworkObservabilityDeployed"

var _ = g.Describe("[sig-network][OCPFeatureGate:NetworkObservabilityInstall][Feature:NetObserv][Serial]", g.Serial, func() {
	oc := exutil.NewCLIWithoutNamespace("netobserv-timeout-e2e")

	g.BeforeEach(func(ctx context.Context) {
		// CNO starts the timeout at the later of cluster availability and the attempt.
		cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		availableLongEnough := false
		for _, condition := range cv.Status.Conditions {
			if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionTrue {
				availableLongEnough = condition.LastTransitionTime.Before(&metav1.Time{Time: time.Now().Add(-21 * time.Minute)})
			}
		}
		if !availableLongEnough {
			g.Skip("cluster must have been available for 21 minutes to accelerate the installation timeout")
		}

		network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		hub, err := oc.AdminConfigClient().ConfigV1().OperatorHubs().Get(ctx, clusterConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		// Independent cleanup callbacks ensure configuration is restored even if a step fails.
		g.DeferCleanup(func(cleanupCtx context.Context) error {
			return netobservSetPolicy(cleanupCtx, oc, network.Spec.NetworkObservability.InstallationPolicy)
		}, g.NodeTimeout(time.Minute))
		g.DeferCleanup(func(cleanupCtx context.Context) error {
			return netobservUpdateDeploymentCondition(cleanupCtx, oc, nil)
		}, g.NodeTimeout(time.Minute))
		g.DeferCleanup(func(cleanupCtx context.Context) error {
			return netobservSetDefaultSourcesDisabled(cleanupCtx, oc, hub.Spec.DisableAllDefaultSources)
		}, g.NodeTimeout(time.Minute))
	})

	g.It("Network Observability should clean up a timed out OLM installation [Timeout:30m]", func(ctx context.Context) {
		g.By("checking that the preinstalled FlowCollector is Ready")
		ready, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"flowcollectors.flows.netobserv.io", flowCollectorName,
			"-o=jsonpath={.status.conditions[?(@.type==\"Ready\")].status}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ready).To(o.Equal("True"))

		g.By("setting NoAction and uninstalling NetObserv")
		o.Expect(netobservSetPolicy(ctx, oc, configv1.NetworkObservabilityNoAction)).To(o.Succeed())
		csv, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"subscription", netobservOperatorNamespace, "-n", netobservOperatorNamespace, "-o=jsonpath={.status.installedCSV}",
		).Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(csv).NotTo(o.BeEmpty())
		for _, args := range [][]string{
			{"subscription", netobservOperatorNamespace, "-n", netobservOperatorNamespace},
			{"flowcollectors.flows.netobserv.io", flowCollectorName},
			{"csv", csv, "-n", netobservOperatorNamespace},
			{"operatorgroup", netobservOperatorNamespace, "-n", netobservOperatorNamespace},
			{"installplan", "--all", "-n", netobservOperatorNamespace},
		} {
			err = oc.AsAdmin().WithoutNamespace().Run("delete").Args(append(args, "--timeout=2m")...).Execute()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete %s", args[0])
		}

		g.By("deleting the FlowCollector CRD and clearing NetworkObservabilityDeployed")
		err = oc.AsAdmin().WithoutNamespace().Run("delete").Args("crd", "flowcollectors.flows.netobserv.io", "--timeout=2m").Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(netobservUpdateDeploymentCondition(ctx, oc, nil)).To(o.Succeed())

		g.By("disabling default OperatorHub sources")
		o.Expect(netobservSetDefaultSourcesDisabled(ctx, oc, true)).To(o.Succeed())
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args(
			"--for=delete", "catalogsource/redhat-operators", "-n", "openshift-marketplace", "--timeout=5m",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		// Ignore ResolutionFailed events from earlier attempts.
		eventOptions := metav1.ListOptions{FieldSelector: "reason=ResolutionFailed"}
		previousEvents, err := oc.AdminKubeClient().CoreV1().Events("default").List(ctx, eventOptions)
		o.Expect(err).NotTo(o.HaveOccurred())
		previousCounts := map[string]int32{}
		for _, event := range previousEvents.Items {
			previousCounts[string(event.UID)] = event.Count
		}

		g.By("setting InstallAndEnable and checking InstallationInProgress without an installed operator")
		o.Expect(netobservSetPolicy(ctx, oc, configv1.NetworkObservabilityInstallAndEnable)).To(o.Succeed())
		o.Eventually(ctx, func() (string, error) {
			return netobservDeploymentReason(ctx, oc)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal("InstallationInProgress"))
		for _, args := range [][]string{
			{"csv", "-n", netobservOperatorNamespace, "-o=name"},
			{"crd", "flowcollectors.flows.netobserv.io", "--ignore-not-found", "-o=name"},
		} {
			output, _, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(args...).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.BeEmpty(), "%s should not exist", args[0])
		}

		g.By("checking ResolutionFailed event counts in the default namespace")
		o.Eventually(ctx, func() (bool, error) {
			events, err := oc.AdminKubeClient().CoreV1().Events("default").List(ctx, eventOptions)
			if err != nil {
				return false, err
			}
			for _, event := range events.Items {
				if event.InvolvedObject.Namespace != netobservOperatorNamespace && event.InvolvedObject.Name != netobservOperatorNamespace && !strings.Contains(event.Message, netobservOperatorNamespace) {
					continue
				}
				if event.Count > previousCounts[string(event.UID)] {
					framework.Logf("ResolutionFailed COUNT=%d FIRST=%s LAST=%s MSG=%s", event.Count, event.FirstTimestamp, event.LastTimestamp, event.Message)
					return true, nil
				}
			}
			return false, nil
		}, 5*time.Minute, 10*time.Second).Should(o.BeTrue())

		g.By("moving the attempt timestamp past 20 minutes and checking DeploymentTimedOut")
		expired := time.Now().Add(-21 * time.Minute)
		o.Expect(netobservUpdateDeploymentCondition(ctx, oc, &expired)).To(o.Succeed())
		o.Eventually(ctx, func() (string, error) {
			return netobservDeploymentReason(ctx, oc)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal("DeploymentTimedOut"))

		g.By("checking that CNO-created NetObserv resources were deleted")
		err = oc.AsAdmin().WithoutNamespace().Run("wait").Args(
			"--for=delete", "namespace/"+netobservOperatorNamespace, "--timeout=5m",
		).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, resource := range []string{"sub", "og", "csv"} {
			output, _, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
				resource, "-n", netobservOperatorNamespace, "-o=name",
			).Outputs()
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(strings.TrimSpace(output)).To(o.BeEmpty(), "%s must be absent after DeploymentTimedOut", resource)
		}
		output, _, err := oc.AsAdmin().WithoutNamespace().Run("get").Args(
			"project", netobservOperatorNamespace, "--ignore-not-found", "-o=name",
		).Outputs()
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(strings.TrimSpace(output)).To(o.BeEmpty(), "project %s must be absent after DeploymentTimedOut", netobservOperatorNamespace)

		g.By("checking that DeploymentTimedOut remains set with no new ResolutionFailed events for two minutes")
		events, err := oc.AdminKubeClient().CoreV1().Events("default").List(ctx, eventOptions)
		o.Expect(err).NotTo(o.HaveOccurred())
		timedOutCounts := map[string]int32{}
		for _, event := range events.Items {
			timedOutCounts[string(event.UID)] = event.Count
		}
		o.Consistently(ctx, func() error {
			reason, err := netobservDeploymentReason(ctx, oc)
			if err != nil {
				return err
			}
			if reason != "DeploymentTimedOut" {
				return fmt.Errorf("expected DeploymentTimedOut, got %q", reason)
			}
			events, err := oc.AdminKubeClient().CoreV1().Events("default").List(ctx, eventOptions)
			if err != nil {
				return err
			}
			for _, event := range events.Items {
				if event.InvolvedObject.Namespace != netobservOperatorNamespace && event.InvolvedObject.Name != netobservOperatorNamespace && !strings.Contains(event.Message, netobservOperatorNamespace) {
					continue
				}
				count, existed := timedOutCounts[string(event.UID)]
				if !existed || event.Count > count {
					return fmt.Errorf("new ResolutionFailed event after timeout: count=%d, previous=%d, message=%s", event.Count, count, event.Message)
				}
			}
			return nil
		}, 2*time.Minute, 5*time.Second).Should(o.Succeed())

		g.By("re-enabling default OperatorHub sources and clearing NetworkObservabilityDeployed")
		o.Expect(netobservSetDefaultSourcesDisabled(ctx, oc, false)).To(o.Succeed())
		o.Expect(netobservUpdateDeploymentCondition(ctx, oc, nil)).To(o.Succeed())
	}, g.SpecTimeout(30*time.Minute))
})

// A nil timestamp clears the terminal state; a timestamp ages an actual failed attempt.
func netobservUpdateDeploymentCondition(ctx context.Context, oc *exutil.CLI, expired *time.Time) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		client := oc.AdminOperatorClient().OperatorV1().Networks()
		network, err := client.Get(ctx, clusterConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		found := false
		conditions := make([]operatorv1.OperatorCondition, 0, len(network.Status.Conditions))
		for _, condition := range network.Status.Conditions {
			if condition.Type == netobservDeployedCondition {
				found = true
				if expired == nil {
					continue
				}
				if condition.Status != operatorv1.ConditionFalse || condition.Reason != "InstallationInProgress" {
					return fmt.Errorf("expected a blocked installation, got %+v", condition)
				}
				condition.LastTransitionTime = metav1.NewTime(*expired)
			}
			conditions = append(conditions, condition)
		}
		if expired != nil && !found {
			return fmt.Errorf("missing %s condition", netobservDeployedCondition)
		}
		network.Status.Conditions = conditions
		_, err = client.UpdateStatus(ctx, network, metav1.UpdateOptions{})
		return err
	})
}

func netobservSetPolicy(ctx context.Context, oc *exutil.CLI, policy configv1.NetworkObservabilityInstallationPolicy) error {
	patch := fmt.Sprintf(`{"spec":{"networkObservability":{"installationPolicy":%q}}}`, policy)
	if policy == "" {
		patch = `{"spec":{"networkObservability":null}}`
	}
	_, err := oc.AdminConfigClient().ConfigV1().Networks().Patch(ctx, clusterConfig, types.MergePatchType, []byte(patch), metav1.PatchOptions{})
	return err
}

func netobservSetDefaultSourcesDisabled(ctx context.Context, oc *exutil.CLI, disabled bool) error {
	patch := []byte(fmt.Sprintf(`{"spec":{"disableAllDefaultSources":%t}}`, disabled))
	_, err := oc.AdminConfigClient().ConfigV1().OperatorHubs().Patch(ctx, clusterConfig, types.MergePatchType, patch, metav1.PatchOptions{})
	return err
}

func netobservDeploymentReason(ctx context.Context, oc *exutil.CLI) (string, error) {
	network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	for _, condition := range network.Status.Conditions {
		if condition.Type == netobservDeployedCondition {
			return condition.Reason, nil
		}
	}
	return "", nil
}
