package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2edeployment "k8s.io/kubernetes/test/e2e/framework/deployment"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
	anpapi "sigs.k8s.io/network-policy-api/apis/v1alpha1"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	anpReadyInZonePrefix          = "Ready-In-Zone-"
	ovnKubeControlPlaneDeployment = "ovnkube-control-plane"
	anpStatusCleanupANPName       = "e2e-anp-status-cleanup"
	anpStatusCleanupBANPName      = "default"
	anpStatusCleanupNSLabel       = "e2e.openshift.io/anp-status-cleanup"
	ovnNodeSubnetsAnnotation      = "k8s.ovn.org/node-subnets"
)

var (
	anpGVR  = anpapi.SchemeGroupVersion.WithResource("adminnetworkpolicies")
	banpGVR = anpapi.SchemeGroupVersion.WithResource("baselineadminnetworkpolicies")
)

// anpStatusPolicy is table glue for ANP vs BANP. The imported API types are
// distinct (only ANP has spec.priority), so DescribeTable still needs a common key.
type anpStatusPolicy struct {
	gvr  schema.GroupVersionResource
	name string
}

func (pt anpStatusPolicy) kind() string {
	if pt.gvr.Resource == banpGVR.Resource {
		return "BaselineAdminNetworkPolicy"
	}
	return "AdminNetworkPolicy"
}

func (pt anpStatusPolicy) isBANP() bool {
	return pt.gvr.Resource == banpGVR.Resource
}

var (
	anpStatusCleanupANP = anpStatusPolicy{
		gvr:  anpGVR,
		name: anpStatusCleanupANPName,
	}
	anpStatusCleanupBANP = anpStatusPolicy{
		gvr:  banpGVR,
		name: anpStatusCleanupBANPName,
	}
)

