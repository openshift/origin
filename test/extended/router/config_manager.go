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

	const ROUTER_BLUEPRINT_ROUTE_POOL_SIZE = 3
	const ROUTER_MAX_DYNAMIC_SERVERS = 2

	// Defines the number of services named `insecure-concurrent-service-NN`, one replica each
	const NUM_CONCURRENT_SERVICES = ROUTER_BLUEPRINT_ROUTE_POOL_SIZE + 1
	// Defines the number of replicas the named service `insecure-concurrent-service-replicas` should have
	const NUM_CONCURRENT_REPLICAS = ROUTER_MAX_DYNAMIC_SERVERS + 1

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

	// Router namespace has the privileged SCC, so we are using the same privilege here.
	// This is needed because haproxy binary has the cap_net_bind_service capability
	// in order to allow binding 80/443 without being root. Without the privilege,
	// the capability does not take effect and haproxy fails to start.
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
      listen 8443 ssl;
      listen [::]:8443 ssl;
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
									Name:  "ROUTER_IP_V4_V6_MODE",
									Value: "v4v6",
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
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.59"),
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
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.59"),
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
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.59"),
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

		// This test includes routes one by one, checking them to respond on every new iteration.
		// It ensures that all of them respond correctly, before and after a reload is needed to add more blueprint backends.
		// The test finishes by iterating over all the created routes again.
		g.It("should add routes beyond the number of blueprint backends", func() {
			execPod, doneExecPod := createExecPod(oc)
			defer doneExecPod()

			routerIP := waitForRouter(oc, execPod)

			g.By("adding new routes")
			for i := range NUM_CONCURRENT_SERVICES {
				// our NUM_CONCURRENT_SERVICES is already configured to go beyond the available blueprint backends

				name := fmt.Sprintf("hapcm-insecure-concurrent-service-%d", i)
				serviceName := fmt.Sprintf("insecure-concurrent-service-%d", i)
				endpointName := fmt.Sprintf("insecure-concurrent-endpoint-%d", i)
				hostName := fmt.Sprintf("insecure-concurrent-%d.hapcm.test", i)

				err := createRoute(oc, routeTypeInsecure, name, serviceName, hostName, "/")
				o.Expect(err).NotTo(o.HaveOccurred())

				err = waitForRouteToRespond(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
				output, err := readURL(ns, execPod.Name, hostName, "/", routerIP)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.Equal(endpointName))
			}

			// check if the created routes continue to respond correctly
			g.By("checking created routes")
			for i := range NUM_CONCURRENT_SERVICES {
				endpointName := fmt.Sprintf("insecure-concurrent-endpoint-%d", i)
				hostName := fmt.Sprintf("insecure-concurrent-%d.hapcm.test", i)

				err := waitForRouteToRespond(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())
				output, err := readURL(ns, execPod.Name, hostName, "/", routerIP)
				o.Expect(err).NotTo(o.HaveOccurred())
				o.Expect(output).To(o.Equal(endpointName))
			}
		})

		// This test includes new replicas into a route, one by one, checking them to respond on every new iteration.
		// It ensures that the replicas respond correctly, before and after a reload is needed to add more empty slots.
		g.It("should add replicas beyond the number of empty slots per backend", func() {
			execPod, doneExecPod := createExecPod(oc)
			defer doneExecPod()

			routerIP := waitForRouter(oc, execPod)

			name := "hapcm-insecure-concurrent-service-replicas"
			serviceName := "insecure-concurrent-service-replicas"
			hostName := "insecure-concurrent-replicas.hapcm.test"
			err := createRoute(oc, routeTypeInsecure, name, serviceName, hostName, "/")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("adding new replicas")
			expectedBackendServers := sets.New[string]()
			endpointReplicaLabelUpdate := []byte(`{"metadata":{"labels":{"endpoints":"insecure-concurrent-endpoint-replicas"}}}`)
			for i := range NUM_CONCURRENT_REPLICAS {
				// our NUM_CONCURRENT_REPLICAS is already configured to go beyond the available blueprint servers per backend

				// adding one backend server at a time - they start to compose the route as soon as
				// its labels match the selector from the service backing the route.
				podName := fmt.Sprintf("insecure-concurrent-endpoint-replicas-%d", i)
				_, err := oc.AdminKubeClient().CoreV1().Pods(ns).Patch(context.Background(), podName, types.StrategicMergePatchType, endpointReplicaLabelUpdate, metav1.PatchOptions{})
				o.Expect(err).NotTo(o.HaveOccurred())

				expectedBackendServers.Insert(podName)
				allBackendServers := sets.New[string]()
				err = wait.PollUntilContextTimeout(context.Background(), time.Second, timeoutSeconds*time.Second, true, func(ctx context.Context) (bool, error) {
					output, err := readURL(ns, execPod.Name, hostName, "/", routerIP)
					if err != nil {
						// possible 503 due to the first pod still missing, just try again
						return false, nil
					}
					if !expectedBackendServers.Has(output) {
						return false, fmt.Errorf("unexpected backend %q", output)
					}
					allBackendServers.Insert(output)

					return allBackendServers.Equal(expectedBackendServers), nil
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		})

		// This test ensures that, in the case two or more paths overlap each other, they always call the correct backend server
		// despite the order they are created. An incorrect routing can happen if "/api" is created before "/api/v1", and requests
		// to "/api/v1/subcommand" would be routed to the former, less specific, instead of going to the later, the more specific route.
		g.It("should not conflict overlapping route configurations", func() {
			execPod, doneExecPod := createExecPod(oc)
			defer doneExecPod()

			routerIP := waitForRouter(oc, execPod)

			// overlapping distinct paths on the same hostname, from the more generic to the more specific one
			// distinct paths lesser than or equal to ROUTER_BLUEPRINT_ROUTE_POOL_SIZE, which ensures that a reload does not happen
			hostName := "insecure-concurrent.hapcm.test"
			// This is strategically built in an order that should make the router to route incorrectly.
			// So the router should handle this and make the more specific match to be chosen.
			paths := []string{"/", "/api", "/api/v1"}
			o.Expect(len(paths)).ShouldNot(o.BeNumerically(">", ROUTER_BLUEPRINT_ROUTE_POOL_SIZE), "number of paths should be lesser than ROUTER_BLUEPRINT_ROUTE_POOL_SIZE")

			for i, path := range paths {
				g.By("adding path " + path)
				name := fmt.Sprintf("hapcm-insecure-concurrent-service-%d", i)
				serviceName := fmt.Sprintf("insecure-concurrent-service-%d", i)
				endpointName := fmt.Sprintf("insecure-concurrent-endpoint-%d", i)
				err := createRoute(oc, routeTypeInsecure, name, serviceName, hostName, path)
				o.Expect(err).NotTo(o.HaveOccurred())

				// wait for the route to be published
				o.Eventually(func(g o.Gomega) {
					output, err := readURL(ns, execPod.Name, hostName, path, routerIP)
					g.Expect(err).NotTo(o.HaveOccurred())

					// the result is the hostname of the target pod, so we are expecting it
					// to match the pod name used in the route configuration.
					g.Expect(output).To(o.Equal(endpointName))
				}).WithTimeout(timeoutSeconds * time.Second).
					WithPolling(time.Second).
					Should(o.Succeed())
			}
		})

		// This test sequentially creates, check to be responding,
		// and then remove all types of routes several times in a loop.
		g.It("should serve the correct routes when running with the haproxy config manager", func() {
			execPod, doneExecPod := createExecPod(oc)
			defer doneExecPod()

			routerIP := waitForRouter(oc, execPod)

			g.By("mini stress test by adding (and removing) different routes and checking that they are exposed")
			for i := 0; i < 16; i++ {
				name := fmt.Sprintf("hapcm-stress-insecure-%d", i)
				hostName := fmt.Sprintf("stress.insecure-%d.hapcm.test", i)
				err := createRoute(oc, routeTypeInsecure, name, "insecure-service", hostName, "/")
				o.Expect(err).NotTo(o.HaveOccurred())

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

func createExecPod(oc *exutil.CLI) (execPod *corev1.Pod, done func()) {
	ns := oc.KubeFramework().Namespace.Name
	execPod = exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
	return execPod, func() {
		oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
	}
}

func waitForRouter(oc *exutil.CLI, execPod *corev1.Pod) (routerIP string) {
	ns := oc.KubeFramework().Namespace.Name

	err := wait.PollUntilContextTimeout(context.Background(), time.Second, timeoutSeconds*time.Second, true, func(ctx context.Context) (bool, error) {
		pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(ctx, "router-haproxy-cfgmgr", metav1.GetOptions{})
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

	return routerIP
}

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

func readURL(ns, execPodName, host, abspath, ipaddr string) (string, error) {
	host = exutil.IPUrl(host)
	proto := "http"
	port := 80
	uri := fmt.Sprintf("%s://%s:%d%s", proto, host, port, abspath)
	cmd := fmt.Sprintf("curl -ksfL -m 5 --resolve %s:%d:%s %q", host, port, ipaddr, uri)
	output, err := e2eoutput.RunHostCmd(ns, execPodName, cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(output), nil
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
