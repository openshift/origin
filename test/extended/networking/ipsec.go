package networking

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	v1 "github.com/openshift/api/operator/v1"
	mg "github.com/openshift/origin/test/extended/machine_config"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/test/e2e/framework"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	"k8s.io/kubernetes/test/e2e/framework/statefulset"
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
			o.Expect(ipsecMode).NotTo(o.Equal(v1.IPsecModeDisabled))

			srcNode, dstNode := &testNodeConfig{}, &testNodeConfig{}
			config = &testConfig{ipsecMode: ipsecMode, srcNodeConfig: srcNode,
				dstNodeConfig: dstNode}

			// Deploy nmstate handler which is used for rolling out IPsec config
			// via NodeNetworkConfigurationPolicy.
			g.By("deploy nmstate handler")
			err = deployNmstateHandler(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Update cluster machine configuration object with few more nodeDisruptionPolicy defined
			// in test/extended/testdata/ipsec/nsconfig-reboot-none-policy.yaml file so that worker
			// nodes don't go for a reboot while rolling out `99-worker-north-south-ipsec-config`
			// machine config which configures certificates for testing IPsec north south traffic.
			g.By("deploy machine configuration policy")
			err = oc.AsAdmin().Run("apply").Args("-f", nsNodeRebootNoneFixture).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())
			mg.WaitForBootImageControllerToComplete(oc)

			g.By("configure IPsec certs on the worker nodes")
			// The certificates for configuring NS IPsec between two worker nodes are deployed through machine config
			// `99-worker-north-south-ipsec-config` which is in the test/extended/testdata/ipsec/nsconfig-machine-config.yaml file.
			// This is a butane generated file via a butane config file available with commit:
			// https://github.com/openshift/origin/pull/28658/commits/7399006f3750c530cfef51fa1044e941ccb85087
			// The machine config mounts cert files into node's /etc/pki/certs directory and runs ipsec-addcert.sh script
			// to import those certs into Libreswan nss db and will be used by Libreswan for IPsec north south connection
			// configured via NodeNetworkConfigurationPolicy on the node.
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
			// Ensure IPsec mode is still correctly configured.
			waitForIPsecConfigToComplete(oc, config.ipsecMode)
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
		})

		g.AfterEach(func() {
			ipsecMode, err := getIPsecMode(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			if g.CurrentSpecReport().Failed() {
				if ipsecMode == v1.IPsecModeFull {
					var ipsecPods []string
					srcIPsecPod, err := findIPsecPodonNode(oc, config.srcNodeConfig.nodeName)
					o.Expect(err).NotTo(o.HaveOccurred())
					ipsecPods = append(ipsecPods, srcIPsecPod)
					dstIPsecPod, err := findIPsecPodonNode(oc, config.dstNodeConfig.nodeName)
					o.Expect(err).NotTo(o.HaveOccurred())
					ipsecPods = append(ipsecPods, dstIPsecPod)
					for _, ipsecPod := range ipsecPods {
						dumpPodCommand(ovnNamespace, ipsecPod, "cat /etc/ipsec.conf")
						dumpPodCommand(ovnNamespace, ipsecPod, "cat /etc/ipsec.d/openshift.conf")
						dumpPodCommand(ovnNamespace, ipsecPod, "ipsec status")
						dumpPodCommand(ovnNamespace, ipsecPod, "ipsec trafficstatus")
						dumpPodCommand(ovnNamespace, ipsecPod, "ip xfrm state")
						dumpPodCommand(ovnNamespace, ipsecPod, "ip xfrm policy")
					}
				}
				exutil.DumpPodStatesInNamespace(nmstateNamespace, oc)
				exutil.DumpPodLogsStartingWithInNamespace("nmstate-handler", nmstateNamespace, oc)
				exutil.DumpPodLogsStartingWithInNamespace("nmstate-operator", nmstateNamespace, oc)
			}
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
		})

		g.It("check traffic with IPsec [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]", func() {
			o.Expect(config).NotTo(o.BeNil())

			g.By("validate traffic before changing IPsec configuration")
			checkPodTraffic(config.ipsecMode)
			// N/S ipsec config is not in effect yet, so node traffic behaves as it were disabled
			checkNodeTraffic(v1.IPsecModeDisabled)

			// TODO: remove this block when https://issues.redhat.com/browse/RHEL-67307 is fixed.
			if config.ipsecMode == v1.IPsecModeFull {
				g.By(fmt.Sprintf("skip testing IPsec NS configuration with %s mode due to nmstate bug RHEL-67307", config.ipsecMode))
				return
			}

			g.By("rollout IPsec configuration via nmstate")
			err := ensureNmstateHandlerRunning(oc)
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
			checkNodeTraffic(v1.IPsecModeExternal)
		})
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

func findIPsecPodonNode(oc *exutil.CLI, nodeName string) (string, error) {
	out, err := runOcWithRetry(oc.AsAdmin(), "get",
		"pods",
		"-o", "name",
		"-n", ovnNamespace,
		"--field-selector", fmt.Sprintf("spec.nodeName=%s", nodeName),
		"-l", "app=ovn-ipsec")
	if err != nil {
		return "", err
	}
	outReader := bufio.NewScanner(strings.NewReader(out))
	re := regexp.MustCompile("^pod/(.*)")
	var podName string
	for outReader.Scan() {
		match := re.FindSubmatch([]byte(outReader.Text()))
		if len(match) != 2 {
			continue
		}
		podName = string(match[1])
		break
	}
	if podName == "" {
		return "", fmt.Errorf("could not find a valid ovn-ipsec-host pod on node '%s'", nodeName)
	}
	return podName, nil
}

func dumpPodCommand(namespace, name, cmd string) {
	g.GinkgoHelper()
	stdout, err := e2eoutput.RunHostCmdWithRetries(namespace, name, cmd, statefulset.StatefulSetPoll, statefulset.StatefulPodTimeout)
	o.Expect(err).NotTo(o.HaveOccurred())
	framework.Logf(name + ": " + strings.Join(strings.Split(stdout, "\n"), fmt.Sprintf("\n%s: ", name)))
}
