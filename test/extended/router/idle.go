package router

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	utilpointer "k8s.io/utils/pointer"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	routev1 "github.com/openshift/api/route/v1"
	unidlingapi "github.com/openshift/api/unidling/v1alpha1"
	exutil "github.com/openshift/origin/test/extended/util"
	exutilimage "github.com/openshift/origin/test/extended/util/image"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = g.Describe("[sig-network-edge][Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()

	var (
		oc = exutil.NewCLIWithPodSecurityLevel("router-idling", admissionapi.LevelBaseline)
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentSpecReport().Failed() {
			exutil.DumpPodLogsStartingWithInNamespace("router", "openshift-ingress", oc.AsAdmin())
		}
	})

	g.Describe("The HAProxy router", func() {
		g.It("should be able to connect to a service that is idled because a GET on the route will unidle it", g.Label("Size:M"), func() {
			network, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster network configuration")
			if !(network.Status.NetworkType == "OVNKubernetes" || network.Status.NetworkType == "OpenShiftSDN") {
				g.Skip("idle feature only supported on OVNKubernetes or OpenShiftSDN")
				return
			}

			infra, err := oc.AdminConfigClient().ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to get cluster-wide infrastructure")
			switch infra.Status.PlatformStatus.Type {
			case configv1.OvirtPlatformType, configv1.KubevirtPlatformType, configv1.LibvirtPlatformType, configv1.VSpherePlatformType:
				// Skip on platforms where the default
				// router is not exposed by a load
				// balancer service.
				g.Skip("https://bugzilla.redhat.com/show_bug.cgi?id=1933114")
			}

			timeout := 15 * time.Minute

			g.By("creating test fixtures")
			ns := oc.KubeFramework().Namespace.Name
			idleRoute := &routev1.Route{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-test",
					Labels: map[string]string{
						"app": "idle-test",
					},
				},
				Spec: routev1.RouteSpec{
					Port: &routev1.RoutePort{
						TargetPort: intstr.FromInt(8080),
					},
					To: routev1.RouteTargetReference{
						Kind: "Service",
						Name: "idle-test",
					},
				},
			}
			_, err = oc.RouteClient().RouteV1().Routes(ns).Create(context.Background(), idleRoute, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			idleService := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-test",
					Labels: map[string]string{
						"app": "idle-test",
					},
				},
				Spec: corev1.ServiceSpec{
					Selector: map[string]string{
						"app": "idle-test",
					},
					Ports: []corev1.ServicePort{
						{
							Port:       8080,
							Name:       "8080-http",
							TargetPort: intstr.FromInt(8080),
							Protocol:   corev1.ProtocolTCP,
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().CoreV1().Services(ns).Create(context.Background(), idleService, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			idleDeployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name: "idle-test",
				},
				Spec: appsv1.DeploymentSpec{
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "idle-test",
						},
					},
					Replicas: utilpointer.Int32(1),
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Name: "idle-test",
							Labels: map[string]string{
								"app": "idle-test",
							},
						},

						Spec: corev1.PodSpec{
							Containers: []corev1.Container{
								{
									Image: exutilimage.ShellImage(),
									Name:  "idle-test",
									ReadinessProbe: &corev1.Probe{
										ProbeHandler: corev1.ProbeHandler{
											HTTPGet: &corev1.HTTPGetAction{
												Path: "/",
												Port: intstr.FromInt(8080),
											},
										},
										InitialDelaySeconds: 3,
										PeriodSeconds:       3,
									},
									Command: []string{
										"/usr/bin/socat",
										"TCP4-LISTEN:8080,reuseaddr,fork",
										`EXEC:'/bin/bash -c \"printf \\\"HTTP/1.0 200 OK\r\n\r\n\\\"; sed -e \\\"/^\r/q\\\"\"'`,
									},
									Ports: []corev1.ContainerPort{
										{
											ContainerPort: 8080,
											Protocol:      corev1.ProtocolTCP,
										},
									},
								},
							},
						},
					},
				},
			}

			_, err = oc.AdminKubeClient().AppsV1().Deployments(ns).Create(context.Background(), idleDeployment, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Waiting for pods to be running")
			err = waitForRunningPods(oc, 1, exutil.ParseLabelsOrDie("app=idle-test"), timeout)
			o.Expect(err).NotTo(o.HaveOccurred(), "pods not running")

			g.By("Getting a 200 status code when accessing the route")
			hostname, err := getHostnameForRoute(oc, "idle-test")
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitHTTPGetStatus(hostname, http.StatusOK, timeout)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected status 200 from the GET request")

			g.By("Idling the service")
			_, err = oc.Run("idle").Args("idle-test").Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to idle the service")

			var annotations map[string]string

			g.By("Fetching the service and checking the idle annotations are present")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				service, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting service: %v", err)
					return false, nil
				}
				annotations = service.Annotations
				_, idledAt := annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := annotations[unidlingapi.UnidleTargetAnnotation]
				return idledAt && unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "failed to fetch the service")
			mustVerifyIdleAnnotationValues(annotations)

			// wait for target deployment to actually scale down
			err = waitForRunningPods(oc, 0, exutil.ParseLabelsOrDie("app=idle-test"), timeout)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("Unidling the service by making a GET request on the route")
			err = waitHTTPGetStatus(hostname, http.StatusOK, timeout)
			o.Expect(err).NotTo(o.HaveOccurred(), "expected status 200 from the GET request")

			g.By("Validating that the idle annotations have been removed from the endpoints")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				endpoints, err := oc.KubeClient().CoreV1().Endpoints(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting endpoints: %v", err)
					return false, nil
				}
				_, idledAt := endpoints.Annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := endpoints.Annotations[unidlingapi.UnidleTargetAnnotation]
				return !idledAt && !unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "idle annotations not removed from endpoints")

			g.By("Validating that the idle annotations have been removed from the service")
			err = wait.PollImmediate(time.Second, timeout, func() (bool, error) {
				service, err := oc.KubeClient().CoreV1().Services(oc.Namespace()).Get(context.Background(), "idle-test", metav1.GetOptions{})
				if err != nil {
					e2e.Logf("Error getting service: %v", err)
					return false, nil
				}
				_, idledAt := service.Annotations[unidlingapi.IdledAtAnnotation]
				_, unidleTarget := service.Annotations[unidlingapi.UnidleTargetAnnotation]
				return !idledAt && !unidleTarget, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "idle annotations not removed from service")
		})
	})
})

