package console

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	consoleNamespace     = "openshift-console"
	additionalRouteLabel = "console.openshift.io/additional-route"
	pollInterval         = 2 * time.Second
	pollTimeout          = 2 * time.Minute
	hcpPollInterval      = 10 * time.Second
	// On HCP the reconciliation chain has multiple hops: HostedCluster CR →
	// HostedControlPlane → HCCO → guest Ingress → console-operator → Route.
	// 5m/10s matches HyperShift's own ingress e2e
	// (test/e2e/v2/tests/hosted_cluster_ingress_test.go) for HostedCluster spec
	// changes propagating to the guest. All stages of one wait share this deadline.
	hcpPollTimeout = 5 * time.Minute

	hostedClusterConfigsNamespace = "clusters"
)

var hostedClusterGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "hostedclusters",
}

var hostedControlPlaneGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "hostedcontrolplanes",
}

// hcpContext holds the management cluster state needed to modify
// ingress config on HyperShift clusters through the HostedCluster CR.
type hcpContext struct {
	dynamicClient     dynamic.Interface
	hostedClusterName string
	// hcpNamespace is the management cluster namespace holding the
	// HostedControlPlane CR (HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE).
	hcpNamespace string
}

// componentRoutesPredicate reports whether an observed componentRoutes list
// reflects the intended mutation. It is evaluated against the HostedControlPlane
// CR, the guest Ingress config, and (indirectly) drives the route wait.
type componentRoutesPredicate func([]configv1.ComponentRouteSpec) bool

