package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	"github.com/openshift/origin/pkg/drae2e"
	"github.com/openshift/origin/pkg/drae2e/example"
	"github.com/openshift/origin/pkg/drae2e/nvidia"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	"k8s.io/utils/ptr"
)

var _ = g.Describe("[sig-node] DRA [Feature:DynamicResourceAllocation] e2e flow", g.Ordered, func() {
	defer g.GinkgoRecover()
	t := g.GinkgoTB()

	f := framework.NewDefaultFramework("dra")
	var holder *drae2e.Holder = &drae2e.Holder{}

	g.BeforeAll(func() {
		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		framework.ExpectNoError(err)
		clientset, err := kubernetes.NewForConfig(config)
		framework.ExpectNoError(err)

		// setup cert-manager
		ns, err := framework.CreateTestingNS(context.TODO(), "cert-manager", clientset, nil)
		framework.ExpectNoError(err)
		g.DeferCleanup(func(ctx context.Context) {
			// TODO: AfterEach sets ClientSet to nil
			f.ClientSet = clientset
			f.DeleteNamespace(context.TODO(), ns.Name)
		})

		cm := drae2e.NewHelmInstaller(t, drae2e.HelmParameters{
			Namespace:       ns.Name,
			CreateNamespace: false,
			ChartURL:        "oci://quay.io/jetstack/charts/cert-manager",
			ReleaseName:     "cert-manager",
			ChartVersion:    "v1.17.2",
			Values: map[string]interface{}{
				"crds": map[string]interface{}{
					"enabled": true,
				},
			},
		})
		g.By("waiting for cert-manager to install")
		o.Expect(cm.Install()).To(o.Succeed(), "cert-manager install should not fail")

		g.DeferCleanup(func(ctx context.Context) {
			g.By("waiting for cert-manager cleanup to complete")
			o.Expect(cm.Remove()).ToNot(o.HaveOccurred(), "cert-manager cleanup should not fail")
		})

		g.By("waiting for cert-manager to be ready")
		o.Eventually(func() error { return nil }).
			WithTimeout(2*time.Minute).WithPolling(2*time.Second).
			WithContext(context.TODO()).Should(o.BeNil(), "cert-manager deployment should be ready")

		// setup dra example driver
		ns, err = framework.CreateTestingNS(context.TODO(), "dra-example-driver", clientset, nil)
		framework.ExpectNoError(err)
		g.DeferCleanup(func(ctx context.Context) {
			f.ClientSet = clientset
			f.DeleteNamespace(context.TODO(), ns.Name)
			g.By("driver namespace cleanup is complete")
		})

		p := drae2e.HelmParameters{
			Namespace:       ns.Name,
			CreateNamespace: false,
			ChartURL:        "oci://registry.k8s.io/dra-example-driver/charts/dra-example-driver",
			ReleaseName:     "dra-example-driver",
			ChartVersion:    "0.1.3",
			Values: map[string]interface{}{
				"webhook": map[string]interface{}{
					"enabled": false,
				},
			},
		}
		driver := example.NewDriver(t, clientset, p)
		holder.Driver = driver

		o.Expect(driver.Setup()).To(o.Succeed(), "example DRA driver deployment should not fail")
		g.By("the driver setup is complete")

		g.DeferCleanup(func(ctx context.Context) {
			o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "example DRA driver cleanup should not fail")
			g.By("the driver cleanup is complete")
		})

		o.Eventually(driver.Ready).
			WithTimeout(2*time.Minute).WithPolling(2*time.Second).
			WithContext(context.TODO()).Should(o.BeNil(), "driver should be ready")
		g.By("driver is ready")
	})

	g.BeforeEach(func(ctx context.Context) {
		driver := holder.Driver
		// we expect the DeviceClass object that represents this driver
		o.Eventually(func() error {
			client := f.ClientSet.ResourceV1beta1().DeviceClasses()
			_, err := client.Get(ctx, driver.Name(), metav1.GetOptions{})
			return err
		}).WithContext(ctx).WithPolling(time.Second).Should(o.BeNil(), "DeviceClass not seen")
		g.By(fmt.Sprintf("deviceclass: %s is present", driver.Name()))

		// we expect ResourceSlice advertised by this driver
		o.Eventually(func() error {
			client := f.ClientSet.ResourceV1beta1().ResourceSlices()
			_, err := client.List(ctx, metav1.ListOptions{
				FieldSelector: resourceapi.ResourceSliceSelectorDriver + "=" + driver.Name(),
			})
			if err != nil {
				return err
			}
			return nil
		}).WithContext(ctx).WithPolling(time.Second).Should(o.BeNil(), "DeviceClass not seen")
		g.By(fmt.Sprintf("deviceclass: %s is present", driver.Name()))
	})

	g.Context("with example driver", func() {
		commonSpec(f, holder)
	})
})

