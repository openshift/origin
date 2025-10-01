package networking

import (
	"context"
	"fmt"
	"net"
	"strings"

	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"
	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	v1 "k8s.io/api/core/v1"
	knet "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/test/e2e/framework"
	admissionapi "k8s.io/pod-security-admission/api"
)

var _ = ginkgo.Describe("[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks] Network Policies", func() {
	defer ginkgo.GinkgoRecover()

	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("network-segmentation-policy-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged
	InOVNKubernetesContext(func() {
		const (
			nodeHostnameKey              = "kubernetes.io/hostname"
			nadName                      = "tenant-red"
			userDefinedNetworkIPv4Subnet = "203.203.0.0/16"
			userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
			port                         = 9000
			randomStringLength           = 5
			nameSpaceYellowSuffix        = "yellow"
			namespaceBlueSuffix          = "blue"
		)

		var (
			cs                  clientset.Interface
			nadClient           nadclient.K8sCniCncfIoV1Interface
			allowServerPodLabel = map[string]string{"foo": "bar"}
			denyServerPodLabel  = map[string]string{"abc": "xyz"}
		)

		ginkgo.BeforeEach(func() {
			cs = f.ClientSet
			namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			})
			f.Namespace = namespace
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			err = udnWaitForOpenShift(oc, namespace.Name)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			namespaceYellow := getNamespaceName(f, nameSpaceYellowSuffix)
			namespaceBlue := getNamespaceName(f, namespaceBlueSuffix)
			for _, namespace := range []string{namespaceYellow, namespaceBlue} {
				ginkgo.By("Creating namespace " + namespace)
				ns, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   namespace,
						Labels: map[string]string{RequiredUDNNamespaceLabel: ""},
					},
				}, metav1.CreateOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				f.AddNamespacesToDelete(ns)
			}
		})

		ginkgo.AfterEach(func() {
			if ginkgo.CurrentSpecReport().Failed() {
				exutil.DumpPodStatesInNamespace(f.Namespace.Name, oc)
				exutil.DumpPodStatesInNamespace(getNamespaceName(f, nameSpaceYellowSuffix), oc)
				exutil.DumpPodStatesInNamespace(getNamespaceName(f, namespaceBlueSuffix), oc)
			}
		})

		ginkgo.DescribeTable(
			"pods within namespace should be isolated when deny policy is present",
			func(
				topology string,
				clientPodConfig podConfiguration,
				serverPodConfig podConfiguration,
			) {
				ginkgo.By("Creating the attachment configuration")
				netConfig := newNetworkAttachmentConfig(networkAttachmentConfigParams{
					name:     nadName,
					topology: topology,
					cidr:     correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet),
					role:     "primary",
				})
				netConfig.namespace = f.Namespace.Name
				_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(
					context.Background(),
					generateNAD(oc, netConfig),
					metav1.CreateOptions{},
				)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				workerNodes, err := getWorkerNodesOrdered(cs)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(len(workerNodes)).To(gomega.BeNumerically(">=", 1))

				ginkgo.By("creating client/server pods")
				clientPodConfig.namespace = f.Namespace.Name
				clientPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[0].Name}
				serverPodConfig.namespace = f.Namespace.Name
				serverPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[len(workerNodes)-1].Name}
				runUDNPod(cs, f.Namespace.Name, serverPodConfig, nil)
				runUDNPod(cs, f.Namespace.Name, clientPodConfig, nil)

				var serverIP string
				for i, cidr := range strings.Split(netConfig.cidr, ",") {
					if cidr != "" {
						ginkgo.By("asserting the server pod has an IP from the configured range")
						serverIP, err = podIPsForUserDefinedPrimaryNetwork(
							cs,
							f.Namespace.Name,
							serverPodConfig.name,
							namespacedName(f.Namespace.Name, netConfig.name),
							i,
						)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						ginkgo.By(fmt.Sprintf("asserting the server pod IP %v is from the configured range %v", serverIP, cidr))
						subnet, err := getNetCIDRSubnet(cidr)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						gomega.Expect(inRange(subnet, serverIP)).To(gomega.Succeed())
					}

					ginkgo.By("asserting the *client* pod can contact the server pod exposed endpoint")
					namespacePodShouldReach(oc, f.Namespace.Name, clientPodConfig.name, formatHostAndPort(net.ParseIP(serverIP), port))
				}

				ginkgo.By("creating a \"default deny\" network policy")
				_, err = makeDenyAllPolicy(f, f.Namespace.Name)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By("asserting the *client* pod can not contact the server pod exposed endpoint")
				podShouldNotReach(oc, clientPodConfig.name, formatHostAndPort(net.ParseIP(serverIP), port))

			},
			ginkgo.Entry(
				"in L2 dualstack primary UDN",
				"layer2",
				*podConfig(
					"client-pod",
				),
				*podConfig(
					"server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
				),
			),
			ginkgo.Entry(
				"in L3 dualstack primary UDN",
				"layer3",
				*podConfig(
					"client-pod",
				),
				*podConfig(
					"server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
				),
			),
		)

		ginkgo.DescribeTable(
			"allow ingress traffic to one pod from a particular namespace",
			func(
				topology string,
				clientPodConfig podConfiguration,
				allowServerPodConfig podConfiguration,
				denyServerPodConfig podConfiguration,
			) {

				namespaceYellow := getNamespaceName(f, nameSpaceYellowSuffix)
				namespaceBlue := getNamespaceName(f, namespaceBlueSuffix)

				nad := networkAttachmentConfigParams{
					topology: topology,
					cidr:     correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet),
					// Both yellow and blue namespaces are going to served by green network.
					// Use random suffix for the network name to avoid race between tests.
					networkName: fmt.Sprintf("%s-%s", "green", rand.String(randomStringLength)),
					role:        "primary",
				}

				// Use random suffix in net conf name to avoid race between tests.
				netConfName := fmt.Sprintf("sharednet-%s", rand.String(randomStringLength))
				for _, namespace := range []string{namespaceYellow, namespaceBlue} {
					ginkgo.By("creating the attachment configuration for " + netConfName + " in namespace " + namespace)
					netConfig := newNetworkAttachmentConfig(nad)
					netConfig.namespace = namespace
					netConfig.name = netConfName

					_, err := nadClient.NetworkAttachmentDefinitions(namespace).Create(
						context.Background(),
						generateNAD(oc, netConfig),
						metav1.CreateOptions{},
					)
					gomega.Expect(err).NotTo(gomega.HaveOccurred())
				}

				workerNodes, err := getWorkerNodesOrdered(cs)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				gomega.Expect(len(workerNodes)).To(gomega.BeNumerically(">=", 1))

				ginkgo.By("creating client/server pods")
				allowServerPodConfig.namespace = namespaceYellow
				allowServerPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[len(workerNodes)-1].Name}
				denyServerPodConfig.namespace = namespaceYellow
				denyServerPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[len(workerNodes)-1].Name}
				clientPodConfig.namespace = namespaceBlue
				clientPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[0].Name}
				runUDNPod(cs, namespaceYellow, allowServerPodConfig, func(pod *v1.Pod) {
					setRuntimeDefaultPSA(pod)
				})
				runUDNPod(cs, namespaceYellow, denyServerPodConfig, func(pod *v1.Pod) {
					setRuntimeDefaultPSA(pod)
				})
				runUDNPod(cs, namespaceBlue, clientPodConfig, func(pod *v1.Pod) {
					setRuntimeDefaultPSA(pod)
				})

				ginkgo.By("asserting the server pods have an IP from the configured range")
				var allowServerPodIP, denyServerPodIP string
				for i, cidr := range strings.Split(nad.cidr, ",") {
					if cidr != "" {
						subnet, err := getNetCIDRSubnet(cidr)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						allowServerPodIP, err = podIPsForUserDefinedPrimaryNetwork(cs, namespaceYellow, allowServerPodConfig.name,
							namespacedName(namespaceYellow, netConfName), i)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						ginkgo.By(fmt.Sprintf("asserting the allow server pod IP %v is from the configured range %v", allowServerPodIP, cidr))
						gomega.Expect(inRange(subnet, allowServerPodIP)).To(gomega.Succeed())
						denyServerPodIP, err = podIPsForUserDefinedPrimaryNetwork(cs, namespaceYellow, denyServerPodConfig.name,
							namespacedName(namespaceYellow, netConfName), i)
						gomega.Expect(err).NotTo(gomega.HaveOccurred())
						ginkgo.By(fmt.Sprintf("asserting the deny server pod IP %v is from the configured range %v", denyServerPodIP, cidr))
						gomega.Expect(inRange(subnet, denyServerPodIP)).To(gomega.Succeed())
					}
				}

				ginkgo.By("asserting the *client* pod can contact the allow server pod exposed endpoint")
				namespacePodShouldReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(allowServerPodIP), port))

				ginkgo.By("asserting the *client* pod can contact the deny server pod exposed endpoint")
				namespacePodShouldReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(denyServerPodIP), port))

				ginkgo.By("creating a \"default deny\" network policy")
				_, err = makeDenyAllPolicy(f, namespaceYellow)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By("asserting the *client* pod can not contact the allow server pod exposed endpoint")
				namespacePodShouldNotReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(allowServerPodIP), port))

				ginkgo.By("asserting the *client* pod can not contact the deny server pod exposed endpoint")
				namespacePodShouldNotReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(denyServerPodIP), port))

				ginkgo.By("creating a \"allow-traffic-to-pod\" network policy")
				_, err = allowTrafficToPodFromNamespacePolicy(f, namespaceYellow, namespaceBlue, "allow-traffic-to-pod", allowServerPodLabel)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())

				ginkgo.By("asserting the *client* pod can contact the allow server pod exposed endpoint")
				namespacePodShouldReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(allowServerPodIP), port))

				ginkgo.By("asserting the *client* pod can not contact deny server pod exposed endpoint")
				namespacePodShouldNotReach(oc, clientPodConfig.namespace, clientPodConfig.name, formatHostAndPort(net.ParseIP(denyServerPodIP), port))
			},
			ginkgo.Entry(
				"in L2 primary UDN",
				"layer2",
				*podConfig(
					"client-pod",
				),
				*podConfig(
					"allow-server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
					withLabels(allowServerPodLabel),
				),
				*podConfig(
					"deny-server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
					withLabels(denyServerPodLabel),
				),
			),
			ginkgo.Entry(
				"in L3 primary UDN",
				"layer3",
				*podConfig(
					"client-pod",
				),
				*podConfig(
					"allow-server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
					withLabels(allowServerPodLabel),
				),
				*podConfig(
					"deny-server-pod",
					withCommand(func() []string {
						return httpServerContainerCmd(port)
					}),
					withLabels(denyServerPodLabel),
				),
			))
	})
})

func getNamespaceName(f *framework.Framework, nsSuffix string) string {
	return fmt.Sprintf("%s-%s", f.Namespace.Name, nsSuffix)
}

func allowTrafficToPodFromNamespacePolicy(f *framework.Framework, namespace, fromNamespace, policyName string, podLabel map[string]string) (*knet.NetworkPolicy, error) {
	policy := &knet.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name: policyName,
		},
		Spec: knet.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: podLabel},
			PolicyTypes: []knet.PolicyType{knet.PolicyTypeIngress},
			Ingress: []knet.NetworkPolicyIngressRule{{From: []knet.NetworkPolicyPeer{
				{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": fromNamespace}}}}}},
		},
	}
	return f.ClientSet.NetworkingV1().NetworkPolicies(namespace).Create(context.TODO(), policy, metav1.CreateOptions{})
}
