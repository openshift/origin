package console

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	configclient "github.com/openshift/client-go/config/clientset/versioned"
	routeclient "github.com/openshift/client-go/route/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const (
	consoleNamespace     = "openshift-console"
	additionalRouteLabel = "console.openshift.io/additional-route"
	pollTimeout          = 2 * time.Minute
	pollInterval         = 2 * time.Second
)

var _ = g.Describe("[sig-console][apigroup:config.openshift.io][OCPFeatureGate:IngressComponentRouteLabels][Serial] Console operator route label propagation", func() {
	defer g.GinkgoRecover()

	var (
		configClient configclient.Interface
		routeV1      routeclient.Interface
		domain       string
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
	})

	g.AfterEach(func() {
		cleanupTestRoutes(configClient, routeV1)
	})

	g.It("should propagate labels from componentRoute spec to the route object", func() {
		name := "console-label-test-1"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test", "env": "ci"}

		g.By("adding a componentRoute with labels")
		addComponentRouteWithLabels(configClient, name, hostname, labels)

		g.By("waiting for the route to be created with the expected labels")
		route := waitForRouteWithLabels(routeV1, name, hostname, labels)

		g.By("verifying operator-managed labels are preserved")
		o.Expect(route.Labels[additionalRouteLabel]).To(o.Equal("true"))
		o.Expect(route.Labels["app"]).To(o.Equal("console"))
	})

	g.It("should update route labels when componentRoute labels change", func() {
		name := "console-label-test-2"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test", "env": "ci"}

		g.By("adding a componentRoute with initial labels")
		addComponentRouteWithLabels(configClient, name, hostname, labels)
		waitForRouteWithLabels(routeV1, name, hostname, labels)

		g.By("updating the componentRoute labels")
		updatedLabels := map[string]configv1.LabelValue{"ingress": "shard-updated", "tier": "frontend"}
		updateComponentRouteLabels(configClient, name, updatedLabels)

		g.By("waiting for the route labels to be reconciled")
		ctx, cancel := context.WithTimeout(context.TODO(), pollTimeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
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
		addComponentRouteWithLabels(configClient, name, hostname, labels)
		waitForRouteWithLabels(routeV1, name, hostname, labels)

		g.By("removing the 'env' label from the componentRoute")
		updateComponentRouteLabels(configClient, name, map[string]configv1.LabelValue{"ingress": "shard-test"})

		g.By("waiting for the stale label to be removed from the route")
		ctx, cancel := context.WithTimeout(context.TODO(), pollTimeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
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
		addComponentRouteWithLabels(configClient, name, hostname, labels)

		g.By("waiting for the route with expected labels")
		route := waitForRouteWithLabels(routeV1, name, hostname, labels)

		g.By("verifying operator-managed labels are not overridden")
		o.Expect(route.Labels[additionalRouteLabel]).To(o.Equal("true"))
		o.Expect(route.Labels["app"]).To(o.Equal("console"))
	})

	g.It("should clean up labeled route when componentRoute is removed", func() {
		name := "console-label-test-5"
		hostname := fmt.Sprintf("%s-%s.%s", name, consoleNamespace, domain)
		labels := map[string]configv1.LabelValue{"ingress": "shard-test"}

		g.By("adding a componentRoute with labels")
		addComponentRouteWithLabels(configClient, name, hostname, labels)
		waitForRouteWithLabels(routeV1, name, hostname, labels)

		g.By("removing the componentRoute")
		removeComponentRoute(configClient, name)

		g.By("waiting for the route to be garbage collected")
		ctx, cancel := context.WithTimeout(context.TODO(), pollTimeout)
		defer cancel()
		err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
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

func addComponentRouteWithLabels(client configclient.Interface, name, hostname string, labels map[string]configv1.LabelValue) {
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

func updateComponentRouteLabels(client configclient.Interface, name string, labels map[string]configv1.LabelValue) {
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

func removeComponentRoute(client configclient.Interface, name string) {
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
func cleanupTestRoutes(configClient configclient.Interface, routeClient routeclient.Interface) {
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
func waitForRouteWithLabels(client routeclient.Interface, name, hostname string, expectedLabels map[string]configv1.LabelValue) *routev1.Route {
	var route *routev1.Route
	ctx, cancel := context.WithTimeout(context.TODO(), pollTimeout)
	defer cancel()
	err := wait.PollUntilContextTimeout(ctx, pollInterval, pollTimeout, true, func(ctx context.Context) (bool, error) {
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
	o.Expect(err).NotTo(o.HaveOccurred(), "route %s not created with expected labels within %s", name, pollTimeout)
	return route
}
