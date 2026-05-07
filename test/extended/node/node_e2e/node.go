package node

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/origin/test/extended/imagepolicy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"

	nodeutils "github.com/openshift/origin/test/extended/node"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-node] [Jira:Node/Kubelet] Kubelet, CRI-O, CPU manager", func() {
	var (
		oc             = exutil.NewCLIWithoutNamespace("node")
		nodeE2EBaseDir = exutil.FixturePath("testdata", "node", "node_e2e")
		podDevFuseYAML = filepath.Join(nodeE2EBaseDir, "pod-dev-fuse.yaml")
	)

	// Skip all tests on MicroShift clusters as MachineConfig resources are not available
	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	//author: asahay@redhat.com
	g.It("[OTP] validate KUBELET_LOG_LEVEL", func() {
		var kubeservice string
		var kubelet string
		var err error

		g.By("Polling to check kubelet log level on ready nodes")
		waitErr := wait.Poll(10*time.Second, 1*time.Minute, func() (bool, error) {
			g.By("Getting all node names in the cluster")
			nodeName, nodeErr := oc.AsAdmin().Run("get").Args("nodes", "-o=jsonpath={.items[*].metadata.name}").Output()
			o.Expect(nodeErr).NotTo(o.HaveOccurred())
			e2e.Logf("\nNode Names are %v", nodeName)
			nodes := strings.Fields(nodeName)

			for _, node := range nodes {
				g.By("Checking if node " + node + " is Ready")
				nodeStatus, statusErr := oc.AsAdmin().Run("get").Args("nodes", node, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
				o.Expect(statusErr).NotTo(o.HaveOccurred())
				e2e.Logf("\nNode %s Status is %s\n", node, nodeStatus)

				if nodeStatus == "True" {
					g.By("Checking KUBELET_LOG_LEVEL in kubelet.service on node " + node)
					kubeservice, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "systemctl show kubelet.service | grep KUBELET_LOG_LEVEL")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Checking kubelet process for --v=2 flag on node " + node)
					kubelet, err = nodeutils.ExecOnNodeWithChroot(oc, node, "/bin/bash", "-c", "ps aux | grep [k]ubelet")
					o.Expect(err).NotTo(o.HaveOccurred())

					g.By("Verifying KUBELET_LOG_LEVEL is set and kubelet is running with --v=2")
					if strings.Contains(kubeservice, "KUBELET_LOG_LEVEL") && strings.Contains(kubelet, "--v=2") {
						e2e.Logf("KUBELET_LOG_LEVEL is 2.\n")
						return true, nil
					} else {
						e2e.Logf("KUBELET_LOG_LEVEL is not 2.\n")
						return false, nil
					}
				} else {
					e2e.Logf("\nNode %s is not Ready, Skipping\n", node)
				}
			}
			return false, nil
		})

		if waitErr != nil {
			e2e.Logf("Kubelet Log level is:\n %v\n", kubeservice)
			e2e.Logf("Running Process of kubelet are:\n %v\n", kubelet)
		}
		o.Expect(waitErr).NotTo(o.HaveOccurred(), "KUBELET_LOG_LEVEL is not expected, timed out")
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] validate cgroupv2 is default [OCP-80983]", func() {
		g.By("Check cgroup version on all Ready worker nodes")
		nodeNames, err := oc.AsAdmin().WithoutNamespace().Run("get").Args("nodes", "-l", "node-role.kubernetes.io/worker", "-o=jsonpath={.items[*].metadata.name}").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		workers := strings.Fields(nodeNames)
		o.Expect(workers).NotTo(o.BeEmpty(), "No worker nodes found")

		for _, worker := range workers {
			nodeStatus, err := oc.AsAdmin().Run("get").Args("nodes", worker, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			if nodeStatus != "True" {
				e2e.Logf("Skipping worker node %s (not Ready)", worker)
				continue
			}
			cgroupV, err := nodeutils.ExecOnNodeWithChroot(oc, worker, "/bin/bash", "-c", "stat -c %T -f /sys/fs/cgroup")
			o.Expect(err).NotTo(o.HaveOccurred())
			e2e.Logf("cgroup version on node %s: [%v]", worker, cgroupV)
			o.Expect(cgroupV).To(o.ContainSubstring("cgroup2fs"), "Node %s does not have cgroupv2", worker)
		}

		g.By("Changing cgroup from v2 to v1 should result in error")
		output, err := oc.AsAdmin().WithoutNamespace().Run("patch").Args("nodes.config.openshift.io", "cluster", "-p", `{"spec": {"cgroupMode": "v1"}}`, "--type=merge").Output()
		o.Expect(err).Should(o.HaveOccurred())
		o.Expect(output).To(o.ContainSubstring("spec.cgroupMode: Unsupported value: \"v1\": supported values: \"v2\", \"\""))
	})

	//author: cmaurya@redhat.com
	g.It("[OTP] Allow dev fuse by default in CRI-O [OCP-70987]", func() {
		podName := "pod-devfuse"
		ns := "devfuse-test"

		g.By("Check if the default CRI-O runtime is runc")
		ctrcfgList, err := oc.MachineConfigurationClient().MachineconfigurationV1().ContainerRuntimeConfigs().List(context.Background(), metav1.ListOptions{})
		o.Expect(err).NotTo(o.HaveOccurred())
		for _, cfg := range ctrcfgList.Items {
			if cfg.Spec.ContainerRuntimeConfig != nil && cfg.Spec.ContainerRuntimeConfig.DefaultRuntime == "runc" {
				g.Skip("Skipping: not applicable to runc runtime")
			}
		}

		g.By("Create a test namespace")
		err = oc.AsAdmin().WithoutNamespace().Run("create").Args("namespace", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())
		defer oc.AsAdmin().WithoutNamespace().Run("delete").Args("namespace", ns, "--ignore-not-found").Execute()

		g.By("Create a pod with dev fuse annotation")
		err = oc.AsAdmin().WithoutNamespace().Run("apply").Args("-f", podDevFuseYAML, "-n", ns).Execute()
		o.Expect(err).NotTo(o.HaveOccurred())

		g.By("Wait for pod to be ready")
		err = wait.Poll(5*time.Second, 1*time.Minute, func() (bool, error) {
			status, pollErr := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status.conditions[?(@.type=='Ready')].status}").Output()
			if pollErr != nil {
				e2e.Logf("Error polling pod status: %v", pollErr)
				return false, nil
			}
			return status == "True", nil
		})
		if err != nil {
			podStatus, _ := oc.AsAdmin().WithoutNamespace().Run("get").Args("pod", podName, "-n", ns, "-o=jsonpath={.status}").Output()
			e2e.Logf("Pod status on timeout: %s", podStatus)
		}
		o.Expect(err).NotTo(o.HaveOccurred(), "pod did not become ready")

		g.By("Check /dev/fuse is mounted inside the pod")
		output, err := oc.AsAdmin().WithoutNamespace().Run("exec").Args(podName, "-n", ns, "--", "stat", "/dev/fuse").Output()
		o.Expect(err).NotTo(o.HaveOccurred())
		e2e.Logf("/dev/fuse mount output: %s", output)
		o.Expect(output).To(o.ContainSubstring("fuse"), "dev fuse is not mounted inside pod")
	})
})