var _ = g.Describe("[sig-console][apigroup:config.openshift.io][OCPFeatureGate:IngressComponentRouteLabels][Serial] Console operator route label propagation", func() {
	defer g.GinkgoRecover()

	var (
		configClient configclient.Interface
		routeV1      routeclient.Interface
		domain       string
		hcp          *hcpContext
		interval     time.Duration
		timeout      time.Duration
	)

	g.BeforeEach(func() {
		kubeconfig, err := e2e.LoadConfig()
		o.Expect(err).NotTo(o.HaveOccurred())

		configClient, err = configclient.NewForConfig(kubeconfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		routeV1, err = routeclient.NewForConfig(kubeconfig)
		o.Expect(err).NotTo(o.HaveOccurred())

		ingress, err := configClient.ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		domain = ingress.Spec.Domain

		isHCP, err := exutil.IsHypershift(context.TODO(), configClient)
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to determine whether the cluster is HyperShift")
		interval = pollInterval
		timeout = pollTimeout
		if isHCP {
			hcp, err = setupHCPContext()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to set up HCP management cluster context")
			interval = hcpPollInterval
			timeout = hcpPollTimeout
		}
	})

	g.AfterEach(func() {
		cleanupTestRoutes(configClient, routeV1, hcp)
	})

	g.It("should propagate labels from componentRoute spec to the route object", func() {
		name := "console-label-test-1"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test", "env": "ci"}

		g.By("adding a componentRoute with labels")
		addComponentRouteWithLabels(configClient, hcp, name, hostname, labels)

		g.By("waiting for the route to be created with the expected labels")
		route := waitForRouteWithLabels(configClient, routeV1, hcp, name, hostname, labels, interval, timeout)

		g.By("verifying operator-managed labels are preserved")
		o.Expect(route.Labels[additionalRouteLabel]).To(o.Equal("true"))
		o.Expect(route.Labels["app"]).To(o.Equal("console"))
	})

	g.It("should update route labels when componentRoute labels change", func() {
		name := "console-label-test-2"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test", "env": "ci"}

		g.By("adding a componentRoute with initial labels")
		addComponentRouteWithLabels(configClient, hcp, name, hostname, labels)
		waitForRouteWithLabels(configClient, routeV1, hcp, name, hostname, labels, interval, timeout)

		g.By("updating the componentRoute labels")
		updatedLabels := map[string]configv1.LabelValue{"ingress": "shard-updated", "tier": "frontend"}
		updateComponentRouteLabels(configClient, hcp, name, updatedLabels)

		g.By("waiting for the route labels to be reconciled")
		err := waitForPropagation(configClient, hcp, interval, timeout, componentRouteHasLabels(name, updatedLabels), func(ctx context.Context) (bool, error) {
			route, err := routeV1.RouteV1().Routes(consoleNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			return route.Labels["ingress"] == "shard-updated" && route.Labels["tier"] == "frontend", nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "route labels should be updated")
	})

	g.It("should remove stale labels from route when removed from componentRoute spec", func() {
		name := "console-label-test-3"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test", "env": "ci"}

		g.By("adding a componentRoute with initial labels")
		addComponentRouteWithLabels(configClient, hcp, name, hostname, labels)
		waitForRouteWithLabels(configClient, routeV1, hcp, name, hostname, labels, interval, timeout)

		g.By("removing the 'env' label from the componentRoute")
		reducedLabels := map[string]configv1.LabelValue{"ingress": "shard-test"}
		updateComponentRouteLabels(configClient, hcp, name, reducedLabels)

		g.By("waiting for the stale label to be removed from the route")
		err := waitForPropagation(configClient, hcp, interval, timeout, componentRouteHasLabels(name, reducedLabels), func(ctx context.Context) (bool, error) {
			route, err := routeV1.RouteV1().Routes(consoleNamespace).Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return false, nil
			}
			_, hasEnv := route.Labels["env"]
			return !hasEnv && route.Labels["ingress"] == "shard-test", nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "stale label 'env' should be removed")
	})

	g.It("should preserve operator-managed labels when user labels are applied", func() {
		name := "console-label-test-4"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"custom-key": "custom-value"}

		g.By("adding a componentRoute with a custom label")
		addComponentRouteWithLabels(configClient, hcp, name, hostname, labels)

		g.By("waiting for the route with expected labels")
		route := waitForRouteWithLabels(configClient, routeV1, hcp, name, hostname, labels, interval, timeout)

		g.By("verifying operator-managed labels are not overridden")
		o.Expect(route.Labels[additionalRouteLabel]).To(o.Equal("true"))
		o.Expect(route.Labels["app"]).To(o.Equal("console"))
	})

	g.It("should clean up labeled route when componentRoute is removed", func() {
		name := "console-label-test-5"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test"}

		g.By("adding a componentRoute with labels")
		addComponentRouteWithLabels(configClient, hcp, name, hostname, labels)
		waitForRouteWithLabels(configClient, routeV1, hcp, name, hostname, labels, interval, timeout)

		g.By("removing the componentRoute")
		removeComponentRoute(configClient, hcp, name)

		g.By("waiting for the route to be garbage collected")
		err := waitForPropagation(configClient, hcp, interval, timeout, componentRouteAbsent(name), func(ctx context.Context) (bool, error) {
			_, err := routeV1.RouteV1().Routes(consoleNamespace).Get(ctx, name, metav1.GetOptions{})
			if apierrors.IsNotFound(err) {
				return true, nil
			}
			if err != nil {
				return false, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "route should be garbage collected")
	})
})

// setupHCPContext initializes the management cluster client for modifying
// ingress config through the HostedCluster CR on HyperShift clusters.
func setupHCPContext() (*hcpContext, error) {
	if os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG") == "" || os.Getenv("HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE") == "" {
		return nil, fmt.Errorf("HYPERSHIFT_MANAGEMENT_CLUSTER_KUBECONFIG and HYPERSHIFT_MANAGEMENT_CLUSTER_NAMESPACE must be set")
	}

	mgmtOC := exutil.NewHypershiftManagementCLI("console-label-test")
	_, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get HyperShift management cluster config: %w", err)
	}

	hostedClusterName := strings.TrimPrefix(hcpNamespace, hostedClusterConfigsNamespace+"-")
	e2e.Logf("HyperShift: HC=%s/%s, HCP NS=%s", hostedClusterConfigsNamespace, hostedClusterName, hcpNamespace)

	return &hcpContext{
		dynamicClient:     mgmtOC.AdminDynamicClient(),
		hostedClusterName: hostedClusterName,
		hcpNamespace:      hcpNamespace,
	}, nil
}

// addComponentRouteWithLabels appends a componentRoute entry with the given
// labels to the ingress config. On HCP it modifies the HostedCluster CR instead.
func addComponentRouteWithLabels(client configclient.Interface, hcp *hcpContext, name, hostname string, labels map[string]configv1.LabelValue) {
	if hcp != nil {
		persisted, err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
			entry := map[string]interface{}{
				"namespace": consoleNamespace,
				"name":      name,
				"hostname":  hostname,
			}
			if len(labels) > 0 {
				labelsMap := make(map[string]interface{}, len(labels))
				for k, v := range labels {
					labelsMap[k] = string(v)
				}
				entry["labels"] = labelsMap
			}
			return append(routes, entry)
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to add componentRoute %s via HostedCluster", name)
		expectHostedClusterPersistedLabels(hcp, persisted, name, labels)
		return
	}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingress, err := client.ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		ingress.Spec.ComponentRoutes = append(ingress.Spec.ComponentRoutes, configv1.ComponentRouteSpec{
			Namespace: consoleNamespace,
			Name:      name,
			Hostname:  configv1.Hostname(hostname),
			Labels:    labels,
		})
		_, err = client.ConfigV1().Ingresses().Update(context.TODO(), ingress, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to add componentRoute %s", name)
}

// updateComponentRouteLabels replaces the labels on an existing componentRoute.
// On HCP it modifies the HostedCluster CR instead.
func updateComponentRouteLabels(client configclient.Interface, hcp *hcpContext, name string, labels map[string]configv1.LabelValue) {
	if hcp != nil {
		persisted, err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
			for i, r := range routes {
				routeMap, ok := r.(map[string]interface{})
				if !ok {
					continue
				}
				if routeMap["name"] == name {
					labelsMap := make(map[string]interface{}, len(labels))
					for k, v := range labels {
						labelsMap[k] = string(v)
					}
					routeMap["labels"] = labelsMap
					routes[i] = routeMap
					break
				}
			}
			return routes
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to update labels on componentRoute %s via HostedCluster", name)
		expectHostedClusterPersistedLabels(hcp, persisted, name, labels)
		return
	}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingress, err := client.ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		for i, cr := range ingress.Spec.ComponentRoutes {
			if cr.Name == name {
				ingress.Spec.ComponentRoutes[i].Labels = labels
				break
			}
		}
		_, err = client.ConfigV1().Ingresses().Update(context.TODO(), ingress, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to update labels on componentRoute %s", name)
}

// removeComponentRoute removes a componentRoute entry by name.
// On HCP it modifies the HostedCluster CR instead.
func removeComponentRoute(client configclient.Interface, hcp *hcpContext, name string) {
	if hcp != nil {
		persisted, err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
			var filtered []interface{}
			for _, r := range routes {
				routeMap, ok := r.(map[string]interface{})
				if !ok {
					filtered = append(filtered, r)
					continue
				}
				if routeMap["name"] != name {
					filtered = append(filtered, r)
				}
			}
			return filtered
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to remove componentRoute %s via HostedCluster", name)
		o.Expect(componentRouteAbsent(name)(componentRoutesFromUnstructured(persisted))).To(o.BeTrue(),
			"HostedCluster %s/%s still contains componentRoutes[%s] after removal; persisted componentRoutes: %s",
			hostedClusterConfigsNamespace, hcp.hostedClusterName, name, formatComponentRoutes(componentRoutesFromUnstructured(persisted)))
		return
	}

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		ingress, err := client.ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		var filtered []configv1.ComponentRouteSpec
		for _, cr := range ingress.Spec.ComponentRoutes {
			if cr.Name != name {
				filtered = append(filtered, cr)
			}
		}
		ingress.Spec.ComponentRoutes = filtered
		_, err = client.ConfigV1().Ingresses().Update(context.TODO(), ingress, metav1.UpdateOptions{})
		return err
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "failed to remove componentRoute %s", name)
}

// cleanupTestRoutes removes test componentRoutes from the Ingress config and
// directly deletes any orphaned route objects as a fallback in case the
// operator hasn't garbage-collected them yet.
func cleanupTestRoutes(configClient configclient.Interface, routeClient routeclient.Interface, hcp *hcpContext) {
	if hcp != nil {
		_, err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
			var filtered []interface{}
			for _, r := range routes {
				routeMap, ok := r.(map[string]interface{})
				if !ok {
					filtered = append(filtered, r)
					continue
				}
				n, _ := routeMap["name"].(string)
				if !strings.HasPrefix(n, "console-label-test-") {
					filtered = append(filtered, r)
				}
			}
			return filtered
		})
		if err != nil {
			e2e.Logf("warning: failed to clean up test componentRoutes via HostedCluster: %v", err)
		}
	} else {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			ingress, err := configClient.ConfigV1().Ingresses().Get(context.TODO(), "cluster", metav1.GetOptions{})
			if err != nil {
				return err
			}
			var filtered []configv1.ComponentRouteSpec
			for _, cr := range ingress.Spec.ComponentRoutes {
				if !strings.HasPrefix(cr.Name, "console-label-test-") {
					filtered = append(filtered, cr)
				}
			}
			if len(filtered) == len(ingress.Spec.ComponentRoutes) {
				return nil
			}
			ingress.Spec.ComponentRoutes = filtered
			_, err = configClient.ConfigV1().Ingresses().Update(context.TODO(), ingress, metav1.UpdateOptions{})
			return err
		})
		if err != nil {
			e2e.Logf("warning: failed to clean up test componentRoutes: %v", err)
		}
	}

	routes, err := routeClient.RouteV1().Routes(consoleNamespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		e2e.Logf("warning: failed to list routes for cleanup: %v", err)
		return
	}
	for _, r := range routes.Items {
		if strings.HasPrefix(r.Name, "console-label-test-") {
			if err := routeClient.RouteV1().Routes(consoleNamespace).Delete(context.TODO(), r.Name, metav1.DeleteOptions{}); err != nil && !apierrors.IsNotFound(err) {
				e2e.Logf("warning: failed to delete orphaned test route %s: %v", r.Name, err)
			}
		}
	}
}

