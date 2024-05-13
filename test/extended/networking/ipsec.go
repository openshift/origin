package networking

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	v1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

const (
	// tcpdumpESPFilter can be used to filter out IPsec packets destined to target node.
	tcpdumpESPFilter = "esp and src %s and dst %s"
	// tcpdumpGeneveFilter can be used to filter out Geneve encapsulated packets destined to target node.
	tcpdumpGeneveFilter = "udp port 6081 and src %s and dst %s"
	// tcpdumpICMPFilter can be used to filter out icmp packets destined to target node.
	tcpdumpICMPFilter            = "icmp and src %s and dst %s"
	masterIPsecMachineConfigName = "80-ipsec-master-extensions"
	workerIPSecMachineConfigName = "80-ipsec-worker-extensions"
	ipsecRolloutWaitDuration     = 40 * time.Minute
	ipsecRolloutWaitInterval     = 1 * time.Minute
	nmstateConfigureManifestFile = "nmstate.yaml"
	nsCertMachineConfigFile      = "ipsec-nsconfig-machine-config.yaml"
	nsCertMachineConfigName      = "99-worker-north-south-ipsec-config"
	leftNodeIPsecPolicyName      = "left-node-ipsec-policy"
	rightNodeIPsecPolicyName     = "right-node-ipsec-policy"
	leftNodeIPsecNNCPYaml        = "ipsec-left-node.yaml"
	rightNodeIPsecNNCPYaml       = "ipsec-right-node.yaml"
	ovnNamespace                 = "openshift-ovn-kubernetes"
	ovnIPsecDsName               = "ovn-ipsec-host"
)

var gvrNodeNetworkConfigurationPolicy = schema.GroupVersionResource{Group: "nmstate.io", Version: "v1", Resource: "nodenetworkconfigurationpolicies"}

// TODO: consider bringing in the NNCP api.
var nodeIPsecConfigManifest = `
kind: NodeNetworkConfigurationPolicy
apiVersion: nmstate.io/v1
metadata:
  name: %s
spec:
  nodeSelector:
    kubernetes.io/hostname: %s
  desiredState:
    interfaces:
    - name: hosta_conn
      type: ipsec
      ipv4:
        enabled: true
        dhcp: true
      libreswan:
        leftrsasigkey: '%%cert'
        left: %s
        leftid: '%%fromcert'
        leftcert: %s
        leftmodecfgclient: false
        right: %s
        rightrsasigkey: '%%cert'
        rightid: '%%fromcert'
        rightsubnet: %[5]s/32
        ike: aes_gcm256-sha2_256
        esp: aes_gcm256
        ikev2: insist
        type: transport
`

// properties of nsCertMachineConfigFile.
var (
	// certificate name of the left server.
	leftServerCertName = "left_server"
	// certificate name of the right server.
	rightServerCertName = "right_server"
	// Expiration date for certificates.
	certExpirationDate = time.Date(2034, time.April, 10, 0, 0, 0, 0, time.UTC)
)

type trafficType string

const (
	esp    trafficType = "esp"
	geneve trafficType = "geneve"
	icmp   trafficType = "icmp"
)

// configureIPsecMode helps to rollout specified IPsec Mode on the cluster. If the cluster is already
// configured with specified mode, then this is almost like no-op for the cluster.
func configureIPsecMode(oc *exutil.CLI, ipsecMode v1.IPsecMode) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		if network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig == nil {
			network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig = &v1.IPsecConfig{Mode: ipsecMode}
		} else if network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig.Mode != ipsecMode {
			network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig.Mode = ipsecMode
		} else {
			// No changes to existing mode, return without updating networks.
			return nil
		}
		_, err = oc.AdminOperatorClient().OperatorV1().Networks().Update(context.Background(), network, metav1.UpdateOptions{})
		return err
	})
}

func getIPsecMode(oc *exutil.CLI) (v1.IPsecMode, error) {
	network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return v1.IPsecModeDisabled, err
	}
	conf := network.Spec.DefaultNetwork.OVNKubernetesConfig
	mode := v1.IPsecModeDisabled
	if conf.IPsecConfig != nil {
		if conf.IPsecConfig.Mode != "" {
			mode = conf.IPsecConfig.Mode
		} else {
			mode = v1.IPsecModeFull // Backward compatibility with existing configs
		}
	}
	return mode, nil
}

