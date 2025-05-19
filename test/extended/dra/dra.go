package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/pkg/drae2e"
	draexample "github.com/openshift/origin/pkg/drae2e/example"
	"github.com/openshift/origin/pkg/drae2e/nvidia"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	"k8s.io/utils/ptr"
)

type holder struct {
	f         *framework.Framework
	oc        *exutil.CLI
	clientset *kubernetes.Clientset
	driver    drae2e.Driver
}

var _ = g.Describe("[sig-node] DRA [Feature:DynamicResourceAllocation] e2e flow", g.Ordered, func() {
	defer g.GinkgoRecover()

	oc := exutil.NewCLIWithoutNamespace("dra")
	f := oc.KubeFramework()
	f.SkipNamespaceCreation = true

	var clientset *kubernetes.Clientset
	g.BeforeAll(func() {
		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		framework.ExpectNoError(err)
		clientset, err = kubernetes.NewForConfig(config)
		framework.ExpectNoError(err)
	})

	g.Context("[Driver:example-dra-driver]", func() {
		var driver *draexample.Driver

		// setup cert-manager
		g.BeforeAll(func() {
			namespace := "cert-manager"
			cm := drae2e.NewHelmInstaller(g.GinkgoTB(), drae2e.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "oci://quay.io/jetstack/charts/cert-manager",
				ReleaseName:     "cert-manager",
				ChartVersion:    "v1.17.2",
				Values: map[string]interface{}{
					"crds": map[string]interface{}{
						"enabled": true,
					},
				},
			})
			g.By("installing cert-manager")
			o.Expect(cm.Install()).To(o.Succeed(), "cert-manager install should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up cert-manager")
				o.Expect(cm.Remove()).ToNot(o.HaveOccurred(), "cert-manager cleanup should not fail")
			})

			g.By("waiting for cert-manager to be ready")
			o.Eventually(func() error { return nil }).
				WithTimeout(2*time.Minute).WithPolling(2*time.Second).
				WithContext(context.TODO()).Should(o.BeNil(), "cert-manager deployment should be ready")
		})

		// setup example DRA driver
		g.BeforeAll(func() {
			namespace := "dra-example-driver"
			p := drae2e.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "oci://registry.k8s.io/dra-example-driver/charts/dra-example-driver",
				ReleaseName:     "dra-example-driver",
				ChartVersion:    "0.1.3",
				Values: map[string]interface{}{
					"webhook": map[string]interface{}{
						"enabled": false,
					},
				},
			}
			driver = draexample.NewDriver(g.GinkgoTB(), clientset, p)

			g.By("installing example-dra-driver")
			o.Expect(driver.Setup()).To(o.Succeed(), "example-dra-driver deployment should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up example-dra-driver")
				o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "example-dra-driver cleanup should not fail")
			})

			g.By("waiting for example-dra-driver to be ready")
			o.Eventually(driver.Ready).
				WithTimeout(2*time.Minute).WithPolling(2*time.Second).
				WithContext(context.TODO()).Should(o.BeNil(), "example-dra-driver should be ready")
		})

		g.BeforeEach(func(ctx context.Context) {
			// we expect the DeviceClass object that represents this driver
			g.By(fmt.Sprintf("deviceclass: %s is present", driver.DeviceClassName()))
			o.Eventually(func() error {
				client := f.ClientSet.ResourceV1beta1().DeviceClasses()
				_, err := client.Get(ctx, driver.DeviceClassName(), metav1.GetOptions{})
				return err
			}).WithContext(ctx).WithPolling(time.Second).Should(o.BeNil(), "DeviceClass not seen")

			// we expect ResourceSlice advertised by this driver
			o.Eventually(func() error {
				client := f.ClientSet.ResourceV1beta1().ResourceSlices()
				_, err := client.List(ctx, metav1.ListOptions{
					FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + driver.DeviceClassName(),
				})
				if err != nil {
					return err
				}
				return nil
			}).WithContext(ctx).WithPolling(time.Second).Should(o.BeNil(), "DeviceClass not seen")
			g.By(fmt.Sprintf("deviceclass: %s is present", driver.DeviceClassName()))
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func() {
			common := commonSpec{f: f, driver: driver, newContainer: drae2e.NewContainer}
			common.Test(g.GinkgoT())
		})
	})

	g.Context("[Driver:nvidia-dra-driver]", func() {
		var driver *nvidia.Driver

		// setup node-feature-discovery
		g.BeforeAll(func() {
			namespace := "node-feature-discovery"
			nfd := drae2e.NewHelmInstaller(g.GinkgoT(), drae2e.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://github.com/kubernetes-sigs/node-feature-discovery/releases/download/v0.17.3/node-feature-discovery-chart-0.17.3.tgz",
				ReleaseName:     "node-feature-discovery",
				ChartVersion:    "v0.17.3",
			})
			g.By("installing node-feature-discovery")
			o.Expect(nfd.Install()).To(o.Succeed(), "node-feature-discovery install should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up node-feature-discovery")
				o.Expect(nfd.Remove()).ToNot(o.HaveOccurred(), "nfd cleanup should not fail")
			})
		})

		// setup nvidia-gpu-operator
		g.BeforeAll(func() {
			namespace := "nvidia-gpu-operator"
			operator := drae2e.NewHelmInstaller(g.GinkgoT(), drae2e.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/gpu-operator-v25.3.0.tgz",
				ReleaseName:     "gpu-operator",
				ChartVersion:    "v25.3.0",
				Values: map[string]interface{}{
					"driver": map[string]interface{}{
						"enabled": false,
					},
					"ccManager": map[string]interface{}{
						"enabled": false,
					},
					"cdi": map[string]interface{}{
						"enabled": false,
					},
				},
			})
			g.By("installing nvidia-gpu-operator")
			o.Expect(operator.Install()).To(o.Succeed(), "nvidia-gpu-operator install should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-gpu-operator")
				o.Expect(operator.Remove()).ToNot(o.HaveOccurred(), "nfd cleanup should not fail")
			})
		})

		// TODO: setup nvidia-dra-driver, today the DRA driver is not included in the operator install
		g.BeforeAll(func() {
			namespace := "nvidia-dra-driver-gpu"
			p := drae2e.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
			}
			driver = nvidia.NewDriver(g.GinkgoTB(), clientset, p)

			g.By("installing nvidia-dra-driver")
			o.Expect(driver.Setup()).To(o.Succeed(), "nvidia-dra-driver deployment should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-dra-driver")
				o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-dra-driver cleanup should not fail")
			})

			g.By("waiting for nvidia-dra-driver to be ready")
			o.Eventually(driver.Ready).
				WithTimeout(2*time.Minute).WithPolling(2*time.Second).
				WithContext(context.TODO()).Should(o.BeNil(), "nvidia-dra-driver should be ready")
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func() {
			dradriver := nvidia.NewDriver(g.GinkgoTB(), clientset, drae2e.HelmParameters{})
			common := commonSpec{f: f, driver: dradriver, newContainer: drae2e.NewNvidiaSMIContainer}
			common.Test(g.GinkgoT())
		})
	})
})

