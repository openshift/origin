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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	tcpdumpGeneveFilter          = "udp port 6081 and src %s and dst %s"
	masterIPsecMachineConfigName = "80-ipsec-master-extensions"
	workerIPSecMachineConfigName = "80-ipsec-worker-extensions"
	ipsecRolloutWaitDuration     = 20 * time.Minute
	ipsecRolloutWaitInterval     = 1 * time.Minute
	nmstateConfigureManifestFile = "nmstate.yaml"
	cnoNamespace                 = "openshift-network-operator"
	cnoSAName                    = "cluster-network-operator"
	nsCertMachineConfigName      = "99-worker-north-south-ipsec-config"
	leftNodeIPsecPolicyName      = "left-node-ipsec-policy"
	rightNodeIPsecPolicyName     = "right-node-ipsec-policy"
	leftNodeIPsecConfigYaml      = "ipsec-left-node.yaml"
	rightNodeIPsecConfigYaml     = "ipsec-right-node.yaml"
)

var ipsecCertConfigScript = `#!/bin/bash
set -o nounset
set -o errexit
set -o pipefail

nssdb="/var/lib/ipsec/nss"
tmp_dir=/tmp/ipsec-ns
CERTUTIL_NOISE="dsadasdasdasdadasdasdasdasdsadfwerwerjfdksdjfksdlfhjsdk"
mcp_role="worker"

left_ip=$LEFT_IP
right_ip=$RIGHT_IP

if [ -d "$tmp_dir" ]; then
    echo "ipsec-ns directory exists. Deleting and recreating..."
    rm -r "$tmp_dir"
fi
mkdir "$tmp_dir" && cd "$tmp_dir"

echo "\n-----Create Certs and export to mcp file----\n"

certutil_noise_file="${tmp_dir}/certutil-noise.txt"
echo ${CERTUTIL_NOISE} > ${certutil_noise_file}

create_cert()
{
echo "----Creating CA cert-----"
certutil -v 120 -S -k rsa -n "CA" -s "CN=CA" -v 12 -t "CT,C,C" -x -d ${nssdb} -z ${certutil_noise_file}
sleep 5s
echo "CA cert was created"
}

create_left_user_cert()
{
echo "----Creating left server cert.----"
certutil -v 120 -S -k rsa -c "CA" -n "left_server" -s "CN=left_server" -v 12 -t "u,u,u" -d ${nssdb} --extSAN "email:pepalani@redhat.com" -z ${certutil_noise_file}
sleep 5s
echo "Left server cert was created"
}

create_right_user_cert()
{
echo "-----Creating right server cert.-----"
certutil -v 120 -S -k rsa -c "CA" -n "right_server" -s "CN=right_server" -v 12 -t "u,u,u" -d ${nssdb} --extSAN "email:pepalani@redhat.com" -z ${certutil_noise_file}
sleep 5s
echo "Right server cert was created"
}

export_ca_cert()
{
echo "---Exporting CA cert to p12 and output to pem-----"
pk12util -o ${tmp_dir}/ca.p12 -n CA -d ${nssdb} -W ""
openssl pkcs12 -in ca.p12 -out ca.pem -clcerts -nokeys -passin pass:""
}

export_left_user_cert()
{
echo "----Exporting left user cert to p12-------"
pk12util -o $tmp_dir/left_server.p12 -n left_server -d $nssdb -W ""
}

export_right_user_cert()
{
echo "----Exporting right user cert to p12-------"
pk12util -o $tmp_dir/right_server.p12 -n right_server -d $nssdb -W ""
}

echo "Create certifications and export CA and certs!!"
create_cert
create_left_user_cert
create_right_user_cert
export_ca_cert
export_left_user_cert
export_right_user_cert
chmod 644 $tmp_dir/ca.pem
chmod 644 $tmp_dir/left_server.p12

echo "Create bu file for ipsec configuration on the host!"
cat > $tmp_dir/config.bu <<EOF
variant: openshift
version: 4.15.0
metadata:
  name: %s
  labels:
    machineconfiguration.openshift.io/role: ${mcp_role}
systemd:
  units:
    - name: ipsec-import.service
      enabled: true
      contents: |
        [Unit]
        Description=Import external certs into ipsec NSS
        Before=ipsec.service

        [Service]
        Type=oneshot
        ExecStart=/usr/local/bin/ipsec-addcert.sh
        RemainAfterExit=false
        StandardOutput=journal

        [Install]
        WantedBy=multi-user.target
storage:
  files:
  - path: /etc/pki/certs/ca.pem
    mode: 0400
    overwrite: true
    contents:
      local: ca.pem
  - path: /etc/pki/certs/left_server.p12
    mode: 0400
    overwrite: true
    contents:
      local: left_server.p12
  - path: /etc/pki/certs/right_server.p12
    mode: 0400
    overwrite: true
    contents:
      local: right_server.p12
  - path: /usr/local/bin/ipsec-addcert.sh
    mode: 0740
    overwrite: true
    contents:
      inline: |
        #!/bin/bash -e
        echo "importing cert to NSS"
        certutil -A -n "CA" -t "CT,C,C" -d /var/lib/ipsec/nss/ -i /etc/pki/certs/ca.pem
        pk12util -W "" -i /etc/pki/certs/left_server.p12 -d /var/lib/ipsec/nss/
        certutil -M -n "left_server" -t "u,u,u" -d /var/lib/ipsec/nss/
        pk12util -W "" -i /etc/pki/certs/right_server.p12 -d /var/lib/ipsec/nss/
        certutil -M -n "right_server" -t "u,u,u" -d /var/lib/ipsec/nss/
EOF

echo "Creating mcp file..."
butane --files-dir $tmp_dir $tmp_dir/config.bu -o $tmp_dir/config_ipsec_ns.yaml

echo "Importing certs to worker nodes."
kubectl apply -f $tmp_dir/config_ipsec_ns.yaml

echo "IPSEC North-South configuration completed!"
sleep infinity
`

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

