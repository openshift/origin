package upgrade

import (
	"fmt"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	"k8s.io/kubernetes/test/e2e/upgrades"

	routev1 "github.com/openshift/api/route/v1"
	operatorclientset "github.com/openshift/client-go/operator/clientset/versioned"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	"github.com/openshift/origin/test/extended/util/url"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// IngressControllerUpgradeTest tests that the default ingress controller is
// available before and after a cluster upgrade. During a master-only upgrade,
// it will test that the ingress controller remains available during as well.
type IngressControllerUpgradeTest struct {
	urlTester *url.Tester
	routeTest *url.Test
}

func (IngressControllerUpgradeTest) Name() string { return "ingress-controller-upgrade" }

// Setup creates a route and makes sure it's reachable through the default
// ingress controller.
func (t *IngressControllerUpgradeTest) Setup(f *framework.Framework) {
	var (
		routeName      = "ingress-controller-test"
		deploymentName = "ingress-controller-test"
		serviceName    = "ingress-controller-test"
	)

	ns := f.Namespace.Name

	g.By("creating an HTTP echo server deployment " + deploymentName + " in namespace " + ns)
	_, err := f.ClientSet.AppsV1().Deployments(ns).Create(&appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: ns,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app":        "http-echo",
					"deployment": deploymentName,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app":        "http-echo",
						"deployment": deploymentName,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "http-echo",
							Image: "openshift/origin-node",
							Command: []string{
								"/bin/socat",
								"TCP4-LISTEN:8080,reuseaddr,fork",
								`EXEC:'/bin/bash -c "printf \\"HTTP/1.0 200 OK\\\\r\\\\n\\\\r\\\\n\\"; sed -e \\"/^\\\\\r/q\\""'`,
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
		},
	})
	o.Expect(err).ToNot(o.HaveOccurred())

	g.By("creating a service " + serviceName + " in namespace " + ns)
	_, err = f.ClientSet.CoreV1().Services(ns).Create(&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      serviceName,
			Namespace: ns,
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Port:     8080,
					Name:     "http",
					Protocol: corev1.ProtocolTCP,
				},
			},
			Selector: map[string]string{
				"app": "http-echo",
			},
		},
	})
	o.Expect(err).ToNot(o.HaveOccurred())

	g.By("creating a route " + routeName + " in namespace " + ns)
	restconfig, err := framework.LoadConfig()
	o.Expect(err).NotTo(o.HaveOccurred())
	routeclient := routeclientset.NewForConfigOrDie(restconfig).RouteV1()
	_, err = routeclient.Routes(ns).Create(&routev1.Route{
		ObjectMeta: metav1.ObjectMeta{
			Name:      routeName,
			Namespace: ns,
		},
		Spec: routev1.RouteSpec{
			Host: "www.example.com",
			To: routev1.RouteTargetReference{
				Kind: "Service",
				Name: serviceName,
			},
		},
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("looking up the ingress controller's endpoint publishing strategy")
	foundLoadBalancerServiceStrategyType := false
	operatorclient := operatorclientset.NewForConfigOrDie(restconfig).OperatorV1()
	err = wait.PollImmediate(2*time.Second, 30*time.Second, func() (bool, error) {
		ic, err := operatorclient.IngressControllers("openshift-ingress-operator").Get("default", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			return false, nil
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if ic.Status.EndpointPublishingStrategy == nil {
			return false, nil
		}
		if ic.Status.EndpointPublishingStrategy.Type == "LoadBalancerService" {
			foundLoadBalancerServiceStrategyType = true
		}
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By("looking up the ingress controller's endpoint")
	routerServiceName := "router-internal-default"
	if foundLoadBalancerServiceStrategyType {
		routerServiceName = "router-default"
	}
	var endpoint string
	err = wait.PollImmediate(2*time.Second, 60*time.Second, func() (bool, error) {
		svc, err := f.ClientSet.CoreV1().Services("openshift-ingress").Get(routerServiceName, metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			return false, nil
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		if svc.Spec.Type == corev1.ServiceTypeLoadBalancer {
			if len(svc.Status.LoadBalancer.Ingress) != 0 {
				if len(svc.Status.LoadBalancer.Ingress[0].IP) != 0 {
					endpoint = svc.Status.LoadBalancer.Ingress[0].IP
					return true, nil
				}
				if len(svc.Status.LoadBalancer.Ingress[0].Hostname) != 0 {
					endpoint = svc.Status.LoadBalancer.Ingress[0].Hostname
					return true, nil
				}
			}
			return false, nil
		}
		endpoint = svc.Spec.ClusterIP
		return true, nil
	})
	o.Expect(err).NotTo(o.HaveOccurred())

	g.By(fmt.Sprintf("hitting the route"))
	t.urlTester = url.NewTester(f.ClientSet, ns)
	t.routeTest = url.Expect("GET", "http://www.example.com").Through(endpoint).HasStatusCode(200)
	t.urlTester.Within(3*time.Minute, t.routeTest)
}

// Test runs a connectivity check.
func (t *IngressControllerUpgradeTest) Test(f *framework.Framework, done <-chan struct{}, upgrade upgrades.UpgradeType) {
	switch upgrade {
	case upgrades.MasterUpgrade, upgrades.NodeUpgrade, upgrades.ClusterUpgrade:
		t.test(f, done, true)
	default:
		t.test(f, done, false)
	}
}

// Teardown cleans up any remaining resources.
func (t *IngressControllerUpgradeTest) Teardown(f *framework.Framework) {
	// Rely on the namespace deletion to clean everything up.
}

func (t *IngressControllerUpgradeTest) test(f *framework.Framework, done <-chan struct{}, testDuringDisruption bool) {
	if testDuringDisruption {
		g.By("continuously hitting the route")
		wait.Until(func() {
			// TODO: Use Within once we have confidence that this
			// test is not flaky.
			// t.urlTester.Within(1*time.Minute, t.routeTest)
			defer g.GinkgoRecover()
			response := t.urlTester.WithErrorPassthrough(true).Response(t.routeTest)
			if response == nil {
				framework.Logf("got nil response")
			} else if len(response.Error) > 0 {
				framework.Logf("got error for route: %v", response.Error)
			}
		}, framework.Poll, done)
	} else {
		g.By("waiting for upgrade to finish without checking if the route remains up")
		<-done
	}

	g.By("hitting the route again")
	wait.Until(func() {
		t.urlTester.Within(1*time.Minute, t.routeTest)
	}, framework.Poll, done)
}
