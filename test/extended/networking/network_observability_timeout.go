package networking

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	operatorv1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
)

const netobservDeployedCondition = "NetworkObservabilityDeployed"

var _ = g.Describe("[sig-network][OCPFeatureGate:NetworkObservabilityInstall][Feature:NetObserv][Serial]", g.Serial, func() {
	oc := exutil.NewCLIWithoutNamespace("netobserv-timeout-e2e")

	g.It("Network Observability should clean up a timed out OLM installation [Timeout:30m]", func(ctx context.Context) {
		const quotaName = "netobserv-e2e-block-installplans"
		kube := oc.AdminKubeClient()
		dc := oc.AdminDynamicClient()
		fcClient := dc.Resource(schema.GroupVersionResource{Group: "flows.netobserv.io", Version: "v1beta2", Resource: "flowcollectors"})
		crdClient := dc.Resource(schema.GroupVersionResource{Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions"})
		subs := dc.Resource(schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "subscriptions"}).Namespace(netobservOperatorNamespace)
		groups := dc.Resource(schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1", Resource: "operatorgroups"}).Namespace(netobservOperatorNamespace)
		csvs := dc.Resource(schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "clusterserviceversions"}).Namespace(netobservOperatorNamespace)
		plans := dc.Resource(schema.GroupVersionResource{Group: "operators.coreos.com", Version: "v1alpha1", Resource: "installplans"}).Namespace(netobservOperatorNamespace)

		g.By("requiring a healthy CNO-owned OLM v0 installation")
		network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(ctx, clusterConfig, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		if network.Spec.NetworkObservability.InstallationPolicy == configv1.NetworkObservabilityNoAction {
			g.Skip("automatic Network Observability installation is disabled")
		}
		ns, err := kube.CoreV1().Namespaces().Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			g.Skip("Network Observability is not installed")
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if ns.Annotations["network.operator.openshift.io/created-by-cno"] != "true" {
			g.Skip("the operator namespace is not owned by CNO")
		}
		sub, err := subs.Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			g.Skip("this test requires an OLM v0 Subscription")
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		group, err := groups.Get(ctx, netobservOperatorNamespace, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		fc, err := fcClient.Get(ctx, flowCollectorName, metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(netobservCollectorReady(fc)).To(o.BeTrue(), "NetObserv must be healthy before disruption")
		crd, err := crdClient.Get(ctx, "flowcollectors.flows.netobserv.io", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		csvName, _, err := unstructured.NestedString(sub.Object, "status", "installedCSV")
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(csvName).To(o.HavePrefix("network-observability-operator."))
		// Aging only the attempt avoids modifying ClusterVersion or stopping CVO.
		// CNO uses the later of cluster availability and the attempt timestamp.
		if err := netobservClusterAvailableBefore(ctx, oc, time.Now().Add(-21*time.Minute)); err != nil {
			g.Skip(fmt.Sprintf("cluster must have been available for at least 21 minutes: %v", err))
		}

		// Register policy restoration first so it runs even if reinstall cleanup fails.
		g.DeferCleanup(func(cleanupCtx context.Context) error {
			return netobservSetPolicy(cleanupCtx, oc, network.Spec.NetworkObservability.InstallationPolicy)
		}, g.NodeTimeout(time.Minute))
		g.DeferCleanup(func(cleanupCtx context.Context) error {
			g.By("restoring the original NetObserv resources and waiting for readiness")
			if err := netobservSetPolicy(cleanupCtx, oc, configv1.NetworkObservabilityNoAction); err != nil {
				return err
			}
			// Rollback may have started namespace deletion before an assertion failed.
			if err := wait.PollUntilContextTimeout(cleanupCtx, 5*time.Second, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
				current, err := kube.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					_, err = kube.CoreV1().Namespaces().Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: ns.Name, Labels: ns.Labels, Annotations: ns.Annotations}}, metav1.CreateOptions{})
					return err == nil, err
				}
				return err == nil && current.DeletionTimestamp == nil, err
			}); err != nil {
				return fmt.Errorf("restore operator namespace: %w", err)
			}
			if err := kube.CoreV1().ResourceQuotas(ns.Name).Delete(cleanupCtx, quotaName, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				return err
			}
			// Restore OLM explicitly: a surviving CRD alone makes CNO consider the
			// operator installed, including when the test fails partway through reset.
			for _, saved := range []struct {
				client dynamic.ResourceInterface
				object *unstructured.Unstructured
			}{{crdClient, crd}, {groups, group}, {subs, sub}} {
				if err := netobservRestoreResource(cleanupCtx, saved.client, saved.object); err != nil {
					return err
				}
			}
			// Wait for the operator's admission webhook before restoring its CR.
			if err := wait.PollUntilContextTimeout(cleanupCtx, 10*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
				csv, err := csvs.Get(ctx, csvName, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				phase, _, err := unstructured.NestedString(csv.Object, "status", "phase")
				return phase == "Succeeded", err
			}); err != nil {
				return fmt.Errorf("restore NetObserv operator: %w", err)
			}
			if err := netobservRestoreResource(cleanupCtx, fcClient, fc); err != nil {
				return err
			}
			if err := netobservSetPolicy(cleanupCtx, oc, configv1.NetworkObservabilityInstallAndEnable); err != nil {
				return err
			}
			if err := netobservUpdateDeploymentCondition(cleanupCtx, oc, nil); err != nil {
				return err
			}
			return wait.PollUntilContextTimeout(cleanupCtx, 10*time.Second, 8*time.Minute, true, func(ctx context.Context) (bool, error) {
				current, err := fcClient.Get(ctx, fc.GetName(), metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				csv, err := csvs.Get(ctx, csvName, metav1.GetOptions{})
				if apierrors.IsNotFound(err) {
					return false, nil
				}
				if err != nil {
					return false, err
				}
				phase, _, err := unstructured.NestedString(csv.Object, "status", "phase")
				if err != nil {
					return false, err
				}
				reason, err := netobservDeploymentReason(ctx, oc)
				return netobservCollectorReady(current) && phase == "Succeeded" && reason == "DeploymentComplete", err
			})
		}, g.NodeTimeout(20*time.Minute))

		g.By("resetting installation while automatic installation is paused")
		o.Expect(netobservSetPolicy(ctx, oc, configv1.NetworkObservabilityNoAction)).To(o.Succeed())
		for _, target := range []struct {
			client dynamic.ResourceInterface
			name   string
		}{{subs, sub.GetName()}, {fcClient, fc.GetName()}, {csvs, csvName}, {groups, group.GetName()}, {crdClient, crd.GetName()}} {
			o.Expect(target.client.Delete(ctx, target.name, metav1.DeleteOptions{})).To(o.Succeed())
			o.Eventually(ctx, func() bool {
				_, err := target.client.Get(ctx, target.name, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, 5*time.Minute, 5*time.Second).Should(o.BeTrue(), "%s should be deleted before continuing", target.name)
		}
		// Completed plans from the previous installation must not bypass the quota.
		o.Expect(plans.DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{})).To(o.Succeed())
		o.Eventually(ctx, func() (int, error) {
			items, err := plans.List(ctx, metav1.ListOptions{})
			if err != nil {
				return 0, err
			}
			return len(items.Items), nil
		}, time.Minute, 5*time.Second).Should(o.Equal(0))

		g.By("blocking only NetObserv InstallPlans with a namespace quota")
		_, err = kube.CoreV1().ResourceQuotas(ns.Name).Create(ctx, &corev1.ResourceQuota{
			ObjectMeta: metav1.ObjectMeta{Name: quotaName},
			Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
				corev1.ResourceName("count/installplans.operators.coreos.com"): resource.MustParse("0"),
			}},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Eventually(ctx, func() bool {
			// A dry-run admission request proves the quota blocks InstallPlans
			// without relying on OLM log wording or creating another resource.
			_, err := plans.Create(ctx, &unstructured.Unstructured{Object: map[string]interface{}{
				"apiVersion": "operators.coreos.com/v1alpha1", "kind": "InstallPlan",
				"metadata": map[string]interface{}{"generateName": "netobserv-quota-probe-"},
				"spec":     map[string]interface{}{"approval": "Automatic", "approved": true, "clusterServiceVersionNames": []interface{}{csvName}},
			}}, metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}})
			return apierrors.IsForbidden(err) && strings.Contains(err.Error(), quotaName)
		}, time.Minute, 5*time.Second).Should(o.BeTrue(), "the test quota must reject InstallPlan creation")
		o.Expect(netobservUpdateDeploymentCondition(ctx, oc, nil)).To(o.Succeed())
		o.Expect(netobservSetPolicy(ctx, oc, configv1.NetworkObservabilityInstallAndEnable)).To(o.Succeed())
		o.Eventually(ctx, func() (string, error) {
			return netobservDeploymentReason(ctx, oc)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal("InstallationInProgress"))
		_, err = subs.Get(ctx, sub.GetName(), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		_, err = groups.Get(ctx, group.GetName(), metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Consistently(ctx, func() bool {
			_, err := crdClient.Get(ctx, crd.GetName(), metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, 30*time.Second, 5*time.Second).Should(o.BeTrue(), "the blocked installation must not create the FlowCollector CRD")

		g.By("aging the failed attempt past the 20-minute installation timeout")
		expired := time.Now().Add(-21 * time.Minute)
		o.Expect(netobservClusterAvailableBefore(ctx, oc, expired)).To(o.Succeed())
		o.Expect(netobservUpdateDeploymentCondition(ctx, oc, &expired)).To(o.Succeed())
		o.Eventually(ctx, func() (string, error) {
			return netobservDeploymentReason(ctx, oc)
		}, 3*time.Minute, 5*time.Second).Should(o.Equal("DeploymentTimedOut"))

		g.By("verifying rollback removes the OLM resources and CNO-owned namespace")
		o.Eventually(ctx, func() bool {
			_, err := kube.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
			return apierrors.IsNotFound(err)
		}, 5*time.Minute, 5*time.Second).Should(o.BeTrue())
		for _, client := range []dynamic.ResourceInterface{subs, groups, csvs} {
			items, err := client.List(ctx, metav1.ListOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(items.Items).To(o.BeEmpty())
		}
		o.Consistently(ctx, func() bool {
			_, err := kube.CoreV1().Namespaces().Get(ctx, ns.Name, metav1.GetOptions{})
			reason, conditionErr := netobservDeploymentReason(ctx, oc)
			return apierrors.IsNotFound(err) && conditionErr == nil && reason == "DeploymentTimedOut"
		}, time.Minute, 5*time.Second).Should(o.BeTrue(), "a terminal timeout must not restart installation")
	}, g.SpecTimeout(30*time.Minute))
})

func netobservSetPolicy(ctx context.Context, oc *exutil.CLI, policy configv1.NetworkObservabilityInstallationPolicy) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		client := oc.AdminConfigClient().ConfigV1().Networks()
		network, err := client.Get(ctx, clusterConfig, metav1.GetOptions{})
		if err != nil {
			return err
		}
		network.Spec.NetworkObservability.InstallationPolicy = policy
		_, err = client.Update(ctx, network, metav1.UpdateOptions{})
		return err
	})
}

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

func netobservClusterAvailableBefore(ctx context.Context, oc *exutil.CLI, before time.Time) error {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, condition := range cv.Status.Conditions {
		if condition.Type == configv1.OperatorAvailable && condition.Status == configv1.ConditionTrue && condition.LastTransitionTime.Time.Before(before) {
			return nil
		}
	}
	return fmt.Errorf("ClusterVersion has not been continuously Available since %s", before.Format(time.RFC3339))
}

func netobservRestoreResource(ctx context.Context, client dynamic.ResourceInterface, saved *unstructured.Unstructured) error {
	return wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		current, err := client.Get(ctx, saved.GetName(), metav1.GetOptions{})
		if err == nil {
			if current.GetDeletionTimestamp() != nil {
				return false, nil
			}
			if reflect.DeepEqual(current.Object["spec"], saved.Object["spec"]) {
				return true, nil
			}
			current.Object["spec"] = saved.DeepCopy().Object["spec"]
			_, err = client.Update(ctx, current, metav1.UpdateOptions{})
			if apierrors.IsConflict(err) {
				return false, nil
			}
			return err == nil, err
		}
		if !apierrors.IsNotFound(err) {
			return false, err
		}
		restored := &unstructured.Unstructured{Object: map[string]interface{}{
			"apiVersion": saved.GetAPIVersion(), "kind": saved.GetKind(), "spec": saved.Object["spec"],
		}}
		restored.SetName(saved.GetName())
		restored.SetNamespace(saved.GetNamespace())
		restored.SetLabels(saved.GetLabels())
		restored.SetAnnotations(saved.GetAnnotations())
		_, err = client.Create(ctx, restored, metav1.CreateOptions{})
		if apierrors.IsAlreadyExists(err) {
			return false, nil
		}
		return err == nil, err
	})
}

func netobservCollectorReady(fc *unstructured.Unstructured) bool {
	conditions, _, _ := unstructured.NestedSlice(fc.Object, "status", "conditions")
	ready := false
	for _, item := range conditions {
		condition, ok := item.(map[string]interface{})
		if ok && condition["type"] == "Ready" && condition["status"] == "True" {
			ready = true
		}
	}
	components, found, err := unstructured.NestedMap(fc.Object, "status", "components")
	if !ready || !found || err != nil || len(components) == 0 {
		return false
	}
	for _, item := range components {
		component, ok := item.(map[string]interface{})
		if !ok || component["state"] != "Ready" || component["readyReplicas"] != component["desiredReplicas"] {
			return false
		}
	}
	return true
}