// ensureIPsecEnabled this function ensure IPsec is enabled by making sure ovn-ipsec-host daemonset
// is completely ready on the cluster and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecEnabled(oc *exutil.CLI) error {
	err := ensureIPsecMachineConfigRolloutComplete(oc)
	if err != nil {
		return err
	}
	return wait.PollUntilContextTimeout(context.Background(), ipsecRolloutWaitInterval,
		ipsecRolloutWaitDuration, true, func(ctx context.Context) (bool, error) {
			done, err := isIPsecDaemonSetRunning(oc)
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

// ensureIPsecMachineConfigRolloutComplete this function ensures ipsec machine config extension is rolled out
// on all of master and worked nodes and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecMachineConfigRolloutComplete(oc *exutil.CLI) error {
	return wait.PollUntilContextTimeout(context.Background(), ipsecRolloutWaitInterval,
		ipsecRolloutWaitDuration, true, func(ctx context.Context) (bool, error) {
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
			ds, err := getIPsecDaemonSet(oc)
			if err != nil && !isConnResetErr(err) {
				return false, err
			}
			var done bool
			if ds == nil && err == nil {
				done, err = areClusterOperatorsReady((oc))
				if err != nil && !isConnResetErr(err) {
					return false, err
				}
			}
			return done, nil
		})
}

func isIPsecDaemonSetRunning(oc *exutil.CLI) (bool, error) {
	ipsecDS, err := getIPsecDaemonSet(oc)
	if ipsecDS == nil {
		return false, err
	}
	// Be sure that it has ovn-ipsec-host pod running in each node.
	ready := ipsecDS.Status.DesiredNumberScheduled == ipsecDS.Status.NumberReady
	return ready, nil
}

func getIPsecDaemonSet(oc *exutil.CLI) (*appsv1.DaemonSet, error) {
	ds, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-ovn-kubernetes").Get(context.Background(), "ovn-ipsec-host", metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return nil, nil
	}
	return ds, err
}

