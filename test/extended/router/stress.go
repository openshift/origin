package images

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	corev1 "k8s.io/api/core/v1"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	clientset "k8s.io/client-go/kubernetes"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	routev1 "github.com/openshift/api/route/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[Conformance][Area:Networking][Feature:Router]", func() {
	defer g.GinkgoRecover()
	var (
		routerImage string
		ns          string
		oc          *exutil.CLI
	)

	// this hook must be registered before the framework namespace teardown
	// hook
	g.AfterEach(func() {
		if g.CurrentGinkgoTestDescription().Failed {
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).Route().Routes(ns)
			if routes, _ := client.List(metav1.ListOptions{}); routes != nil {
				outputIngress(routes.Items...)
			}
			exutil.DumpPodLogsStartingWith("router-", oc)
		}
	})

	oc = exutil.NewCLI("router-stress", exutil.KubeConfigPath())

	g.BeforeEach(func() {
		ns = oc.Namespace()

		dc, err := oc.AdminAppsClient().Apps().DeploymentConfigs("default").Get("router", metav1.GetOptions{})
		if kapierrs.IsNotFound(err) {
			g.Skip("no router installed on the cluster")
			imagePrefix := os.Getenv("OS_IMAGE_PREFIX")
			if len(imagePrefix) == 0 {
				imagePrefix = "openshift/origin"
			}
			routerImage = imagePrefix + "-haproxy-router:latest"
			return
		}
		o.Expect(err).NotTo(o.HaveOccurred())
		routerImage = dc.Spec.Template.Spec.Containers[0].Image

		_, err = oc.AdminKubeClient().Rbac().RoleBindings(ns).Create(&rbacv1.RoleBinding{
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
		})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("converges when multiple routers are writing status", func() {
			g.By("deploying a scaled out namespace scoped router")
			rs, err := oc.KubeClient().Extensions().ReplicaSets(ns).Create(
				scaledRouter(
					routerImage,
					[]string{
						"--loglevel=4",
						fmt.Sprintf("--namespace=%s", ns),
						"--resync-interval=2m",
						"--name=namespaced",
					},
				),
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating multiple routes")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).Route().Routes(ns)
			var rv string
			for i := 0; i < 10; i++ {
				_, err := client.Create(&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%d", i),
					},
					Spec: routev1.RouteSpec{
						To: routev1.RouteTargetReference{Name: "test"},
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
					},
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting for all routes to have a status")
			err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				routes, err := client.List(metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				o.Expect(routes.Items).To(o.HaveLen(10))
				for _, route := range routes.Items {
					ingress := findIngress(&route, "namespaced")
					if ingress == nil {
						return false, nil
					}
					o.Expect(ingress.Host).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions[0].LastTransitionTime).NotTo(o.BeNil())
					o.Expect(ingress.Conditions[0].Type).To(o.Equal(routev1.RouteAdmitted))
					o.Expect(ingress.Conditions[0].Status).To(o.Equal(corev1.ConditionTrue))
				}
				outputIngress(routes.Items...)
				rv = routes.ResourceVersion
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we don't continue to write")
			writes := 0
			w, err := client.Watch(metav1.ListOptions{Watch: true, ResourceVersion: rv})
			o.Expect(err).NotTo(o.HaveOccurred())
			defer w.Stop()
			timer := time.NewTimer(10 * time.Second)
			ch := w.ResultChan()
		Wait:
			for i := 0; ; i++ {
				select {
				case _, ok := <-ch:
					writes++
					o.Expect(ok).To(o.BeTrue())
				case <-timer.C:
					break Wait
				}
			}
			o.Expect(writes).To(o.BeNumerically("<", 10))

			verifyCommandEquivalent(oc.KubeClient(), rs, "md5sum /var/lib/haproxy/conf/*")
		})

		g.It("converges when multiple routers are writing conflicting status", func() {
			g.By("deploying a scaled out namespace scoped router")

			rs, err := oc.KubeClient().Extensions().ReplicaSets(ns).Create(
				scaledRouter(
					routerImage,
					[]string{
						"--loglevel=4",
						fmt.Sprintf("--namespace=%s", ns),
						"--resync-interval=2m",
						"--name=conflicting",
						"--override-hostname",
						// causes each pod to have a different value
						"--hostname-template=${name}-${namespace}.$(NAME).local",
					},
				),
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating multiple routes")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).Route().Routes(ns)
			var rv string
			for i := 0; i < 10; i++ {
				_, err := client.Create(&routev1.Route{
					ObjectMeta: metav1.ObjectMeta{
						Name: fmt.Sprintf("%d", i),
					},
					Spec: routev1.RouteSpec{
						To: routev1.RouteTargetReference{Name: "test"},
						Port: &routev1.RoutePort{
							TargetPort: intstr.FromInt(8080),
						},
					},
				})
				o.Expect(err).NotTo(o.HaveOccurred())
			}

			g.By("waiting for sufficient routes to have a status")
			err = wait.Poll(time.Second, 2*time.Minute, func() (bool, error) {
				routes, err := client.List(metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				o.Expect(routes.Items).To(o.HaveLen(10))
				other := 0
				conflicting := 0
				for _, route := range routes.Items {
					ingress := findIngress(&route, "conflicting")
					if ingress == nil {
						if len(route.Status.Ingress) > 0 {
							other++
						}
						continue
					}
					if len(route.Status.Ingress) > 1 {
						other++
					}
					conflicting++
					o.Expect(ingress.Host).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions[0].LastTransitionTime).NotTo(o.BeNil())
					o.Expect(ingress.Conditions[0].Type).To(o.Equal(routev1.RouteAdmitted))
					o.Expect(ingress.Conditions[0].Status).To(o.Equal(corev1.ConditionTrue))
				}
				// if other routers are writing status, wait until we get a complete
				// set since we don't have a way to tell other routers to ignore us
				if conflicting < 3 && other%10 != 0 {
					return false, nil
				}
				outputIngress(routes.Items...)
				rv = routes.ResourceVersion
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we stop writing conflicts rapidly")
			writes := 0
			w, err := client.Watch(metav1.ListOptions{Watch: true, ResourceVersion: rv})
			o.Expect(err).NotTo(o.HaveOccurred())
			func() {
				defer w.Stop()
				timer := time.NewTimer(10 * time.Second)
				ch := w.ResultChan()
			Wait:
				for i := 0; ; i++ {
					select {
					case _, ok := <-ch:
						writes++
						o.Expect(ok).To(o.BeTrue())
					case <-timer.C:
						break Wait
					}
				}
				o.Expect(writes).To(o.BeNumerically("<", 10))
			}()

			// the os_http_be.map file will vary, so only check the haproxy config
			verifyCommandEquivalent(oc.KubeClient(), rs, "md5sum /var/lib/haproxy/conf/haproxy.config")

			g.By("clearing a single route's status")
			route, err := client.Patch("9", types.MergePatchType, []byte(`{"status":{"ingress":[]}}`), "status")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that only get a few updates")
			writes = 0
			w, err = client.Watch(metav1.ListOptions{Watch: true, ResourceVersion: route.ResourceVersion})
			o.Expect(err).NotTo(o.HaveOccurred())
			func() {
				defer w.Stop()
				timer := time.NewTimer(10 * time.Second)
				ch := w.ResultChan()
			Wait:
				for i := 0; ; i++ {
					select {
					case obj, ok := <-ch:
						o.Expect(ok).To(o.BeTrue())
						if r, ok := obj.Object.(*routev1.Route); ok {
							if r == nil || r.Name != "9" {
								continue
							}
						}
						writes++
					case <-timer.C:
						break Wait
					}
				}
				o.Expect(writes).To(o.BeNumerically("<", 5))
			}()
		})
	})
})

func findIngress(route *routev1.Route, name string) *routev1.RouteIngress {
	for i, ingress := range route.Status.Ingress {
		if ingress.RouterName == name {
			return &route.Status.Ingress[i]
		}
	}
	return nil
}

func scaledRouter(image string, args []string) *extensionsv1beta1.ReplicaSet {
	one := int64(1)
	scale := int32(3)
	return &extensionsv1beta1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "router",
		},
		Spec: extensionsv1beta1.ReplicaSetSpec{
			Replicas: &scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "router"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "router"},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &one,
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{Name: "NAME", ValueFrom: &corev1.EnvVarSource{FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.name"}}},
							},
							Name:  "router",
							Image: image,
							Args:  args,
						},
					},
				},
			},
		},
	}
}