var _ = g.Describe("[sig-node] DRA [Feature:DynamicResourceAllocation] e2e flow", g.Ordered, func() {
	defer g.GinkgoRecover()
	t := g.GinkgoTB()

	f := framework.NewDefaultFramework("dra")
	var holder *drae2e.Holder = &drae2e.Holder{}

	g.BeforeAll(func() {
		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		framework.ExpectNoError(err)
		clientset, err := kubernetes.NewForConfig(config)
		framework.ExpectNoError(err)

		// setup nvidia driver
		ns, err := framework.CreateTestingNS(context.TODO(), "nvidia-gpu-operator", clientset, nil)
		framework.ExpectNoError(err)
		g.DeferCleanup(func(ctx context.Context) {
			f.ClientSet = clientset
			f.DeleteNamespace(context.TODO(), ns.Name)
			g.By("driver namespace cleanup is complete")
		})

		p := drae2e.HelmParameters{
			Namespace:       ns.Name,
			CreateNamespace: false,
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
		}
		driver := nvidia.NewDriver(t, clientset, p)
		holder.Driver = driver

		g.By("waiting for driver setup to be complete")
		o.Expect(driver.Setup()).To(o.Succeed(), "nvidia gpu operator deployment should not fail")

		g.DeferCleanup(func(ctx context.Context) {
			g.By("waiting for the driver to cleanup")
			o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia gpu operator cleanup should not fail")
		})

		g.By("waiting for the driver to be ready")
		o.Eventually(driver.Ready).
			WithTimeout(2*time.Minute).WithPolling(2*time.Second).
			WithContext(context.TODO()).Should(o.BeNil(), "driver should be ready")
	})

	g.Context("with nvidia driver", func() {
		g.It("containers in a pod share gpu", func() {})
	})
})

func commonSpec(f *framework.Framework, holder *drae2e.Holder) {
	g.It("containers in a pod share gpu", func() {
		driver := holder.Driver
		// one pod, two containers, each asking for shared access to a single GPU
		helper := drae2e.NewHelper(f, driver.Name())

		// external claim
		claim := helper.NewResourceClaimTemplate("single-gpu")

		// pod with two containers
		sharedClaim := "shared-gpu"
		pod := helper.NewPod("pod0")
		ctr0 := helper.NewContainer("ctr0")
		ctr0.Resources.Claims = []corev1.ResourceClaim{{Name: sharedClaim}}
		ctr1 := helper.NewContainer("ctr1")
		ctr1.Resources.Claims = []corev1.ResourceClaim{{Name: sharedClaim}}
		pod.Spec.Containers = append(pod.Spec.Containers, ctr0, ctr1)
		pod.Spec.ResourceClaims = []corev1.PodResourceClaim{
			{
				Name:                      sharedClaim,
				ResourceClaimTemplateName: ptr.To(claim.Name),
			},
		}

		g.By("creating external claim and pod")
		resource := f.ClientSet.ResourceV1beta1()
		claim, err := resource.ResourceClaimTemplates(f.Namespace.Name).Create(context.TODO(), claim, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil())

		core := f.ClientSet.CoreV1()
		pod, err = core.Pods(f.Namespace.Name).Create(context.TODO(), pod, metav1.CreateOptions{})
		o.Expect(err).To(o.BeNil())

		g.By("waiting for pod to be running")
		err = e2epod.WaitForPodRunningInNamespace(context.TODO(), f.ClientSet, pod)
		o.Expect(err).To(o.BeNil())
	})
}