type commonSpec struct {
	f            *framework.Framework
	driver       drae2e.Driver
	newContainer func(name string) corev1.Container
}

func (spec commonSpec) Test(t g.GinkgoTInterface) {
	// create a namespace
	ns, err := spec.f.CreateNamespace(context.TODO(), "dra", nil)
	framework.ExpectNoError(err)

	deviceclass := spec.driver.DeviceClassName()
	helper := drae2e.NewHelper(ns.Name, spec.driver.DeviceClassName())

	claim := &resourceapi.ResourceClaimTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns.Name,
			Name:      "single-gpu",
		},
		Spec: resourceapi.ResourceClaimTemplateSpec{
			Spec: resourceapi.ResourceClaimSpec{
				Devices: resourceapi.DeviceClaim{
					Requests: []resourceapi.DeviceRequest{
						{
							Name:            "gpu",
							DeviceClassName: deviceclass,
						},
					},
				},
			},
		},
	}

	// on pod, one container
	pod := helper.NewPod("pod")
	ctr := spec.newContainer("ctr")
	ctr.Resources.Claims = []corev1.ResourceClaim{{Name: "gpu"}}
	pod.Spec.Containers = append(pod.Spec.Containers, ctr)
	pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
		{
			Name:                      "gpu",
			ResourceClaimTemplateName: ptr.To(claim.Name),
		},
	}
	pod.Spec.Tolerations = []corev1.Toleration{
		{
			Key:      "nvidia.com/gpu",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}

	g.By("creating external claim and pod")
	resource := spec.f.ClientSet.ResourceV1beta1()
	claim, err = resource.ResourceClaimTemplates(ns.Name).Create(context.TODO(), claim, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	core := spec.f.ClientSet.CoreV1()
	pod, err = core.Pods(ns.Name).Create(context.TODO(), pod, metav1.CreateOptions{})
	o.Expect(err).To(o.BeNil())

	g.By("waiting for pod to be running")
	err = e2epod.WaitForPodRunningInNamespace(context.TODO(), spec.f.ClientSet, pod)
	o.Expect(err).To(o.BeNil())
}
