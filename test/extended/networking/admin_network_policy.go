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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/utils/ptr"

	exutil "github.com/openshift/origin/test/extended/util"
)

const (
	anpAPIVersion                 = "policy.networking.k8s.io/v1alpha1"
	anpReadyInZonePrefix          = "Ready-In-Zone-"
	ovnKubeControlPlaneDeployment = "ovnkube-control-plane"
	anpStatusCleanupANPName       = "e2e-anp-status-cleanup"
	anpStatusCleanupBANPName      = "default"
	anpStatusCleanupNSLabel       = "e2e.openshift.io/anp-status-cleanup"
)

var (
	anpGVR = schema.GroupVersionResource{
		Group:    "policy.networking.k8s.io",
		Version:  "v1alpha1",
		Resource: "adminnetworkpolicies",
	}
	banpGVR = schema.GroupVersionResource{
		Group:    "policy.networking.k8s.io",
		Version:  "v1alpha1",
		Resource: "baselineadminnetworkpolicies",
	}
)

// anpStatusPolicy describes a cluster-scoped ANP or BANP used by the status-cleanup tests.
type anpStatusPolicy struct {
	kind     string
	gvr      schema.GroupVersionResource
	name     string
	priority *int64
}

var (
	anpStatusCleanupANP = anpStatusPolicy{
		kind:     "AdminNetworkPolicy",
		gvr:      anpGVR,
		name:     anpStatusCleanupANPName,
		priority: ptr.To(int64(50)),
	}
	anpStatusCleanupBANP = anpStatusPolicy{
		kind: "BaselineAdminNetworkPolicy",
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
							e2e.Logf("failed to delete %s %s: %v", pt.kind, pt.name, err)
						}
					}
				})

				g.By(fmt.Sprintf("Waiting for %s %q Ready-In-Zone conditions to match current nodes", pt.kind, pt.name))
				waitForANPReadyInZoneTypes(ctx, dyn, cs, pt)

				g.By("Restarting ovnkube-control-plane")
				restartOVNKubeControlPlane(ctx, cs)

				g.By(fmt.Sprintf("Verifying %s %q still has Ready-In-Zone conditions for every current node", pt.kind, pt.name))
				o.Eventually(func() error {
					return assertReadyInZoneTypes(ctx, dyn, cs, pt, "")
				}).WithContext(ctx).WithTimeout(2*time.Minute).WithPolling(5*time.Second).Should(o.Succeed(),
					"%s %q should keep Ready-In-Zone conditions for current nodes after restart", pt.kind, pt.name)
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
							e2e.Logf("failed to delete %s %s: %v", pt.kind, pt.name, err)
						}
					}
				})

				g.By(fmt.Sprintf("Waiting for %s %q Ready-In-Zone conditions to match current nodes", pt.kind, pt.name))
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
					"expected injected stale condition %s on %s %q", staleType, pt.kind, pt.name)

				g.By("Restarting ovnkube-control-plane so startup cleanup runs")
				restartOVNKubeControlPlane(ctx, cs)

				g.By("Verifying the stale condition is removed and real node conditions remain")
				o.Eventually(func() error {
					return assertReadyInZoneTypes(ctx, dyn, cs, pt, staleType)
				}).WithContext(ctx).WithTimeout(2*time.Minute).WithPolling(5*time.Second).Should(o.Succeed(),
					"%s %q should drop stale zone %s and keep Ready-In-Zone conditions for current nodes", pt.kind, pt.name, staleZone)
			},
			g.Entry("AdminNetworkPolicy", anpStatusCleanupANP),
			g.Entry("BaselineAdminNetworkPolicy", anpStatusCleanupBANP),
		)
	})
})