var _ = g.Describe("[sig-network][Feature:IPsec]", g.Ordered, func() {

	oc := exutil.NewCLIWithPodSecurityLevel("ipsec", admissionapi.LevelPrivileged)
	f := oc.KubeFramework()

	waitForIPsecNSConfigApplied := func() {
		o.Eventually(func() bool {
			out, err := oc.AsAdmin().Run("get").Args("NodeNetworkConfigurationPolicy/"+leftNodeIPsecPolicyName, "-o", "yaml").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("rendered left node network config policy:\n%s", out)
			if !strings.Contains(out, "1/1 nodes successfully configured") {
				return false
			}
			out, err = oc.AsAdmin().Run("get").Args("NodeNetworkConfigurationPolicy/"+rightNodeIPsecPolicyName, "-o", "yaml").Output()
			o.Expect(err).NotTo(o.HaveOccurred())
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
			controlPlaneNode string
			ipsecMode        v1.IPsecMode
			srcNodeConfig    *testNodeConfig
			dstNodeConfig    *testNodeConfig
		}
		// The config object contains test configuration that can be leveraged by each ipsec test.
		var config *testConfig
		// This function helps to generate ping traffic from src pod to dst pod and at the same time captures its
		// node traffic on both src and dst node.
		pingAndCheckNodeTraffic := func(src, dst *testNodeConfig, ipsecTraffic bool) error {
			tcpDumpSync := errgroup.Group{}
			pingSync := errgroup.Group{}
			// use tcpdump pod's ip address it's a node ip address because it's a hostnetworked pod.
			srcNodeTrafficFilter := fmt.Sprintf(tcpdumpGeneveFilter, src.tcpdumpPod.Status.PodIP, dst.tcpdumpPod.Status.PodIP)
			dstNodeTrafficFilter := fmt.Sprintf(tcpdumpGeneveFilter, dst.tcpdumpPod.Status.PodIP, src.tcpdumpPod.Status.PodIP)
			if ipsecTraffic {
				srcNodeTrafficFilter = fmt.Sprintf(tcpdumpESPFilter, src.tcpdumpPod.Status.PodIP, dst.tcpdumpPod.Status.PodIP)
				dstNodeTrafficFilter = fmt.Sprintf(tcpdumpESPFilter, dst.tcpdumpPod.Status.PodIP, src.tcpdumpPod.Status.PodIP)
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

		setupTestPods := func(config *testConfig) error {
			tcpdumpImage, err := exutil.DetermineImageFromRelease(oc, "network-tools")
			o.Expect(err).NotTo(o.HaveOccurred())
			createSync := errgroup.Group{}
			createSync.Go(func() error {
				var err error
				config.srcNodeConfig.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, config.srcNodeConfig.nodeName, "ipsec-tcpdump-hostpod-")
				if err != nil {
					return err
				}
				config.srcNodeConfig.pingPod = e2epod.CreateExecPodOrFail(context.TODO(), f.ClientSet, f.Namespace.Name, "ipsec-test-srcpod-", func(p *corev1.Pod) {
					p.Spec.NodeName = config.srcNodeConfig.nodeName
				})
				return err
			})
			createSync.Go(func() error {
				var err error
				config.dstNodeConfig.tcpdumpPod, err = launchHostNetworkedPodForTCPDump(f, tcpdumpImage, config.dstNodeConfig.nodeName, "ipsec-tcpdump-hostpod-")
				if err != nil {
					return err
				}
				config.dstNodeConfig.pingPod = e2epod.CreateExecPodOrFail(context.TODO(), f.ClientSet, f.Namespace.Name, "ipsec-test-dstpod-", func(p *corev1.Pod) {
					p.Spec.NodeName = config.dstNodeConfig.nodeName
				})
				return err
			})
			return createSync.Wait()
		}

		cleanupTestPods := func(config *testConfig) {
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

		checkForGeneveOnlyTraffic := func(config *testConfig) {
			err := setupTestPods(config)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				// Don't cleanup test pods in error scenario.
				if err != nil && !framework.TestContext.DeleteNamespaceOnFailure {
					return
				}
				cleanupTestPods(config)
			}()
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, true)
			o.Expect(err).To(o.HaveOccurred())
		}

		checkForESPOnlyTraffic := func(config *testConfig) {
			err := setupTestPods(config)
			o.Expect(err).NotTo(o.HaveOccurred())
			defer func() {
				// Don't cleanup test pods in error scenario.
				if err != nil && !framework.TestContext.DeleteNamespaceOnFailure {
					return
				}
				cleanupTestPods(config)
			}()
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, true)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, false)
			o.Expect(err).To(o.HaveOccurred())
		}

		checkTraffic := func(mode v1.IPsecMode) {
			if mode == v1.IPsecModeFull {
				checkForESPOnlyTraffic(config)
			} else {
				checkForGeneveOnlyTraffic(config)
			}
		}

		g.BeforeAll(func() {
			// Set up the config object with existing IPsecConfig, setup testing config on
			// the selected nodes.
			ipsecMode, err := getIPsecMode(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			// Choose a control plane node which is used for scheduling a pod to rollout
			// IPsec certificates for north south traffic test.
			node, err := findControlPlaneNode(f)
			o.Expect(err).NotTo(o.HaveOccurred())
			g.By(fmt.Sprintf("Chosen control plane node %s", node.Name))

			srcNode, dstNode := &testNodeConfig{}, &testNodeConfig{}
			config = &testConfig{ipsecMode: ipsecMode, controlPlaneNode: node.Name,
				srcNodeConfig: srcNode, dstNodeConfig: dstNode}
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
			// Restore the cluster back into original state after running all the tests.
			g.By("restoring ipsec config into original state")
			err := configureIPsecMode(oc, config.ipsecMode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, config.ipsecMode)

			g.By("remove right node ipsec configuration")
			err = oc.AsAdmin().Run("delete").Args("-f", rightNodeIPsecConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("remove left node ipsec configuration")
			err = oc.AsAdmin().Run("delete").Args("-f", leftNodeIPsecConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("undeploy nmstate handler")
			err = undeployNmstateHandler(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("removing IPsec certs from worker nodes")
			err = deleteNSCertMachineConfig(oc)
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Eventually(func() bool {
				ready, err := areWorkerMachineConfigPoolsReady(oc, nsCertMachineConfigName, false)
				o.Expect(err).NotTo(o.HaveOccurred())
				return ready
			}, ipsecRolloutWaitDuration, ipsecRolloutWaitInterval).Should(o.BeTrue())
		})

		g.DescribeTable("check traffic between local pod to a remote pod [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]", func(mode v1.IPsecMode) {
			o.Expect(config).NotTo(o.BeNil())
			g.By("validate pod traffic before changing IPsec configuration")
			// Ensure pod traffic is working with right encapsulation before rolling out IPsec configuration.
			checkTraffic(config.ipsecMode)
			g.By(fmt.Sprintf("configure IPsec in %s mode and validate pod traffic", mode))
			err := configureIPsecMode(oc, mode)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, mode)
			// Ensure pod traffic is working with right encapsulation after rolling out IPsec configuration.
			checkTraffic(mode)
		},
			g.Entry("with IPsec in full mode", v1.IPsecModeFull),
			g.Entry("with IPsec in external mode", v1.IPsecModeExternal),
			g.Entry("with IPsec in disabled mode", v1.IPsecModeDisabled),
		)

		// This test checks pod traffic to verify that N/S ipsec is enabled, and this wouldn't work to verify
		// a working N/S ipsec in Full ipsec mode as in that case pod traffic would be encrypted anyway
		// due to E/W ipsec configuration.
		g.It("validate node traffic is IPsec encrypted for corresponding IPsec north south configuration [apigroup:config.openshift.io] [Suite:openshift/network/ipsec]", func() {
			o.Expect(config).NotTo(o.BeNil())

			g.By("validate pod traffic before changing IPsec configuration")
			// Ensure pod traffic is working before rolling out IPsec configuration.
			checkTraffic(config.ipsecMode)

			g.By("configure IPsec in External mode")
			// Change IPsec mode to External and packet capture on the node's interface
			// must be geneve encapsulated ones.
			err := configureIPsecMode(oc, v1.IPsecModeExternal)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, v1.IPsecModeExternal)
			checkForGeneveOnlyTraffic(config)

			// TODO: replace private image with network-tools after updating it with
			// required packages (libreswan, bluto, openssl) to configure the certs.
			// It also spins up the pod in openshift-network-operator namespace so that
			// pod has ability to rollout a machine config for importing the certs into
			// worker nodes.
			g.By("configure IPsec certs on the worker nodes")
			nsCertMachineConfig, err := launchIPsecCertsConfig(oc, "quay.io/pepalani/ipsec-tools:latest",
				config.srcNodeConfig.nodeIP, config.dstNodeConfig.nodeIP, config.controlPlaneNode,
				"ipsec-ns-config")
			o.Expect(err).NotTo(o.HaveOccurred())
			o.Expect(nsCertMachineConfig).NotTo(o.BeNil())
			o.Eventually(func() bool {
				exists, err := areWorkerMachineConfigPoolsReady(oc, nsCertMachineConfigName, true)
				o.Expect(err).NotTo(o.HaveOccurred())
				return exists
			}, ipsecRolloutWaitDuration, ipsecRolloutWaitInterval).Should(o.BeTrue())

			// Deploy nmstate handler which is used for rolling out IPsec config
			// via NodeNetworkConfigurationPolicy.
			g.By("deploy nmstate handler")
			err = deployNmstateHandler(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("rollout IPsec configuration via nmstate")
			leftConfig := fmt.Sprintf(nodeIPsecConfigManifest, leftNodeIPsecPolicyName, config.srcNodeConfig.nodeName,
				config.srcNodeConfig.nodeIP, "left_server", config.dstNodeConfig.nodeIP)
			err = os.WriteFile(leftNodeIPsecConfigYaml, []byte(leftConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("desired left node network config policy:\n%s", leftConfig)
			err = oc.AsAdmin().Run("apply").Args("-f", leftNodeIPsecConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			rightConfig := fmt.Sprintf(nodeIPsecConfigManifest, rightNodeIPsecPolicyName, config.dstNodeConfig.nodeName,
				config.dstNodeConfig.nodeIP, "right_server", config.srcNodeConfig.nodeIP)
			err = os.WriteFile(rightNodeIPsecConfigYaml, []byte(rightConfig), 0644)
			o.Expect(err).NotTo(o.HaveOccurred())
			framework.Logf("desired right node network config policy:\n%s", rightConfig)
			err = oc.AsAdmin().Run("apply").Args("-f", rightNodeIPsecConfigYaml).Execute()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for nmstate to roll out")
			waitForIPsecNSConfigApplied()

			g.By("validate IPsec traffic between nodes")
			checkForESPOnlyTraffic(config)
		})
	})
})

func waitForIPsecConfigToComplete(oc *exutil.CLI, ipsecMode v1.IPsecMode) {
	switch ipsecMode {
	case v1.IPsecModeDisabled:
		err := ensureIPsecDisabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	case v1.IPsecModeExternal:
		err := ensureIPsecMachineConfigRolloutComplete(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	case v1.IPsecModeFull:
		err := ensureIPsecEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
	}
}