// waitForRouteWithLabels polls until a route exists with the expected hostname,
// the operator-managed additional-route label, and all expected user labels.
// On HCP it first waits for the componentRoute to reach the HostedControlPlane
// and the guest Ingress config (see waitForPropagation).
func waitForRouteWithLabels(configClient configclient.Interface, client routeclient.Interface, hcp *hcpContext, name, hostname string, expectedLabels map[string]configv1.LabelValue, interval, timeout time.Duration) *routev1.Route {
	var route *routev1.Route
	err := waitForPropagation(configClient, hcp, interval, timeout, componentRouteHasLabels(name, expectedLabels), func(ctx context.Context) (bool, error) {
		var err error
		route, err = client.RouteV1().Routes(consoleNamespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return false, nil
		}
		if route.Spec.Host != hostname {
			return false, nil
		}
		if route.Labels[additionalRouteLabel] != "true" {
			return false, nil
		}
		for k, v := range expectedLabels {
			if route.Labels[k] != string(v) {
				return false, nil
			}
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred(), "route %s not created with expected labels within %s", name, timeout)
	return route
}

// hcpModifyComponentRoutes performs a read-modify-write on the HostedCluster CR's
// spec.configuration.ingress.componentRoutes using the management cluster client.
// It returns the componentRoutes as persisted by the API server (taken from the
// object returned by Update), so callers can detect fields pruned by the CRD
// schema without an extra Get.
func hcpModifyComponentRoutes(hcp *hcpContext, modify func([]interface{}) []interface{}) ([]interface{}, error) {
	var persisted []interface{}
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		hc, err := hcp.dynamicClient.Resource(hostedClusterGVR).Namespace(hostedClusterConfigsNamespace).Get(context.TODO(), hcp.hostedClusterName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get HostedCluster %s/%s: %w", hostedClusterConfigsNamespace, hcp.hostedClusterName, err)
		}

		routes, _, err := unstructured.NestedSlice(hc.Object, "spec", "configuration", "ingress", "componentRoutes")
		if err != nil {
			return fmt.Errorf("failed to extract componentRoutes from HostedCluster: %w", err)
		}
		routes = modify(routes)

		if err := unstructured.SetNestedSlice(hc.Object, routes, "spec", "configuration", "ingress", "componentRoutes"); err != nil {
			return fmt.Errorf("failed to set componentRoutes on HostedCluster: %w", err)
		}

		updated, err := hcp.dynamicClient.Resource(hostedClusterGVR).Namespace(hostedClusterConfigsNamespace).Update(context.TODO(), hc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update HostedCluster %s/%s: %w", hostedClusterConfigsNamespace, hcp.hostedClusterName, err)
		}
		persisted, _, err = unstructured.NestedSlice(updated.Object, "spec", "configuration", "ingress", "componentRoutes")
		if err != nil {
			return fmt.Errorf("failed to extract persisted componentRoutes from HostedCluster: %w", err)
		}
		return nil
	})
	return persisted, err
}

