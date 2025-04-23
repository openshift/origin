package router

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/google/go-cmp/cmp"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/kubernetes/test/e2e/framework/statefulset"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/watch"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
	"k8s.io/utils/ptr"

	routev1 "github.com/openshift/api/route/v1"
	v2 "github.com/openshift/api/security/v1"
	routeclientset "github.com/openshift/client-go/route/clientset/versioned"
	v1 "github.com/openshift/client-go/route/clientset/versioned/typed/route/v1"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:Router][apigroup:route.openshift.io]", func() {
	defer g.GinkgoRecover()
	var (
		routerImage string
		ns          string
		oc          *exutil.CLI
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

	oc = exutil.NewCLIWithPodSecurityLevel("router-stress", admissionapi.LevelBaseline)

	g.BeforeEach(func() {
		ns = oc.Namespace()

		var err error
		routerImage, err = exutil.FindRouterImage(oc)
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
		// The router typically runs with allowPrivilegeEscalation enabled; however, all service accounts are assigned
		// to restricted-v2 scc by default, which disallows privilege escalation. The restricted policy permits
		// privilege escalation.
		_, err = oc.AdminKubeClient().RbacV1().RoleBindings(ns).Create(context.Background(), &rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "router-restricted",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind: "ServiceAccount",
					Name: "default",
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind: "ClusterRole",
				Name: "system:openshift:scc:restricted",
			},
		}, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
	})

	g.Describe("The HAProxy router", func() {
		g.It("converges when multiple routers are writing status", func() {
			g.By("deploying a scaled out namespace scoped router")
			routerName := "namespaced"
			rs, err := oc.KubeClient().AppsV1().ReplicaSets(ns).Create(
				context.Background(),
				scaledRouter(
					"router",
					routerImage,
					[]string{
						"-v=4",
						fmt.Sprintf("--namespace=%s", ns),
						"--resync-interval=2m",
						fmt.Sprintf("--name=%s", routerName),
					},
				),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating multiple routes")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)

			// Start recording updates BEFORE the routes get created, so we capture all the updates.
			err, stopRecordingRouteUpdates, updateCountCh := startRecordingRouteStatusUpdates(client, routerName, "")
			o.Expect(err).NotTo(o.HaveOccurred())

			err = createTestRoutes(client, 10)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for all routes to have a status")
			err = wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
				routes, err := client.List(context.Background(), metav1.ListOptions{})
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
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we don't continue to write")
			err, writes := waitForRouteStatusUpdates(stopRecordingRouteUpdates, updateCountCh, 15*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Number of writes should be exactly equal to ten because there are only 10 routes to update.
			o.Expect(writes).To(o.BeNumerically("==", 10))

			verifyCommandEquivalent(oc.KubeClient(), rs, "md5sum /var/lib/haproxy/conf/*")
		})

		g.It("converges when multiple routers are writing conflicting status", func() {
			g.By("deploying a scaled out namespace scoped router")
			routerName := "conflicting"
			numOfRoutes := 20
			rs, err := oc.KubeClient().AppsV1().ReplicaSets(ns).Create(
				context.Background(),
				scaledRouter(
					"router",
					routerImage,
					[]string{
						"-v=4",
						fmt.Sprintf("--namespace=%s", ns),
						// Make resync interval high to avoid contention flushes.
						"--resync-interval=24h",
						fmt.Sprintf("--name=%s", routerName),
						"--override-hostname",
						// causes each pod to have a different value
						"--hostname-template=${name}-${namespace}.$(NAME).local",
					},
				),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rs.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)

			// Start recording updates BEFORE the routes get created, so we capture all the updates.
			err, stopRecordingRouteUpdates, updateCountCh := startRecordingRouteStatusUpdates(client, routerName, "")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating multiple routes")
			err = createTestRoutes(client, numOfRoutes)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for sufficient routes to have a status")
			err = wait.Poll(5*time.Second, 2*time.Minute, func() (bool, error) {
				routes, err := client.List(context.Background(), metav1.ListOptions{})
				if err != nil {
					return false, err
				}
				o.Expect(routes.Items).To(o.HaveLen(numOfRoutes))
				conflicting := 0
				for _, route := range routes.Items {
					ingress := findIngress(&route, routerName)
					if ingress == nil {
						continue
					}
					conflicting++
					o.Expect(ingress.Host).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions).NotTo(o.BeEmpty())
					o.Expect(ingress.Conditions[0].LastTransitionTime).NotTo(o.BeNil())
					o.Expect(ingress.Conditions[0].Type).To(o.Equal(routev1.RouteAdmitted))
					o.Expect(ingress.Conditions[0].Status).To(o.Equal(corev1.ConditionTrue))
				}
				// We will wait until all routes get an ingress status for conflicting.
				if conflicting != numOfRoutes {
					e2e.Logf("waiting for %d ingresses for %q, got %d", numOfRoutes, routerName, conflicting)
					return false, nil
				}
				outputIngress(routes.Items...)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we stop writing conflicts rapidly")

			// Start recording updates BEFORE the routes get created, so we capture all the updates.
			err, writes := waitForRouteStatusUpdates(stopRecordingRouteUpdates, updateCountCh, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())

			// First, we expect at least 20 writes for 20 routes, as every route should get a conflicting status.
			// Next, we expect 1-2 more writes per route until per-route contention activates.
			// Next, we expect the maxContention logic to activate and stop all updates when the routers detect > 5
			// contentions.
			// In total, we expect around 30-35 writes, but we generously allow for up to 50 writes to accommodate for
			// minor discrepancies in contention tracker logic.
			o.Expect(writes).To(o.BeNumerically(">=", numOfRoutes))
			o.Expect(writes).To(o.BeNumerically("<=", 50))

			// the os_http_be.map file will vary, so only check the haproxy config
			verifyCommandEquivalent(oc.KubeClient(), rs, "md5sum /var/lib/haproxy/conf/haproxy.config")

			g.By("clearing a single route's status")
			// Start recording updates BEFORE the route gets updated, so we capture all the updates.
			err, stopRecordingRouteUpdates, updateCountCh = startRecordingRouteStatusUpdates(client, routerName, "9")
			o.Expect(err).NotTo(o.HaveOccurred())

			_, err = client.Patch(context.Background(), "9", types.MergePatchType, []byte(`{"status":{"ingress":[]}}`), metav1.PatchOptions{}, "status")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that only get a few updates")
			err, writes = waitForRouteStatusUpdates(stopRecordingRouteUpdates, updateCountCh, 15*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Ideally, this should be at least 1 write (our patch). MaxContentions should have kicked in for most
			// routers so the updates should be limited.
			o.Expect(writes).To(o.BeNumerically(">=", 1))
			o.Expect(writes).To(o.BeNumerically("<=", 5))
		})

		g.It("converges when multiple routers are writing conflicting upgrade validation status", func() {
			g.By("deploying a scaled out namespace scoped router that adds the UnservableInFutureVersions condition")

			routerName := "conflicting"
			numOfRoutes := 20
			rsAdd, err := oc.KubeClient().AppsV1().ReplicaSets(ns).Create(
				context.Background(),
				scaledRouter(
					"router-add-condition",
					routerImage,
					[]string{
						"-v=5",
						fmt.Sprintf("--namespace=%s", ns),
						// Make resync interval high to avoid contention flushes.
						"--resync-interval=24h",
						fmt.Sprintf("--name=%s", routerName),
						"--debug-upgrade-validation-force-add-condition",
					},
				),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rsAdd.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("creating multiple routes")
			client := routeclientset.NewForConfigOrDie(oc.AdminConfig()).RouteV1().Routes(ns)
			err = createTestRoutes(client, numOfRoutes)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("waiting for sufficient routes to have a UnservableInFutureVersions and Admitted status condition")
			err = wait.PollUntilContextTimeout(context.Background(), 10*time.Second, 10*time.Minute, false, func(ctx context.Context) (bool, error) {
				routes, err := client.List(ctx, metav1.ListOptions{})
				if err != nil {
					e2e.Logf("failed to list routes: %v", err)
					return false, nil
				}
				o.Expect(routes.Items).To(o.HaveLen(numOfRoutes))
				unservableCondition := 0
				admittedCondition := 0
				for _, route := range routes.Items {
					ingress := findIngress(&route, routerName)
					if ingress == nil {
						continue
					}
					// Find UnservableInFutureVersions condition.
					if cond := findIngressCondition(ingress, routev1.RouteUnservableInFutureVersions); cond != nil {
						unservableCondition++
						o.Expect(ingress.Host).NotTo(o.BeEmpty())
						o.Expect(ingress.Conditions).NotTo(o.BeEmpty())
						o.Expect(cond.LastTransitionTime).NotTo(o.BeNil())
						o.Expect(cond.Status).To(o.Equal(corev1.ConditionTrue))
					}
					// Find Admitted condition.
					if cond := findIngressCondition(ingress, routev1.RouteAdmitted); cond != nil {
						admittedCondition++
						o.Expect(ingress.Host).NotTo(o.BeEmpty())
						o.Expect(ingress.Conditions).NotTo(o.BeEmpty())
						o.Expect(cond.LastTransitionTime).NotTo(o.BeNil())
						o.Expect(cond.Status).To(o.Equal(corev1.ConditionTrue))
					}
				}
				// Wait for both conditions to be on all routes.
				if unservableCondition != numOfRoutes || admittedCondition != numOfRoutes {
					e2e.Logf("waiting for %d conditions for %q, got UnservableInFutureVersions=%d and Admitted=%d", numOfRoutes, routerName, unservableCondition, admittedCondition)
					return false, nil
				}
				outputIngress(routes.Items...)
				return true, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred())

			// Start recording updates BEFORE the second router that removes the conditions gets created,
			// so we capture all the updates.
			err, stopRecordingRouteUpdates, updateCountCh := startRecordingRouteStatusUpdates(client, routerName, "")
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("deploying a scaled out namespace scoped router that removes the UnservableInFutureVersions condition")
			rsRemove, err := oc.KubeClient().AppsV1().ReplicaSets(ns).Create(
				context.Background(),
				scaledRouter(
					"router-remove-condition",
					routerImage,
					[]string{
						"-v=5",
						fmt.Sprintf("--namespace=%s", ns),
						// Make resync interval high to avoid contention flushes.
						"--resync-interval=24h",
						fmt.Sprintf("--name=%s", routerName),
						"--debug-upgrade-validation-force-remove-condition",
					},
				),
				metav1.CreateOptions{},
			)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = waitForReadyReplicaSet(oc.KubeClient(), ns, rsRemove.Name)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that we stop writing conflicts rapidly")
			err, writes := waitForRouteStatusUpdates(stopRecordingRouteUpdates, updateCountCh, 30*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Ideally, we expect at least 20 writes for 20 routes, as the router-add-condition routers already consider
			// all routes a candidate for contention. When router-remove-condition begins to remove these conditions,
			// the router-add-condition routers should immediately consider each route as contended and won't attempt to
			// add the condition back. However, a few additional conflicting writes might occur if the contention
			// tracker is late in detecting route writes. Therefore, we generously allow for up to 50 writes to
			// accommodate these discrepancies.
			o.Expect(writes).To(o.BeNumerically(">=", numOfRoutes))
			o.Expect(writes).To(o.BeNumerically("<=", 50))

			g.By("toggling a single route's status condition")

			// Start recording updates BEFORE the route gets modified, so we capture all the updates.
			err, stopRecordingRouteUpdates, updateCountCh = startRecordingRouteStatusUpdates(client, routerName, "9")
			o.Expect(err).NotTo(o.HaveOccurred())

			// Though it is highly likely that the router-remove-conditions won the conflict and the condition is
			// removed, we will be safe and not make that assumption. We will add or remove the condition based on its
			// presence.
			route9, err := client.Get(context.Background(), "9", metav1.GetOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
			route9Ingress := findIngress(route9, routerName)
			if cond := findIngressCondition(route9Ingress, routev1.RouteUnservableInFutureVersions); cond != nil {
				e2e.Logf("removing %q condition from route %q", routev1.RouteUnservableInFutureVersions, route9.Name)
				removeIngressCondition(route9Ingress, routev1.RouteUnservableInFutureVersions)
			} else {
				e2e.Logf("adding %q condition to route %q", routev1.RouteUnservableInFutureVersions, route9.Name)
				cond := routev1.RouteIngressCondition{
					Type:    routev1.RouteUnservableInFutureVersions,
					Status:  corev1.ConditionFalse,
					Message: "foo",
					Reason:  "foo",
				}
				route9Ingress.Conditions = append(route9Ingress.Conditions, cond)
			}

			route9, err = client.UpdateStatus(context.Background(), route9, metav1.UpdateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("verifying that only get a few updates")
			err, writes = waitForRouteStatusUpdates(stopRecordingRouteUpdates, updateCountCh, 15*time.Second)
			o.Expect(err).NotTo(o.HaveOccurred())
			// Ideally, this should be 1 write. If we are adding the status condition, then the router-remove-condition
			// routers should now consider the route contended. If we are removing the status condition, then the
			// router-add-conditions should already consider the route contended and/or have reach max contentions.
			// Though its very unlikely, we allow up to 5 writes for discrepancies in slow contention tracking.
			o.Expect(writes).To(o.BeNumerically(">=", 1))
			o.Expect(writes).To(o.BeNumerically("<=", 5))
		})
	})
})

// waitForRouteStatusUpdates waits for an observation time, then calls the context.CancelFunc,
// and receives the update count from the provided channel.
func waitForRouteStatusUpdates(stopRecordingRouteUpdates context.CancelFunc, updateCountCh <-chan int, observeTime time.Duration) (error, int) {
	// Wait for the observation.
	time.AfterFunc(observeTime, func() { stopRecordingRouteUpdates() })

	// Set a timeout for receiving the updateCount.
	timeout := time.After(1 * time.Minute)

	select {
	case updates := <-updateCountCh:
		e2e.Logf("recorded %d route updates", updates)
		return nil, updates
	case <-timeout:
		return fmt.Errorf("timeout waiting for the update count to be received"), 0
	}
}

// startRecordingRouteStatusUpdates starts an informer in a separate go routine that monitors route status updates
// for a specific routerName. The informer can be stopped with the returned context.CancelFunc. The returned channel
// receives counts of route status updates. Updates can be filtered by a routeNameMatch, if specified.
func startRecordingRouteStatusUpdates(client v1.RouteInterface, routerName string, routeNameMatch string) (error, context.CancelFunc, <-chan int) {
	lw := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			return client.List(context.Background(), options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			return client.Watch(context.Background(), options)
		},
	}

	updateCount := 0
	informer := cache.NewSharedIndexInformer(lw, &routev1.Route{}, 0, nil)
	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(oldObj, obj interface{}) {
			oldRoute, ok := oldObj.(*routev1.Route)
			if !ok {
				return
			}
			route, ok := obj.(*routev1.Route)
			if !ok {
				return
			}
			if routeNameMatch != "" {
				if route.Name != routeNameMatch {
					return
				}
			}
			oldRouteIngress := findIngress(oldRoute, routerName)
			routeIngress := findIngress(route, routerName)

			if diff := cmp.Diff(oldRouteIngress, routeIngress); diff != "" {
				updateCount++
				e2e.Logf("route ingress status updated, router: %s, write count: %d, diff: %s", routerName, updateCount, diff)
			} else {
				diff := cmp.Diff(oldRoute, route)
				e2e.Logf("not counting route update because route ingress status is the same, route diff: %s", diff)
			}
		},
	})
	if err != nil {
		return err, nil, nil
	}

	ctx, stopRecordingRouteUpdates := context.WithCancel(context.Background())
	updateCountCh := make(chan int)

	// Start the informer and handle context cancellation.
	go func() {
		informer.Run(ctx.Done())
		updateCountCh <- updateCount
		close(updateCountCh)
	}()

	return nil, stopRecordingRouteUpdates, updateCountCh
}

// createTestRoutes creates test routes with the name as the index number
// and returns errors if not successful.
func createTestRoutes(client v1.RouteInterface, numOfRoutes int) error {
	var errs []error
	for i := 0; i < numOfRoutes; i++ {
		_, err := client.Create(context.Background(), &routev1.Route{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("%d", i),
			},
			Spec: routev1.RouteSpec{
				To: routev1.RouteTargetReference{Name: "test"},
				Port: &routev1.RoutePort{
					TargetPort: intstr.FromInt(8080),
				},
			},
		}, metav1.CreateOptions{})
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create route %d: %w", i, err))
		}
	}
	if len(errs) > 0 {
		return fmt.Errorf("multiple errors occurred: %v", errs)
	}
	return nil
}

func findIngress(route *routev1.Route, name string) *routev1.RouteIngress {
	for i, ingress := range route.Status.Ingress {
		if ingress.RouterName == name {
			return &route.Status.Ingress[i]
		}
	}
	return nil
}

// findIngressCondition locates the first condition that corresponds to the requested type.
func findIngressCondition(ingress *routev1.RouteIngress, t routev1.RouteIngressConditionType) (_ *routev1.RouteIngressCondition) {
	for i := range ingress.Conditions {
		if ingress.Conditions[i].Type == t {
			return &ingress.Conditions[i]
		}
	}
	return nil
}

// removeIngressCondition removes a condition of type t from the ingress conditions
func removeIngressCondition(ingress *routev1.RouteIngress, t routev1.RouteIngressConditionType) {
	for i, v := range ingress.Conditions {
		if v.Type == t {
			ingress.Conditions = append(ingress.Conditions[:i], ingress.Conditions[i+1:]...)
			return
		}
	}
}

func scaledRouter(name, image string, args []string) *appsv1.ReplicaSet {
	one := int64(1)
	scale := int32(3)
	return &appsv1.ReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: appsv1.ReplicaSetSpec{
			Replicas: &scale,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": name},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": name},
					Annotations: map[string]string{
						// The restricted-v2 scc preempts restricted, so we must pin to restricted.
						v2.RequiredSCCAnnotation: "restricted",
					},
				},
				Spec: corev1.PodSpec{
					TerminationGracePeriodSeconds: &one,
					Containers: []corev1.Container{
						{
							Env: []corev1.EnvVar{
								{
									Name: "NAME", ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.name",
										},
									},
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							Name:  "router",
							Image: image,
							Args:  append(args, "--stats-port=1936", "--metrics-type=haproxy"),
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 1936,
									Name:          "stats",
									Protocol:      corev1.ProtocolTCP,
								},
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz/ready",
										Port: intstr.FromInt32(1936),
									},
								},
							},
							SecurityContext: &corev1.SecurityContext{
								// Default is true, but explicitly specified here for clarity.
								AllowPrivilegeEscalation: ptr.To[bool](true),
							},
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
	fmt.Fprintf(w, "NAME\tROUTER\tHOST\tCONDITIONS\tLAST TRANSITION\n")
	for _, route := range routes {
		for _, ingress := range route.Status.Ingress {
			conditions := ""
			for _, condition := range ingress.Conditions {
				conditions += fmt.Sprintf("%s=%s ", condition.Type, condition.Status)
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", route.Name, ingress.RouterName, ingress.Host, conditions, findMostRecentConditionTime(ingress.Conditions))
		}
	}
	w.Flush()
	e2e.Logf("Routes:\n%s", b.String())
}

// findMostRecentConditionTime returns the time of the most recent condition.
func findMostRecentConditionTime(conditions []routev1.RouteIngressCondition) time.Time {
	var recent time.Time
	for j := range conditions {
		if conditions[j].LastTransitionTime != nil && conditions[j].LastTransitionTime.Time.After(recent) {
			recent = conditions[j].LastTransitionTime.Time
		}
	}
	return recent
}

func verifyCommandEquivalent(c clientset.Interface, rs *appsv1.ReplicaSet, cmd string) {
	selector, err := metav1.LabelSelectorAsSelector(rs.Spec.Selector)
	o.Expect(err).NotTo(o.HaveOccurred())
	podList, err := c.CoreV1().Pods(rs.Namespace).List(context.Background(), metav1.ListOptions{LabelSelector: selector.String()})
	o.Expect(err).NotTo(o.HaveOccurred())

	var values map[string]string
	err = wait.PollImmediate(5*time.Second, time.Minute, func() (bool, error) {
		values = make(map[string]string)
		uniques := make(map[string]struct{})
		for _, pod := range podList.Items {
			stdout, err := e2eoutput.RunHostCmdWithRetries(pod.Namespace, pod.Name, cmd, statefulset.StatefulSetPoll, statefulset.StatefulPodTimeout)
			o.Expect(err).NotTo(o.HaveOccurred())
			values[pod.Name] = stdout
			uniques[stdout] = struct{}{}
		}
		return len(uniques) == 1, nil
	})
	for name, stdout := range values {
		stdout = strings.TrimSuffix(stdout, "\n")
		e2e.Logf("%s: %s", name, strings.Join(strings.Split(stdout, "\n"), fmt.Sprintf("\n%s: ", name)))
	}
	o.Expect(err).NotTo(o.HaveOccurred())
}

// waitForReadyReplicaSet waits until the replicaset has all of its replicas ready.
// Waits for longer than the standard e2e method.
func waitForReadyReplicaSet(c clientset.Interface, ns, name string) error {
	err := wait.Poll(3*time.Second, 3*time.Minute, func() (bool, error) {
		rs, err := c.AppsV1().ReplicaSets(ns).Get(context.Background(), name, metav1.GetOptions{})
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
