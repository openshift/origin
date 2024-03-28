package networking

import (
	"context"
	"fmt"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	mcfgv1 "github.com/openshift/api/machineconfiguration/v1"
	v1 "github.com/openshift/api/operator/v1"
	exutil "github.com/openshift/origin/test/extended/util"
	"golang.org/x/sync/errgroup"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/util/retry"
	admissionapi "k8s.io/pod-security-admission/api"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
)

const (
	// tcpdumpESPFilter can be used to filter out IPsec packets destined to target node.
	tcpdumpESPFilter = "esp or udp port 4500 and src %s and dst %s"
	// tcpdumpGeneveFilter can be used to filter out Geneve encapsulated packets destined to target node.
	tcpdumpGeneveFilter      = "udp port 6081 and src %s and dst %s"
	nmstateDeployManifest    = "deploy-nmstate.yaml"
	nmstateConfigureManifest = "nmstate.yaml"
)

var ipsecConfigScript = `#!/bin/bash
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
certutil -v 120 -S -k rsa -c "CA" -n "left_server" -s "CN=left_server" -v 12 -t "u,u,u" -d ${nssdb} --extSAN "ip:${left_ip}" -z ${certutil_noise_file}
sleep 5s
echo "Left server cert was created"
}

create_right_user_cert()
{
echo "-----Creating right server cert.-----"
certutil -v 120 -S -k rsa -c "CA" -n "right_server" -s "CN=right_server" -v 12 -t "u,u,u" -d ${nssdb} --extSAN "ip:${right_ip}" -z ${certutil_noise_file}
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
version: %s
metadata:
  name: 99-worker-configure-ipsec-ns
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

echo "IPSEC North-Sourth configuration completed!"
sleep infinity
env:
- name: LEFT_IP
value: %s
- name: RIGHT_IP
value: %s
`

var nodeConfigIPSec = `
kind: NodeNetworkConfigurationPolicy
apiVersion: nmstate.io/v1
metadata:
  name: ipsec-policy-%s-config
spec:
  nodeSelector:
    kubernetes.io/hostname: %s
  desiredState:
    interfaces:
    - name: vpn1
      type: ipsec
      libreswan:
        left: %s
        leftid: '%fromcert'
        leftrsasigkey: '%cert'
        leftcert: left_server
        leftmodecfgclient: false
        right: %s
        rightid: '%fromcert'
        rightrsasigkey: '%cert'
        ike: aes_gcm256-sha2_256
        esp: aes_gcm256
        ikev2: insist
        type: transport
`

// configureIPsec helps to rollout specified IPsecConfig on the cluster.
func configureIPsec(oc *exutil.CLI, ipsecConfig *v1.IPsecConfig) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		if err != nil {
			return err
		}
		network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig = ipsecConfig
		_, err = oc.AdminOperatorClient().OperatorV1().Networks().Update(context.Background(), network, metav1.UpdateOptions{})
		return err
	})
}

// configureIPsec helps to rollout specified IPsec Mode on the cluster. If the cluster is already configured with specified mode, then
// this is almost like no-op for the cluster.
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

func getIPsecConfig(oc *exutil.CLI) (*v1.IPsecConfig, error) {
	network, err := oc.AdminOperatorClient().OperatorV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	ipsecConfig := network.Spec.DefaultNetwork.OVNKubernetesConfig.IPsecConfig
	if ipsecConfig != nil {
		ipsecConfig = ipsecConfig.DeepCopy()
	}
	return ipsecConfig, nil
}

// ensureIPsecEnabled this function ensure IPsec is enabled by making sure ovn-ipsec-host daemonset
// is completely ready on the cluster and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecEnabled(oc *exutil.CLI) (bool, error) {
	mcComplete, err := ensureIPsecMachineConfigRolloutComplete(oc)
	if !mcComplete {
		return mcComplete, err
	}
	for {
		running, err := isIPsecDaemonSetRunning(oc)
		if err != nil {
			return false, err
		}
		if running {
			ready, err := isClusterOperatorsReady((oc))
			if err != nil {
				return false, err
			}
			if ready {
				return true, nil
			}
		}
		time.Sleep(60 * time.Second)
	}
}

// ensureIPsecMachineConfigRolloutComplete this function ensures ipsec machine config extension is rolled out
// on all of master and worked nodes and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecMachineConfigRolloutComplete(oc *exutil.CLI) (bool, error) {
	for {
		done, err := isMachineConfigPoolReadyWithIPsec(oc)
		if err != nil {
			return false, err
		}
		if done {
			ready, err := isClusterOperatorsReady((oc))
			if err != nil {
				return false, err
			}
			if ready {
				return true, nil
			}
		}
		time.Sleep(60 * time.Second)
	}
}

