package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	draexample "github.com/openshift/origin/test/extended/dra/example"
	helper "github.com/openshift/origin/test/extended/dra/helper"
	"github.com/openshift/origin/test/extended/dra/nvidia"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	clientgodynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epodutil "k8s.io/kubernetes/test/e2e/framework/pod"
)

const (
	// the test applies this label to the worker node that has been
	// selected, the example driver will be installed on this node
	exampleNodeLabel = "dra.e2e.openshift.io/example"

	// NFD will apply this label to the worker node if
	// an Nvidia GPU is present on the node
	nvidiaGPU = "feature.node.kubernetes.io/pci-10de.present"

	// the test applies this label to the gpu worker node that has been
	// selected, the nvidia dra driver will be installed on this node
	gpuNodeLabel = "dra.e2e.openshift.io/nvidia-dra-driver"

	// this is the node label that bears the RHCOS version
	rhcosVersion = "feature.node.kubernetes.io/system-os_release.OSTREE_VERSION"
)

var _ = g.Describe("[sig-node] [Suite:openshift/dra-gpu-validation] [Feature:DynamicResourceAllocation]", g.Ordered, func() {
	defer g.GinkgoRecover()

	// whether drivers/operators should be uninstalled after the test suite is done
	var removeDriver bool

	// clients used by the setup code inside BeforeAll
	var (
		config    *rest.Config
		clientset *kubernetes.Clientset
		dynamic   *clientgodynamic.DynamicClient
	)

	t := g.GinkgoTB()
	// this framework object used by setup code only
	// TODO: can we avoid BeforeEach, and AfterEach being invoked
	setup := framework.NewDefaultFramework("dra-setup")
	setup.SkipNamespaceCreation = true
	oc := exutil.NewCLIWithFramework(setup)

	g.BeforeAll(func(ctx context.Context) {
		var err error
		config, err = clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		framework.ExpectNoError(err)
		clientset, err = kubernetes.NewForConfig(config)
		framework.ExpectNoError(err)
		dynamic, err = clientgodynamic.NewForConfig(clientgodynamic.ConfigFor(config))
		framework.ExpectNoError(err)

		// TODO: the framework fills these when BeforeEach is invoked
		// we are using this framework object in setup code
		setup.ClientSet = clientset
		setup.DynamicClient = dynamic
	})

	g.Context("[Driver:dra-example-driver]", func() {
		var (
			example *draexample.ExampleDRADriver
			// the node on which the test pod will run
			node *corev1.Node
		)

		// setup cert-manager, it's a dependency if you enable
		// webook for the example dra driver
		g.BeforeAll(func(ctx context.Context) {
			namespace := "cert-manager"
			cm := helper.NewHelmInstaller(g.GinkgoTB(), helper.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				Wait:            true,
				ChartURL:        "oci://quay.io/jetstack/charts/cert-manager",
				ReleaseName:     "cert-manager",
				ChartVersion:    "v1.17.2",
				Values: map[string]any{
					"crds": map[string]any{
						"enabled": true,
					},
				},
			})
			g.By("installing cert-manager")
			o.Expect(cm.Install(ctx)).To(o.Succeed(), "cert-manager install should not fail")

			if removeDriver {
				g.DeferCleanup(func(ctx context.Context) {

					g.By("cleaning up cert-manager")
					o.Expect(cm.Remove(ctx)).ToNot(o.HaveOccurred(), "cert-manager cleanup should not fail")
				})
			}
		})

		// pick a worker node onto which the DRA driver will be
		// installed, and the test pod(s) will run.
		g.BeforeAll(func(ctx context.Context) {
			client := clientset.CoreV1().Nodes()
			result, err := client.List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=", "node-role.kubernetes.io/worker"),
			})
			o.Expect(err).Should(o.BeNil())
			o.Expect(len(result.Items)).To(o.BeNumerically(">=", 1))

			// choose the first worker node
			node = &result.Items[0]

			err = helper.EnsureNodeLabel(ctx, clientset, node.Name, exampleNodeLabel, "")
			o.Expect(err).Should(o.BeNil())
			t.Logf("node=%s has been selected, the test will be exercised on this node", node.Name)
		})

		// setup the example DRA driver
		g.BeforeAll(func(ctx context.Context) {
			namespace := "dra-example-driver"
			o.Expect(helper.UsePrivilegedSCC(ctx, clientset, "dra-example-driver-service-account", namespace)).To(o.BeNil())
			example = draexample.NewExampleDRADriver(g.GinkgoTB(), clientset, helper.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "oci://registry.k8s.io/dra-example-driver/charts/dra-example-driver",
				ReleaseName:     "dra-example-driver",
				ChartVersion:    "0.2.0",
				Values: map[string]any{
					"webhook": map[string]any{
						"enabled": false,
					},
					// the plugin should run on the worker node we selected
					"kubeletPlugin": map[string]any{
						"nodeSelector": map[string]any{
							exampleNodeLabel: "",
						},
					},
				},
			})

			g.By("installing dra-example-driver")
			o.Expect(example.Setup(ctx)).To(o.Succeed(), "dra-example-driver deployment should not fail")

			if removeDriver {
				g.DeferCleanup(func(ctx context.Context) {
					g.By("cleaning up dra-example-driver")
					o.Expect(example.Cleanup(ctx)).ToNot(o.HaveOccurred(), "dra-example-driver cleanup should not fail")
				})
			}

			g.By("waiting for dra-example-driver to be ready")
			o.Eventually(ctx, example.Ready).WithPolling(2*time.Second).Should(o.BeNil(), "dra-example-driver should be ready")
		})

		// initialize the framework object before any of our own BeforeEach func
		var f *framework.Framework = framework.NewDefaultFramework("dra-common")

		g.BeforeEach(func(ctx context.Context) {
			g.By(fmt.Sprintf("waiting for the driver: %s to advertise its resources", example.Class()))
			dc, slices := example.EventuallyPublishResources(ctx, node)
			t.Logf("the driver has published deviceclasses: %s", framework.PrettyPrintJSON(dc))
			t.Logf("the driver has published resourceslices: %s", framework.PrettyPrintJSON(slices))
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func(ctx context.Context) {
			devices, err := example.ListPublishedDevices(ctx, node)
			o.Expect(err).Should(o.BeNil())
			t.Logf("collected advertised devices from the resourceslices: %v", devices)
			o.Expect(len(devices)).To(o.BeNumerically(">=", 1))

			common := commonSpec{
				f:                     f,
				class:                 example.Class(),
				node:                  node,
				deviceNamesAdvertised: devices,
				newContainer: func(name string) corev1.Container {
					return corev1.Container{
						Name:            name,
						Image:           e2epodutil.GetDefaultTestImage(),
						Command:         e2epodutil.GenerateScriptCmd("env && sleep 100000"),
						SecurityContext: e2epodutil.GetRestrictedContainerSecurityContext(),
					}
				},
			}
			common.Test(ctx, g.GinkgoTB())
		})
	})

	// Nvidia GPU driver setup and tests follow
	g.Context("[Driver:dra-nvidia-driver]", func() {
		// the gpu worker node we select where the test pods will run
		var node *corev1.Node

		// setup node-feature-discovery
		g.BeforeAll(func(ctx context.Context) {
			namespace := "node-feature-discovery"
			// the service account associated with the nfd worker should use the privileged SCC
			o.Expect(helper.UsePrivilegedSCC(ctx, clientset, "node-feature-discovery-worker", namespace)).To(o.BeNil())

			nfd := helper.NewHelmInstaller(g.GinkgoTB(), helper.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://github.com/kubernetes-sigs/node-feature-discovery/releases/download/v0.17.3/node-feature-discovery-chart-0.17.3.tgz",
				Wait:            true,
				ReleaseName:     "node-feature-discovery",
				ChartVersion:    "v0.17.3",
				Values:          nvidia.DefaultNFDHelmValues(),
			})
			g.By("installing node-feature-discovery")
			o.Expect(nfd.Install(ctx)).To(o.Succeed(), "node-feature-discovery install should not fail")

			if removeDriver {
				g.DeferCleanup(func(ctx context.Context) {
					g.By("cleaning up node-feature-discovery")
					o.Expect(nfd.Remove(ctx)).ToNot(o.HaveOccurred(), "node-feature-discovery cleanup should not fail")
				})
			}

			g.By("waiting for nfd to label the gpu worker node")
			o.Eventually(ctx, func(ctx context.Context) error {
				result, err := clientset.CoreV1().Nodes().List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=true", nvidiaGPU),
				})
				if err != nil || len(result.Items) == 0 {
					return fmt.Errorf("still waiting for node labels to show up: %w", err)
				}
				// choose the first gpu worker node
				node = &result.Items[0]

				err = helper.EnsureNodeLabel(ctx, clientset, node.Name, gpuNodeLabel, "")
				o.Expect(err).Should(o.BeNil())
				t.Logf("node=%s has been selected, the test will be exercised on this node", node.Name)
				return nil
			}).WithPolling(time.Second).Should(o.BeNil(), "timeout while waiting for the expected node label to show up")

			t.Logf("node=%s has been selected, test will be exercised on this node", node.Name)
			t.Logf("labels=%s", framework.PrettyPrintJSON(node.Labels))
		})

		// we build the nvidia gpu driver using the driver toolkit, let's
		// ensure that the driver toolkit is installed properly
		g.BeforeAll(func(ctx context.Context) {
			dtk := nvidia.NewDriverToolkitProber(oc, clientset)
			osrelease, err := dtk.GetOSReleaseInfo(t, node)
			o.Expect(err).Should(o.BeNil())
			o.Expect(osrelease.RHCOSVersion).NotTo(o.BeEmpty())

			t.Logf("node=%s os-release: %+v", node.Name, osrelease)
			// TODO: nfd does not always label the node with the right RHCOS version
			// let's make sure, the selected gpu node has the right RHCOS
			// version, otherwise the nvidia gpu operator will fail to
			// install the gpu driver
			if _, ok := node.Labels[rhcosVersion]; !ok {
				g.By(fmt.Sprintf("adding label: %q to node: %s", rhcosVersion, node.Name))
				err := helper.EnsureNodeLabel(ctx, clientset, node.Name, rhcosVersion, osrelease.RHCOSVersion)
				o.Expect(err).Should(o.BeNil())
			}
		})

		var operator *nvidia.GpuOperator
		// setup nvidia gpu operator
		g.BeforeAll(func(ctx context.Context) {
			parameters := helper.HelmParameters{
				Namespace:       "nvidia-gpu-operator",
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/gpu-operator-v25.3.2.tgz",
				ReleaseName:     "gpu-operator",
				ChartVersion:    "v25.3.2",
				Wait:            true,
				Values:          nvidia.DefaultGPUOperatorHelmValues(),
			}
			operator = nvidia.NewGPUOperatorInstaller(g.GinkgoTB(), clientset, setup, parameters)
			g.By("installing nvidia-gpu-operator")
			o.Expect(operator.Install(ctx)).To(o.Succeed(), "nvidia-gpu-operator install should not fail")

			g.By("waiting for nvidia-gpu-operator to be ready")
			o.Expect(operator.Ready(ctx, node)).To(o.Succeed(), "nvidia-gpu-operator should be ready")

			if removeDriver {
				g.DeferCleanup(func(ctx context.Context) {
					g.By("cleaning up nvidia-gpu-operator")
					o.Expect(operator.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-gpu-operator cleanup should not fail")
				})
			}
		})

		// setup nvidia-dra-driver-gpu
		// TODO: the DRA driver is not included in the gpu operator yet
		var driver *nvidia.NvidiaDRADriverGPU
		g.BeforeAll(func(ctx context.Context) {
			namespace := "nvidia-dra-driver-gpu"
			// TODO: mps-control-daemon uses the default service account in this namespace
			// and it needs to use the privileged SCC
			o.Expect(helper.UsePrivilegedSCC(ctx, clientset, "default", namespace)).To(o.BeNil())

			driver = nvidia.NewNvidiaDRADriverGPU(g.GinkgoTB(), clientset, helper.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/nvidia-dra-driver-gpu-25.3.1.tgz",
				ReleaseName:     "nvidia-dra-driver-gpu",
				ChartVersion:    "25.3.1",
				Wait:            true,
				Values:          nvidia.DefaultDRADriverHelmValues(),
			})

			g.By("installing nvidia-dra-driver-gpu")
			o.Expect(driver.Setup(ctx)).To(o.Succeed(), "nvidia-dra-driver-gpu deployment should not fail")

			if removeDriver {
				g.DeferCleanup(func(ctx context.Context) {
					g.By("cleaning up nvidia-dra-driver-gpu")
					o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-dra-driver-gpu cleanup should not fail")
				})
			}

			g.By("waiting for nvidia-dra-driver-gpu to be ready")
			o.Expect(driver.Ready(ctx, node)).To(o.Succeed(), "nvidia-dra-driver-gpu should be ready")
		})

		g.BeforeAll(func(ctx context.Context) {
			// TODO: nvidia gpu operator checks if the following
			// label is present on the node as a precondition, before
			// it installs mig-manager on the gpu node, or other
			// components
			product := "nvidia.com/gpu.product"
			if p, ok := node.Labels[product]; !ok || p == "" {
				t.Logf("node: %s label: %s is not set", node.Name, product)

				g.By(fmt.Sprintf("discovering gpu feature on node: %s", node.Name))
				gpuProductName, err := operator.DiscoverGPUProudct(ctx, node)
				o.Expect(err).Should(o.BeNil())
				o.Expect(gpuProductName).ShouldNot(o.BeEmpty())
				t.Logf("node: %s gpu proudct name: %s", node.Name, gpuProductName)

				err = helper.EnsureNodeLabel(ctx, clientset, node.Name, product, gpuProductName)
				o.Expect(err).Should(o.BeNil())
			}
		})

		// initialize the framework object before any of our BeforeEach func
		var f *framework.Framework = framework.NewDefaultFramework("dra-nvidia")

		g.BeforeEach(func(ctx context.Context) {
			g.By(fmt.Sprintf("waiting for the driver: %s to advertise its resources", driver.Class()))
			dc, slices := driver.EventuallyPublishResources(ctx, node)
			t.Logf("the driver has published deviceclasses: %s", framework.PrettyPrintJSON(dc))
			t.Logf("the driver has published resourceslices: %s", framework.PrettyPrintJSON(slices))
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func(ctx context.Context) {
			advertised, err := driver.ListPublishedDevicesFromResourceSlice(ctx, node)
			o.Expect(err).Should(o.BeNil())
			t.Logf("collected advertised devices from the resourceslices: %v", advertised)

			// we want the pod to use a whole gpu
			gpus := advertised.FilterBy(func(gpu nvidia.NvidiaGPU) bool { return gpu.Type == "gpu" }).Names()
			t.Logf("the following whole gpus are available to be used: %v", gpus)
			o.Expect(len(gpus)).To(o.BeNumerically(">=", 1))

			common := commonSpec{
				f:                     f,
				class:                 driver.Class(),
				node:                  node,
				deviceNamesAdvertised: gpus,
				newContainer: func(name string) corev1.Container {
					return corev1.Container{
						Name:    name,
						Image:   "ubuntu:22.04",
						Command: []string{"bash", "-c"},
						Args:    []string{"nvidia-smi -L; trap 'exit 0' TERM; sleep 9999 & wait"},
					}
				},
			}
			common.Test(ctx, g.GinkgoTB())
		})

		g.It("two pods, one container each, asking for 1 distinct GPU", func(ctx context.Context) {
			advertised, err := driver.ListPublishedDevicesFromResourceSlice(ctx, node)
			o.Expect(err).Should(o.BeNil())
			t.Logf("collected advertised devices from the resourceslices: %v", advertised)

			gpus := advertised.FilterBy(func(gpu nvidia.NvidiaGPU) bool { return gpu.Type == "gpu" })
			t.Logf("the following whole gpus are available to be used: %v", gpus)
			o.Expect(len(gpus)).To(o.BeNumerically(">=", 2), "need at least two whole gpus for this test")

			spec := distinctGPUsSpec{
				f:     f,
				class: driver.Class(),
				node:  node,
				gpu0:  gpus[0].UUID,
				gpu1:  gpus[1].UUID,
			}
			spec.Test(ctx, g.GinkgoTB())
		})
	})
})