func ensureANPStatusPolicy(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy) bool {
	g.GinkgoHelper()

	if pt.gvr.Resource == anpGVR.Resource {
		err := dyn.Resource(pt.gvr).Delete(ctx, pt.name, metav1.DeleteOptions{})
		if err != nil && !apierrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete leftover %s %q", pt.kind, pt.name)
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
		o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for leftover %s %q to be deleted", pt.kind, pt.name)
	} else {
		_, err := dyn.Resource(pt.gvr).Get(ctx, pt.name, metav1.GetOptions{})
		if err == nil {
			e2e.Logf("%s %q already exists; reusing it", pt.kind, pt.name)
			return false
		}
		if !apierrors.IsNotFound(err) {
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get %s %q", pt.kind, pt.name)
		}
	}

	obj, err := newANPStatusPolicyObject(pt)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to build %s object", pt.kind)

	g.By(fmt.Sprintf("Creating %s %q", pt.kind, pt.name))
	_, err = dyn.Resource(pt.gvr).Create(ctx, obj, metav1.CreateOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to create %s %q", pt.kind, pt.name)
	return true
}

func newANPStatusPolicyObject(pt anpStatusPolicy) (*unstructured.Unstructured, error) {
	spec := map[string]any{
		"subject": map[string]any{
			"namespaces": map[string]any{
				"matchLabels": map[string]any{
					anpStatusCleanupNSLabel: "true",
				},
			},
		},
		"ingress": []any{
			map[string]any{
				"name":   "allow-all",
				"action": "Allow",
				"from": []any{
					map[string]any{
						"namespaces": map[string]any{
							"matchLabels": map[string]any{
								anpStatusCleanupNSLabel: "true",
							},
						},
					},
				},
			},
		},
	}
	if pt.priority != nil {
		spec["priority"] = *pt.priority
	}

	raw, err := json.Marshal(map[string]any{
		"apiVersion": anpAPIVersion,
		"kind":       pt.kind,
		"metadata": map[string]any{
			"name": pt.name,
		},
		"spec": spec,
	})
	if err != nil {
		return nil, err
	}

	obj := &unstructured.Unstructured{}
	if err := obj.UnmarshalJSON(raw); err != nil {
		return nil, err
	}
	return obj, nil
}

func expectedReadyInZoneTypes(nodes []corev1.Node) sets.Set[string] {
	expected := sets.New[string]()
	for _, node := range nodes {
		expected.Insert(anpReadyInZonePrefix + node.Name)
	}
	return expected
}

func waitForANPReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy) {
	g.GinkgoHelper()

	err := wait.PollUntilContextTimeout(ctx, 5*time.Second, 2*time.Minute, true, func(ctx context.Context) (bool, error) {
		actual, expected, err := currentReadyInZoneTypes(ctx, dyn, cs, pt)
		if err != nil {
			return false, err
		}
		return actual.Equal(expected), nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(),
		"timed out waiting for %s %q to have Ready-In-Zone conditions for current nodes", pt.kind, pt.name)
}

func assertReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy, staleType string) error {
	actual, expected, err := currentReadyInZoneTypes(ctx, dyn, cs, pt)
	if err != nil {
		return err
	}
	if staleType != "" && actual.Has(staleType) {
		return fmt.Errorf("stale condition %s is still present", staleType)
	}
	if !actual.Equal(expected) {
		return fmt.Errorf("Ready-In-Zone types %v, expected %v", sets.List(actual), sets.List(expected))
	}
	return nil
}

func currentReadyInZoneTypes(ctx context.Context, dyn dynamic.Interface, cs kubernetes.Interface, pt anpStatusPolicy) (actual, expected sets.Set[string], err error) {
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
	raw, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, nil
	}
	var types []string
	for _, item := range raw {
		cond, ok := item.(map[string]any)
		if !ok {
			continue
		}
		t, _ := cond["type"].(string)
		if strings.HasPrefix(t, anpReadyInZonePrefix) {
			types = append(types, t)
		}
	}
	return types, nil
}