// These tests verify that stale Ready-In-Zone-* conditions on AdminNetworkPolicy
// and BaselineAdminNetworkPolicy are removed when ovnkube-control-plane starts
// and the owning zone is gone.
//
// OpenShift CI cannot delete a node to create a gone zone (kubelet re-registers
// it, and CNO reconciles the control-plane deployment). Instead we inject a
// Server-Side Apply status condition owned by a fake zone field manager, restart
// ovnkube-control-plane, and assert startup cleanup drops that condition while
// real node conditions remain.
var _ = g.Describe("[sig-network][Feature:AdminNetworkPolicy][Serial][apigroup:policy.networking.k8s.io]", func() {
	oc := exutil.NewCLIWithoutNamespace("anp-status-cleanup")

	InOVNKubernetesContext(func() {
		g.BeforeEach(func(ctx context.Context) {
			isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isMicroShift {
				g.Skip("AdminNetworkPolicy status cleanup is not applicable to MicroShift")
			}

			isHyperShift, err := exutil.IsHypershift(ctx, oc.AdminConfigClient())
			o.Expect(err).NotTo(o.HaveOccurred())
			if isHyperShift {
				g.Skip("ovnkube-control-plane is not on the hosted cluster")
			}

			_, err = oc.AdminKubeClient().AppsV1().Deployments(ovnNamespace).Get(ctx, ovnKubeControlPlaneDeployment, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				g.Skip("ovnkube-control-plane deployment not found")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ovnkube-control-plane deployment")

			_, err = oc.AdminDynamicClient().Resource(anpGVR).List(ctx, metav1.ListOptions{Limit: 1})
			if meta.IsNoMatchError(err) || apierrors.IsNotFound(err) {
				g.Skip("AdminNetworkPolicy API is not available on this cluster")
			}
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to list AdminNetworkPolicies")
		})

		g.DescribeTable("should keep Ready-In-Zone conditions for current nodes after ovnkube-control-plane restart",
			func(ctx context.Context, pt anpStatusPolicy) {
				cs := oc.AdminKubeClient()
				dyn := oc.AdminDynamicClient()

				created := ensureANPStatusPolicy(ctx, dyn, pt)
				g.DeferCleanup(func(ctx context.Context) {
					if created {
						err := dyn.Resource(pt.gvr).Delete(ctx, pt.name, metav1.DeleteOptions{})
						if err != nil && !apierrors.IsNotFound(err) {
							e2e.Logf("failed to delete %s %s: %v", pt.kind(), pt.name, err)
						}
					}
				})

				g.By(fmt.Sprintf("Waiting for %s %q Ready-In-Zone conditions to match current nodes", pt.kind(), pt.name))
				waitForANPReadyInZoneTypes(ctx, dyn, cs, pt)

				g.By("Restarting ovnkube-control-plane")
				restartOVNKubeControlPlane(ctx, cs)

				g.By(fmt.Sprintf("Verifying %s %q still has Ready-In-Zone conditions for every current node", pt.kind(), pt.name))
				o.Eventually(func() error {
					return assertReadyInZoneTypes(ctx, dyn, cs, pt, "")
				}).WithContext(ctx).WithTimeout(3*time.Minute).WithPolling(5*time.Second).Should(o.Succeed(),
					"%s %q should keep Ready-In-Zone conditions for current nodes after restart", pt.kind(), pt.name)
			},
			g.Entry("AdminNetworkPolicy", anpStatusCleanupANP),
			g.Entry("BaselineAdminNetworkPolicy", anpStatusCleanupBANP),
		)

		g.DescribeTable("should remove stale Ready-In-Zone conditions after ovnkube-control-plane restart",
			func(ctx context.Context, pt anpStatusPolicy) {
				cs := oc.AdminKubeClient()
				dyn := oc.AdminDynamicClient()

				created := ensureANPStatusPolicy(ctx, dyn, pt)
				g.DeferCleanup(func(ctx context.Context) {
					if created {
						err := dyn.Resource(pt.gvr).Delete(ctx, pt.name, metav1.DeleteOptions{})
						if err != nil && !apierrors.IsNotFound(err) {
							e2e.Logf("failed to delete %s %s: %v", pt.kind(), pt.name, err)
						}
					}
				})

				g.By(fmt.Sprintf("Waiting for %s %q Ready-In-Zone conditions to match current nodes", pt.kind(), pt.name))
				waitForANPReadyInZoneTypes(ctx, dyn, cs, pt)

				staleZone := fmt.Sprintf("e2e-stale-%s", uuid.NewUUID()[:8])
				staleType := anpReadyInZonePrefix + staleZone
				g.DeferCleanup(func(ctx context.Context) {
					clearANPZoneStatus(ctx, dyn, pt, staleZone)
				})

				g.By(fmt.Sprintf("Injecting stale condition %s via SSA field manager %s", staleType, staleZone))
				injectStaleANPZoneStatus(ctx, dyn, pt, staleZone, staleType)

				g.By("Confirming the injected stale condition is present")
				o.Eventually(func() (bool, error) {
					obj, err := dyn.Resource(pt.gvr).Get(ctx, pt.name, metav1.GetOptions{})
					if err != nil {
						return false, err
					}
					types, err := readyInZoneConditionTypes(obj)
					if err != nil {
						return false, err
					}
					return sets.New(types...).Has(staleType), nil
				}).WithContext(ctx).WithTimeout(30*time.Second).WithPolling(2*time.Second).Should(o.BeTrue(),
					"expected injected stale condition %s on %s %q", staleType, pt.kind(), pt.name)

				g.By("Restarting ovnkube-control-plane so startup cleanup runs")
				restartOVNKubeControlPlane(ctx, cs)

				g.By("Verifying the stale condition is removed and real node conditions remain")
				// Extended timeout to cover pod startup + leader election window (~4 minutes)
				// PodReady ≠ leader; default lease is 137s, so allow time for startup cleanup after new pod becomes leader
				o.Eventually(func() error {
					return assertReadyInZoneTypes(ctx, dyn, cs, pt, staleType)
				}).WithContext(ctx).WithTimeout(4*time.Minute).WithPolling(5*time.Second).Should(o.Succeed(),
					"%s %q should drop stale zone %s and keep Ready-In-Zone conditions for current nodes", pt.kind(), pt.name, staleZone)
			},
			g.Entry("AdminNetworkPolicy", anpStatusCleanupANP),
			g.Entry("BaselineAdminNetworkPolicy", anpStatusCleanupBANP),
		)
	})
})

func ensureANPStatusPolicy(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy) bool {
	g.GinkgoHelper()

	if !pt.isBANP() {
		err := dyn.Resource(pt.gvr).Delete(ctx, pt.name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete leftover %s %q", pt.kind(), pt.name)
		}
		err = wait.PollUntilContextTimeout(ctx, time.Second, 30*time.Second, true, func(ctx context.Context) (bool, error) {
			_, err := dyn.Resource(pt.gvr).Get(ctx, pt.name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, err
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for leftover %s %q to be deleted", pt.kind(), pt.name)
	} else {
		_, err := dyn.Resource(pt.gvr).Get(ctx, pt.name, metav1.GetOptions{})
		if err == nil {
			e2e.Logf("%s %q already exists; reusing it", pt.kind(), pt.name)
			return false
		}
		if !apierrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get %s %q", pt.kind(), pt.name)
		}
	}

	obj, err := newANPStatusPolicyObject(pt)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to build %s object", pt.kind())

	g.By(fmt.Sprintf("Creating %s %q", pt.kind(), pt.name))
	_, err = dyn.Resource(pt.gvr).Create(ctx, obj, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create %s %q", pt.kind(), pt.name)
	return true
}

func newANPStatusPolicyObject(pt anpStatusPolicy) (*unstructured.Unstructured, error) {
	if pt.isBANP() {
		return toUnstructured(newBaselineAdminNetworkPolicy(pt.name))
	}
	return toUnstructured(newAdminNetworkPolicy(pt.name, 50))
}

func anpStatusSubject() anpapi.AdminNetworkPolicySubject {
	return anpapi.AdminNetworkPolicySubject{
		Namespaces: &metav1.LabelSelector{
			MatchLabels: map[string]string{anpStatusCleanupNSLabel: "true"},
		},
	}
}

func anpStatusIngressPeer() anpapi.AdminNetworkPolicyIngressPeer {
	return anpapi.AdminNetworkPolicyIngressPeer{
		Namespaces: &metav1.LabelSelector{
			MatchLabels: map[string]string{anpStatusCleanupNSLabel: "true"},
		},
	}
}

func newAdminNetworkPolicy(name string, priority int32) *anpapi.AdminNetworkPolicy {
	return &anpapi.AdminNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: anpapi.GroupVersion.String(),
			Kind:       "AdminNetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: anpapi.AdminNetworkPolicySpec{
			Priority: priority,
			Subject:  anpStatusSubject(),
			Ingress: []anpapi.AdminNetworkPolicyIngressRule{{
				Name:   "allow-all",
				Action: anpapi.AdminNetworkPolicyRuleActionAllow,
				From:   []anpapi.AdminNetworkPolicyIngressPeer{anpStatusIngressPeer()},
			}},
		},
	}
}

func newBaselineAdminNetworkPolicy(name string) *anpapi.BaselineAdminNetworkPolicy {
	return &anpapi.BaselineAdminNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			APIVersion: anpapi.GroupVersion.String(),
			Kind:       "BaselineAdminNetworkPolicy",
		},
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: anpapi.BaselineAdminNetworkPolicySpec{
			Subject: anpStatusSubject(),
			Ingress: []anpapi.BaselineAdminNetworkPolicyIngressRule{{
				Name:   "allow-all",
				Action: anpapi.BaselineAdminNetworkPolicyRuleActionAllow,
				From:   []anpapi.AdminNetworkPolicyIngressPeer{anpStatusIngressPeer()},
			}},
		},
	}
}

func toUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	return &unstructured.Unstructured{Object: raw}, nil
}

func expectedReadyInZoneTypes(nodes []corev1.Node) sets.Set[string] {
	expected := sets.New[string]()
	zones := getOVNZones(nodes)
	for zone := range zones {
		expected.Insert(anpReadyInZonePrefix + zone)
	}
	return expected
}

// getOVNZones returns node names that have an OVN host subnet.
// Cluster-manager sets k8s.ovn.org/node-subnets on OVN-managed nodes. There is
// no k8s.ovn.org/no-hostsubnet-node label; NoHostSubnet is a config selector
// against ordinary node labels (for example kubernetes.io/os=windows).
func getOVNZones(nodes []corev1.Node) sets.Set[string] {
	zones := sets.New[string]()
	for _, node := range nodes {
		if node.Annotations[ovnNodeSubnetsAnnotation] == "" {
			continue
		}
		zones.Insert(node.Name)
	}
	return zones
}

func waitForANPReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy) {
	g.GinkgoHelper()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		actual, expected, err := currentReadyInZoneTypes(ctx, dyn, cs, pt)
		if err != nil {
			// Retry transient errors (timeout, connection reset, 5xx)
			if apierrors.IsServerTimeout(err) || apierrors.IsServiceUnavailable(err) || apierrors.IsTimeout(err) {
				return false, nil
			}
			// Fail on permanent errors (Forbidden, NotFound on ANP itself, etc.)
			return false, err
		}
		if !actual.Equal(expected) {
			e2e.Logf("Waiting for %s %q Ready-In-Zone conditions: actual %v, expected %v", pt.kind(), pt.name, sets.List(actual), sets.List(expected))
		}
		return actual.Equal(expected), nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(),
		"timed out waiting for %s %q to have Ready-In-Zone conditions for current nodes", pt.kind(), pt.name)
}

func assertReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy, staleType string) error {
	actual, expected, err := currentReadyInZoneTypes(ctx, dyn, cs, pt)
	if err != nil {
		e2e.Logf("Error getting Ready-In-Zone types for %s %q: %v", pt.kind(), pt.name, err)
		return err
	}
	actualList := sets.List(actual)
	expectedList := sets.List(expected)
	e2e.Logf("Ready-In-Zone check for %s %q: actual=%v, expected=%v", pt.kind(), pt.name, actualList, expectedList)

	if staleType != "" && actual.Has(staleType) {
		return fmt.Errorf("stale condition %s is still present; actual: %v", staleType, actualList)
	}
	if !actual.Equal(expected) {
		return fmt.Errorf("Ready-In-Zone types mismatch; actual: %v, expected: %v", actualList, expectedList)
	}
	return nil
}

func currentReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy) (actual, expected sets.Set[string], err error) {
	// Get all nodes - we'll filter to OVN zones in expectedReadyInZoneTypes
	nodes, err := cs.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, nil, err
	}
	obj, err := dyn.Resource(pt.gvr).Get(ctx, pt.name, metav1.GetOptions{})
	if err != nil {
		return nil, nil, err
	}
	types, err := readyInZoneConditionTypes(obj)
	if err != nil {
		return nil, nil, err
	}
	return sets.New(types...), expectedReadyInZoneTypes(nodes.Items), nil
}

func readyInZoneConditionTypes(obj *unstructured.Unstructured) ([]string, error) {
	conditions, err := anpConditionsFromUnstructured(obj)
	if err != nil {
		return nil, err
	}
	var types []string
	for _, cond := range conditions {
		if strings.HasPrefix(cond.Type, anpReadyInZonePrefix) {
			types = append(types, cond.Type)
		}
	}
	return types, nil
}