// expectHostedClusterPersistedLabels fails the test immediately if the
// HostedCluster returned by Update does not contain componentRoutes[name] with
// exactly the requested labels. On a management cluster whose HostedCluster CRD
// does not declare componentRoutes[].labels the API server silently prunes them,
// and waiting for propagation would only burn the full poll timeout.
func expectHostedClusterPersistedLabels(hcp *hcpContext, persisted []interface{}, name string, labels map[string]configv1.LabelValue) {
	routes := componentRoutesFromUnstructured(persisted)
	o.Expect(componentRouteAbsent(name)(routes)).To(o.BeFalse(),
		"HostedCluster %s/%s has no componentRoutes[%s] entry after update; persisted componentRoutes: %s",
		hostedClusterConfigsNamespace, hcp.hostedClusterName, name, formatComponentRoutes(routes))
	o.Expect(componentRouteHasLabels(name, labels)(routes)).To(o.BeTrue(),
		"HostedCluster %s/%s did not persist componentRoutes[%s].labels; the management cluster's HostedCluster CRD does not declare this field (requires openshift/hypershift#9815 on a TechPreviewNoUpgrade management cluster). Not waiting for route propagation. Requested labels: %v; persisted componentRoutes: %s",
		hostedClusterConfigsNamespace, hcp.hostedClusterName, name, labels, formatComponentRoutes(routes))
}

