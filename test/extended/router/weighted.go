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

	g "github.com/onsi/ginkgo"
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

var _ = g.Describe("[sig-network][Feature:Router][apigroup:config.openshift.io][apigroup:image.openshift.io]", func() {
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
									Name: "DEFAULT_CERTIFICATE",
									Value: `-----BEGIN CERTIFICATE-----
MIIDuTCCAqGgAwIBAgIUZYD30F0sJl7HqxE7gAequtxk/HowDQYJKoZIhvcNAQEL
BQAwgaExCzAJBgNVBAYTAlVTMQswCQYDVQQIDAJTQzEVMBMGA1UEBwwMRGVmYXVs
dCBDaXR5MRwwGgYDVQQKDBNEZWZhdWx0IENvbXBhbnkgTHRkMRAwDgYDVQQLDAdU
ZXN0IENBMRowGAYDVQQDDBF3d3cuZXhhbXBsZWNhLmNvbTEiMCAGCSqGSIb3DQEJ
ARYTZXhhbXBsZUBleGFtcGxlLmNvbTAeFw0yMjAxMjgwMjU0MDlaFw0zMjAxMjYw
MjU0MDlaMHwxGDAWBgNVBAMMD3d3dy5leGFtcGxlLmNvbTELMAkGA1UECAwCU0Mx
CzAJBgNVBAYTAlVTMSIwIAYJKoZIhvcNAQkBFhNleGFtcGxlQGV4YW1wbGUuY29t
MRAwDgYDVQQKDAdFeGFtcGxlMRAwDgYDVQQLDAdFeGFtcGxlMIIBIjANBgkqhkiG
9w0BAQEFAAOCAQ8AMIIBCgKCAQEA71W7gdEnM+Nm4/SA/4jEJ2SPQfVjkCMsIYGO
WrLLHq23HkMGstQoPyBnjLY8LmkKQsNhhWGRMWQz6+yGKgI1gh8huhfocuw+HODE
K3ugP/3DlaVEQlIQbVzwxDx+K78UqZHecQAJfvakuS/JThxsMf8/pqLuhjAf+t9N
k0CO8Z6mNVALtSvyQ+e+zjmzepVtu6WmtJ+8zW9dBQEmg0QCfWFd06836LrfixLk
vTRgCn0lzTuj7rSuGjY45JDIvKK4jZGQJKsYN59Wxg1d2CEoXBUJOJjecVdS3NhY
ubHNdcm+6Equ5ZmyVEkBmv462rOcednsHU6Ggt/vWSe05EOPVQIDAQABow0wCzAJ
BgNVHRMEAjAAMA0GCSqGSIb3DQEBCwUAA4IBAQCHI+fkEr27bJ2IMtFuHpSLpFF3
E4R5oVHt8XjflwKmuclyyLa8Z7nXnuvQLHa4jwf0tWUixsmtOyQN4tBI/msMk2PF
+ao2amcPoIo2lAg63+jFsIzkr2MEXBPu09wwt86e3XCoqmqT1Psnihh+Ys9KIPnc
wMr9muGkOh03O61vo71iaV17UKeGM4bzod333pSQIXLdYnoOuvmKdCsnD00lADoI
93DmG/4oYR/mD93QjxPFPDxDxR4isvWGoj7iXx7CFkN7PR9B3IhZt+T//ddeau3y
kXK0iSxOhyaqHvl15hHQ8tKPBBJRSDVU4qmaqAYWRXr65yxBoelHhTJQ6Gt4
-----END CERTIFICATE-----
-----BEGIN PRIVATE KEY-----
MIIEvgIBADANBgkqhkiG9w0BAQEFAASCBKgwggSkAgEAAoIBAQDvVbuB0Scz42bj
9ID/iMQnZI9B9WOQIywhgY5assserbceQway1Cg/IGeMtjwuaQpCw2GFYZExZDPr
7IYqAjWCHyG6F+hy7D4c4MQre6A//cOVpURCUhBtXPDEPH4rvxSpkd5xAAl+9qS5
L8lOHGwx/z+mou6GMB/6302TQI7xnqY1UAu1K/JD577OObN6lW27paa0n7zNb10F
ASaDRAJ9YV3Trzfout+LEuS9NGAKfSXNO6PutK4aNjjkkMi8oriNkZAkqxg3n1bG
DV3YIShcFQk4mN5xV1Lc2Fi5sc11yb7oSq7lmbJUSQGa/jras5x52ewdToaC3+9Z
J7TkQ49VAgMBAAECggEAaCBzqOI3XSLlo+2/pe158e2VSkwZ2h8DVzyHk7xQFPPd
RKRCqNEXBYfypUyv2D1JAo0Aw8gUJFoFIPLR2DsHzqn+wXkfX8iaqXO8xXJO4Shl
zJiPnw8XKI2UDryG5D+JHNFi5uTuPLfQKOW6fmptRD9aEQS4I9eSQlKe7J7c0g+t
pCR1vCp6ZMFIXDgpHhquArI1fjA36nWK0dJkaO9LrTYPgeMIr0KFjEF+W3UPh/af
uw/KLjzyzHExwfVBcGZonb6rG1nU/7isUHqK75OhOKDcXpv+7NCBYZ6fu4COlE0O
+yGztbRXojWo1upKzzGPM+yoLyNA1aSljpCGOCSljQKBgQD+4i5FzRQ+e1XZxvUt
izypHHQcc7y9DfwKTwLXb9EUhmGCmrxVIuM+gm5N/Y/eXDjqtR2bqg7iIFjj3KTS
f9djCYT8FqlTtyDBk/qFNLchDX/mrykOuhqIXfT7JpQbk5+qkCy8k2ZJMl2ToNXA
WRqRCP4oa1WJMmoJFwo3BIVRIwKBgQDwYh2ryrs/QFE0W082oHAQ3Nrce5JmOtFp
70X/v8zZ8ESdeo7KOS0tNLeirBxlDGvUAesKwUHU1YwTgWhl/DkoPtv9INgT8kxS
VRcrix9kq62uiD+TKI732mwoG36keJdRECrQYRYjX+mf364EI+DeNmbPs3xsigaF
Zdbg+umxJwKBgF4fFelOvuAH2X8PGnDUDvV//VyYXKUPqfgAj1MRBotmyFFbZJqn
xHTL44HHVb5OHfKGKUXXeaGFQm36h573+Iio9kPE9ohkgqMZSxSvj8ST4JxGKIo4
rR2YXKP17hF05SwuC2cjo0z6XVXruaNLBCV0xa4VXMPKKx/qMyp37+czAoGBAL8c
woo6e/QlpmoBzlCX7YD6leaFODeeu6+FVBmo26zJoUOylKOiIZC3QOhL/ac44OGF
ROEgFL6pqNw5Hk824BpnH294FVKGaLdsfydXTHY1J7iDCkhtDn1vYl3gvib02RjR
ybgx9+/X6V3579fKzpTcm5C2Gk4Qzm5wMQ5dbj4xAoGBANYzYbBu8bItAEE6ohgf
D27SPW7VJsHGzbgRNC2SGCBzo3XaTJ0A8IMP+ghl5ndCJdLBz2FpeZLQvxOuopQD
J5dJXQxp7y20vh2C1e3wTPlA5CHHKpU1JZAe4THCJUg+EPwa4I+BOlvp71EB7BaH
bk65iLoLrUSkxMDi46qTAs5K
-----END PRIVATE KEY-----`,
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
							Image: image.LocationFor("k8s.gcr.io/e2e-test-images/agnhost:2.39"),
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
							Image: image.LocationFor("k8s.gcr.io/e2e-test-images/agnhost:2.39"),
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
							Image: image.LocationFor("k8s.gcr.io/e2e-test-images/agnhost:2.39"),
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
		g.It("should serve a route that points to two services and respect weights", func() {
			defer func() {
				if g.CurrentGinkgoTestDescription().Failed {
					dumpWeightedRouterLogs(oc, g.CurrentGinkgoTestDescription().FullTestText)
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
			routerURL := fmt.Sprintf("http://%s", routerIP)

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
	log, _ := pod.GetPodLogs(oc.AdminKubeClient(), oc.KubeFramework().Namespace.Name, "weighted-router", "router")
	e2e.Logf("Weighted Router test %s logs:\n %s", name, log)
}