// ensureIPsecDisabled this function ensure IPsec is disabled by making sure ovn-ipsec-host daemonset
// is completely removed from the cluster and cluster operators are coming back into ready state
// once ipsec rollout is complete.
func ensureIPsecDisabled(oc *exutil.CLI) (bool, error) {
	for {
		running, err := isIPsecDaemonSetRunning(oc)
		if err != nil {
			return false, err
		}
		if !running {
			ready, err := isClusterOperatorsReady((oc))
			if err != nil {
				return false, err
			}
			if ready {
				return true, nil
			}
		}
		time.Sleep(60 * time.Second)
	}
}

// This checks master and worker machine config pool status are set with ipsec extension which
// confirms extension is successfully rolled out on all nodes.
func isMachineConfigPoolReadyWithIPsec(oc *exutil.CLI) (bool, error) {
	// Retrieve IPsec Machine Configs which is created for master role and ensure that is being
	// completely available from master machine config pool.
	masterIPsecMachineConfigs, err := findIPsecMachineConfigsWithLabel(oc, "machineconfiguration.openshift.io/role=master")
	if err != nil {
		return false, fmt.Errorf("failed to get ipsec machine configs for master: %v", err)
	}
	masterMCPool, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(),
		"master", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get ipsec machine config pool for master: %v", err)
	}
	if !hasSourceInMachineConfigStatus(masterMCPool.Status, masterIPsecMachineConfigs) {
		return false, nil
	}

	// Retrieve IPsec Machine Configs which is created for worker role and ensure that is being
	// completely available from worker machine config pool.
	workerIPsecMachineConfigs, err := findIPsecMachineConfigsWithLabel(oc, "machineconfiguration.openshift.io/role=worker")
	if err != nil {
		return false, fmt.Errorf("failed to get ipsec machine configs for worker: %v", err)
	}
	workerMCPool, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigPools().Get(context.Background(),
		"worker", metav1.GetOptions{})
	if err != nil {
		return false, fmt.Errorf("failed to get ipsec machine config pool for worker: %v", err)
	}
	if !hasSourceInMachineConfigStatus(workerMCPool.Status, workerIPsecMachineConfigs) {
		return false, nil
	}
	return true, nil
}

// This function retrieves all the machine configs which has ipsec extension set. It is most
// likely being the one which is rolled out by network operator.
func findIPsecMachineConfigsWithLabel(oc *exutil.CLI, selector string) ([]*mcfgv1.MachineConfig, error) {
	lSelector, err := labels.Parse(selector)
	if err != nil {
		return nil, err
	}
	machineConfigs, err := oc.MachineConfigurationClient().MachineconfigurationV1().MachineConfigs().List(context.Background(),
		metav1.ListOptions{LabelSelector: lSelector.String()})
	if err != nil {
		return nil, err
	}
	var ipsecMachineConfigs []*mcfgv1.MachineConfig
	for i, machineConfig := range machineConfigs.Items {
		if sets.New(machineConfig.Spec.Extensions...).Has("ipsec") {
			ipsecMachineConfigs = append(ipsecMachineConfigs, &machineConfigs.Items[i])
		}
	}
	return ipsecMachineConfigs, nil
}

func hasSourceInMachineConfigStatus(machineConfigStatus mcfgv1.MachineConfigPoolStatus, machineConfigs []*mcfgv1.MachineConfig) bool {
	sourceNames := sets.New[string]()
	for _, machineConfig := range machineConfigs {
		sourceNames.Insert(machineConfig.Name)
	}
	for _, source := range machineConfigStatus.Configuration.Source {
		if sourceNames.Has(source.Name) {
			return true
		}
	}
	return false
}

// isClusterOperatorsReady returns true when every cluster operator is with available state and neither in degraded
// nor in progressing state, otherwise returns false.
func isClusterOperatorsReady(oc *exutil.CLI) (bool, error) {
	cos, err := oc.AdminConfigClient().ConfigV1().ClusterOperators().List(context.Background(), metav1.ListOptions{})
	if err != nil {
		return false, err
	}
	for _, co := range cos.Items {
		available, degraded, progressing := false, true, true
		for _, condition := range co.Status.Conditions {
			isConditionTrue := condition.Status == configv1.ConditionTrue
			switch condition.Type {
			case configv1.OperatorAvailable:
				available = isConditionTrue
			case configv1.OperatorDegraded:
				degraded = isConditionTrue
			case configv1.OperatorProgressing:
				progressing = isConditionTrue
			}
		}
		isCOReady := available && !degraded && !progressing
		if !isCOReady {
			return false, nil
		}
	}
	return true, nil
}

