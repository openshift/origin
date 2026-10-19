package console

import (
	"context"
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
	// On HCP the reconciliation chain is HostedCluster CR → HostedControlPlane →
	// HCCO → guest Ingress → console-operator → Route, which takes longer.
	hcpPollTimeout = 25 * time.Minute

	hostedClusterConfigsNamespace = "clusters"
)

var hostedClusterGVR = schema.GroupVersionResource{
	Group:    "hypershift.openshift.io",
	Version:  "v1beta1",
	Resource: "hostedclusters",
}

// hcpContext holds the management cluster state needed to modify
// ingress config on HyperShift clusters through the HostedCluster CR.
type hcpContext struct {
	dynamicClient     dynamic.Interface
	hostedClusterName string
}

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
		o.Expect(err).NotTo(o.HaveOccurred())
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
		route := waitForRouteWithLabels(routeV1, name, hostname, labels, interval, timeout)

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
		waitForRouteWithLabels(routeV1, name, hostname, labels, interval, timeout)

		g.By("updating the componentRoute labels")
		updatedLabels := map[string]configv1.LabelValue{"ingress": "shard-updated", "tier": "frontend"}
		updateComponentRouteLabels(configClient, hcp, name, updatedLabels)

		g.By("waiting for the route labels to be reconciled")
		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
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
		waitForRouteWithLabels(routeV1, name, hostname, labels, interval, timeout)

		g.By("removing the 'env' label from the componentRoute")
		updateComponentRouteLabels(configClient, hcp, name, map[string]configv1.LabelValue{"ingress": "shard-test"})

		g.By("waiting for the stale label to be removed from the route")
		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
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
		route := waitForRouteWithLabels(routeV1, name, hostname, labels, interval, timeout)

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
		waitForRouteWithLabels(routeV1, name, hostname, labels, interval, timeout)

		g.By("removing the componentRoute")
		removeComponentRoute(configClient, hcp, name)

		g.By("waiting for the route to be garbage collected")
		ctx, cancel := context.WithTimeout(context.TODO(), timeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
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
	}, nil
}

// addComponentRouteWithLabels appends a componentRoute entry with the given
// labels to the ingress config. On HCP it modifies the HostedCluster CR instead.
func addComponentRouteWithLabels(client configclient.Interface, hcp *hcpContext, name, hostname string, labels map[string]configv1.LabelValue) {
	if hcp != nil {
		err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
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
		err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
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
		err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
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
		err := hcpModifyComponentRoutes(hcp, func(routes []interface{}) []interface{} {
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
func waitForRouteWithLabels(client routeclient.Interface, name, hostname string, expectedLabels map[string]configv1.LabelValue, interval, timeout time.Duration) *routev1.Route {
	var route *routev1.Route
	ctx, cancel := context.WithTimeout(context.TODO(), timeout)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, interval, timeout, true, func(ctx context.Context) (bool, error) {
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
func hcpModifyComponentRoutes(hcp *hcpContext, modify func([]interface{}) []interface{}) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
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

		_, err = hcp.dynamicClient.Resource(hostedClusterGVR).Namespace(hostedClusterConfigsNamespace).Update(context.TODO(), hc, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update HostedCluster %s/%s: %w", hostedClusterConfigsNamespace, hcp.hostedClusterName, err)
		}
		return nil
	})
}