// componentRouteHasLabels matches when the componentRoute named name exists and
// its labels equal the given labels exactly.
func componentRouteHasLabels(name string, labels map[string]configv1.LabelValue) componentRoutesPredicate {
	return func(routes []configv1.ComponentRouteSpec) bool {
		for _, r := range routes {
			if r.Name != name {
				continue
			}
			if len(r.Labels) != len(labels) {
				return false
			}
			for k, v := range labels {
				if got, ok := r.Labels[k]; !ok || got != v {
					return false
				}
			}
			return true
		}
		return false
	}
}

// componentRouteAbsent matches when no componentRoute named name exists.
func componentRouteAbsent(name string) componentRoutesPredicate {
	return func(routes []configv1.ComponentRouteSpec) bool {
		for _, r := range routes {
			if r.Name == name {
				return false
			}
		}
		return true
	}
}

// componentRoutesFromUnstructured converts an unstructured componentRoutes list
// (HostedCluster / HostedControlPlane spec.configuration.ingress.componentRoutes)
// into typed entries. Labels stay nil when the "labels" key is absent.
func componentRoutesFromUnstructured(raw []interface{}) []configv1.ComponentRouteSpec {
	routes := make([]configv1.ComponentRouteSpec, 0, len(raw))
	for _, r := range raw {
		m, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		var cr configv1.ComponentRouteSpec
		if v, ok := m["namespace"].(string); ok {
			cr.Namespace = v
		}
		if v, ok := m["name"].(string); ok {
			cr.Name = v
		}
		if v, ok := m["hostname"].(string); ok {
			cr.Hostname = configv1.Hostname(v)
		}
		if l, ok := m["labels"].(map[string]interface{}); ok {
			cr.Labels = make(map[string]configv1.LabelValue, len(l))
			for k, v := range l {
				s, _ := v.(string)
				cr.Labels[k] = configv1.LabelValue(s)
			}
		}
		routes = append(routes, cr)
	}
	return routes
}

