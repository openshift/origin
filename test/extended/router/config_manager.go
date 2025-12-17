package router

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
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

	oc = exutil.NewCLI("router-config-manager")

	g.BeforeEach(func() {
		// the test has been skipped since July 2018 because it was flaking.
		// TODO: Fix the test and re-enable it in https://issues.redhat.com/browse/NE-906.
		g.Skip("HAProxy dynamic config manager tests skipped in 4.x")
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
									SecretName: "service-cert",
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

		for _, pod := range routerPods {
			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), &pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve the correct routes when running with the haproxy config manager", g.Label("Size:L"), func() {
			// the test has been skipped since July 2018 because it was flaking.
			// TODO: Fix the test and re-enable it in https://issues.redhat.com/browse/NE-906.
			g.Skip("HAProxy dynamic config manager tests skipped in 4.x")
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

			g.By("mini stress test by adding (and removing) different routes and checking that they are exposed")
			for i := 0; i < 16; i++ {
				name := fmt.Sprintf("hapcm-stress-insecure-%d", i)
				hostName := fmt.Sprintf("stress.insecure-%d.hapcm.test", i)
				err := oc.AsAdmin().Run("expose").Args("service", "insecure-service", "--name", name, "--hostname", hostName, "--labels", "select=haproxy-cfgmgr").Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				err = waitForRouteToRespond(ns, execPod.Name, "http", hostName, "/", routerIP, 0)
				o.Expect(err).NotTo(o.HaveOccurred())

				err = oc.AsAdmin().Run("delete").Args("route", name).Execute()
				o.Expect(err).NotTo(o.HaveOccurred())

				routeTypes := []string{"edge", "reencrypt", "passthrough"}
				for _, t := range routeTypes {
					name := fmt.Sprintf("hapcm-stress-%s-%d", t, i)
					hostName := fmt.Sprintf("stress.%s-%d.hapcm.test", t, i)
					serviceName := "secure-service"
					if t == "edge" {
						serviceName = "insecure-service"
					}

					err := oc.AsAdmin().Run("create").Args("route", t, name, "--service", serviceName, "--hostname", hostName).Execute()
					o.Expect(err).NotTo(o.HaveOccurred())
					err = oc.AsAdmin().Run("label").Args("route", name, "select=haproxy-cfgmgr").Execute()
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