func anpConditionsFromUnstructured(obj *unstructured.Unstructured) ([]metav1.Condition, error) {
	if obj.GetKind() == "BaselineAdminNetworkPolicy" {
		var banp anpapi.BaselineAdminNetworkPolicy
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &banp); err != nil {
			return nil, err
		}
		return banp.Status.Conditions, nil
	}
	var anp anpapi.AdminNetworkPolicy
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(obj.Object, &anp); err != nil {
		return nil, err
	}
	return anp.Status.Conditions, nil
}

func injectStaleANPZoneStatus(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy, zone, conditionType string) {
	g.GinkgoHelper()

	cond := metav1.Condition{
		Type:               conditionType,
		Status:             metav1.ConditionTrue,
		Reason:             "SetupSucceeded",
		Message:            "injected stale zone status for e2e",
		LastTransitionTime: metav1.Now(),
	}
	raw, err := marshalANPStatusApply(pt, &cond)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to marshal SSA status patch")

	_, err = dyn.Resource(pt.gvr).Patch(ctx, pt.name, types.ApplyPatchType, raw, metav1.PatchOptions{
		FieldManager: zone,
		Force:        ptr.To(true),
	}, "status")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to inject stale %s status for zone %s", pt.kind(), zone)
}

func clearANPZoneStatus(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy, zone string) {
	raw, err := marshalANPStatusApply(pt, nil)
	if err != nil {
		e2e.Logf("failed to marshal SSA clear patch for %s %q zone %s: %v", pt.kind(), pt.name, zone, err)
		return
	}
	_, err = dyn.Resource(pt.gvr).Patch(ctx, pt.name, types.ApplyPatchType, raw, metav1.PatchOptions{
		FieldManager: zone,
		Force:        ptr.To(true),
	}, "status")
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("failed to clear injected %s status for zone %s: %v", pt.kind(), zone, err)
	}
}

func marshalANPStatusApply(pt anpStatusPolicy, cond *metav1.Condition) ([]byte, error) {
	var obj runtime.Object
	if pt.isBANP() {
		banp := &anpapi.BaselineAdminNetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: anpapi.GroupVersion.String(),
				Kind:       "BaselineAdminNetworkPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pt.name},
		}
		if cond != nil {
			banp.Status.Conditions = []metav1.Condition{*cond}
		}
		obj = banp
	} else {
		anp := &anpapi.AdminNetworkPolicy{
			TypeMeta: metav1.TypeMeta{
				APIVersion: anpapi.GroupVersion.String(),
				Kind:       "AdminNetworkPolicy",
			},
			ObjectMeta: metav1.ObjectMeta{Name: pt.name},
		}
		if cond != nil {
			anp.Status.Conditions = []metav1.Condition{*cond}
		}
		obj = anp
	}
	raw, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		return nil, err
	}
	// Status apply must not include spec; the typed objects always serialize a zero spec.
	delete(raw, "spec")
	return json.Marshal(raw)
}

func restartOVNKubeControlPlane(ctx context.Context, cs kubernetes.Interface) {
	g.GinkgoHelper()

	deploy, err := cs.AppsV1().Deployments(ovnNamespace).Get(ctx, ovnKubeControlPlaneDeployment, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ovnkube-control-plane")

	pods, err := e2edeployment.GetPodsForDeployment(ctx, cs, deploy)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ovnkube-control-plane pods")
	o.Expect(pods.Items).NotTo(o.BeEmpty(), "no ovnkube-control-plane pods to restart")

	toDelete := make([]*corev1.Pod, 0, len(pods.Items))
	for i := range pods.Items {
		toDelete = append(toDelete, &pods.Items[i])
	}
	e2e.Logf("Deleting %d ovnkube-control-plane pods", len(toDelete))
	e2epod.DeletePodsWithWait(ctx, cs, toDelete)

	deploy, err = cs.AppsV1().Deployments(ovnNamespace).Get(ctx, ovnKubeControlPlaneDeployment, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ovnkube-control-plane after delete")
	o.Expect(e2edeployment.WaitForDeploymentComplete(cs, deploy)).To(o.Succeed(),
		"timed out waiting for ovnkube-control-plane to become ready after restart")
}
