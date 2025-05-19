package dra

import (
	"context"
	"fmt"
	"os"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	drae2eutility "github.com/openshift/origin/test/extended/dra/utility"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	resourceapi "k8s.io/api/resource/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	clientgodynamic "k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"

	"github.com/google/go-cmp/cmp"
)

const (
	gpuNodeLabel     = "gpu.openshift.io/testing-gpu"
	exampleNodeLabel = "gpu.openshift.io/testing-example"
	nvidiaGPU        = "feature.node.kubernetes.io/pci-10de.present"
	rhcosVersion     = "feature.node.kubernetes.io/system-os_release.OSTREE_VERSION"

	gpuOperatorNamespace = "nvidia-gpu-operator"
)

var _ = g.Describe("[sig-node] [Suite:openshift/dra-gpu-validation] [Feature:DynamicResourceAllocation]", g.Ordered, func() {
	defer g.GinkgoRecover()

	// clients used by the setup code inside BeforeAll
	var (
		config    *rest.Config
		clientset *kubernetes.Clientset
		dynamic   *clientgodynamic.DynamicClient
	)

	t := g.GinkgoT()
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
			driver *drae2eutility.ExampleDRADriver
			// the node on which the test pod will run
			node *corev1.Node
		)

		// setup cert-manager, it's a dependency if you enable
		// webook for the example dra driver
		g.BeforeAll(func(ctx context.Context) {
			namespace := "cert-manager"
			cm := drae2eutility.NewHelmInstaller(g.GinkgoTB(), drae2eutility.HelmParameters{
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

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up cert-manager")
				o.Expect(cm.Remove(ctx)).ToNot(o.HaveOccurred(), "cert-manager cleanup should not fail")
			})
		})

		// pick a worker node onto which the DRA driver will be
		// installed, and the test pod will run.
		g.BeforeAll(func(ctx context.Context) {
			client := clientset.CoreV1().Nodes()
			result, err := client.List(ctx, metav1.ListOptions{
				LabelSelector: fmt.Sprintf("%s=", "node-role.kubernetes.io/worker"),
			})
			o.Expect(err).Should(o.BeNil())
			o.Expect(len(result.Items)).To(o.BeNumerically(">=", 1))

			// choose the first worker node
			node = &result.Items[0]

			err = drae2eutility.EnsureNodeLabel(ctx, clientset, node.Name, exampleNodeLabel, "")
			o.Expect(err).Should(o.BeNil())
			t.Logf("node=%s has been selected, the test will be exercised on this node", node.Name)
		})

		// setup example DRA driver
		g.BeforeAll(func(ctx context.Context) {
			namespace := "dra-example-driver"
			o.Expect(drae2eutility.UsePrivilegedSCC(ctx, clientset, "dra-example-driver-service-account", namespace)).To(o.BeNil())
			driver = drae2eutility.NewExampleDRADriver(g.GinkgoTB(), clientset, drae2eutility.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "oci://registry.k8s.io/dra-example-driver/charts/dra-example-driver",
				ReleaseName:     "dra-example-driver",
				ChartVersion:    "0.1.3",
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
			o.Expect(driver.Setup(ctx)).To(o.Succeed(), "dra-example-driver deployment should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up dra-example-driver")
				o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "dra-example-driver cleanup should not fail")
			})

			g.By("waiting for dra-example-driver to be ready")
			o.Eventually(ctx, driver.Ready).WithPolling(2*time.Second).Should(o.BeNil(), "dra-example-driver should be ready")
		})

		var (
			// initialize the framework object before any of our own BeforeEach func
			f *framework.Framework = framework.NewDefaultFramework("dra-common")
			// we save the devices advertised by the dra driver
			deviceNamesAdvertised = []string{}
		)

		g.BeforeEach(func(ctx context.Context) {
			class := driver.DeviceClassName()
			g.By(fmt.Sprintf("waiting for the driver: %s to advertise its resources", class))
			var (
				dc     *resourceapi.DeviceClass
				slices []resourceapi.ResourceSlice
			)
			o.Eventually(ctx, func(ctx context.Context) error {
				var err error
				dc, slices, err = driver.GetPublishedResources(ctx, node)
				return err
			}).WithPolling(time.Second).Should(o.BeNil(), "timeout while waiting for the driver to advertise its resources")

			t.Logf("the driver has published deviceclasses: %s", framework.PrettyPrintJSON(dc))
			t.Logf("the driver has published resourceslices: %s", framework.PrettyPrintJSON(slices))

			deviceNamesAdvertised = drae2eutility.CollectDevices(slices)
			t.Logf("collected advertised devices from the resourceslices: %v", deviceNamesAdvertised)
			o.Expect(len(deviceNamesAdvertised)).To(o.BeNumerically(">=", 1))
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func(ctx context.Context) {
			common := commonSpec{
				f:                     f,
				class:                 driver.DeviceClassName(),
				node:                  node,
				deviceNamesAdvertised: deviceNamesAdvertised,
				newContainer:          drae2eutility.NewContainer,
			}
			common.Test(ctx, g.GinkgoT())
		})
	})

	g.Context("[Driver:dra-nvidia-driver]", func() {
		var (
			operator *drae2eutility.GpuOperator
			driver   *drae2eutility.NvidiaDRADriverGPU
			node     *corev1.Node
		)

		// setup node-feature-discovery
		g.BeforeAll(func(ctx context.Context) {
			namespace := "node-feature-discovery"
			// the service account associated with the nfd worker should use the privileged SCC
			o.Expect(drae2eutility.UsePrivilegedSCC(ctx, clientset, "node-feature-discovery-worker", namespace)).To(o.BeNil())

			nfd := drae2eutility.NewHelmInstaller(g.GinkgoT(), drae2eutility.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://github.com/kubernetes-sigs/node-feature-discovery/releases/download/v0.17.3/node-feature-discovery-chart-0.17.3.tgz",
				Wait:            true,
				ReleaseName:     "node-feature-discovery",
				ChartVersion:    "v0.17.3",
				Values: map[string]any{
					"worker": map[string]any{
						"config": map[string]any{
							"sources": map[string]any{
								"pci": map[string]any{
									"deviceLabelFields": []string{"vendor"},
								},
								"custom": []map[string]any{
									{
										"name": "nvidia-gpu-testing",
										"labels": map[string]any{
											"nvidia.com": true,
											gpuNodeLabel: "",
										},
										"matchFeatures": []map[string]any{
											{
												"feature": "pci.device",
												"matchExpressions": map[string]any{
													"class": map[string]any{
														"op":    "In",
														"value": []string{"0302"},
													},
													"vendor": map[string]any{
														"op":    "In",
														"value": []string{"10de"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			})
			g.By("installing node-feature-discovery")
			o.Expect(nfd.Install(ctx)).To(o.Succeed(), "node-feature-discovery install should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up node-feature-discovery")
				o.Expect(nfd.Remove(ctx)).ToNot(o.HaveOccurred(), "node-feature-discovery cleanup should not fail")
			})

			g.By("waiting for nfd to label the gpu worker node")
			o.Eventually(ctx, func(ctx context.Context) error {
				client := clientset.CoreV1().Nodes()
				result, err := client.List(ctx, metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=true,%s=", nvidiaGPU, gpuNodeLabel),
				})
				if err != nil || len(result.Items) == 0 {
					return fmt.Errorf("still waiting for node labels to show up: %w", err)
				}
				// choose the first gpu worker node
				node = &result.Items[0]
				return nil
			}).WithPolling(time.Second).Should(o.BeNil(), "timeout while waiting for the expected node label to show up")

			t.Logf("node=%s has been selected, test will be exercised on this node", node.Name)
			t.Logf("labels=%s", framework.PrettyPrintJSON(node.Labels))
		})

		// we build the nvidia gpu driver using the driver toolkit, let's
		// ensure that the driver toolkit is installed properly
		g.BeforeAll(func(ctx context.Context) {
			dtk := drae2eutility.NewDriverToolkitProber(oc, clientset)
			osrelease, err := dtk.GetOSReleaseInfo(t, node)
			o.Expect(err).Should(o.BeNil())
			o.Expect(osrelease.RHCOSVersion).NotTo(o.BeEmpty())

			t.Logf("node=%s os-release: %+v", node.Name, osrelease)
			// nfd does not always label the node with the right RHCOS version
			// let's make sure, the selected gpu node has the right RHCOS
			// version, otherwise the nvidia gpu operator will fail to
			// install the gpu driver
			if _, ok := node.Labels[rhcosVersion]; !ok {
				g.By(fmt.Sprintf("adding label: %q to node: %s", rhcosVersion, node.Name))
				err := drae2eutility.EnsureNodeLabel(ctx, clientset, node.Name, rhcosVersion, osrelease.RHCOSVersion)
				o.Expect(err).Should(o.BeNil())
			}
		})

		// setup nvidia gpu operator
		g.BeforeAll(func(ctx context.Context) {
			parameters := drae2eutility.HelmParameters{
				Namespace:       gpuOperatorNamespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/gpu-operator-v25.3.0.tgz",
				ReleaseName:     "gpu-operator",
				ChartVersion:    "v25.3.0",
				Wait:            true,
				Values: map[string]any{
					"devicePlugin": map[string]any{
						// we will use nvidia DRA driver, so disable the plugin
						"enabled": false,
					},
					"driver": map[string]any{
						"version": "570.148.08",
					},
					"cdi": map[string]any{
						"enabled": true,
					},
					"toolkit": map[string]any{
						"version": "v1.17.5-ubi8",
					},
					"nfd": map[string]any{
						// we hav already installed nfd with custom configuration
						"enabled": false,
					},
					"platform": map[string]any{
						"openshift": true,
					},
					"operator": map[string]any{
						"use_ocp_driver_toolkit": true,
						"logging": map[string]any{
							"level": "debug",
						},
					},
				},
			}
			operator = drae2eutility.NewGPUOperatorInstaller(g.GinkgoTB(), clientset, setup, parameters)
			g.By("installing nvidia-gpu-operator")
			o.Expect(operator.Install(ctx)).To(o.Succeed(), "nvidia-gpu-operator install should not fail")

			g.By("waiting for nvidia-gpu-operator to be ready")
			o.Expect(operator.Ready(ctx, node)).To(o.Succeed(), "nvidia-gpu-operator should be ready")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-gpu-operator")
				o.Expect(operator.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-gpu-operator cleanup should not fail")
			})
		})

		// setup nvidia-dra-driver-gpu
		// TODO: the DRA driver is not included in the gpu operator yet
		g.BeforeAll(func(ctx context.Context) {
			namespace := "nvidia-dra-driver-gpu"
			// TODO: mps-control-daemon uses the default service account in this namespace
			// and it needs to use the privileged SCC
			o.Expect(drae2eutility.UsePrivilegedSCC(ctx, clientset, "default", namespace)).To(o.BeNil())

			driver = drae2eutility.NewNvidiaDRADriverGPU(g.GinkgoTB(), clientset, drae2eutility.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/nvidia-dra-driver-gpu-25.3.0-rc.4.tgz",
				ReleaseName:     "nvidia-dra-driver-gpu",
				ChartVersion:    "25.3.0-rc.4",
				Wait:            true,
				Values: map[string]any{
					"nvidiaDriverRoot": "/run/nvidia/driver",
					// Now a diffent binary is used called nvidia-cdi-hook that is installed by the DRA driver itself.\nThis renders the need for passing this user-defined flag obsolete.
					// "nvidiaCtkPath":               "/var/usrlocal/nvidia/toolkit/nvidia-ctk", // "usr/local/nvidia/toolkit/nvidia-ctk",
					"gpuResourcesEnabledOverride": true,
					// the controller can run on the master node
					"controller": map[string]any{
						"tolerations": []map[string]any{
							{
								"key":      "node-role.kubernetes.io/master",
								"operator": "Exists",
								"effect":   "NoSchedule",
							},
						},
						"affinity": map[string]any{
							"nodeAffinity": map[string]any{
								"requiredDuringSchedulingIgnoredDuringExecution": map[string]any{
									"nodeSelectorTerms": []map[string]any{
										{
											"matchExpressions": []map[string]any{
												{
													"key":      "node-role.kubernetes.io/master",
													"operator": "Exists",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			})

			g.By("installing nvidia-dra-driver-gpu")
			o.Expect(driver.Setup(ctx)).To(o.Succeed(), "nvidia-dra-driver-gpu deployment should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-dra-driver-gpu")
				o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-dra-driver-gpu cleanup should not fail")
			})

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

				err = drae2eutility.EnsureNodeLabel(ctx, clientset, node.Name, product, gpuProductName)
				o.Expect(err).Should(o.BeNil())
			}
		})

		var (
			// initialize the framework object before any of our BeforeEach func
			f *framework.Framework = framework.NewDefaultFramework("dra-nvidia")
			// we save the devices advertised by the dra driver
			deviceNamesAdvertised = []string{}
		)

		g.BeforeEach(func(ctx context.Context) {
			class := driver.DeviceClassName()
			g.By(fmt.Sprintf("waiting for the driver: %s to advertise its resources", class))
			var (
				dc     *resourceapi.DeviceClass
				slices []resourceapi.ResourceSlice
			)
			o.Eventually(ctx, func(ctx context.Context) error {
				var err error
				dc, slices, err = driver.GetPublishedResources(ctx, node)
				return err
			}).WithPolling(time.Second).Should(o.BeNil(), "timeout while waiting for the driver to advertise its resources")
			t.Logf("the driver has published deviceclasses: %s", framework.PrettyPrintJSON(dc))
			t.Logf("the driver has published resourceslices: %s", framework.PrettyPrintJSON(slices))

			deviceNamesAdvertised = drae2eutility.CollectDevices(slices)
			t.Logf("collected advertised devices from the resourceslices: %v", deviceNamesAdvertised)
			o.Expect(len(deviceNamesAdvertised)).To(o.BeNumerically(">=", 1))
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func(ctx context.Context) {
			common := commonSpec{
				f:                     f,
				class:                 driver.DeviceClassName(),
				node:                  node,
				deviceNamesAdvertised: deviceNamesAdvertised,
				newContainer:          drae2eutility.NewNvidiaSMIContainer,
			}
			common.Test(ctx, g.GinkgoT())
		})

		g.Context("[MPS=true]", func() {
			// 			g.BeforeAll(func(ctx context.Context) {
			// 				namespace := "nvidia-device-plugin"
			// 				o.Expect(drae2eutility.UsePrivilegedSCC(ctx, clientset, "nvidia-device-plugin-service-account", namespace)).To(o.BeNil())
			// 				parameters := drae2eutility.HelmParameters{
			// 					Namespace:       namespace,
			// 					CreateNamespace: true,
			// 					// source: https://nvidia.github.io/k8s-device-plugin/index.yaml
			// 					ChartURL:     "https://nvidia.github.io/k8s-device-plugin/stable/nvidia-device-plugin-0.17.2.tgz",
			// 					ReleaseName:  "nvidia-device-plugin",
			// 					ChartVersion: "0.17.2",
			// 					Wait:         true,
			// 					Values: map[string]any{
			// 						"config": map[string]any{
			// 							"default": "mps10",
			// 							"map": map[string]any{
			// 								"mps10": `
			// version: v1
			// sharing:
			//   mps:
			//     renameByDefault: true
			//     resources:
			//     - name: nvidia.com/gpu
			//       replicas: 10
			// `,
			// 							},
			// 						},
			// 					},
			// 				}

			// 				dplugin := drae2eutility.NewHelmInstaller(g.GinkgoT(), parameters)
			// 				g.By("installing nvidia-device-plugin")
			// 				o.Expect(dplugin.Install(ctx)).To(o.Succeed(), "nvidia-device-plugin install should not fail")

			// 				g.DeferCleanup(func(ctx context.Context) {
			// 					g.By("cleaning up nvidia-device-plugin")
			// 					o.Expect(operator.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-device-plugin cleanup should not fail")
			// 				})
			// 			})

			g.It("one pod, 4 containers, with time slice and mps", func(ctx context.Context) {
				common := gpuTimeSlicingAndMPSWithCUDASpec{
					f:            f,
					class:        driver.DeviceClassName(),
					node:         node,
					newContainer: drae2eutility.NewCUDASampleContainer,
				}
				common.Test(ctx, g.GinkgoT())
				o.Expect(operator.LogNvidiSMIOutput(ctx, node)).Should(o.BeNil())
			})

			g.It("one pod, 2 containers share GPU using MPS", func(ctx context.Context) {
				mpsShared := mpsWithCUDASpec{
					t:            g.GinkgoTB(),
					f:            f,
					class:        driver.DeviceClassName(),
					node:         node,
					newContainer: drae2eutility.NewCUDASampleContainer,
				}
				mpsShared.Test(ctx, g.GinkgoT())
				o.Expect(operator.LogNvidiSMIOutput(ctx, node)).Should(o.BeNil())
			})
		})

		g.Context("[MIGEnabled=true]", func() {
			g.BeforeAll(func(ctx context.Context) {
				// we will use a custom MIG configuration
				config := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dra-e2e-mig-parted-config",
						Namespace: gpuOperatorNamespace,
					},
					Data: map[string]string{
						"config.yaml": `
    version: v1
    mig-configs:
      all-disabled:
        - devices: all
          mig-enabled: false

      gpu-e2e:
        - devices: all
          mig-enabled: true
          mig-devices:
            "3g.20gb": 1
            "2g.10gb": 1
            "1g.5gb": 1
`},
				}
				o.Expect(drae2eutility.EnsureConfigMap(ctx, clientset, &config)).Should(o.BeNil())

				g.By("configuring nvidia-mig-manager")
				patchBytes := `
{
  "spec": {
    "mig": {
      "strategy": "mixed"
    },
    "migManager": {
      "config": {
        "default": "all-disabled",
        "name": "dra-e2e-mig-parted-config"
      },
      "env": [
        {
          "name": "WITH_REBOOT",
          "value": "true"
        }
      ]
    },
    "validator": {
      "cuda": {
        "env": [
          {
            "name": "WITH_WORKLOAD",
            "value": "false"
          }
        ]
      }
    }
  }
}
`
				resource := dynamic.Resource(schema.GroupVersionResource{Group: "nvidia.com", Version: "v1", Resource: "clusterpolicies"})
				policy, err := resource.Patch(ctx, "cluster-policy", types.MergePatchType, []byte(patchBytes), metav1.PatchOptions{})
				o.Expect(err).Should(o.BeNil())
				t.Logf("gpu operator cluster policy: \n%s\n", framework.PrettyPrintJSON(policy))

				g.By(fmt.Sprintf("waiting for nvidia-mig-manager to be ready"))
				o.Expect(operator.MIGManagerReady(ctx, node)).Should(o.BeNil())
			})

			g.It("one pod, three containers, asking for 3g.20gb, 2g.10gb, and 1g.5gb respectively", func(ctx context.Context) {
				var (
					// MIG devices we want to setup
					migDevicesWant = []string{"3g.20gb", "2g.10gb", "1g.5gb"}
					// MIG device names advertised in the resourceslice object
					migDeviceNamesAdvertised = []string{}
				)
				// apply our desired MIG configuration
				g.By(fmt.Sprintf("labeling node: %s for nvidia.com/mig.config: %s", node.Name, "gpu-e2e"))
				err := drae2eutility.EnsureNodeLabel(ctx, clientset, node.Name, "nvidia.com/mig.config", "gpu-e2e")
				o.Expect(err).Should(o.BeNil())

				g.By("waiting for the expected MIG devices to show up")
				o.Eventually(ctx, func(ctx context.Context) error {
					got, err := operator.DiscoverMIGDevices(ctx, node)
					if err != nil {
						return err
					}
					if !cmp.Equal(migDevicesWant, got) {
						return fmt.Errorf("still waiting for MIG devices to show up, want: %v, got: %v", migDevicesWant, got)
					}
					return nil
				}).WithPolling(10*time.Second).Should(o.BeNil(), "timeout waiting for expected mig devices")
				t.Logf("mig devices %v are ready to be used", migDevicesWant)

				// TODO: DRA driver does not pick up the MIG slices, restarting the plugin seems to do the trick
				o.Expect(driver.RemovePluginFromNode(node)).Should(o.BeNil())
				g.By("waiting for nvidia-dra-driver-gpu to be ready")
				o.Expect(driver.Ready(ctx, node)).To(o.Succeed(), "nvidia-dra-driver-gpu should be ready")

				class := driver.DeviceClassName()
				g.By(fmt.Sprintf("waiting for the driver: %s to advertise its resources", class))
				var (
					dc     *resourceapi.DeviceClass
					slices []resourceapi.ResourceSlice
				)
				o.Eventually(ctx, func(ctx context.Context) error {
					var err error
					dc, slices, err = driver.GetPublishedResources(ctx, node)
					return err
				}).WithPolling(time.Second).Should(o.BeNil(), "timeout while waiting for the driver to advertise its resources")
				t.Logf("the driver has published deviceclasses: %s", framework.PrettyPrintJSON(dc))
				t.Logf("the driver has published resourceslices: %s", framework.PrettyPrintJSON(slices))

				migDeviceNamesAdvertised = drae2eutility.CollectDevices(slices)
				t.Logf("collected advertised devices from the resourceslices: %v", migDeviceNamesAdvertised)
				// MIG devices plus the default gpu-0 device
				o.Expect(len(migDeviceNamesAdvertised)).Should(o.Equal(len(migDevicesWant) + 1))

				mig := gpuMIGSpec{
					f:                     f,
					class:                 driver.DeviceClassName(),
					node:                  node,
					deviceNamesAdvertised: migDeviceNamesAdvertised,
					devices:               migDevicesWant,
					newContainer:          drae2eutility.NewNvidiaSMIContainer,
				}
				mig.Test(ctx, g.GinkgoT())
			})
		})
	})
})