// author: asahay@redhat.com
var _ = g.Describe("[sig-node][Suite:openshift/disruptive-longrunning][Disruptive][Serial] ImageTagMirrorSet and ImageDigestMirrorSet", func() {
	var (
		oc  = exutil.NewCLIWithoutNamespace("image-mirror-set")
		ctx = context.Background()
	)

	g.BeforeEach(func() {
		isMicroShift, err := exutil.IsMicroShiftCluster(oc.AdminKubeClient())
		o.Expect(err).NotTo(o.HaveOccurred())
		if isMicroShift {
			g.Skip("Skipping test on MicroShift cluster - MachineConfig resources are not available")
		}
	})

	g.It("[OTP] Create ImageDigestMirrorSet and ImageTagMirrorSet and verify registries.conf [OCP-57401]", func() {
		configClient := oc.AdminConfigClient().ConfigV1()
		suffix := utilrand.String(5)
		idmsName := fmt.Sprintf("digest-mirror-%s", suffix)
		itmsName := fmt.Sprintf("tag-mirror-%s", suffix)

		g.By("Step 1: Create an ImageDigestMirrorSet")
		idms := &configv1.ImageDigestMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: idmsName,
			},
			Spec: configv1.ImageDigestMirrorSetSpec{
				ImageDigestMirrors: []configv1.ImageDigestMirrors{
					{
						Source: "registry.redhat.io/openshift4",
						Mirrors: []configv1.ImageMirror{
							"mirror.example.com/redhat",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
					{
						Source: "registry.redhat.io/rhel8",
						Mirrors: []configv1.ImageMirror{
							"mirror.example.com/rhel8",
						},
						MirrorSourcePolicy: configv1.NeverContactSource,
					},
				},
			},
		}

		initialWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		initialMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		createdIDMS, err := configClient.ImageDigestMirrorSets().Create(ctx, idms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageDigestMirrorSet")
		e2e.Logf("ImageDigestMirrorSet %q created successfully", createdIDMS.Name)

		g.DeferCleanup(func() {
			g.By("Cleanup: Delete IDMS and ITMS resources")
			cleanupWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
			cleanupMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")
			if delErr := configClient.ImageTagMirrorSets().Delete(ctx, itmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageTagMirrorSet: %v", delErr)
			}
			if delErr := configClient.ImageDigestMirrorSets().Delete(ctx, idmsName, metav1.DeleteOptions{}); delErr != nil {
				e2e.Logf("Warning: failed to delete ImageDigestMirrorSet: %v", delErr)
			}
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", cleanupWorkerSpec)
			imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", cleanupMasterSpec)
		})

		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", initialWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", initialMasterSpec)
		e2e.Logf("IDMS MCP rollout complete")

		g.By("Step 2: Create an ImageTagMirrorSet")
		itms := &configv1.ImageTagMirrorSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: itmsName,
			},
			Spec: configv1.ImageTagMirrorSetSpec{
				ImageTagMirrors: []configv1.ImageTagMirrors{
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal",
							"example.com/example/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.AllowContactingSource,
					},
					{
						Source: "registry.access.redhat.com/ubi8/ubi-minimal-1",
						Mirrors: []configv1.ImageMirror{
							"example.io/example/ubi-minimal",
						},
						MirrorSourcePolicy: configv1.NeverContactSource,
					},
				},
			},
		}

		itmsWorkerSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "worker")
		itmsMasterSpec := imagepolicy.GetMCPCurrentSpecConfigName(oc, "master")

		createdITMS, err := configClient.ImageTagMirrorSets().Create(ctx, itms, metav1.CreateOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to create ImageTagMirrorSet")
		e2e.Logf("ImageTagMirrorSet %q created successfully", createdITMS.Name)

		g.By("Step 3: Wait for all nodes to finish rolling out")
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "worker", itmsWorkerSpec)
		imagepolicy.WaitForMCPConfigSpecChangeAndUpdated(oc, "master", itmsMasterSpec)
		e2e.Logf("All MCPs have finished rolling out")

		g.By("Step 4: Verify /etc/containers/registries.conf on a worker node")
		workerNodeName := nodeutils.GetFirstReadyWorkerNode(oc)
		o.Expect(workerNodeName).NotTo(o.BeEmpty(), "no ready worker node found")

		registriesConf, err := nodeutils.ExecOnNodeWithChroot(oc, workerNodeName, "cat", "/etc/containers/registries.conf")
		o.Expect(err).NotTo(o.HaveOccurred(), "failed to read registries.conf from node %s", workerNodeName)
		e2e.Logf("registries.conf content:\n%s", registriesConf)

		g.By("Verify IDMS entries (digest-only mirrors)")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/openshift4"`),
			"registries.conf should contain the IDMS source for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "mirror.example.com/redhat"`),
			"registries.conf should contain the IDMS mirror for openshift4")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "digest-only"`),
			"registries.conf should have pull-from-mirror set to digest-only for IDMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.redhat.io/rhel8"`),
			"registries.conf should contain the IDMS source for rhel8")

		g.By("Verify ITMS entries (tag-only mirrors)")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal"`),
			"registries.conf should contain the ITMS source for ubi-minimal")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "example.io/example/ubi-minimal"`),
			"registries.conf should contain the ITMS mirror location")
		o.Expect(registriesConf).To(o.ContainSubstring(`pull-from-mirror = "tag-only"`),
			"registries.conf should have pull-from-mirror set to tag-only for ITMS mirrors")
		o.Expect(registriesConf).To(o.ContainSubstring(`location = "registry.access.redhat.com/ubi8/ubi-minimal-1"`),
			"registries.conf should contain the ITMS source for ubi-minimal-1")

		g.By("Verify NeverContactSource entries are blocked")
		nodeutils.VerifyRegistryBlocked(registriesConf, "registry.access.redhat.com/ubi8/ubi-minimal-1")
		nodeutils.VerifyRegistryBlocked(registriesConf, "registry.redhat.io/rhel8")
	})
})
