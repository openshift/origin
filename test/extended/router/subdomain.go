package router

import (
	"context"
	"fmt"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	admissionapi "k8s.io/pod-security-admission/api"

	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc                   *exutil.CLI
		ns                   string
		clusterIngressDomain string
		routerImage          string
	)

	// This hook must be registered before the framework namespace teardown
	// hook.
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLIWithPodSecurityLevel("router-subdomain", admissionapi.LevelBaseline)

	g.BeforeEach(func() {
		ns = oc.Namespace()

		var err error
		routerImage, err = exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		clusterIngressDomain, err = getDefaultIngressClusterDomainName(oc, time.Minute)
		o.Expect(err).NotTo(o.HaveOccurred())

		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "router",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "default",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "system:router",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("reports the expected host names in admitted routes' statuses", g.Label("Size:M"), func() {
			g.By("deploying two routers with distinct domains")
			routers := map[string]string{
				"router1": "bar.tld",
				"router2": "baz.tld",
			}
			for routerName, routerDomain := range routers {
				one := int32(1)
				container := corev1.Container{
					Name:  routerName,
					Image: routerImage,
					Args: []string{
						"-v=4",
						fmt.Sprintf("--namespace=%s", ns),
						fmt.Sprintf("--name=%s", routerName),
						fmt.Sprintf("--router-domain=%s", routerDomain),
					},
				}
				rs, err := oc.KubeClient().AppsV1().ReplicaSets(ns).Create(
					context.Background(),
					&appsv1.ReplicaSet{
						ObjectMeta: metav1.ObjectMeta{
							Name: routerName,
						},
						Spec: appsv1.ReplicaSetSpec{
							Replicas: &one,
							Selector: &metav1.LabelSelector{
								MatchLabels: map[string]string{"app": routerName},
							},
							Template: corev1.PodTemplateSpec{
								ObjectMeta: metav1.ObjectMeta{
									Labels: map[string]string{"app": routerName},
								},
								Spec: corev1.PodSpec{
									Containers: []corev1.Container{container},
								},
							},
						},
					},
					metav1.CreateOptions{},
				)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = waitForReadyReplicaSet(oc.KubeClient(), ns, rs.Name)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("creating a route for every combination of having or omitting spec.host and having or omitting spec.subdomain")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			routes := map[string]struct {
				// host is the value for spec.host that this
				// test specifies when it creates the route.
				host string
				// subdomain is the value for spec.subdomain
				// that this test specifies when it creates the
				// route.
				subdomain string
				// expectedSpecHost is the value for spec.host
				// that this test expects the route to have
				// after going through API admission.  This
				// value may include the substrings "NS" and
				// "CLUSTER_DOMAIN", which are substituted by
				// the route's namespace and cluster's ingress
				// domain, respectively, before the expected
				// value is compared with the actual value.
				expectedSpecHost string
				// expectedStatusHost is the value for
				// status.ingress[].host that this test expects
				// the route to have after going through router
				// admission.  This value may include the
				// substrings "NS", "CLUSTER_DOMAIN", and
				// "ROUTER_DOMAIN", which are substituted by the
				// route's namespace, the cluster's ingress
				// domain, and the router's domain,
				// respectively, before the expected value is
				// compared with the actual value.
				expectedStatusHost string
			}{
				"test1": {"", "", "test1-NS.CLUSTER_DOMAIN", "test1-NS.CLUSTER_DOMAIN"},
				"test2": {"foo.tld", "", "foo.tld", "foo.tld"},
				"test3": {"", "foo", "", "foo.ROUTER_DOMAIN"},
				"test4": {"foo.tld", "foo", "foo.tld", "foo.tld"},
			}
			for name, route := range routes {
				_, err := client.Create(context.Background(), &routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: name,
					},
					Spec: routev1.RouteSpec{
						Host:      route.host,
						Subdomain: route.subdomain,
						To:        routev1.RouteTargetReference{Name: "test"},
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
					},
				}, metav1.CreateOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting for all routes to have status entries for both routers")
			var routeList *routev1.RouteList
			err := wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				var err error
				routeList, err = client.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				o.Expect(routeList.Items).To(o.HaveLen(4))
				for _, route := range routeList.Items {
					for routerName := range routers {
						ingress := findIngress(&route, routerName)
						if ingress == nil {
							return false, nil
						}
						o.Expect(ingress.Host).NotTo(o.BeEmpty())
					}
				}
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())
			outputIngress(routeList.Items...)

			g.By("verifying that routes have the expected status")
			for _, actual := range routeList.Items {
				route, ok := routes[actual.Name]
				o.Expect(ok).To(o.BeTrue())
				expectedSpecHost := route.expectedSpecHost
				expectedSpecHost = strings.ReplaceAll(expectedSpecHost, "NS", ns)
				expectedSpecHost = strings.ReplaceAll(expectedSpecHost, "CLUSTER_DOMAIN", clusterIngressDomain)
				o.Expect(actual.Spec.Host).To(o.Equal(expectedSpecHost))
				for routerName, routerDomain := range routers {
					ingress := findIngress(&actual, routerName)
					o.Expect(ingress).NotTo(o.BeNil())
					expectedStatusHost := route.expectedStatusHost
					expectedStatusHost = strings.ReplaceAll(expectedStatusHost, "NS", ns)
					expectedStatusHost = strings.ReplaceAll(expectedStatusHost, "CLUSTER_DOMAIN", clusterIngressDomain)
					expectedStatusHost = strings.ReplaceAll(expectedStatusHost, "ROUTER_DOMAIN", routerDomain)
					o.Expect(ingress.Host).To(o.Equal(expectedStatusHost))
				}
			}
		})
	})
})