func outputIngress(routes ...routev1.Route) {
	b := &bytes.Buffer{}
	w := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "NAME\tROUTER\tHOST\tLAST TRANSITION\n")
	for _, route := range routes {
		for _, ingress := range route.Status.Ingress {
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", route.Name, ingress.RouterName, ingress.Host, ingress.Conditions[0].LastTransitionTime)
		}
	}
	w.Flush()
	e2e.Logf("Routes:\n%s", b.String())
}

func verifyCommandEquivalent(c clientset.Interface, rs *extensionsv1beta1.ReplicaSet, cmd string) {
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	o.Expect(err).NotTo(o.HaveOccurred())
	podList, err := c.CoreV1().Pods(rs.Namespace).List(metav1.ListOptions{LabelSelector: selector.String()})
	o.Expect(err).NotTo(o.HaveOccurred())

	var values map[string]string
	err = wait.PollImmediate(5*time.Second, time.Minute, func() (bool, error) {
		values = make(map[string]string)
		uniques := make(map[string]struct{})
		for _, pod := range podList.Items {
			stdout, err := e2e.RunHostCmdWithRetries(pod.Namespace, pod.Name, cmd, e2e.StatefulSetPoll, e2e.StatefulPodTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			values[pod.Name] = stdout
			uniques[stdout] = struct{}{}
		}
		return len(uniques) == 1, nil
	})
	for name, stdout := range values {
		stdout = strings.TrimSuffix(stdout, "\n")
		e2e.Logf(name + ": " + strings.Join(strings.Split(stdout, "\n"), fmt.Sprintf("\n%s: ", name)))
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

// waitForReadyReplicaSet waits until the replicaset has all of its replicas ready.
// Waits for longer than the standard e2e method.
func waitForReadyReplicaSet(c clientset.Interface, ns, name string) error {
	err := wait.Poll(3*time.Second, 3*time.Minute, func() (bool, error) {
		rs, err := c.ExtensionsV1beta1().ReplicaSets(ns).Get(name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return *(rs.Spec.Replicas) == rs.Status.Replicas && *(rs.Spec.Replicas) == rs.Status.ReadyReplicas, nil
	})
	if err == wait.ErrWaitTimeout {
		err = fmt.Errorf("replicaset %q never became ready", name)
	}
	return err
}
