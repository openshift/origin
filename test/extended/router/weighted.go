package router

import (
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	utilpointer "k8s.io/utils/pointer"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	routev1 "github.com/openshift/api/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"github.com/openshift/origin/test/extended/util/image"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:image.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLIWithPodSecurityLevel("weighted-router", admissionapi.LevelBaseline)
	)

	g.BeforeEach(func() {
		routerImage, err := exutil.FindRouterImage(oc)
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating a weighted router")

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

		ns := oc.Namespace()
		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), roleBinding, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("creating Services")
		services := []corev1.Service{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "weightedendpoints1",
					Labels: map[string]string{
						"test": "router",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"test":      "weightedrouter1",
						"endpoints": "weightedrouter1",
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
					Name: "weightedendpoints2",
					Labels: map[string]string{
						"test": "router",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"test":      "weightedrouter2",
						"endpoints": "weightedrouter2",
					},
					Ports: []corev1.ServicePort{
						{
							Port: 8080,
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
					Name: "weightedroute",
					Labels: map[string]string{
						"test":   "router",
						"select": "weighted",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "weighted.example.com",
					To: routev1.RouteTargetReference{
						Name:   "weightedendpoints1",
						Kind:   "Service",
						Weight: utilpointer.Int32(90),
					},
					AlternateBackends: []routev1.RouteTargetReference{
						{
							Name:   "weightedendpoints2",
							Kind:   "Service",
							Weight: utilpointer.Int32(10),
						},
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "zeroweightroute",
					Labels: map[string]string{
						"test":   "router",
						"select": "weighted",
					},
				},
				Spec: routev1.RouteSpec{
					Host: "zeroweight.example.com",
					To: routev1.RouteTargetReference{
						Name:   "weightedendpoints1",
						Kind:   "Service",
						Weight: utilpointer.Int32(0),
					},
					AlternateBackends: []routev1.RouteTargetReference{
						{
							Name:   "weightedendpoints2",
							Kind:   "Service",
							Weight: utilpointer.Int32(0),
						},
					},
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
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
					Name: "weighted-router",
					Labels: map[string]string{
						"test": "weighted-router",
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
									Name:  "DEFAULT_CERTIFICATE",
									Value: defaultPemData,
								},
							},
							Args: []string{
								"--namespace=$(POD_NAMESPACE)",
								"-v=4",
								"--labels=select=weighted",
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
					Name: "endpoint-1",
					Labels: map[string]string{
						"test":      "weightedrouter1",
						"endpoints": "weightedrouter1",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args: []string{
								"netexec",
							},
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
					Name: "endpoint-2",
					Labels: map[string]string{
						"test":      "weightedrouter2",
						"endpoints": "weightedrouter2",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args: []string{
								"netexec",
							},
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
					Name: "endpoint-3",
					Labels: map[string]string{
						"test":      "weightedrouter2",
						"endpoints": "weightedrouter2",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: utilpointer.Int64(1),
					Containers: []corev1.Container{
						{
							Name:  "test",
							Image: image.LocationFor("registry.k8s.io/e2e-test-images/agnhost:2.56"),
							Args: []string{
								"netexec",
							},
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
		}

		for _, pod := range routerPods {
			_, err = oc.AdminKubeClient().CoreV1().Pods(ns).Create(context.Background(), &pod, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should serve a route that points to two services and respect weights", g.Label("Size:M"), func() {
			defer func() {
				if g.CurrentSpecReport().Failed() {
					dumpWeightedRouterLogs(oc, g.CurrentSpecReport().FullText())
				}
			}()

			ns := oc.KubeFramework().Namespace.Name
			execPod := exutil.CreateExecPodOrFail(oc.AdminKubeClient(), ns, "execpod")
			defer func() {
				oc.AdminKubeClient().CoreV1().Pods(ns).Delete(context.Background(), execPod.Name, *metav1.NewDeleteOptions(1))
			}()

			var routerIP string
			err := wait.Poll(time.Second, changeTimeoutSeconds*time.Second, func() (bool, error) {
				pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(oc.KubeFramework().Namespace.Name).Get(context.Background(), "weighted-router", metav1.GetOptions{})
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

			// router expected to listen on port 80
			routerURL := fmt.Sprintf("http://%s", exutil.IPUrl(routerIP))

			g.By("waiting for the healthz endpoint to respond")
			healthzURI := fmt.Sprintf("http://%s/healthz", net.JoinHostPort(routerIP, "1936"))
			err = waitForRouterOKResponseExec(ns, execPod.Name, healthzURI, routerIP, changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())

			host := "weighted.example.com"
			times := 100
			g.By(fmt.Sprintf("checking that %d requests go through successfully", times))
			// wait for the request to stabilize
			err = waitForRouterOKResponseExec(ns, execPod.Name, routerURL, "weighted.example.com", changeTimeoutSeconds)
			o.Expect(err).NotTo(o.HaveOccurred())
			// all requests should now succeed
			err = expectRouteStatusCodeRepeatedExec(ns, execPod.Name, routerURL, "weighted.example.com", http.StatusOK, times, false)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By(fmt.Sprintf("checking that there are three weighted backends in the router stats"))
			var trafficValues []string
			err = wait.PollImmediate(100*time.Millisecond, changeTimeoutSeconds*time.Second, func() (bool, error) {
				statsURL := fmt.Sprintf("http://%s/;csv", net.JoinHostPort(routerIP, "1936"))
				stats, err := getAuthenticatedRouteURLViaPod(ns, execPod.Name, statsURL, host, "admin", "password")
				o.Expect(err).NotTo(o.HaveOccurred())
				trafficValues, err = parseStats(stats, "weightedroute", 7)
				o.Expect(err).NotTo(o.HaveOccurred())
				return len(trafficValues) == 3, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			trafficEP1, err := strconv.Atoi(trafficValues[0])
			o.Expect(err).NotTo(o.HaveOccurred())
			trafficEP2, err := strconv.Atoi(trafficValues[1])
			o.Expect(err).NotTo(o.HaveOccurred())

			weightedRatio := float32(trafficEP1) / float32(trafficEP2)
			if weightedRatio < 5 && weightedRatio > 0.2 {
				e2e.Failf("Unexpected weighted ratio for incoming traffic: %v (%d/%d)", weightedRatio, trafficEP1, trafficEP2)
			}

			g.By(fmt.Sprintf("checking that zero weights are also respected by the router"))
			host = "zeroweight.example.com"
			err = expectRouteStatusCodeExec(ns, execPod.Name, routerURL, host, http.StatusServiceUnavailable)
			o.Expect(err).NotTo(o.HaveOccurred())
		})
	})
})

func parseStats(stats string, backendSubstr string, statsField int) ([]string, error) {
	r := csv.NewReader(strings.NewReader(stats))
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}

	fieldValues := make([]string, 0)
	for _, rec := range records {
		if strings.Contains(rec[0], backendSubstr) && !strings.Contains(rec[1], "BACKEND") {
			fieldValues = append(fieldValues, rec[statsField])
		}
	}
	return fieldValues, nil
}

func dumpWeightedRouterLogs(oc *exutil.CLI, name string) {
	log, _ := pod.GetPodLogs(context.TODO(), oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name, "weighted-router", "router")
	e2e.Logf("Weighted Router test %s logs:\n %s", name, log)
}