func mustVerifyIdleAnnotationValues(annotations map[string]string) {
	o.Expect(annotations).To(o.HaveKey(unidlingapi.IdledAtAnnotation))
	o.Expect(annotations).To(o.HaveKey(unidlingapi.UnidleTargetAnnotation))

	idledAtAnnotation := annotations[unidlingapi.IdledAtAnnotation]
	idledAtTime, err := time.Parse(time.RFC3339, idledAtAnnotation)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(idledAtTime).To(o.BeTemporally("~", time.Now(), 5*time.Minute))

	g.By("Checking the idle targets")
	unidleTargetAnnotation := annotations[unidlingapi.UnidleTargetAnnotation]
	var unidleTargets []unidlingapi.RecordedScaleReference
	err = json.Unmarshal([]byte(unidleTargetAnnotation), &unidleTargets)
	o.Expect(err).ToNot(o.HaveOccurred())
	o.Expect(unidleTargets).To(o.Equal([]unidlingapi.RecordedScaleReference{
		{
			Replicas: 1,
			CrossGroupObjectReference: unidlingapi.CrossGroupObjectReference{
				Kind:  "Deployment",
				Group: "apps",
				Name:  "idle-test",
			},
		},
	}))
}

// waitForRunningPods waits for podCount pods matching podSelector are
// in the running state. It retries the request every second and will
// return an error if the conditions are not met after the specified
// timeout.
func waitForRunningPods(oc *exutil.CLI, podCount int, podLabels labels.Selector, timeout time.Duration) error {
	ns := oc.KubeFramework().Namespace.Name

	if err := wait.PollImmediate(time.Second, timeout, func() (bool, error) {
		podList, err := oc.AdminKubeClient().CoreV1().Pods(ns).List(context.Background(), metav1.ListOptions{LabelSelector: podLabels.String()})
		if err != nil {
			e2e.Logf("Error listing pods: %v", err)
			return false, nil
		}
		return len(podList.Items) == podCount, nil
	}); err != nil {
		return fmt.Errorf("failed to list pods: %v", err)
	}

	e2e.Logf("Waiting for %d pods in namespace %s", podCount, ns)
	c := oc.AdminKubeClient()
	pods, err := exutil.WaitForPods(c.CoreV1().Pods(ns), podLabels, exutil.CheckPodIsRunning, podCount, timeout)
	if err != nil {
		return fmt.Errorf("error in pod wait: %v", err)
	} else if len(pods) < podCount {
		return fmt.Errorf("only got %v out of %v pods in %s (timeout)", len(pods), podCount, timeout)
	}

	e2e.Logf("All expected pods in namespace %s are running", ns)
	return nil
}

// waitHTTPGetStatus repeatedly makes a HTTP GET request to hostname
// until the GET response equals statusCode. It retries every second
// and will return an error if the conditions are not met after the
// specified timeout.
func waitHTTPGetStatus(hostname string, statusCode int, timeout time.Duration) error {
	client := makeHTTPClient(false, 30*time.Second)
	var attempt int

	url := "http://" + hostname

	return wait.Poll(time.Second, timeout, func() (bool, error) {
		attempt += 1
		resp, err := client.Get(url)
		if err != nil {
			e2e.Logf("GET#%v %q error=%v", attempt, url, err)
			return false, nil // could be 503 if service not ready
		}
		defer resp.Body.Close()
		io.Copy(ioutil.Discard, resp.Body)
		e2e.Logf("GET#%v %q status=%v", attempt, url, resp.StatusCode)
		return resp.StatusCode == statusCode, nil
	})
}