func injectStaleANPZoneStatus(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy, zone, conditionType string) {
	g.GinkgoHelper()

	patch := map[string]any{
		"apiVersion": anpAPIVersion,
		"kind":       pt.kind,
		"metadata": map[string]any{
			"name": pt.name,
		},
		"status": map[string]any{
			"conditions": []any{
				map[string]any{
					"type":               conditionType,
					"status":             "True",
					"reason":             "SetupSucceeded",
					"message":            "injected stale zone status for e2e",
					"lastTransitionTime": time.Now().UTC().Format(time.RFC3339),
				},
			},
		},
	}
	raw, err := json.Marshal(patch)
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to marshal SSA status patch")

	_, err = dyn.Resource(pt.gvr).Patch(ctx, pt.name, types.ApplyPatchType, raw, metav1.PatchOptions{
		FieldManager: zone,
		Force:        ptr.To(true),
	}, "status")
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to inject stale %s status for zone %s", pt.kind, zone)
}

func clearANPZoneStatus(ctx context.Context, dyn dynamic.Interface, pt anpStatusPolicy, zone string) {
	raw, err := json.Marshal(map[string]any{
		"apiVersion": anpAPIVersion,
		"kind":       pt.kind,
		"metadata": map[string]any{
			"name": pt.name,
		},
	})
	if err != nil {
		e2e.Logf("failed to marshal SSA clear patch for %s %q zone %s: %v", pt.kind, pt.name, zone, err)
		return
	}
	_, err = dyn.Resource(pt.gvr).Patch(ctx, pt.name, types.ApplyPatchType, raw, metav1.PatchOptions{
		FieldManager: zone,
		Force:        ptr.To(true),
	}, "status")
	if err != nil && !apierrors.IsNotFound(err) {
		e2e.Logf("failed to clear injected %s status for zone %s: %v", pt.kind, zone, err)
	}
}

func restartOVNKubeControlPlane(ctx context.Context, cs kubernetes.Interface) {
	g.GinkgoHelper()

	deploy, err := cs.AppsV1().Deployments(ovnNamespace).Get(ctx, ovnKubeControlPlaneDeployment, metav1.GetOptions{})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to get ovnkube-control-plane")
	o.Expect(deploy.Spec.Selector).NotTo(o.BeNil(), "ovnkube-control-plane has no selector")

	expected := int32(1)
	if deploy.Spec.Replicas != nil {
		expected = *deploy.Spec.Replicas
	}
	o.Expect(expected).To(o.BeNumerically(">", 0), "ovnkube-control-plane has 0 replicas")

	selector := metav1.FormatLabelSelector(deploy.Spec.Selector)
	pods, err := cs.CoreV1().Pods(ovnNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to list ovnkube-control-plane pods")

	oldUIDs := sets.New[types.UID]()
	for _, pod := range pods.Items {
		oldUIDs.Insert(pod.UID)
	}
	e2e.Logf("Deleting %d ovnkube-control-plane pods to trigger startup cleanup", len(pods.Items))

	err = cs.CoreV1().Pods(ovnNamespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: selector})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to delete ovnkube-control-plane pods")

	err = wait.PollUntilContextTimeout(ctx, 5*time.Second, 3*time.Minute, true, func(ctx context.Context) (bool, error) {
		current, err := cs.CoreV1().Pods(ovnNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			return false, err
		}
		if int32(len(current.Items)) != expected {
			e2e.Logf("ovnkube-control-plane pod count %d, expected %d", len(current.Items), expected)
			return false, nil
		}
		for _, pod := range current.Items {
			if oldUIDs.Has(pod.UID) {
				e2e.Logf("waiting for old ovnkube-control-plane pod %s to be replaced", pod.Name)
				return false, nil
			}
			if !anpControlPlanePodReady(pod) {
				e2e.Logf("ovnkube-control-plane pod %s is not ready", pod.Name)
				return false, nil
			}
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "timed out waiting for ovnkube-control-plane pods to become ready after restart")
}

func anpControlPlanePodReady(pod corev1.Pod) bool {
	if pod.Status.Phase != corev1.PodRunning {
		return false
	}
	for _, cond := range pod.Status.Conditions {
		if cond.Type == corev1.PodReady && cond.Status == corev1.ConditionTrue {
			return true
		}
	}
	return false
}