func isIPsecDaemonSetRunning(oc *exutil.CLI) (bool, error) {
	ipsecDS, err := oc.AdminKubeClient().AppsV1().DaemonSets("openshift-ovn-kubernetes").Get(context.Background(), "ovn-ipsec-host", metav1.GetOptions{})
	if err != nil && apierrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, err
	}
	// Be sure that it has ovn-ipsec-host pod running in each node.
	ready := ipsecDS.Status.DesiredNumberScheduled == ipsecDS.Status.NumberReady
	return ready, nil
}

var _ = g.Describe("[sig-network][Feature:IPsec]", g.Ordered, func() {

	oc := exutil.NewCLIWithPodSecurityLevel("ipsec", admissionapi.LevelPrivileged)
	f := oc.KubeFramework()

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
		}
		type testConfig struct {
			ipsecConfig   *v1.IPsecConfig
			srcNodeConfig *testNodeConfig
			dstNodeConfig *testNodeConfig
		}
		// The config object contains test configuration that can be leveraged by each ipsec test.
		var config *testConfig
		// This function helps to generate ping traffic from src pod to dst pod and at the same captures its
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
			if err != nil {
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
				config.srcNodeConfig.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ipsec-test-srcpod-", func(p *corev1.Pod) {
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
				config.dstNodeConfig.pingPod, err = createPod(f.ClientSet, f.Namespace.Name, "ipsec-test-dstpod-", func(p *corev1.Pod) {
					p.Spec.NodeName = config.dstNodeConfig.nodeName
				})
				return err
			})
			return createSync.Wait()
		}

		checkForGeneveOnlyTraffic := func(config *testConfig) {
			err := setupTestPods(config)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, false)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, true)
			o.Expect(err).To(o.HaveOccurred())
		}

		checkForESPOnlyTraffic := func(config *testConfig) {
			err := setupTestPods(config)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, true)
			o.Expect(err).NotTo(o.HaveOccurred())
			err = pingAndCheckNodeTraffic(config.srcNodeConfig, config.dstNodeConfig, false)
			o.Expect(err).To(o.HaveOccurred())
		}

		g.BeforeAll(func() {
			// Set up the config object with existing IPsecConfig, setup testing config on
			// the selected nodes.
			ipsecConfig, err := getIPsecConfig(oc)
			o.Expect(err).NotTo(o.HaveOccurred())

			srcNode, dstNode := &testNodeConfig{}, &testNodeConfig{}
			config = &testConfig{ipsecConfig: ipsecConfig, srcNodeConfig: srcNode, dstNodeConfig: dstNode}
		})

		g.BeforeEach(func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("Choosing 2 different nodes")
			node1, node2, err := findAppropriateNodes(f, DIFFERENT_NODE)
			o.Expect(err).NotTo(o.HaveOccurred())
			config.srcNodeConfig.nodeName = node1.Name
			config.srcNodeConfig.hostInf, err = findBridgePhysicalInterface(oc, node1.Name, "br-ex")
			o.Expect(err).NotTo(o.HaveOccurred())
			config.dstNodeConfig.nodeName = node2.Name
			config.dstNodeConfig.hostInf, err = findBridgePhysicalInterface(oc, node2.Name, "br-ex")
			o.Expect(err).NotTo(o.HaveOccurred())
		})

		g.AfterEach(func() {
			// Restore the cluster back into original state after running all the tests.
			g.By("restoring ipsec config into original state")
			err := configureIPsec(oc, config.ipsecConfig)
			o.Expect(err).NotTo(o.HaveOccurred())
			if config.ipsecConfig == nil {
				waitForIPsecConfigToComplete(oc, v1.IPsecModeDisabled)
			} else {
				waitForIPsecConfigToComplete(oc, config.ipsecConfig.Mode)
			}
		})

		g.It("ensure traffic between local pod to a remote pod is IPsec encrypted when IPsecMode configured in Full mode [apigroup:config.openshift.io] [Serial]", func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("validate pod traffic before changing IPsec configuration")
			// Ensure pod traffic is working before rolling out IPsec configuration.
			if config.ipsecConfig == nil || config.ipsecConfig.Mode == v1.IPsecModeDisabled {
				checkForGeneveOnlyTraffic(config)
			} else {
				checkForESPOnlyTraffic(config)
			}

			g.By("configure IPsec in Full mode and validate pod traffic")
			// Rollout IPsec with Full mode which sets up IPsec in OVS dataplane which makes pod traffic across nodes
			// would be always ipsec encrypted.
			err := configureIPsecMode(oc, v1.IPsecModeFull)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, v1.IPsecModeFull)
			checkForESPOnlyTraffic(config)
		})

		g.It("ensure traffic between local pod to a remote pod is not IPsec encrypted when IPsecMode configured in External mode [apigroup:config.openshift.io] [Serial]", func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("validate pod traffic before changing IPsec configuration")
			// Ensure pod traffic is working before rolling out IPsec configuration.
			if config.ipsecConfig == nil || config.ipsecConfig.Mode == v1.IPsecModeDisabled {
				checkForGeneveOnlyTraffic(config)
			} else {
				checkForESPOnlyTraffic(config)
			}

			g.By("configure IPsec in External mode and validate pod traffic")
			// Change IPsec mode to External and packet capture on the node's interface
			// must be geneve encapsulated ones.
			err := configureIPsecMode(oc, v1.IPsecModeExternal)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, v1.IPsecModeExternal)
			checkForGeneveOnlyTraffic(config)
		})

		g.It("ensure traffic between local pod to a remote pod is not IPsec encrypted when IPsec is Disabled [apigroup:config.openshift.io] [Serial]", func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("validate pod traffic before changing IPsec configuration")
			// Ensure pod traffic is working before rolling out IPsec configuration.
			if config.ipsecConfig == nil || config.ipsecConfig.Mode == v1.IPsecModeDisabled {
				checkForGeneveOnlyTraffic(config)
			} else {
				checkForESPOnlyTraffic(config)
			}

			g.By("configure IPsec in Disabled mode and validate pod traffic")
			// Configure IPsec mode to Disabled and packet capture on the node's interface
			// must be geneve encapsulated ones.
			err := configureIPsecMode(oc, v1.IPsecModeDisabled)
			o.Expect(err).NotTo(o.HaveOccurred())
			waitForIPsecConfigToComplete(oc, v1.IPsecModeDisabled)
			checkForGeneveOnlyTraffic(config)
		})

		g.It("ensure traffic between local pod to a remote pod is working when IPsec rollout in progress [apigroup:config.openshift.io] [Serial]", func() {
			o.Expect(config).NotTo(o.BeNil())
			g.By("validate pod traffic before changing IPsec configuration")
			var desiredIPsecMode v1.IPsecMode
			// Ensure pod traffic is working before rolling out any IPsec configuration.
			if config.ipsecConfig == nil || config.ipsecConfig.Mode == v1.IPsecModeDisabled {
				checkForGeneveOnlyTraffic(config)
				desiredIPsecMode = v1.IPsecModeFull
			} else {
				checkForESPOnlyTraffic(config)
				desiredIPsecMode = v1.IPsecModeDisabled
			}

			g.By(fmt.Sprintf("configure IPsec in %s mode", desiredIPsecMode))
			// Rollout IPsec with desired mode and check pod traffic is still intact.
			err := configureIPsecMode(oc, desiredIPsecMode)
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("validate pod traffic while IPsec config rollout in progress")
			for {
				ready, err := isClusterOperatorsReady(oc)
				o.Expect(err).NotTo(o.HaveOccurred())
				if !ready {
					break
				}
				time.Sleep(30 * time.Second)
			}
			pingSync := errgroup.Group{}
			pingTestFromPod := func(src, dst *testNodeConfig) error {
				_, err := oc.AsAdmin().Run("exec").Args(src.pingPod.Name, "-n", src.pingPod.Namespace, "--",
					"ping", "-c", "3", dst.pingPod.Status.PodIP).Output()
				return err
			}
			pingSync.Go(func() error {
				return pingTestFromPod(config.srcNodeConfig, config.dstNodeConfig)
			})
			// Wait for both ping to complete and check the result.
			err = pingSync.Wait()
			o.Expect(err).NotTo(o.HaveOccurred())

			g.By("wait for IPsec rollout to complete")
			waitForIPsecConfigToComplete(oc, desiredIPsecMode)
		})

		g.It("validate north south traffic when IPsec mode is set to External [apigroup:config.openshift.io] [Serial]", func() {
			// TODO:
		})
	})
})

func waitForIPsecConfigToComplete(oc *exutil.CLI, ipsecMode v1.IPsecMode) {
	switch ipsecMode {
	case v1.IPsecModeDisabled:
		disabled, err := ensureIPsecDisabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(disabled).Should(o.Equal(true))
	case v1.IPsecModeExternal:
		ready, err := ensureIPsecMachineConfigRolloutComplete(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(ready).Should(o.Equal(true))
	case v1.IPsecModeFull:
		enabled, err := ensureIPsecEnabled(oc)
		o.Expect(err).NotTo(o.HaveOccurred())
		o.Expect(enabled).Should(o.Equal(true))
	}
}