// formatComponentRoutes renders componentRoutes for failure messages.
func formatComponentRoutes(routes []configv1.ComponentRouteSpec) string {
	b, err := json.Marshal(routes)
	if err != nil {
		return fmt.Sprintf("%+v", routes)
	}
	return string(b)
}

// waitForPropagation waits for a componentRoute mutation to take effect. All
// stages share one context bounded by timeout, so the worst case per call is
// timeout, not timeout per stage.
//
// On HCP:
//
//	stage 1: HostedControlPlane spec.configuration.ingress.componentRoutes matches pred
//	stage 2: guest ingresses.config.openshift.io/cluster spec.componentRoutes matches pred
//	stage 3: routeCheck succeeds
//
// Stages 1 and 2 fail the test directly, naming the stalled stage and the last
// observed componentRoutes. On standalone OCP only stage 3 runs. The stage 3
// error is returned (prefixed with the stage on HCP) so callers keep their
// existing failure messages.
func waitForPropagation(configClient configclient.Interface, hcp *hcpContext, interval, timeout time.Duration, pred componentRoutesPredicate, routeCheck wait.ConditionWithContextFunc) error {
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()

	if hcp != nil {
		var lastHCP []configv1.ComponentRouteSpec
		var lastHCPErr error
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
			obj, err := hcp.dynamicClient.Resource(hostedControlPlaneGVR).Namespace(hcp.hcpNamespace).Get(ctx, hcp.hostedClusterName, metav1.GetOptions{})
			if err != nil {
				lastHCPErr = err
				return false, nil
			}
			raw, _, err := unstructured.NestedSlice(obj.Object, "spec", "configuration", "ingress", "componentRoutes")
			if err != nil {
				lastHCPErr = err
				return false, nil
			}
			lastHCPErr = nil
			lastHCP = componentRoutesFromUnstructured(raw)
			return pred(lastHCP), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"stage 1/3 stalled: HostedControlPlane %s/%s spec.configuration.ingress.componentRoutes did not reflect the change within %s; last observed componentRoutes: %s; last error: %v",
			hcp.hcpNamespace, hcp.hostedClusterName, timeout, formatComponentRoutes(lastHCP), lastHCPErr)

		var lastGuest []configv1.ComponentRouteSpec
		var lastGuestErr error
		err = wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
			ingress, err := configClient.ConfigV1().Ingresses().Get(ctx, "cluster", metav1.GetOptions{})
			if err != nil {
				lastGuestErr = err
				return false, nil
			}
			lastGuestErr = nil
			lastGuest = ingress.Spec.ComponentRoutes
			return pred(lastGuest), nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(),
			"stage 2/3 stalled: guest ingresses.config.openshift.io/cluster spec.componentRoutes did not reflect the change within %s (HostedControlPlane already did); last observed componentRoutes: %s; last error: %v",
			timeout, formatComponentRoutes(lastGuest), lastGuestErr)
	}

	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, routeCheck)
	if err != nil && hcp != nil {
		return fmt.Errorf("stage 3/3 stalled: guest Ingress config is correct but the route in %s did not converge within %s: %w", consoleNamespace, timeout, err)
	}
	return err
}
