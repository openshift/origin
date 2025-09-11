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
})