// ensureIPsecFullEnabled this function ensure IPsec is enabled by making sure ovn-ipsec-host daemonset
// is completely ready on the cluster and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecFullEnabled(oc *exutil.CLI) error {
	return wait.PollUntilContextTimeout(context.Background(), ipsecRolloutWaitInterval,
		ipsecRolloutWaitDuration, true, func(ctx context.Context) (bool, error) {
			done, err := areMachineConfigPoolsReadyWithIPsec(oc)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if !done {
				return false, nil
			}
			done, err = isDaemonSetRunning(oc, ovnNamespace, ovnIPsecDsName)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if done {
				done, err = areClusterOperatorsReady((oc))
				if err != nil && !isConnResetErr(err) {
					return false, err
				}
			}
			return done, nil
		})
}

// ensureIPsecExternalEnabled this function ensures ipsec machine config extension is rolled out
// on all of master and worked nodes and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecExternalEnabled(oc *exutil.CLI) error {
	return wait.PollUntilContextTimeout(context.Background(), ipsecRolloutWaitInterval,
		ipsecRolloutWaitDuration, true, func(ctx context.Context) (bool, error) {
			// Make sure ovn-ipsec-host daemonset is not deployed. When IPsec mode
			// is changed from Full to External mode, then it may take a while to
			// delete daemonset.
			ds, err := getDaemonSet(oc, ovnNamespace, ovnIPsecDsName)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if ds != nil {
				return false, nil
			}
			done, err := areMachineConfigPoolsReadyWithIPsec(oc)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if done {
				done, err = areClusterOperatorsReady((oc))
				if err != nil && !isConnResetErr(err) {
					return false, err
				}
			}
			return done, nil
		})
}

// ensureIPsecDisabled this function ensure IPsec is disabled by making sure ovn-ipsec-host daemonset
// is completely removed from the cluster and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecDisabled(oc *exutil.CLI) error {
	return wait.PollUntilContextTimeout(context.Background(), ipsecRolloutWaitInterval,
		ipsecRolloutWaitDuration, true, func(ctx context.Context) (bool, error) {
			done, err := areMachineConfigPoolsReadyWithoutIPsec(oc)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if !done {
				return false, nil
			}
			ds, err := getDaemonSet(oc, ovnNamespace, ovnIPsecDsName)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			if ds == nil && err == nil {
				done, err = areClusterOperatorsReady((oc))
				if err != nil && !isConnResetErr(err) {
					return false, err
				}
			}
			return done, nil
		})
}

