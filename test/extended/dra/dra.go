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
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/kubernetes/test/e2e/framework"

	nvidia "github.com/NVIDIA/gpu-operator/api/versioned"

	"github.com/google/go-cmp/cmp"
)

const (
	gpuNodeLabel = "gpu.openshift.io/testing"
	nvidiaGPU    = "feature.node.kubernetes.io/pci-10de.present"
	rhcosVersion = "feature.node.kubernetes.io/system-os_release.OSTREE_VERSION"

	gpuOperatorMamespace = "nvidia-gpu-operator"
)

var _ = g.Describe("[sig-node] DRA [Feature:DynamicResourceAllocation] e2e flow", g.Ordered, func() {
	defer g.GinkgoRecover()
	t := g.GinkgoT()

	oc := exutil.NewCLIWithoutNamespace("dra")
	f := oc.KubeFramework()
	f.SkipNamespaceCreation = true

	var (
		clientset *kubernetes.Clientset
		nvidia    *nvidia.Clientset
	)

	g.BeforeAll(func() {
		config, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
		framework.ExpectNoError(err)
		clientset, err = kubernetes.NewForConfig(config)
		framework.ExpectNoError(err)
		nvidia, err = nvidia.NewForConfig(config)
		framework.ExpectNoError(err)
	})

	g.Context("[Driver:example-dra-driver]", func() {
		var driver *drae2eutility.ExampleDRADriver

		// setup cert-manager
		g.BeforeAll(func() {
			namespace := "cert-manager"
			cm := drae2eutility.NewHelmInstaller(g.GinkgoTB(), drae2eutility.HelmParameters{
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
			driver = drae2eutility.NewExampleDRADriver(g.GinkgoTB(), clientset, drae2eutility.HelmParameters{
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
			})

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
			// TODO: set the node
			common := commonSpec{f: f, oc: oc, driver: driver, node: nil, newContainer: drae2eutility.NewContainer}
			common.Test(g.GinkgoT())
		})
	})

	g.Context("[Driver:nvidia-dra-driver]", func() {
		var (
			operator *drae2eutility.GpuOperator
			driver   *drae2eutility.DRADriverGPU
			node     *corev1.Node
		)

		// setup node-feature-discovery
		g.BeforeAll(func() {
			namespace := "node-feature-discovery"
			// the service account associated with the nfd worker should use the privileged SCC
			o.Expect(drae2eutility.UsePrivilegedSCC(clientset, "node-feature-discovery-worker", namespace)).To(o.BeNil())

			nfd := drae2eutility.NewHelmInstaller(g.GinkgoT(), drae2eutility.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://github.com/kubernetes-sigs/node-feature-discovery/releases/download/v0.17.3/node-feature-discovery-chart-0.17.3.tgz",
				Wait:            true,
				ReleaseName:     "node-feature-discovery",
				ChartVersion:    "v0.17.3",
				Values: map[string]interface{}{
					"worker": map[string]interface{}{
						"config": map[string]interface{}{
							"sources": map[string]interface{}{
								"pci": map[string]interface{}{
									"deviceLabelFields": []string{"vendor"},
								},
								"custom": []map[string]interface{}{
									{
										"name": "nvidia-gpu-testing",
										"labels": map[string]interface{}{
											"nvidia.com": true,
											gpuNodeLabel: true,
										},
										"matchFeatures": []map[string]interface{}{
											{
												"feature": "pci.device",
												"matchExpressions": map[string]interface{}{
													"class": map[string]interface{}{
														"op":    "In",
														"value": []string{"0302"},
													},
													"vendor": map[string]interface{}{
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
			o.Expect(nfd.Install()).To(o.Succeed(), "node-feature-discovery install should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up node-feature-discovery")
				// o.Expect(nfd.Remove()).ToNot(o.HaveOccurred(), "node-feature-discovery cleanup should not fail")
			})

			g.By("waiting for nfd labels to show up so a gpu node can be selected for testing")
			o.Eventually(func() error {
				client := clientset.CoreV1().Nodes()
				result, err := client.List(context.Background(), metav1.ListOptions{
					LabelSelector: fmt.Sprintf("%s=true,%s=true", nvidiaGPU, gpuNodeLabel),
				})
				if err != nil || len(result.Items) == 0 {
					return fmt.Errorf("still waiting for node label to show up: %w", err)
				}
				// choose the first node
				node = &result.Items[0]
				return nil
			}).WithPolling(time.Second).WithTimeout(5*time.Minute).Should(o.BeNil(), "node label not seen")
			t.Logf("node=%s will be used for testing", node.Name)
			t.Logf("node labels=%v", node.Labels)

		})

		// driver toolkit
		g.BeforeAll(func() {
			dtk := drae2eutility.NewDriverToolkitProber(oc, clientset)
			osrelease, err := dtk.Probe(t, node)
			o.Expect(err).Should(o.BeNil())
			o.Expect(osrelease.RHCOSVersion).NotTo(o.BeEmpty())

			t.Logf("node=%s os-release: %+v", node.Name, osrelease)
			if _, ok := node.Labels[rhcosVersion]; !ok {
				g.By(fmt.Sprintf("adding label: %q to node: %s", rhcosVersion, node.Name))
				err := drae2eutility.EnsureNodeLabel(clientset, node.Name, rhcosVersion, osrelease.RHCOSVersion)
				o.Expect(err).Should(o.BeNil())
			}
		})

		// setup nvidia-gpu-operator
		g.BeforeAll(func() {
			operator = drae2eutility.NewGPUOperatorInstaller(g.GinkgoTB(), clientset, oc, nvidia, drae2eutility.HelmParameters{
				Namespace:       gpuOperatorMamespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/gpu-operator-v25.3.0.tgz",
				ReleaseName:     "gpu-operator",
				ChartVersion:    "v25.3.0",
				Wait:            true,
				Values: map[string]interface{}{
					"devicePlugin": map[string]interface{}{
						"enabled": false,
					},
					"driver": map[string]interface{}{
						"version": "570.148.08",
					},
					"cdi": map[string]interface{}{
						"enabled": true,
					},
					"toolkit": map[string]interface{}{
						"version": "v1.17.5-ubi8",
					},
					"nfd": map[string]interface{}{
						"enabled": false,
					},
					"platform": map[string]interface{}{
						"openshift": true,
					},
					"operator": map[string]interface{}{
						"use_ocp_driver_toolkit": true,
						"logging": map[string]interface{}{
							"level": "debug",
						},
					},
				},
			})
			g.By("installing nvidia-gpu-operator")
			o.Expect(operator.Install()).To(o.Succeed(), "nvidia-gpu-operator install should not fail")

			g.By("waiting for nvidia-gpu-operator to be ready")
			o.Expect(operator.Ready(node)).To(o.Succeed(), "nvidia-gpu-operator should be ready")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-gpu-operator")
				// o.Expect(operator.Cleanup(context.Background())).ToNot(o.HaveOccurred(), "nvidia-gpu-operator cleanup should not fail")
			})
		})

		// TODO: setup nvidia-dra-driver, the DRA driver is not included in the gpu operator yet
		g.BeforeAll(func() {
			namespace := "nvidia-dra-driver-gpu"
			driver = drae2eutility.NewDRADriverGPU(g.GinkgoTB(), clientset, drae2eutility.HelmParameters{
				Namespace:       namespace,
				CreateNamespace: true,
				ChartURL:        "https://helm.ngc.nvidia.com/nvidia/charts/nvidia-dra-driver-gpu-25.3.0-rc.2.tgz",
				ReleaseName:     "nvidia-dra-driver-gpu",
				ChartVersion:    "25.3.0-rc.2",
				Wait:            true,
				Values: map[string]interface{}{
					"nvidiaDriverRoot":            "/run/nvidia/driver",
					"nvidiaCtkPath":               "/var/usrlocal/nvidia/toolkit/nvidia-ctk", // "usr/local/nvidia/toolkit/nvidia-ctk",
					"gpuResourcesEnabledOverride": true,
					"controller": map[string]interface{}{
						"tolerations": []map[string]interface{}{
							{
								"key":      "node-role.kubernetes.io/master",
								"operator": "Exists",
								"effect":   "NoSchedule",
							},
						},
						"affinity": map[string]interface{}{
							"nodeAffinity": map[string]interface{}{
								"requiredDuringSchedulingIgnoredDuringExecution": map[string]interface{}{
									"nodeSelectorTerms": []map[string]interface{}{
										{
											"matchExpressions": []map[string]interface{}{
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
			o.Expect(driver.Setup()).To(o.Succeed(), "nvidia-dra-driver-gpu deployment should not fail")

			g.DeferCleanup(func(ctx context.Context) {
				g.By("cleaning up nvidia-dra-driver-gpu")
				// o.Expect(driver.Cleanup(ctx)).ToNot(o.HaveOccurred(), "nvidia-dra-driver-gpu cleanup should not fail")
			})

			g.By("waiting for nvidia-dra-driver-gpu to be ready")
			o.Expect(driver.Ready(node)).To(o.Succeed(), "nvidia-dra-driver-gpu should be ready")
		})

		g.BeforeEach(func(ctx context.Context) {
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
		})

		g.It("one pod, one container, asking for 1 distinct GPU", func() {
			common := commonSpec{f: f, oc: oc, driver: driver, node: node, newContainer: drae2eutility.NewNvidiaSMIContainer}
			common.Test(g.GinkgoT())
		})

		g.Context("with static MIG partition", func() {
			g.BeforeAll(func() {
				// we will setup a custom MIG configuration
				config := corev1.ConfigMap{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "dra-e2e-mig-parted-config",
						Namespace: gpuOperatorMamespace,
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
				_, err := clientset.CoreV1().ConfigMaps(gpuOperatorMamespace).Create(context.Background(), &config, metav1.CreateOptions{})
				o.Expect(err).Should(o.BeNil())

				g.By("configuring nvidia-mig-manager")
				patchBytes := `
{
  "spec": {
    "migManager": {
      "config": {
        "name": "gpu-e2e-mig-config"
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
				_, err = nvidia.NvidiaV1().ClusterPolicies().Patch(context.Background(), "cluster-policy", types.MergePatchType, []byte(patchBytes), metav1.PatchOptions{})
				o.Expect(err).Should(o.BeNil())

				product := "nvidia.com/gpu.product"
				if p, ok := node.Labels[product]; !ok || p == "" {
					t.Logf("node: %s label: %s is not set", node.Name, product)

					g.By(fmt.Sprintf("discovering gpu feature on node: %s", node.Name))
					gpuProductName, err := operator.DiscoverGPUProudct(node)
					o.Expect(err).Should(o.BeNil())
					o.Expect(err).ShouldNot(o.BeEmpty())
					t.Logf("node: %s gpu proudct name: %s", node.Name, gpuProductName)
					err = drae2eutility.EnsureNodeLabel(d.clientset, node.Name, product, gpuProductName)
					o.Expect(err).Should(o.BeNil())
				}

				g.By(fmt.Sprintf("waiting for %s to be ready", "nvidia-mig-manager-daemonset"))
				o.Expect(operator.MIGReady).Should(o.BeNil())

				g.By("waiting for MIG devices to show up")
				want := []string{"3g.20gb", "2g.10gb", "1g.5gb"}
				o.Eventually(func() error {
					got, err := operator.DiscoverMIGDevices(node)
					if err != nil {
						return err
					}
					if !cmp.Equal(want, got) {
						return fmt.Errorf("still waiting for MIG devices to show up, want: %v, got: %v", want, got)
					}
					return nil
				}).WithPolling(30*time.Second).WithTimeout(10*time.Minute).Should(o.BeNil(), "nvidia-mig-manage pod should be ready")

				g.By("waiting for DRA driver to publish the MIG devices")
			})

			g.It("one pod, three containers, asking for 3g.20gb, 2g.10gb, and 1g.5gb respectively", func() {

			})
		})
	})
})
