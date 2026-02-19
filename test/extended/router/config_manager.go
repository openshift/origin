package router

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"

	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

const timeoutSeconds = 3 * 60

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc *exutil.CLI
		ns string
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			if routes, _ := client.List(context.Background(), metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	const ROUTER_BLUEPRINT_ROUTE_POOL_SIZE = 3
	const ROUTER_MAX_DYNAMIC_SERVERS = 2

	// Defines the number of services named `insecure-concurrent-service-NN`, one replica each
	const NUM_CONCURRENT_SERVICES = ROUTER_BLUEPRINT_ROUTE_POOL_SIZE + 1
	// Defines the number of replicas the named service `insecure-concurrent-service-replicas` should have
	const NUM_CONCURRENT_REPLICAS = ROUTER_MAX_DYNAMIC_SERVERS + 1

	oc = exutil.NewCLIWithPodSecurityLevel("router-config-manager", api.LevelPrivileged)

	g.BeforeEach(func() {
		ns = oc.Namespace()

		routerImage, err := exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a RoleBinding")
		roleBinding := &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "system-router",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "default",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     "system:router",
			},
		}

		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), roleBinding, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a ConfigMap")
		configMap := &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name: "serving-cert",
			},
			Data: map[string]string{
				"nginx.conf": `
daemon off;
events { }
http {
  server {
      listen 8443;
      ssl    on;
      ssl_certificate     /etc/serving-cert/tls.crt;
      ssl_certificate_key    /etc/serving-cert/tls.key;
      server_name  "*.svc";
      location / {
          root   /usr/share/nginx/html;
          index  index.html index.htm;
      }
      error_page   500 502 503 504  /50x.html;
      location = /50x.html {
          root   /usr/share/nginx/html;
      }
  }
}
				`,
			},
		}

		_, err = oc.AdminKubeClient().CoreV1().ConfigMaps(ns).Create(context.Background(), configMap, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating Services")
		services := []corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "insecure-service",
					Labels: map[string]string{
						"test": "router",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"test":      "haproxy-cfgmgr",
						"endpoints": "insecure-endpoint",
					},
					Ports: []corev1.ServicePort{
						{
							Port: 8080,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secure-service",
					Annotations: map[string]string{
						"service.alpha.openshift.io/serving-cert-secret-name": "serving-cert",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "secure-endpoint",
					},
					Ports: []corev1.ServicePort{
						{
							Port:       443,
							Name:       "https",
							TargetPort: intstr.FromInt(8443),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "insecure-concurrent-service-replicas",
					Labels: map[string]string{
						"test": "router",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"test":      "haproxy-cfgmgr",
						"endpoints": "insecure-concurrent-endpoint-replicas",
					},
					Ports: []corev1.ServicePort{
						{
							Port: 9376,
						},
					},
				},
			},
		}
		for i := range NUM_CONCURRENT_SERVICES {
			services = append(services, corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("insecure-concurrent-service-%d", i),
					Labels: map[string]string{
						"test": "router",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"test":      "haproxy-cfgmgr",
						"endpoints": fmt.Sprintf("insecure-concurrent-endpoint-%d", i),
					},
					Ports: []corev1.ServicePort{
						{
							Port: 9376,
						},
					},
				},
			})
		}

		for _, service := range services {
			_, err = oc.AdminKubeClient().CoreV1().Services(ns).Create(context.Background(), &service, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("creating Routes")
		routes := []routev1.Route{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "edge-blueprint",
					Labels: map[string]string{
						"test":   "router",
						"select": "hapcm-blueprint",
					},
					Annotations: map[string]string{
						"router.openshift.io/cookie_name": "empire",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationEdge,
					},
					Host: "edge.blueprint.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "insecure-service",
						Kind: "Service",
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "reencrypt-blueprint",
					Labels: map[string]string{
						"test":   "router",
						"select": "hapcm-blueprint",
					},
					Annotations: map[string]string{
						"ren": "stimpy",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationReencrypt,
					},
					Host: "reencrypt.blueprint.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "secure-service",
						Kind: "Service",
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8443),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "passthrough-blueprint",
					Labels: map[string]string{
						"test":   "router",
						"select": "hapcm-blueprint",
					},
					Annotations: map[string]string{
						"test": "ptcruiser",
						"foo":  "bar",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationPassthrough,
					},
					Host: "passthrough.blueprint.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "secure-service",
						Kind: "Service",
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "insecure-route",
					Labels: map[string]string{
						"test":   "haproxy-cfgmgr",
						"select": "haproxy-cfgmgr",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "insecure.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "insecure-service",
						Kind: "Service",
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "edge-allow-http-route",
					Labels: map[string]string{
						"test":   "haproxy-cfgmgr",
						"select": "haproxy-cfgmgr",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination:                   routev1.TLSTerminationEdge,
						InsecureEdgeTerminationPolicy: routev1.InsecureEdgeTerminationPolicyAllow,
					},
					Host: "edge.allow.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "insecure-service",
						Kind: "Service",
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "reencrypt-route",
					Labels: map[string]string{
						"test":   "haproxy-cfgmgr",
						"select": "haproxy-cfgmgr",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationReencrypt,
					},
					Host: "reencrypt.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "secure-service",
						Kind: "Service",
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8443),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "passthrough-route",
					Labels: map[string]string{
						"test":   "haproxy-cfgmgr",
						"select": "haproxy-cfgmgr",
					},
				},
				Spec: routev1.RouteSpec{
					TLS: &routev1.TLSConfig{
						Termination: routev1.TLSTerminationPassthrough,
					},
					Host: "passthrough.hapcm.test",
					To: routev1.RouteTargetReference{
						Name: "secure-service",
						Kind: "Service",
					},
				},
			},
		}

		for _, route := range routes {
			_, err := oc.RouteClient().RouteV1().Routes(ns).Create(context.Background(), &route, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}

		g.By("creating route Pods")
		routerPods := []corev1.Pod{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "router-haproxy-cfgmgr",
					Labels: map[string]string{
						"test": "router-haproxy-cfgmgr",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:            "router",
							Image:           routerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
								{
									Name:  "ROUTER_BLUEPRINT_ROUTE_POOL_SIZE",
									Value: strconv.Itoa(ROUTER_BLUEPRINT_ROUTE_POOL_SIZE),
								},
								{
									Name:  "ROUTER_MAX_DYNAMIC_SERVERS",
									Value: strconv.Itoa(ROUTER_MAX_DYNAMIC_SERVERS),
								},
							},
							Args: []string{
								"--namespace=$(POD_NAMESPACE)",
								"-v=4",
								"--haproxy-config-manager=true",
								"--blueprint-route-labels=select=hapcm-blueprint",
								"--labels=select=haproxy-cfgmgr",
								"--stats-password=password",
								"--stats-port=1936",
								"--stats-user=admin",
							},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
								{
									ContainerPort: 443,
								},
								{
									ContainerPort: 1936,
									Name:          "stats",
									Protocol:      corev1.ProtocolTCP,
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "insecure-endpoint",
					Labels: map[string]string{
						"test":      "haproxy-cfgmgr",
						"endpoints": "insecure-endpoint",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args:  []string{"netexec"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8080,
									Name:          "http",
								},
								{
									ContainerPort: 100,
									Protocol:      corev1.ProtocolUDP,
								},
							},
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "secure-endpoint",
					Labels: map[string]string{
						"app": "secure-endpoint",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:    "serve",
							Image:   image.LocationFor("registry.k8s.io/e2e-test-images/nginx:1.15-4"),
							Command: []string{"/usr/sbin/nginx"},
							Args:    []string{"-c", "/etc/nginx/nginx.conf"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 8443,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "cert",
									MountPath: "/etc/serving-cert",
								},
								{
									Name:      "conf",
									MountPath: "/etc/nginx",
								},
								{
									Name:      "tmp",
									MountPath: "/var/cache/nginx",
								},
								{
									Name:      "tmp2",
									MountPath: "/var/run",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "conf",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "serving-cert",
									},
								},
							},
						},
						{
							Name: "cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{
									SecretName: "serving-cert",
								},
							},
						},
						{
							Name: "tmp",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
						{
							Name: "tmp2",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
				},
			},
		}
		for i := range NUM_CONCURRENT_SERVICES {
			routerPods = append(routerPods, corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("insecure-concurrent-endpoint-%d", i),
					Labels: map[string]string{
						"test":      "haproxy-cfgmgr",
						"endpoints": fmt.Sprintf("insecure-concurrent-endpoint-%d", i),
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args:  []string{"serve-hostname"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9376,
									Name:          "http",
								},
							},
						},
					},
				},
			})
		}
		for i := range NUM_CONCURRENT_REPLICAS {
			routerPods = append(routerPods, corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("insecure-concurrent-endpoint-replicas-%d", i),
					Labels: map[string]string{
						"test": "haproxy-cfgmgr",
						// this is the service selector, but added instead by the test itself.
						// "endpoints": "insecure-concurrent-endpoint-replicas",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args:  []string{"serve-hostname"},
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 9376,
									Name:          "http",
								},
							},
						},
					},
				},
			})
		}

		for _, pod := range routerPods {
			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), &pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when running with the haproxy config manager", func() {
			ns := oc.KubeFramework().Namespace.Name
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			var routerIP string
			err := wait.Poll(time.Second, timeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "router-haproxy-cfgmgr", metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				if len(pod.Status.PodIP) == 0 {
					return false, nil
				}
				routerIP = pod.Status.PodIP
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(routerIP, "1936"))
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, routerIP, timeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for the valid routes to respond")
			err = waitForRouteToRespond(ns, execPod.Name, "http", "insecure.hapcm.test", "/", routerIP, 0)
			o.Expect(err).NotTo(o.HaveOccurred())

			for _, host := range []string{"edge.allow.hapcm.test", "reencrypt.hapcm.test", "passthrough.hapcm.test"} {
				err = waitForRouteToRespond(ns, execPod.Name, "https", host, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("adding routes beyond the number of blueprint backends")
			var pendingRoutes []string
			for i := range NUM_CONCURRENT_SERVICES {
				// our NUM_CONCURRENT_SERVICES is already configured to go beyond the available blueprint backends

				name := fmt.Sprintf("hapcm-insecure-concurrent-service-%d", i)
				serviceName := fmt.Sprintf("insecure-concurrent-service-%d", i)
				hostName := fmt.Sprintf("insecure-concurrent-%d.hapcm.test", i)

				err := createRoute(oc, routeTypeInsecure, name, serviceName, hostName, "/")
				o.Expect(err).NotTo(o.HaveOccurred())
				pendingRoutes = append(pendingRoutes, name)

				err = waitForRouteToRespond(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("adding replicas beyond the number of blueprint slots per backend")
			{
				name := "hapcm-insecure-concurrent-service-replicas"
				serviceName := "insecure-concurrent-service-replicas"
				hostName := "insecure-concurrent-replicas.hapcm.test"
				err := createRoute(oc, routeTypeInsecure, name, serviceName, hostName, "/")
				o.Expect(err).NotTo(o.HaveOccurred())
				pendingRoutes = append(pendingRoutes, name)

				var expectedBackendServersCount int
				endpointReplicaLabelUpdate := []byte(`{"metadata":{"labels":{"endpoints":"insecure-concurrent-endpoint-replicas"}}}`)
				for i := range NUM_CONCURRENT_REPLICAS {
					// our NUM_CONCURRENT_REPLICAS is already configured to go beyond the available blueprint servers per backend

					// adding one backend server at a time - they start to compose the route as soon as
					// its labels match the selector from the service backing the route.
					podName := fmt.Sprintf("insecure-concurrent-endpoint-replicas-%d", i)
					_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Patch(context.Background(), podName, types.StrategicMergePatchType, endpointReplicaLabelUpdate, metav1.PatchOptions{})
					o.Expect(err).NotTo(o.HaveOccurred())

					expectedBackendServersCount++
					allBackendServers := sets.New[string]()
					err = wait.PollUntilContextTimeout(context.Background(), time.Second, timeoutSeconds*time.Second, true, func(ctx context.Context) (bool, error) {
						output, err := readURL(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
						if err != nil {
							// possible 503 due to the first pod still missing
							return false, nil
						}
						allBackendServers.Insert(output)

						// we are done as soon as we found as much backend servers as we have behind the route's configuration
						return allBackendServers.Len() == expectedBackendServersCount, nil
					})
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			}

			g.By("adding overlapping route configurations")
			{
				// Missing
			}

			g.By("removing unused routes")
			for _, name := range pendingRoutes {
				err := oc.AsAdmin().Run("delete").Args("route", name).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("mini stress test by adding (and removing) different routes and checking that they are exposed")
			for i := 0; i < 16; i++ {
				name := fmt.Sprintf("hapcm-stress-insecure-%d", i)
				hostName := fmt.Sprintf("stress.insecure-%d.hapcm.test", i)
				err := createRoute(oc, routeTypeInsecure, name, "insecure-service", hostName, "/")

				err = waitForRouteToRespond(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.AsAdmin().Run("delete").Args("route", name).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				routeTypes := []routeType{routeTypeEdge, routeTypeReencrypt, routeTypePassthrough}
				for _, t := range routeTypes {
					name := fmt.Sprintf("hapcm-stress-%s-%d", t, i)
					hostName := fmt.Sprintf("stress.%s-%d.hapcm.test", t, i)
					serviceName := "secure-service"
					if t == "edge" {
						serviceName = "insecure-service"
					}

					err := createRoute(oc, t, name, serviceName, hostName, "/")
					o.Expect(err).NotTo(o.HaveOccurred())

					err = waitForRouteToRespond(ns, execPod.Name, "https", hostName, "/", routerIP, 0)
					o.Expect(err).NotTo(o.HaveOccurred())

					err = oc.AsAdmin().Run("delete").Args("route", name).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
				}
			}
		})
	})
})

type routeType string

const (
	routeTypeInsecure    = "insecure"
	routeTypeEdge        = "edge"
	routeTypeReencrypt   = "reencrypt"
	routeTypePassthrough = "passthrough"
)

func createRoute(oc *exutil.CLI, routeType routeType, routeName, serviceName, hostName, path string) error {
	var err error
	switch routeType {
	case routeTypeInsecure:
		// --labels on `oc expose` up to 4.21 does not override the ones coming from service's selector,
		// so we're labeling the router after creating it. https://issues.redhat.com/browse/OCPBUGS-74543
		err = oc.AsAdmin().Run("expose").Args("service", serviceName, "--name", routeName, "--hostname", hostName, "--path", path).Execute()
	case routeTypePassthrough:
		err = oc.AsAdmin().Run("create").Args("route", routeTypePassthrough, routeName, "--service", serviceName, "--hostname", hostName).Execute()
	default:
		err = oc.AsAdmin().Run("create").Args("route", string(routeType), routeName, "--service", serviceName, "--hostname", hostName, "--path", path).Execute()
	}
	if err != nil {
		return err
	}
	return oc.AsAdmin().Run("label").Args("route", routeName, "select=haproxy-cfgmgr").Execute()
}

func readURL(ns, execPodName, proto, host, path, ipaddr string, port int) (string, error) {
	host = exutil.IPUrl(host)
	if port == 0 {
		port = 80
		if proto == "https" {
			port = 443
		}
	}
	uri := fmt.Sprintf("%s://%s:%d%s", proto, host, port, path)
	cmd := fmt.Sprintf("curl -ksfL -m 5 --resolve %s:%d:%s %q", host, port, ipaddr, uri)
	return e2eoutput.RunHostCmd(ns, execPodName, cmd)
}

func waitForRouteToRespond(ns, execPodName, proto, host, abspath, ipaddr string, port int) error {
	// bracket IPv6 IPs when used as URI
	host = exutil.IPUrl(host)
	if port == 0 {
		switch proto {
		case "http":
			port = 80
		case "https":
			port = 443
		default:
			port = 80
		}
	}
	uri := fmt.Sprintf("%s://%s:%d%s", proto, host, port, abspath)
	cmd := fmt.Sprintf(`
		set -e
		STOP=$(($(date '+%%s') + %d))
		while [ $(date '+%%s') -lt $STOP ]; do
			rc=0
			code=$( curl -k -s -m 5 -o /dev/null -w '%%{http_code}\n' --resolve %s:%d:%s %q ) || rc=$?
			if [[ "${rc:-0}" -eq 0 ]]; then
				echo $code
				if [[ $code -eq 200 ]]; then
					exit 0
				fi
				if [[ $code -ne 503 ]]; then
					exit 1
				fi
			else
				echo "error ${rc}" 1>&2
			fi
			sleep 1
		done
		`, timeoutSeconds, host, port, ipaddr, uri)
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return fmt.Errorf("host command failed: %v\n%s", err, output)
	}
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if lines[len(lines)-1] != "200" {
		return fmt.Errorf("last response from server was not 200:\n%s", output)
	}
	return nil
}