var _ = g.Describe("[sig-network][Feature:IPsec]", g.Ordered, func() {

	oc := exutil.NewCLIWithPodSecurityLevel("ipsec", admissionapi.LevelPrivileged)
	f := oc.KubeFramework()

	waitForIPsecNSConfigApplied := func() {
		g.GinkgoHelper()
		o.Eventually(func(g o.Gomega) bool {
			out, err := oc.AsAdmin().Run("get").Args("NodeNetworkConfigurationPolicy/"+leftNodeIPsecPolicyName, "-o", "yaml").Output()
			g.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("rendered left node network config policy:\n%s", out)
			if !strings.Contains(out, "1/1 nodes successfully configured") {
				return false
			}
			out, err = oc.AsAdmin().Run("get").Args("NodeNetworkConfigurationPolicy/"+rightNodeIPsecPolicyName, "-o", "yaml").Output()
			g.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("rendered right node network config policy:\n%s", out)
			return strings.Contains(out, "1/1 nodes successfully configured")
		}, 30*time.Second).Should(o.BeTrue())
	}

	// IPsec is supported only with OVN-Kubernetes CNI plugin.
	InOVNKubernetesContext(func() {
		// The tests chooses two different nodes. Each node has two pods running, one is ping pod and another one is tcpdump pod.
		// The ping pod is used to send ping packet to its peer pod whereas tcpdump hostnetworked pod used for capturing the packet
		// at node's primary interface. Based on the IPsec configuration on the cluster, the captured packet on the host interface
		// must be either geneve or esp packet type.
		type testNodeConfig struct {
			pingPod    *corev1.Pod
			tcpdumpPod *corev1.Pod
			hostInf    string
			nodeName   string
			nodeIP     string
		}
		type testConfig struct {
			ipsecMode     v1.IPsecMode
			srcNodeConfig *testNodeConfig
			dstNodeConfig *testNodeConfig
		}
		// The config object contains test configuration that can be leveraged by each ipsec test.
		var config *testConfig
		// This function helps to generate ping traffic from src pod to dst pod and at the same time captures its
		// node traffic on both src and dst node.
		pingAndCheckNodeTraffic := func(src, dst *testNodeConfig, traffic trafficType) error {
			tcpDumpSync := errgroup.Group{}
			pingSync := errgroup.Group{}
			var srcNodeTrafficFilter string
			var dstNodeTrafficFilter string
			// use tcpdump pod's ip address it's a node ip address because it's a hostnetworked pod.
			switch traffic {
			case esp:
				srcNodeTrafficFilter = fmt.Sprintf(tcpdumpESPFilter, src.tcpdumpPod.Status.PodIP, dst.tcpdumpPod.Status.PodIP)
				dstNodeTrafficFilter = fmt.Sprintf(tcpdumpESPFilter, dst.tcpdumpPod.Status.PodIP, src.tcpdumpPod.Status.PodIP)
			case geneve:
				srcNodeTrafficFilter = fmt.Sprintf(tcpdumpGeneveFilter, src.tcpdumpPod.Status.PodIP, dst.tcpdumpPod.Status.PodIP)
				dstNodeTrafficFilter = fmt.Sprintf(tcpdumpGeneveFilter, dst.tcpdumpPod.Status.PodIP, src.tcpdumpPod.Status.PodIP)
			case icmp:
				srcNodeTrafficFilter = fmt.Sprintf(tcpdumpICMPFilter, src.tcpdumpPod.Status.PodIP, dst.tcpdumpPod.Status.PodIP)
				dstNodeTrafficFilter = fmt.Sprintf(tcpdumpICMPFilter, dst.tcpdumpPod.Status.PodIP, src.tcpdumpPod.Status.PodIP)
			}
			checkSrcNodeTraffic := func(src *testNodeConfig) error {
				_, err := oc.AsAdmin().Run("exec").Args(src.tcpdumpPod.Name, "-n", src.tcpdumpPod.Namespace, "--",
					"timeout", "10", "tcpdump", "-i", src.hostInf, "-c", "1", "-v", "--direction=out", srcNodeTrafficFilter).Output()
				return err
			}
			checkDstNodeTraffic := func(dst *testNodeConfig) error {
				_, err := oc.AsAdmin().Run("exec").Args(dst.tcpdumpPod.Name, "-n", dst.tcpdumpPod.Namespace, "--",
					"timeout", "10", "tcpdump", "-i", dst.hostInf, "-c", "1", "-v", "--direction=out", dstNodeTrafficFilter).Output()
				return err
			}
			pingTestFromPod := func(src, dst *testNodeConfig) error {
				_, err := oc.AsAdmin().Run("exec").Args(src.pingPod.Name, "-n", src.pingPod.Namespace, "--",
					"ping", "-c", "3", dst.pingPod.Status.PodIP).Output()
				return err
			}
			tcpDumpSync.Go(func() error {
				err := checkSrcNodeTraffic(src)
				if err != nil {
					return fmt.Errorf("error capturing traffic on the source node: %v", err)
				}
				return nil
			})
			tcpDumpSync.Go(func() error {
				err := checkDstNodeTraffic(dst)
				if err != nil {
					return fmt.Errorf("error capturing traffic on the dst node: %v", err)
				}
				return nil
			})
			pingSync.Go(func() error {
				return pingTestFromPod(src, dst)
			})
			// Wait for both ping and tcpdump capture complete and check the results.
			pingErr := pingSync.Wait()
			err := tcpDumpSync.Wait()
			if err != nil || pingErr != nil {
				return fmt.Errorf("failed to detect underlay traffic on node, node tcpdump err: %v, ping err: %v", err, pingErr)
			}
			return nil
		}

		setupTestPods := func(config *testConfig, isHostNetwork bool) error {
			tcpdumpImage, err := exutil.DetermineImageFromRelease(context.TODO(), oc, "network-tools")
			o.Expect(err).NotTo(o.HaveOccurred())
			createSync := errgroup.Group{}
			createSync.Go(func() error {
				var err error
				config.srcNodeConfig.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, config.srcNodeConfig.nodeName, "ipsec-tcpdump-hostpod-")
				if err != nil {
					return err
				}
				srcPingPod := e2epod.CreateExecPodOrFail(context.TODO(), f.ClientSet, f.Namespace.Name, "ipsec-test-srcpod-", func(p *corev1.Pod) {
					p.Spec.NodeName = config.srcNodeConfig.nodeName
					p.Spec.HostNetwork = isHostNetwork
				})
				config.srcNodeConfig.pingPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), srcPingPod.Name, metav1.GetOptions{})
				return err
			})
			createSync.Go(func() error {
				var err error
				config.dstNodeConfig.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, config.dstNodeConfig.nodeName, "ipsec-tcpdump-hostpod-")
				if err != nil {
					return err
				}
				dstPingPod := e2epod.CreateExecPodOrFail(context.TODO(), f.ClientSet, f.Namespace.Name, "ipsec-test-dstpod-", func(p *corev1.Pod) {
					p.Spec.NodeName = config.dstNodeConfig.nodeName
					p.Spec.HostNetwork = isHostNetwork
				})
				config.dstNodeConfig.pingPod, err = f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), dstPingPod.Name, metav1.GetOptions{})
				return err
			})
			return createSync.Wait()
		}

		cleanupTestPods := func(config *testConfig) {
			g.GinkgoHelper()
			err := e2epod.DeletePodWithWait(context.Background(), f.ClientSet, config.srcNodeConfig.pingPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.srcNodeConfig.pingPod = nil
			err = e2epod.DeletePodWithWait(context.Background(), f.ClientSet, config.srcNodeConfig.tcpdumpPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.srcNodeConfig.tcpdumpPod = nil

			err = e2epod.DeletePodWithWait(context.Background(), f.ClientSet, config.dstNodeConfig.pingPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.dstNodeConfig.pingPod = nil
			err = e2epod.DeletePodWithWait(context.Background(), f.ClientSet, config.dstNodeConfig.tcpdumpPod)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.dstNodeConfig.tcpdumpPod = nil
		}

		checkForGeneveOnlyPodTraffic := func(config *testConfig) {
			g.GinkgoHelper()
			err := setupTestPods(config, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				// Don't cleanup test pods in error scenario.
				if err != nil && !framework.TestContext.DeleteNamespaceOnFailure {
					return
				}
				cleanupTestPods(config)
			}()
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, geneve)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, esp)
			o.Expect(err).To(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, icmp)
			o.Expect(err).To(o.HaveOccurred())
			err = nil
		}

		checkForESPOnlyPodTraffic := func(config *testConfig) {
			g.GinkgoHelper()
			err := setupTestPods(config, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				// Don't cleanup test pods in error scenario.
				if err != nil && !framework.TestContext.DeleteNamespaceOnFailure {
					return
				}
				cleanupTestPods(config)
			}()
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, esp)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, geneve)
			o.Expect(err).To(o.HaveOccurred())
			err = nil
		}

		checkPodTraffic := func(mode v1.IPsecMode) {
			g.GinkgoHelper()
			if mode == v1.IPsecModeFull {
				checkForESPOnlyPodTraffic(config)
			} else {
				checkForGeneveOnlyPodTraffic(config)
			}
		}

		checkNodeTraffic := func(mode v1.IPsecMode) {
			g.GinkgoHelper()
			err := setupTestPods(config, true)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				// Don't cleanup test pods in error scenario.
				if err != nil && !framework.TestContext.DeleteNamespaceOnFailure {
					return
				}
				cleanupTestPods(config)
			}()
			if mode == v1.IPsecModeFull || mode == v1.IPsecModeExternal {
				err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, esp)
				o.Expect(err).NotTo(o.HaveOccurred())
				err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, icmp)
				o.Expect(err).To(o.HaveOccurred())
				err = nil
				return
			} else {
				err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, icmp)
				o.Expect(err).NotTo(o.HaveOccurred())
			}
		}

		g.BeforeAll(func() {
			// Set up the config object with existing IPsecConfig, setup testing config on
			// the selected nodes.
			ipsecMode, err := getIPsecMode(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(ipsecMode).To(o.Equal(v1.IPsecModeFull))

			srcNode, dstNode := &testNodeConfig{}, &testNodeConfig{}
			config = &testConfig{ipsecMode: ipsecMode, srcNodeConfig: srcNode,
				dstNodeConfig: dstNode}

			// Deploy nmstate handler which is used for rolling out IPsec config
			// via NodeNetworkConfigurationPolicy.
			g.By("deploy nmstate handler")
			err = deployNmstateHandler(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.BeforeEach(func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("Choosing 2 different nodes")
			node1, node2, err := findAppropriateNodes(f, DIFFERENT_NODE)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.srcNodeConfig.nodeName = node1.Name
			config.srcNodeConfig.hostInf, err = findBridgePhysicalInterface(oc, node1.Name, "br-ex")
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, address := range node1.Status.Addresses {
				if address.Type == corev1.NodeInternalIP {
					config.srcNodeConfig.nodeIP = address.Address
					break
				}
			}
			o.Expect(config.srcNodeConfig.nodeIP).NotTo(o.BeEmpty())
			config.dstNodeConfig.nodeName = node2.Name
			config.dstNodeConfig.hostInf, err = findBridgePhysicalInterface(oc, node2.Name, "br-ex")
			o.Expect(err).NotTo(o.HaveOccurred())
			for _, address := range node2.Status.Addresses {
				if address.Type == corev1.NodeInternalIP {
					config.dstNodeConfig.nodeIP = address.Address
					break
				}
			}
			o.Expect(config.dstNodeConfig.nodeIP).NotTo(o.BeEmpty())

			g.By("configure IPsec certs on the worker nodes")
			// The certificates in the Machine Config has validity period of 120 months starting from April 11, 2024.
			// so proceed with test if system date is before April 10, 2034. Otherwise fail the test.
			if !time.Now().Before(certExpirationDate) {
				framework.Failf("certficates in the Machine Config are expired, Please consider recreating those certificates")
			}
			nsCertMachineConfig, err := createIPsecCertsMachineConfig(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nsCertMachineConfig).NotTo(o.BeNil())
			o.Eventually(func(g o.Gomega) bool {
				pools, err := getMachineConfigPoolByLabel(oc, workerRoleMachineConfigLabel)
				g.Expect(err).NotTo(o.HaveOccurred())
				return areMachineConfigPoolsReadyWithMachineConfig(pools, nsCertMachineConfigName)
			}, ipsecRolloutWaitDuration, ipsecRolloutWaitInterval).Should(o.BeTrue())
			// wait for ovn-ipsec-host pod to get rolled out after certs installation.
			waitForIPsecConfigToComplete(oc, config.ipsecMode)
		})

		g.AfterEach(func() {
			g.By("remove left node ipsec configuration")
			oc.AsAdmin().Run("delete").Args("-f", leftNodeIPsecNNCPYaml).Execute()
			o.Eventually(func(g o.Gomega) bool {
				_, err := oc.AdminDynamicClient().Resource(gvrNodeNetworkConfigurationPolicy).Get(context.Background(),
					leftNodeIPsecPolicyName, metav1.GetOptions{})
				if err != nil && apierrors.IsNotFound(err) {
					return true
				}
				g.Expect(err).NotTo(o.HaveOccurred())
				return false
			}).Should(o.Equal(true))

			g.By("remove right node ipsec configuration")
			oc.AsAdmin().Run("delete").Args("-f", rightNodeIPsecNNCPYaml).Execute()
			o.Eventually(func(g o.Gomega) bool {
				_, err := oc.AdminDynamicClient().Resource(gvrNodeNetworkConfigurationPolicy).Get(context.Background(),
					rightNodeIPsecPolicyName, metav1.GetOptions{})
				if err != nil && apierrors.IsNotFound(err) {
					return true
				}
				g.Expect(err).NotTo(o.HaveOccurred())
				return false
			}).Should(o.Equal(true))

			// Removal of IPsec certs are needed otherwise worker nodes still keeping
			// stale ip xfrm state and policy entries created for north south traffic.
			g.By("removing IPsec certs from worker nodes")
			err := deleteNSCertMachineConfig(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func(g o.Gomega) bool {
				pools, err := getMachineConfigPoolByLabel(oc, workerRoleMachineConfigLabel)
				g.Expect(err).NotTo(o.HaveOccurred())
				return areMachineConfigPoolsReadyWithoutMachineConfig(pools, nsCertMachineConfigName)
			}, ipsecRolloutWaitDuration, ipsecRolloutWaitInterval).Should(o.BeTrue())

			// Restore the cluster back into original state after running each test.
			g.By("restoring ipsec config into original state")
			err = configureIPsecMode(oc, config.ipsecMode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, config.ipsecMode)
		})

		g.DescribeTable("check traffic [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]", func(mode v1.IPsecMode) {
			o.Expect(config).NotTo(o.BeNil())

			g.By("validate traffic before changing IPsec configuration")
			checkPodTraffic(config.ipsecMode)
			// N/S ipsec config is not in effect yet, so node traffic behaves as it were disabled
			checkNodeTraffic(v1.IPsecModeDisabled)

			g.By(fmt.Sprintf("configure IPsec in %s mode and validate traffic", mode))
			// Change IPsec mode to given mode and do packet capture on the node's interface
			err := configureIPsecMode(oc, mode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, mode)
			checkPodTraffic(mode)
			// N/S ipsec config is not in effect yet, so node traffic behaves as it were disabled
			checkNodeTraffic(v1.IPsecModeDisabled)

			g.By("rollout IPsec configuration via nmstate")
			err = ensureNmstateHandlerRunning(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			leftConfig := fmt.Sprintf(nodeIPsecConfigManifest, leftNodeIPsecPolicyName, config.srcNodeConfig.nodeName,
				config.srcNodeConfig.nodeIP, leftServerCertName, config.dstNodeConfig.nodeIP)
			err = os.WriteFile(leftNodeIPsecNNCPYaml, []byte(leftConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("desired left node network config policy:\n%s", leftConfig)
			err = oc.AsAdmin().Run("apply").Args("-f", leftNodeIPsecNNCPYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			rightConfig := fmt.Sprintf(nodeIPsecConfigManifest, rightNodeIPsecPolicyName, config.dstNodeConfig.nodeName,
				config.dstNodeConfig.nodeIP, rightServerCertName, config.srcNodeConfig.nodeIP)
			err = os.WriteFile(rightNodeIPsecNNCPYaml, []byte(rightConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("desired right node network config policy:\n%s", rightConfig)
			err = oc.AsAdmin().Run("apply").Args("-f", rightNodeIPsecNNCPYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for nmstate to roll out")
			waitForIPsecNSConfigApplied()

			g.By("validate IPsec traffic between nodes")
			// Pod traffic will be encrypted as a result N/S encryption being enabled between this two nodes
			checkPodTraffic(v1.IPsecModeFull)
			checkNodeTraffic(mode)
		},
			g.Entry("with IPsec in full mode", v1.IPsecModeFull),
			g.Entry("with IPsec in external mode", v1.IPsecModeExternal),
			// TODO add test for v1.IPsecModeDisabled mode once IPsec tests stabilized in CI.
		)
	})
})

func waitForIPsecConfigToComplete(oc *exutil.CLI, ipsecMode v1.IPsecMode) {
	g.GinkgoHelper()
	switch ipsecMode {
	case v1.IPsecModeDisabled:
		err := ensureIPsecDisabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	case v1.IPsecModeExternal:
		err := ensureIPsecExternalEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	case v1.IPsecModeFull:
		err := ensureIPsecFullEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
