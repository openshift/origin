package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	nadapi "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned/typed/k8s.cni.cncf.io/v1"

	kubeauthorizationv1 "k8s.io/api/authorization/v1"
	v1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/rand"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/kubectl/pkg/util/podutils"
	"k8s.io/kubernetes/test/e2e/framework"
	e2ekubectl "k8s.io/kubernetes/test/e2e/framework/kubectl"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	admissionapi "k8s.io/pod-security-admission/api"
	utilnet "k8s.io/utils/net"
	"k8s.io/utils/pointer"

	exutil "github.com/openshift/origin/test/extended/util"
)

const openDefaultPortsAnnotation = "k8s.ovn.org/open-default-ports"
const RequiredUDNNamespaceLabel = "k8s.ovn.org/primary-user-defined-network"

// NOTE: We are observing pod creation requests taking more than two minutes t
// reach the CNI for the CNI to do the necessary plumbing. This is causing tests
// to timeout since pod doesn't go into ready state.
// See https://issues.redhat.com/browse/OCPBUGS-48362 for details. We can revisit
// these values when that bug is fixed but given the Kubernetes test default for a
// pod to startup is 5mins: https://github.com/kubernetes/kubernetes/blob/60c4c2b2521fb454ce69dee737e3eb91a25e0535/test/e2e/framework/timeouts.go#L22-L23
// we are not too far from the mark or against test policy
const podReadyPollTimeout = 10 * time.Minute
const podReadyPollInterval = 6 * time.Second

// NOTE: Upstream, we use either the default of gomega which is 1sec polltimeout with 10ms pollinterval OR
// the tests have hardcoded values with 5sec being common for polltimeout and 10ms for pollinterval
// This is being changed to be 10seconds poll timeout to account for infrastructure complexity between
// OpenShift and KIND clusters. Also changing the polling interval to be 1 second so that in both
// Eventually and Consistently blocks we get at least 10 retries (10/1) in good conditions and 5 retries (10/2) in
// bad conditions since connectToServer util has a 2 second timeout.
// FIXME: Timeout increased to 30 seconds because default network controller does not receive the pod event after its annotations
// are updated. Reduce timeout back to sensible value once issue is understood.
const serverConnectPollTimeout = 30 * time.Second
const serverConnectPollInterval = 1 * time.Second

var _ = Describe("[sig-network][OCPFeatureGate:NetworkSegmentation][Feature:UserDefinedPrimaryNetworks]", func() {
	// TODO: so far, only the isolation tests actually require this PSA ... Feels wrong to run everything priviliged.
	// I've tried to have multiple kubeframeworks (from multiple OCs) running (with different project names) but
	// it didn't work.
	// disable automatic namespace creation, we need to add the required UDN label
	oc := exutil.NewCLIWithoutNamespace("network-segmentation-e2e")
	f := oc.KubeFramework()
	f.NamespacePodSecurityLevel = admissionapi.LevelPrivileged

	InOVNKubernetesContext(func() {
		const (
			nodeHostnameKey              = "kubernetes.io/hostname"
			port                         = 9000
			defaultPort                  = 8080
			userDefinedNetworkIPv4Subnet = "203.203.0.0/16"
			userDefinedNetworkIPv6Subnet = "2014:100:200::0/60"
			nadName                      = "gryffindor"

			udnCrReadyTimeout = 60 * time.Second
		)

		var (
			cs        clientset.Interface
			nadClient nadclient.K8sCniCncfIoV1Interface
		)

		BeforeEach(func() {
			cs = f.ClientSet

			var err error
			nadClient, err = nadclient.NewForConfig(f.ClientConfig())
			Expect(err).NotTo(HaveOccurred())
		})

		DescribeTableSubtree("created using",
			func(createNetworkFn func(c *networkAttachmentConfigParams) error) {

				DescribeTable(
					"can perform east/west traffic between nodes",
					func(
						netConfig *networkAttachmentConfigParams,
						clientPodConfig podConfiguration,
						serverPodConfig podConfiguration,
					) {
						var err error
						l := map[string]string{
							"e2e-framework": f.BaseName,
						}
						if netConfig.role == "primary" {
							l[RequiredUDNNamespaceLabel] = ""
						}
						ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
						Expect(err).NotTo(HaveOccurred())
						err = udnWaitForOpenShift(oc, ns.Name)
						Expect(err).NotTo(HaveOccurred())
						f.Namespace = ns

						netConfig.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfig.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						Expect(len(workerNodes)).To(BeNumerically(">=", 1))

						clientPodConfig.namespace = f.Namespace.Name
						clientPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[0].Name}
						serverPodConfig.namespace = f.Namespace.Name
						serverPodConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[len(workerNodes)-1].Name}

						By("creating the network")
						netConfig.namespace = f.Namespace.Name
						Expect(createNetworkFn(netConfig)).To(Succeed())

						By("creating client/server pods")
						runUDNPod(cs, f.Namespace.Name, serverPodConfig, nil)
						runUDNPod(cs, f.Namespace.Name, clientPodConfig, nil)

						var serverIP string
						for i, cidr := range strings.Split(netConfig.cidr, ",") {
							if cidr != "" {
								By("asserting the server pod has an IP from the configured range")
								serverIP, err = podIPsForUserDefinedPrimaryNetwork(
									cs,
									f.Namespace.Name,
									serverPodConfig.name,
									namespacedName(f.Namespace.Name, netConfig.name),
									i,
								)
								Expect(err).NotTo(HaveOccurred())
								const netPrefixLengthPerNode = 24
								By(fmt.Sprintf("asserting the server pod IP %v is from the configured range %v/%v", serverIP, cidr, netPrefixLengthPerNode))
								subnet, err := getNetCIDRSubnet(cidr)
								Expect(err).NotTo(HaveOccurred())
								Expect(inRange(subnet, serverIP)).To(Succeed())
							}

							By("asserting the *client* pod can contact the server pod exposed endpoint")
							namespacePodShouldReach(oc, f.Namespace.Name, clientPodConfig.name, formatHostAndPort(net.ParseIP(serverIP), port))
						}
					},
					Entry(
						"for two pods connected over a L2 primary UDN",
						&networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer2",
							role:     "primary",
						},
						*podConfig(
							"client-pod",
						),
						*podConfig("server-pod", withCommand(func() []string {
							return httpServerContainerCmd(port)
						})),
					),
					Entry(
						"two pods connected over a L3 primary UDN",
						&networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer3",
							role:     "primary",
						},
						*podConfig(
							"client-pod",
						),
						*podConfig("server-pod", withCommand(func() []string {
							return httpServerContainerCmd(port)
						})),
					),
				)

				DescribeTable(
					"is isolated from the default network",
					func(
						netConfigParams *networkAttachmentConfigParams,
						udnPodConfig podConfiguration,
					) {
						l := map[string]string{
							"e2e-framework": f.BaseName,
						}
						if netConfigParams.role == "primary" {
							l[RequiredUDNNamespaceLabel] = ""
						}
						ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
						Expect(err).NotTo(HaveOccurred())
						err = udnWaitForOpenShift(oc, ns.Name)
						Expect(err).NotTo(HaveOccurred())
						f.Namespace = ns
						By("Creating second namespace for default network pods")
						defaultNetNamespace := f.Namespace.Name + "-default"
						_, err = cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
							ObjectMeta: metav1.ObjectMeta{
								Name: defaultNetNamespace,
							},
						}, metav1.CreateOptions{})
						Expect(err).NotTo(HaveOccurred())
						defer func() {
							Expect(cs.CoreV1().Namespaces().Delete(context.Background(), defaultNetNamespace, metav1.DeleteOptions{})).To(Succeed())
						}()

						By("creating the network")
						netConfigParams.namespace = f.Namespace.Name
						// correctCIDRFamily makes use of the ginkgo framework so it needs to be in the testcase
						netConfigParams.cidr = correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet)
						Expect(createNetworkFn(netConfigParams)).To(Succeed())
						Expect(err).NotTo(HaveOccurred())

						udnPodConfig.namespace = f.Namespace.Name

						udnPod := runUDNPod(cs, f.Namespace.Name, udnPodConfig, func(pod *v1.Pod) {
							pod.Spec.Containers[0].ReadinessProbe = &v1.Probe{
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(port),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       1,
								// FIXME: On OCP we have seen readiness probe failures happening for the UDN pod which
								// causes immediate container restarts - the first readiness probe failure usually happens because
								// connection gets reset by the pod since normally a liveness probe fails first causing a
								// restart that also causes the readiness probes to start failing.
								// Hence increase the failure threshold to 3 tries.
								FailureThreshold: 3,
								TimeoutSeconds:   3,
							}
							pod.Spec.Containers[0].LivenessProbe = &v1.Probe{
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(port),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       1,
								// FIXME: On OCP we have seen liveness probe failures happening for the UDN pod which
								// causes immediate container restarts. Hence increase the failure threshold to 3 tries
								// TBD: We unfortunately don't know why the 1st liveness probe timesout - once we know the
								// why we could bring this back to 1 even though 1 is still aggressive.
								FailureThreshold: 3,
								// FIXME: On OCP, we have seen this flake in the CI; example:
								// Pod event: Type=Warning Reason=Unhealthy Message=Liveness probe failed: Get "http://[fd01:0:0:5::2ed]:9000/healthz":
								// context deadline exceeded (Client.Timeout exceeded while awaiting headers) LastTimestamp=2025-01-21 15:16:43 +0000 UTC Count=1
								// Pod event: Type=Normal Reason=Killing Message=Container agnhost-container failed liveness probe, will be restarted
								// LastTimestamp=2025-01-21 15:16:43 +0000 UTC Count=1
								// Pod event: Type=Warning Reason=Unhealthy Message=Readiness probe failed: Get "http://[fd01:0:0:5::2ed]:9000/healthz":
								// context deadline exceeded (Client.Timeout exceeded while awaiting headers) LastTimestamp=2025-01-21 15:16:43 +0000 UTC Count=1
								// Pod event: Type=Warning Reason=Unhealthy Message=Readiness probe failed: Get "http://[fd01:0:0:5::2ed]:9000/healthz":
								// read tcp [fd01:0:0:5::2]:33400->[fd01:0:0:5::2ed]:9000: read: connection reset by peer LastTimestamp=2025-01-21 15:16:43 +0000 UTC Count=1
								// While we don't know why 1second wasn't enough to receive the headers for the liveness probe
								// it is clear the TCP conn is getting established but 1second is not enough to complete the probe.
								// Let's increase the timeout to 3seconds till we understand what causes the 1st probe failure.
								TimeoutSeconds: 3,
							}
							pod.Spec.Containers[0].StartupProbe = &v1.Probe{
								ProbeHandler: v1.ProbeHandler{
									HTTPGet: &v1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(port),
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       1,
								FailureThreshold:    3,
								// FIXME: Figure out why it sometimes takes more than 3seconds for the healthcheck to complete
								TimeoutSeconds: 3,
							}
							// add NET_ADMIN to change pod routes
							pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
								Capabilities: &v1.Capabilities{
									Add: []v1.Capability{"NET_ADMIN"},
								},
							}
						})

						const podGenerateName = "udn-test-pod-"
						By("creating default network pod")
						defaultPod := frameworkpod.CreateExecPodOrFail(
							context.Background(),
							f.ClientSet,
							defaultNetNamespace,
							podGenerateName,
							func(pod *v1.Pod) {
								pod.Spec.Containers[0].Args = []string{"netexec"}
								setRuntimeDefaultPSA(pod)
							},
						)

						By("creating default network client pod")
						defaultClientPod := frameworkpod.CreateExecPodOrFail(
							context.Background(),
							f.ClientSet,
							defaultNetNamespace,
							podGenerateName,
							func(pod *v1.Pod) {
								setRuntimeDefaultPSA(pod)
							},
						)

						udnIPv4, udnIPv6, err := podIPsForDefaultNetwork(
							cs,
							f.Namespace.Name,
							udnPod.GetName(),
						)
						Expect(err).NotTo(HaveOccurred())

						for _, destIP := range []string{udnIPv4, udnIPv6} {
							if destIP == "" {
								continue
							}
							// positive case for UDN pod is a successful healthcheck, checked later
							By("checking the default network pod can't reach UDN pod on IP " + destIP)
							Consistently(func() bool {
								return connectToServer(podConfiguration{namespace: defaultPod.Namespace, name: defaultPod.Name}, destIP, port) != nil
							}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
						}

						defaultIPv4, defaultIPv6, err := podIPsForDefaultNetwork(
							cs,
							defaultPod.Namespace,
							defaultPod.Name,
						)
						Expect(err).NotTo(HaveOccurred())

						for _, destIP := range []string{defaultIPv4, defaultIPv6} {
							if destIP == "" {
								continue
							}
							By("checking the default network client pod can reach default pod on IP " + destIP)
							Eventually(func() bool {
								return connectToServer(podConfiguration{namespace: defaultClientPod.Namespace, name: defaultClientPod.Name}, destIP, defaultPort) == nil
							}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
							By("checking the UDN pod can't reach the default network pod on IP " + destIP)
							Consistently(func() bool {
								return connectToServer(udnPodConfig, destIP, defaultPort) != nil
							}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
						}

						// connectivity check is run every second + 1sec initialDelay
						// By this time we have spent at least 20 seconds doing the above consistently checks
						udnPod, err = cs.CoreV1().Pods(udnPod.Namespace).Get(context.Background(), udnPod.Name, metav1.GetOptions{})
						Expect(err).NotTo(HaveOccurred())
						Expect(udnPod.Status.ContainerStatuses[0].RestartCount).To(Equal(int32(0)))

						By("asserting healthcheck works (kubelet can access the UDN pod)")
						// The pod should be ready
						Expect(podutils.IsPodReady(udnPod)).To(BeTrue())

						// TODO
						//By("checking non-kubelet default network host process can't reach the UDN pod")

						By("asserting UDN pod can't reach host via default network interface")
						// Now try to reach the host from the UDN pod
						defaultPodHostIP := udnPod.Status.HostIPs
						for _, hostIP := range defaultPodHostIP {
							By("checking the UDN pod can't reach the host on IP " + hostIP.IP)
							ping := "ping"
							if utilnet.IsIPv6String(hostIP.IP) {
								ping = "ping6"
							}
							Consistently(func() bool {
								_, err := e2ekubectl.RunKubectl(udnPod.Namespace, "exec", udnPod.Name, "--",
									ping, "-I", "eth0", "-c", "1", "-W", "1", hostIP.IP,
								)
								return err == nil
							}, 4*time.Second, 1*time.Second).Should(BeFalse())
						}

						By("asserting UDN pod can reach the kapi service in the default network")
						// Use the service name to get test the DNS access
						Consistently(func() bool {
							_, err := e2ekubectl.RunKubectl(
								udnPodConfig.namespace,
								"exec",
								udnPodConfig.name,
								"--",
								"curl",
								"--connect-timeout",
								// FIXME: We have seen in OCP CI that it can take two seconds or maybe more
								// for a single curl to succeed. Example:
								//     STEP: asserting UDN pod can reach the kapi service in the default network @ 01/20/25 00:38:42.32
								// I0120 00:38:42.320808 70120 builder.go:121] Running '/usr/bin/kubectl
								// --server=https://api.ci-op-bkg2qwwq-4edbf.XXXXXXXXXXXXXXXXXXXXXX:6443 --kubeconfig=/tmp/kubeconfig-1734723086
								// --namespace=e2e-test-network-segmentation-e2e-kzdw7 exec udn-pod -- curl --connect-timeout 2 --insecure https://kubernetes.default/healthz'
								// I0120 00:38:44.108334 70120 builder.go:146] stderr: "  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current\n                                 Dload  Upload   Total   Spent    Left  Speed\n\r  0     0    0     0    0     0      0      0 --:--:-- --:--:-- --:--:--     0\r100     2  100     2    0     0      9      0 --:--:-- --:--:-- --:--:--     9\r100     2  100     2    0     0      9      0 --:--:-- --:--:-- --:--:--     9\n"
								// I0120 00:38:44.108415 70120 builder.go:147] stdout: "ok" --> 2 seconds later
								// I0120 00:38:45.109237 70120 builder.go:121] Running '/usr/bin/kubectl
								// --server=https://api.ci-op-bkg2qwwq-4edbf.XXXXXXXXXXXXXXXXXXXXXX:6443 --kubeconfig=/tmp/kubeconfig-1734723086
								// --namespace=e2e-test-network-segmentation-e2e-kzdw7 exec udn-pod -- curl --connect-timeout 2 --insecure https://kubernetes.default/healthz'
								// I0120 00:38:48.460089 70120 builder.go:135] rc: 28
								// around the same time we have observed OVS issues like:
								// Jan 20 00:38:45.329999 ci-op-bkg2qwwq-4edbf-xv8kb-worker-b-flqxd ovs-vswitchd[1094]: ovs|03661|timeval|WARN|context switches: 0 voluntary, 695 involuntary
								// Jan 20 00:38:45.329967 ci-op-bkg2qwwq-4edbf-xv8kb-worker-b-flqxd ovs-vswitchd[1094]: ovs|03660|timeval|WARN|Unreasonably long 1730ms poll interval (32ms user, 903ms system)
								// which might need more investigation. Bumping the timeout to 5seconds can help with this
								// but we need to figure out what exactly is causing random timeouts in CI when trying to reach kapi-server
								// sometimes we have also seen more than 2seconds being taken for the timeout which also needs to be investigated:
								// I0118 13:35:50.419638 87083 builder.go:121] Running '/usr/bin/kubectl
								// --server=https://api.ostest.test.metalkube.org:6443 --kubeconfig=/tmp/secret/kubeconfig
								// --namespace=e2e-test-network-segmentation-e2e-d4fzk exec udn-pod -- curl --connect-timeout 2 --insecure https://kubernetes.default/healthz'
								// I0118 13:35:54.093268 87083 builder.go:135] rc: 28 --> takes close to 4seconds?
								"5",
								"--insecure",
								"https://kubernetes.default/healthz")
							return err == nil
						}, 15*time.Second, 3*time.Second).Should(BeTrue())

						By("asserting UDN pod can't reach default services via default network interface")
						// route setup is already done, get kapi IPs
						kapi, err := cs.CoreV1().Services("default").Get(context.Background(), "kubernetes", metav1.GetOptions{})
						Expect(err).NotTo(HaveOccurred())
						for _, kapiIP := range kapi.Spec.ClusterIPs {
							By("checking the UDN pod can't reach kapi service on IP " + kapiIP)
							Consistently(func() bool {
								_, err := e2ekubectl.RunKubectl(
									udnPodConfig.namespace,
									"exec",
									udnPodConfig.name,
									"--",
									"curl",
									"--connect-timeout",
									"2",
									"--interface",
									"eth0",
									"--insecure",
									fmt.Sprintf("https://%s/healthz", kapiIP))
								return err != nil
							}, 5*time.Second, 1*time.Second).Should(BeTrue())
						}
					},
					Entry(
						"with L2 primary UDN",
						&networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer2",
							role:     "primary",
						},
						*podConfig("udn-pod", withCommand(func() []string {
							return httpServerContainerCmd(port)
						})),
					),
					Entry(
						"with L3 primary UDN",
						&networkAttachmentConfigParams{
							name:     nadName,
							topology: "layer3",
							role:     "primary",
						},
						*podConfig("udn-pod", withCommand(func() []string {
							return httpServerContainerCmd(port)
						})),
					),
				)
				DescribeTable(
					"isolates overlapping CIDRs",
					func(
						topology string,
						numberOfPods int,
						userDefinedv4Subnet string,
						userDefinedv6Subnet string,

					) {
						l := map[string]string{
							"e2e-framework":           f.BaseName,
							RequiredUDNNamespaceLabel: "",
						}
						ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
						Expect(err).NotTo(HaveOccurred())
						err = udnWaitForOpenShift(oc, ns.Name)
						Expect(err).NotTo(HaveOccurred())
						f.Namespace = ns
						red := "red"
						blue := "blue"

						namespaceRed := f.Namespace.Name + "-" + red
						namespaceBlue := f.Namespace.Name + "-" + blue

						for _, namespace := range []string{namespaceRed, namespaceBlue} {
							By("Creating namespace " + namespace)
							_, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
								ObjectMeta: metav1.ObjectMeta{
									Name:   namespace,
									Labels: l,
								},
							}, metav1.CreateOptions{})
							Expect(err).NotTo(HaveOccurred())
							defer func() {
								By("Removing namespace " + namespace)
								Expect(cs.CoreV1().Namespaces().Delete(
									context.Background(),
									namespace,
									metav1.DeleteOptions{},
								)).To(Succeed())
							}()
						}
						networkNamespaceMap := map[string]string{namespaceRed: red, namespaceBlue: blue}
						for namespace, network := range networkNamespaceMap {
							By("creating the network " + network + " in namespace " + namespace)
							netConfig := &networkAttachmentConfigParams{
								topology:  topology,
								cidr:      correctCIDRFamily(oc, userDefinedv4Subnet, userDefinedv6Subnet),
								role:      "primary",
								namespace: namespace,
								name:      network,
							}

							Expect(createNetworkFn(netConfig)).To(Succeed())
							// update the name because createNetworkFn may mutate the netConfig.name
							// for cluster scope objects (i.g.: CUDN cases) to enable parallel testing.
							networkNamespaceMap[namespace] = netConfig.name

						}
						red = networkNamespaceMap[namespaceRed]
						blue = networkNamespaceMap[namespaceBlue]

						workerNodes, err := getWorkerNodesOrdered(cs)
						Expect(err).NotTo(HaveOccurred())
						pods := []*v1.Pod{}
						redIPs := map[string]bool{}
						blueIPs := map[string]bool{}
						podIPs := []string{}
						bluePort := int(9091)
						redPort := int(9092)
						for namespace, network := range networkNamespaceMap {
							for i := 0; i < numberOfPods; i++ {
								httpServerPort := redPort
								if network != red {
									httpServerPort = bluePort
								}
								podConfig := *podConfig(
									fmt.Sprintf("%s-pod-%d", network, i),
									withCommand(func() []string {
										return httpServerContainerCmd(uint16(httpServerPort))
									}),
								)
								podConfig.namespace = namespace
								//ensure testing accross nodes
								if i%2 == 0 {
									podConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[0].Name}

								} else {

									podConfig.nodeSelector = map[string]string{nodeHostnameKey: workerNodes[len(workerNodes)-1].Name}
								}
								By("creating pod " + podConfig.name + " in " + podConfig.namespace)
								pod := runUDNPod(
									cs,
									podConfig.namespace,
									podConfig,
									func(pod *v1.Pod) {
										setRuntimeDefaultPSA(pod)
									})
								pods = append(pods, pod)
								podIP, err := podIPsForUserDefinedPrimaryNetwork(
									cs,
									pod.Namespace,
									pod.Name,
									namespacedName(namespace, network),
									0,
								)
								Expect(err).NotTo(HaveOccurred())
								podIPs = append(podIPs, podIP)
								if network == red {
									redIPs[podIP] = true
								} else {
									blueIPs[podIP] = true
								}
							}
						}

						By("ensuring pods only communicate with pods in their network")
						for _, pod := range pods {
							isRedPod := strings.Contains(pod.Name, red)
							expectedHostname := red
							if !isRedPod {
								expectedHostname = blue
							}
							for _, ip := range podIPs {
								isRedIP := redIPs[ip]
								httpServerPort := redPort
								if !isRedIP {
									httpServerPort = bluePort
								}
								sameNetwork := isRedPod == isRedIP
								if !sameNetwork {
									_, err := connectToServerWithPath(pod.Namespace, pod.Name, ip, "/hostname", httpServerPort)
									Expect(err).Should(HaveOccurred(), "should isolate from different networks")
								} else {
									Eventually(func(g Gomega) {
										result, err := connectToServerWithPath(pod.Namespace, pod.Name, ip, "/hostname", httpServerPort)
										g.Expect(err).NotTo(HaveOccurred())
										g.Expect(result).To(ContainSubstring(expectedHostname))
									}).
										WithTimeout(serverConnectPollTimeout).
										WithPolling(serverConnectPollInterval).
										Should(Succeed(), "should not isolate from same network")
								}
							}
						}
					},
					// can completely fill the L2 topology because it does not depend on the size of the clusters hostsubnet
					Entry(
						"with L2 primary UDN",
						"layer2",
						4,
						"203.203.0.0/29",
						"2014:100:200::0/125",
					),
					// limit the number of pods to 5
					Entry(
						"with L3 primary UDN",
						"layer3",
						5,
						userDefinedNetworkIPv4Subnet,
						userDefinedNetworkIPv6Subnet,
					),
				)
			},
			Entry("NetworkAttachmentDefinitions", func(c *networkAttachmentConfigParams) error {
				netConfig := newNetworkAttachmentConfig(*c)
				nad := generateNAD(netConfig)
				_, err := nadClient.NetworkAttachmentDefinitions(c.namespace).Create(context.Background(), nad, metav1.CreateOptions{})
				return err
			}),
			Entry("UserDefinedNetwork", func(c *networkAttachmentConfigParams) error {
				udnManifest := generateUserDefinedNetworkManifest(c)
				cleanup, err := createManifest(c.namespace, udnManifest)
				DeferCleanup(cleanup)
				Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), c.namespace, c.name), udnCrReadyTimeout, time.Second).Should(Succeed())
				return err
			}),
			Entry("ClusterUserDefinedNetwork", func(c *networkAttachmentConfigParams) error {
				cudnName := randomNetworkMetaName()
				c.name = cudnName
				cudnManifest := generateClusterUserDefinedNetworkManifest(c)
				cleanup, err := createManifest("", cudnManifest)
				DeferCleanup(func() {
					cleanup()
					By(fmt.Sprintf("delete pods in %s namespace to unblock CUDN CR & associate NAD deletion", c.namespace))
					Expect(cs.CoreV1().Pods(c.namespace).DeleteCollection(context.Background(), metav1.DeleteOptions{}, metav1.ListOptions{})).To(Succeed())
					_, err := e2ekubectl.RunKubectl("", "delete", "clusteruserdefinednetwork", cudnName, "--wait", fmt.Sprintf("--timeout=%ds", 120))
					Expect(err).NotTo(HaveOccurred())
				})
				Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), c.name), udnCrReadyTimeout, time.Second).Should(Succeed())
				return err
			}),
		)

		Context("UserDefinedNetwork CRD controller", func() {
			const (
				testUdnName                = "test-net"
				userDefinedNetworkResource = "userdefinednetwork"
			)

			BeforeEach(func() {
				namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
					"e2e-framework": f.BaseName,
				})
				Expect(err).NotTo(HaveOccurred())
				err = udnWaitForOpenShift(oc, namespace.Name)
				Expect(err).NotTo(HaveOccurred())
				f.Namespace = namespace

				By("create tests UserDefinedNetwork")
				cleanup, err := createManifest(f.Namespace.Name, newUserDefinedNetworkManifest(testUdnName))
				DeferCleanup(cleanup)
				Expect(err).NotTo(HaveOccurred())
				Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), f.Namespace.Name, testUdnName), udnCrReadyTimeout, time.Second).Should(Succeed())
			})

			It("should create NetworkAttachmentDefinition according to spec", func() {
				udnUidRaw, err := e2ekubectl.RunKubectl(f.Namespace.Name, "get", userDefinedNetworkResource, testUdnName, "-o", "jsonpath='{.metadata.uid}'")
				Expect(err).NotTo(HaveOccurred(), "should get the UserDefinedNetwork UID")
				testUdnUID := strings.Trim(udnUidRaw, "'")

				By("verify a NetworkAttachmentDefinition is created according to spec")
				assertNetAttachDefManifest(nadClient, f.Namespace.Name, testUdnName, testUdnUID)
			})

			It("should delete NetworkAttachmentDefinition when UserDefinedNetwork is deleted", func() {
				By("delete UserDefinedNetwork")
				_, err := e2ekubectl.RunKubectl(f.Namespace.Name, "delete", userDefinedNetworkResource, testUdnName)
				Expect(err).NotTo(HaveOccurred())

				By("verify a NetworkAttachmentDefinition has been deleted")
				Eventually(func() bool {
					_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Get(context.Background(), testUdnName, metav1.GetOptions{})
					return err != nil && kerrors.IsNotFound(err)
				}, time.Second*3, time.Second*1).Should(BeTrue(),
					"NetworkAttachmentDefinition should be deleted following UserDefinedNetwork deletion")
			})

			Context("pod connected to UserDefinedNetwork", func() {
				const testPodName = "test-pod-udn"

				var (
					udnInUseDeleteTimeout = 65 * time.Second
					deleteNetworkTimeout  = 5 * time.Second
					deleteNetworkInterval = 1 * time.Second
				)

				BeforeEach(func() {
					By("create pod")
					networkAttachments := []nadapi.NetworkSelectionElement{
						{Name: testUdnName, Namespace: f.Namespace.Name},
					}
					cfg := podConfig(testPodName, withNetworkAttachment(networkAttachments))
					cfg.namespace = f.Namespace.Name
					runUDNPod(cs, f.Namespace.Name, *cfg, nil)
				})

				It("cannot be deleted when being used", func() {
					By("verify UserDefinedNetwork cannot be deleted")
					cmd := e2ekubectl.NewKubectlCommand(f.Namespace.Name, "delete", userDefinedNetworkResource, testUdnName)
					cmd.WithTimeout(time.NewTimer(deleteNetworkTimeout).C)
					_, err := cmd.Exec()
					Expect(err).To(HaveOccurred(),
						"should fail to delete UserDefinedNetwork when used")

					By("verify UserDefinedNetwork associated NetworkAttachmentDefinition cannot be deleted")
					Eventually(func() error {
						ctx, cancel := context.WithTimeout(context.Background(), deleteNetworkTimeout)
						defer cancel()
						_ = nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Delete(ctx, testUdnName, metav1.DeleteOptions{})
						_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Get(ctx, testUdnName, metav1.GetOptions{})
						return err
					}, udnInUseDeleteTimeout, deleteNetworkInterval).ShouldNot(HaveOccurred(),
						"should fail to delete UserDefinedNetwork associated NetworkAttachmentDefinition when used")

					By("verify UserDefinedNetwork status reports consuming pod")
					err = validateUDNStatusReportsConsumers(oc.AdminDynamicClient(), f.Namespace.Name, testUdnName, testPodName)
					Expect(err).ToNot(HaveOccurred())

					By("delete test pod")
					err = cs.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), testPodName, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())

					By("verify UserDefinedNetwork has been deleted")
					Eventually(func() error {
						_, err := e2ekubectl.RunKubectl(f.Namespace.Name, "get", userDefinedNetworkResource, testUdnName)
						return err
					}, udnInUseDeleteTimeout, deleteNetworkInterval).Should(HaveOccurred(),
						"UserDefinedNetwork should be deleted following test pod deletion")

					By("verify UserDefinedNetwork associated NetworkAttachmentDefinition has been deleted")
					Eventually(func() bool {
						_, err := nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Get(context.Background(), testUdnName, metav1.GetOptions{})
						return err != nil && kerrors.IsNotFound(err)
					}, deleteNetworkTimeout, deleteNetworkInterval).Should(BeTrue(),
						"NetworkAttachmentDefinition should be deleted following UserDefinedNetwork deletion")
				})
			})
		})

		It("when primary network exist, UserDefinedNetwork status should report not-ready", func() {
			const (
				primaryNadName = "cluster-primary-net"
				primaryUdnName = "primary-net"
			)

			l := map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			}
			ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
			Expect(err).NotTo(HaveOccurred())
			err = udnWaitForOpenShift(oc, ns.Name)
			Expect(err).NotTo(HaveOccurred())
			f.Namespace = ns

			By("create primary network NetworkAttachmentDefinition")
			primaryNetNad := generateNAD(newNetworkAttachmentConfig(networkAttachmentConfigParams{
				role:        "primary",
				topology:    "layer3",
				name:        primaryNadName,
				networkName: primaryNadName,
				cidr:        correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet),
			}))
			_, err = nadClient.NetworkAttachmentDefinitions(f.Namespace.Name).Create(context.Background(), primaryNetNad, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("create primary network UserDefinedNetwork")
			cleanup, err := createManifest(f.Namespace.Name, newPrimaryUserDefinedNetworkManifest(oc, primaryUdnName))
			DeferCleanup(cleanup)
			Expect(err).NotTo(HaveOccurred())

			expectedMessage := fmt.Sprintf("primary network already exist in namespace %q: %q", f.Namespace.Name, primaryNadName)
			Eventually(func(g Gomega) []metav1.Condition {
				conditionsJSON, err := e2ekubectl.RunKubectl(f.Namespace.Name, "get", "userdefinednetwork", primaryUdnName, "-o", "jsonpath={.status.conditions}")
				g.Expect(err).NotTo(HaveOccurred())
				var actualConditions []metav1.Condition
				g.Expect(json.Unmarshal([]byte(conditionsJSON), &actualConditions)).To(Succeed())
				return normalizeConditions(actualConditions)
			}, 5*time.Second, 1*time.Second).Should(SatisfyAny(
				ConsistOf(metav1.Condition{
					Type:    "NetworkCreated",
					Status:  metav1.ConditionFalse,
					Reason:  "SyncError",
					Message: expectedMessage,
				}),
				ConsistOf(metav1.Condition{
					Type:    "NetworkReady",
					Status:  metav1.ConditionFalse,
					Reason:  "SyncError",
					Message: expectedMessage,
				}),
			))
		})

		Context("ClusterUserDefinedNetwork CRD Controller", func() {
			const clusterUserDefinedNetworkResource = "clusteruserdefinednetwork"

			var testTenantNamespaces []string
			var defaultNetNamespace *v1.Namespace

			BeforeEach(func() {
				namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
					"e2e-framework":           f.BaseName,
					RequiredUDNNamespaceLabel: "",
				})
				f.Namespace = namespace
				Expect(err).NotTo(HaveOccurred())
				err = udnWaitForOpenShift(oc, namespace.Name)
				Expect(err).NotTo(HaveOccurred())
				testTenantNamespaces = []string{
					f.Namespace.Name + "blue",
					f.Namespace.Name + "red",
				}

				By("Creating test tenants namespaces")
				for _, nsName := range testTenantNamespaces {
					_, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name:   nsName,
							Labels: map[string]string{RequiredUDNNamespaceLabel: ""},
						}}, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() error {
						err := cs.CoreV1().Namespaces().Delete(context.Background(), nsName, metav1.DeleteOptions{})
						return err
					})
				}
				// default cluster network namespace, for use when only testing secondary UDNs/NADs
				defaultNetNamespace = &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: f.Namespace.Name + "-default",
					},
				}
				f.AddNamespacesToDelete(defaultNetNamespace)
				_, err = cs.CoreV1().Namespaces().Create(context.Background(), defaultNetNamespace, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				testTenantNamespaces = append(testTenantNamespaces, defaultNetNamespace.Name)
			})

			var testClusterUdnName string

			BeforeEach(func() {
				testClusterUdnName = randomNetworkMetaName()
				By("create test CR")
				cleanup, err := createManifest("", newClusterUDNManifest(testClusterUdnName, testTenantNamespaces...))
				DeferCleanup(func() error {
					cleanup()
					_, _ = e2ekubectl.RunKubectl("", "delete", clusterUserDefinedNetworkResource, testClusterUdnName)
					Eventually(func() error {
						_, err := e2ekubectl.RunKubectl("", "get", clusterUserDefinedNetworkResource, testClusterUdnName)
						return err
					}, 1*time.Minute, 3*time.Second).Should(MatchError(ContainSubstring(fmt.Sprintf("clusteruserdefinednetworks.k8s.ovn.org %q not found", testClusterUdnName))))
					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), testClusterUdnName), udnCrReadyTimeout, time.Second).Should(Succeed())
			})

			It("should create NAD according to spec in each target namespace and report active namespaces", func() {
				Eventually(
					validateClusterUDNStatusReportsActiveNamespacesFunc(oc.AdminDynamicClient(), testClusterUdnName, testTenantNamespaces...),
					1*time.Minute, 3*time.Second).Should(Succeed())

				udnUidRaw, err := e2ekubectl.RunKubectl("", "get", clusterUserDefinedNetworkResource, testClusterUdnName, "-o", "jsonpath='{.metadata.uid}'")
				Expect(err).NotTo(HaveOccurred(), "should get the ClsuterUserDefinedNetwork UID")
				testUdnUID := strings.Trim(udnUidRaw, "'")

				By("verify a NetworkAttachmentDefinition is created according to spec")
				for _, testNsName := range testTenantNamespaces {
					assertClusterNADManifest(nadClient, testNsName, testClusterUdnName, testUdnUID)
				}
			})

			It("when CR is deleted, should delete all managed NAD in each target namespace", func() {
				By("delete test CR")
				_, err := e2ekubectl.RunKubectl("", "delete", clusterUserDefinedNetworkResource, testClusterUdnName)
				Expect(err).NotTo(HaveOccurred())

				for _, nsName := range testTenantNamespaces {
					By(fmt.Sprintf("verify a NAD has been deleted from namesapce %q", nsName))
					Eventually(func() bool {
						_, err := nadClient.NetworkAttachmentDefinitions(nsName).Get(context.Background(), testClusterUdnName, metav1.GetOptions{})
						return err != nil && kerrors.IsNotFound(err)
					}, time.Second*3, time.Second*1).Should(BeTrue(),
						"NADs in target namespaces should be deleted following ClusterUserDefinedNetwork deletion")
				}
			})

			It("should create NAD in new created namespaces that apply to namespace-selector", func() {
				testNewNs := f.Namespace.Name + "green"

				By("add new target namespace to CR namespace-selector")
				patch := fmt.Sprintf(`[{"op": "add", "path": "./spec/namespaceSelector/matchExpressions/0/values/-", "value": "%s"}]`, testNewNs)
				_, err := e2ekubectl.RunKubectl("", "patch", clusterUserDefinedNetworkResource, testClusterUdnName, "--type=json", "-p="+patch)
				Expect(err).NotTo(HaveOccurred())
				Eventually(clusterUserDefinedNetworkReadyFunc(oc.AdminDynamicClient(), testClusterUdnName), udnCrReadyTimeout, time.Second).Should(Succeed())
				Eventually(
					validateClusterUDNStatusReportsActiveNamespacesFunc(oc.AdminDynamicClient(), testClusterUdnName, testTenantNamespaces...),
					1*time.Minute, 3*time.Second).Should(Succeed())

				By("create the new target namespace")
				_, err = cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   testNewNs,
						Labels: map[string]string{RequiredUDNNamespaceLabel: ""},
					}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() error {
					err := cs.CoreV1().Namespaces().Delete(context.Background(), testNewNs, metav1.DeleteOptions{})
					return err
				})

				expectedActiveNamespaces := append(testTenantNamespaces, testNewNs)
				Eventually(
					validateClusterUDNStatusReportsActiveNamespacesFunc(oc.AdminDynamicClient(), testClusterUdnName, expectedActiveNamespaces...),
					1*time.Minute, 3*time.Second).Should(Succeed())

				udnUidRaw, err := e2ekubectl.RunKubectl("", "get", clusterUserDefinedNetworkResource, testClusterUdnName, "-o", "jsonpath='{.metadata.uid}'")
				Expect(err).NotTo(HaveOccurred(), "should get the ClsuterUserDefinedNetwork UID")
				testUdnUID := strings.Trim(udnUidRaw, "'")

				By("verify a NAD exist in new namespace according to spec")
				assertClusterNADManifest(nadClient, testNewNs, testClusterUdnName, testUdnUID)
			})

			When("namespace-selector is mutated", func() {
				It("should create NAD in namespaces that apply to mutated namespace-selector", func() {
					testNewNs := f.Namespace.Name + "green"

					By("create new namespace")
					_, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
						ObjectMeta: metav1.ObjectMeta{
							Name:   testNewNs,
							Labels: map[string]string{RequiredUDNNamespaceLabel: ""},
						}}, metav1.CreateOptions{})
					Expect(err).NotTo(HaveOccurred())
					DeferCleanup(func() error {
						err := cs.CoreV1().Namespaces().Delete(context.Background(), testNewNs, metav1.DeleteOptions{})
						return err
					})

					By("add new namespace to CR namespace-selector")
					patch := fmt.Sprintf(`[{"op": "add", "path": "./spec/namespaceSelector/matchExpressions/0/values/-", "value": "%s"}]`, testNewNs)
					_, err = e2ekubectl.RunKubectl("", "patch", clusterUserDefinedNetworkResource, testClusterUdnName, "--type=json", "-p="+patch)
					Expect(err).NotTo(HaveOccurred())

					By("verify status reports the new added namespace as active")
					expectedActiveNs := append(testTenantNamespaces, testNewNs)
					Eventually(
						validateClusterUDNStatusReportsActiveNamespacesFunc(oc.AdminDynamicClient(), testClusterUdnName, expectedActiveNs...),
						1*time.Minute, 3*time.Second).Should(Succeed())

					By("verify a NAD is created in new target namespace according to spec")
					udnUidRaw, err := e2ekubectl.RunKubectl("", "get", clusterUserDefinedNetworkResource, testClusterUdnName, "-o", "jsonpath='{.metadata.uid}'")
					Expect(err).NotTo(HaveOccurred(), "should get the ClusterUserDefinedNetwork UID")
					testUdnUID := strings.Trim(udnUidRaw, "'")
					assertClusterNADManifest(nadClient, testNewNs, testClusterUdnName, testUdnUID)
				})

				It("should delete managed NAD in namespaces that no longer apply to namespace-selector", func() {
					By("remove one active namespace from CR namespace-selector")
					activeTenantNs := testTenantNamespaces[1]
					patch := fmt.Sprintf(`[{"op": "replace", "path": "./spec/namespaceSelector/matchExpressions/0/values", "value": [%q]}]`, activeTenantNs)
					_, err := e2ekubectl.RunKubectl("", "patch", clusterUserDefinedNetworkResource, testClusterUdnName, "--type=json", "-p="+patch)
					Expect(err).NotTo(HaveOccurred())

					By("verify status reports remained target namespaces only as active")
					expectedActiveNs := []string{activeTenantNs}
					Eventually(
						validateClusterUDNStatusReportsActiveNamespacesFunc(oc.AdminDynamicClient(), testClusterUdnName, expectedActiveNs...),
						1*time.Minute, 3*time.Second).Should(Succeed())

					removedTenantNs := testTenantNamespaces[0]
					By("verify managed NAD not exist in removed target namespace")
					Eventually(func() bool {
						_, err := nadClient.NetworkAttachmentDefinitions(removedTenantNs).Get(context.Background(), testClusterUdnName, metav1.GetOptions{})
						return err != nil && kerrors.IsNotFound(err)
					}, time.Second*300, time.Second*1).Should(BeTrue(),
						"NAD in target namespaces should be deleted following CR namespace-selector mutation")
				})
			})

			Context("pod connected to ClusterUserDefinedNetwork", func() {
				const testPodName = "test-pod-cluster-udn"

				var (
					udnInUseDeleteTimeout = 65 * time.Second
					deleteNetworkTimeout  = 5 * time.Second
					deleteNetworkInterval = 1 * time.Second

					inUseNetTestTenantNamespace string
				)

				BeforeEach(func() {
					inUseNetTestTenantNamespace = defaultNetNamespace.Name

					By("create pod in one of the test tenant namespaces")
					networkAttachments := []nadapi.NetworkSelectionElement{
						{Name: testClusterUdnName, Namespace: inUseNetTestTenantNamespace},
					}
					cfg := podConfig(testPodName, withNetworkAttachment(networkAttachments))
					cfg.namespace = inUseNetTestTenantNamespace
					runUDNPod(cs, inUseNetTestTenantNamespace, *cfg, setRuntimeDefaultPSA)
				})

				It("CR & managed NADs cannot be deleted when being used", func() {
					By("verify CR cannot be deleted")
					cmd := e2ekubectl.NewKubectlCommand("", "delete", clusterUserDefinedNetworkResource, testClusterUdnName)
					cmd.WithTimeout(time.NewTimer(deleteNetworkTimeout).C)
					_, err := cmd.Exec()
					Expect(err).To(HaveOccurred(), "should fail to delete ClusterUserDefinedNetwork when used")

					By("verify CR associate NAD cannot be deleted")
					Eventually(func() error {
						ctx, cancel := context.WithTimeout(context.Background(), deleteNetworkTimeout)
						defer cancel()
						_ = nadClient.NetworkAttachmentDefinitions(inUseNetTestTenantNamespace).Delete(ctx, testClusterUdnName, metav1.DeleteOptions{})
						_, err := nadClient.NetworkAttachmentDefinitions(inUseNetTestTenantNamespace).Get(ctx, testClusterUdnName, metav1.GetOptions{})
						return err
					}, udnInUseDeleteTimeout, deleteNetworkInterval).ShouldNot(HaveOccurred(),
						"should fail to delete UserDefinedNetwork associated NetworkAttachmentDefinition when used")

					By("verify CR status reports consuming pod")
					err = validateClusterUDNStatusReportConsumers(oc.AdminDynamicClient(), testClusterUdnName, inUseNetTestTenantNamespace, testPodName)
					Expect(err).NotTo(HaveOccurred())

					By("delete test pod")
					err = cs.CoreV1().Pods(inUseNetTestTenantNamespace).Delete(context.Background(), testPodName, metav1.DeleteOptions{})
					Expect(err).ToNot(HaveOccurred())

					By("verify CR is gone")
					Eventually(func() error {
						_, err := e2ekubectl.RunKubectl("", "get", clusterUserDefinedNetworkResource, testClusterUdnName)
						return err
					}, udnInUseDeleteTimeout, deleteNetworkInterval).Should(HaveOccurred(),
						"ClusterUserDefinedNetwork should be deleted following test pod deletion")

					By("verify CR associate NADs are gone")
					for _, nsName := range testTenantNamespaces {
						Eventually(func() bool {
							_, err := nadClient.NetworkAttachmentDefinitions(nsName).Get(context.Background(), testClusterUdnName, metav1.GetOptions{})
							return err != nil && kerrors.IsNotFound(err)
						}, deleteNetworkTimeout, deleteNetworkInterval).Should(BeTrue(),
							"NADs in target namespaces should be deleted following ClusterUserDefinedNetwork deletion")
					}
				})
			})
		})

		It("when primary network exist, ClusterUserDefinedNetwork status should report not-ready", func() {
			namespace, err := f.CreateNamespace(context.TODO(), f.BaseName, map[string]string{
				"e2e-framework":           f.BaseName,
				RequiredUDNNamespaceLabel: "",
			})
			Expect(err).NotTo(HaveOccurred())
			err = udnWaitForOpenShift(oc, namespace.Name)
			Expect(err).NotTo(HaveOccurred())
			f.Namespace = namespace
			testTenantNamespaces := []string{
				f.Namespace.Name + "blue",
				f.Namespace.Name + "red",
			}
			By("Creating test tenants namespaces")
			for _, nsName := range testTenantNamespaces {
				_, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name:   nsName,
						Labels: map[string]string{RequiredUDNNamespaceLabel: ""},
					}}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				DeferCleanup(func() error {
					err := cs.CoreV1().Namespaces().Delete(context.Background(), nsName, metav1.DeleteOptions{})
					return err
				})
			}

			By("create primary network NAD in one of the tenant namespaces")
			const primaryNadName = "some-primary-net"
			primaryNetTenantNs := testTenantNamespaces[0]
			primaryNetNad := generateNAD(newNetworkAttachmentConfig(networkAttachmentConfigParams{
				role:        "primary",
				topology:    "layer3",
				name:        primaryNadName,
				networkName: primaryNadName,
				cidr:        correctCIDRFamily(oc, userDefinedNetworkIPv4Subnet, userDefinedNetworkIPv6Subnet),
			}))
			_, err = nadClient.NetworkAttachmentDefinitions(primaryNetTenantNs).Create(context.Background(), primaryNetNad, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("create primary Cluster UDN CR")
			cudnName := randomNetworkMetaName()
			cleanup, err := createManifest(f.Namespace.Name, newPrimaryClusterUDNManifest(oc, cudnName, testTenantNamespaces...))
			Expect(err).NotTo(HaveOccurred())
			DeferCleanup(func() {
				cleanup()
				_, err := e2ekubectl.RunKubectl("", "delete", "clusteruserdefinednetwork", cudnName, "--wait", fmt.Sprintf("--timeout=%ds", 60))
				Expect(err).NotTo(HaveOccurred())
			})

			expectedMessage := fmt.Sprintf("primary network already exist in namespace %q: %q", primaryNetTenantNs, primaryNadName)
			Eventually(func(g Gomega) []metav1.Condition {
				conditionsJSON, err := e2ekubectl.RunKubectl(f.Namespace.Name, "get", "clusteruserdefinednetwork", cudnName, "-o", "jsonpath={.status.conditions}")
				g.Expect(err).NotTo(HaveOccurred())
				var actualConditions []metav1.Condition
				g.Expect(json.Unmarshal([]byte(conditionsJSON), &actualConditions)).To(Succeed())
				return normalizeConditions(actualConditions)
			}, 5*time.Second, 1*time.Second).Should(SatisfyAny(
				ConsistOf(metav1.Condition{
					Type:    "NetworkReady",
					Status:  metav1.ConditionFalse,
					Reason:  "NetworkAttachmentDefinitionSyncError",
					Message: expectedMessage,
				}),
				ConsistOf(metav1.Condition{
					Type:    "NetworkCreated",
					Status:  metav1.ConditionFalse,
					Reason:  "NetworkAttachmentDefinitionSyncError",
					Message: expectedMessage,
				}),
			))
		})

		Context("UDN Pod", func() {
			const (
				testUdnName = "test-net"
				testPodName = "test-pod-udn"
			)

			var udnPod *v1.Pod

			BeforeEach(func() {
				l := map[string]string{
					"e2e-framework":           f.BaseName,
					RequiredUDNNamespaceLabel: "",
				}
				ns, err := f.CreateNamespace(context.TODO(), f.BaseName, l)
				Expect(err).NotTo(HaveOccurred())
				err = udnWaitForOpenShift(oc, ns.Name)
				Expect(err).NotTo(HaveOccurred())
				f.Namespace = ns
				By("create tests UserDefinedNetwork")
				cleanup, err := createManifest(f.Namespace.Name, newPrimaryUserDefinedNetworkManifest(oc, testUdnName))
				DeferCleanup(cleanup)
				Expect(err).NotTo(HaveOccurred())
				Eventually(userDefinedNetworkReadyFunc(oc.AdminDynamicClient(), f.Namespace.Name, testUdnName), udnCrReadyTimeout, time.Second).Should(Succeed())
				By("create UDN pod")
				cfg := podConfig(testPodName, withCommand(func() []string {
					return httpServerContainerCmd(port)
				}))
				cfg.namespace = f.Namespace.Name
				udnPod = runUDNPod(cs, f.Namespace.Name, *cfg, nil)
			})

			It("should react to k8s.ovn.org/open-default-ports annotations changes", func() {
				By("Creating second namespace for default network pod")
				defaultNetNamespace := f.Namespace.Name + "-default"
				_, err := cs.CoreV1().Namespaces().Create(context.Background(), &v1.Namespace{
					ObjectMeta: metav1.ObjectMeta{
						Name: defaultNetNamespace,
					},
				}, metav1.CreateOptions{})
				Expect(err).NotTo(HaveOccurred())
				defer func() {
					Expect(cs.CoreV1().Namespaces().Delete(context.Background(), defaultNetNamespace, metav1.DeleteOptions{})).To(Succeed())
				}()

				By("creating default network client pod")
				defaultClientPod := frameworkpod.CreateExecPodOrFail(
					context.Background(),
					f.ClientSet,
					defaultNetNamespace,
					"default-net-client-pod",
					func(pod *v1.Pod) {
						pod.Spec.Containers[0].Args = []string{"netexec"}
						setRuntimeDefaultPSA(pod)
					},
				)

				udnIPv4, udnIPv6, err := podIPsForDefaultNetwork(
					cs,
					f.Namespace.Name,
					udnPod.GetName(),
				)
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("verify default network client pod can't access UDN pod on port %d", port))
				for _, destIP := range []string{udnIPv4, udnIPv6} {
					if destIP == "" {
						continue
					}
					By("checking the default network pod can't reach UDN pod on IP " + destIP)
					Consistently(func() bool {
						return connectToServer(podConfiguration{namespace: defaultClientPod.Namespace, name: defaultClientPod.Name}, destIP, port) != nil
					}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
				}

				By("Open UDN pod port")
				udnPod.Annotations[openDefaultPortsAnnotation] = fmt.Sprintf(
					`- protocol: tcp
  port: %d`, port)
				udnPod, err = cs.CoreV1().Pods(udnPod.Namespace).Update(context.Background(), udnPod, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("verify default network client pod can access UDN pod on open port %d", port))
				for _, destIP := range []string{udnIPv4, udnIPv6} {
					if destIP == "" {
						continue
					}
					By("checking the default network pod can reach UDN pod on IP " + destIP)
					Eventually(func() bool {
						return connectToServer(podConfiguration{namespace: defaultClientPod.Namespace, name: defaultClientPod.Name}, destIP, port) == nil
					}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
				}

				By("Update UDN pod port with the wrong syntax")
				// this should clean up open ports and throw an event
				udnPod.Annotations[openDefaultPortsAnnotation] = fmt.Sprintf(
					`- protocol: ppp
  port: %d`, port)
				udnPod, err = cs.CoreV1().Pods(udnPod.Namespace).Update(context.Background(), udnPod, metav1.UpdateOptions{})
				Expect(err).NotTo(HaveOccurred())

				By(fmt.Sprintf("verify default network client pod can't access UDN pod on port %d", port))
				for _, destIP := range []string{udnIPv4, udnIPv6} {
					if destIP == "" {
						continue
					}
					By("checking the default network pod can't reach UDN pod on IP " + destIP)
					Eventually(func() bool {
						return connectToServer(podConfiguration{namespace: defaultClientPod.Namespace, name: defaultClientPod.Name}, destIP, port) != nil
					}, serverConnectPollTimeout, serverConnectPollInterval).Should(BeTrue())
				}
				By("Verify syntax error is reported via event")
				events, err := cs.CoreV1().Events(udnPod.Namespace).List(context.Background(), metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())
				found := false
				for _, event := range events.Items {
					if event.Reason == "ErrorUpdatingResource" && strings.Contains(event.Message, "invalid protocol ppp") {
						found = true
						break
					}
				}
				Expect(found).To(BeTrue(), "should have found an event for invalid protocol")
			})
		})
	})
})

// randomNetworkMetaName return pseudo random name for network related objects (NAD,UDN,CUDN).
// CUDN is cluster-scoped object, in case tests running in parallel, having random names avoids
// conflicting with other tests.
func randomNetworkMetaName() string {
	return fmt.Sprintf("test-net-%s", rand.String(5))
}

var nadToUdnParams = map[string]string{
	"primary":   "Primary",
	"secondary": "Secondary",
	"layer2":    "Layer2",
	"layer3":    "Layer3",
}

func generateUserDefinedNetworkManifest(params *networkAttachmentConfigParams) string {
	subnets := generateSubnetsYaml(params)
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + params.name + `
spec:
  topology: ` + nadToUdnParams[params.topology] + `
  ` + params.topology + `: 
    role: ` + nadToUdnParams[params.role] + `
    subnets: ` + subnets + `
    ` + generateIPAMLifecycle(params) + `
`
}

func generateClusterUserDefinedNetworkManifest(params *networkAttachmentConfigParams) string {
	subnets := generateSubnetsYaml(params)
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + params.name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [` + params.namespace + `]
  network:
    topology: ` + nadToUdnParams[params.topology] + `
    ` + params.topology + `: 
      role: ` + nadToUdnParams[params.role] + `
      subnets: ` + subnets + `
`
}

func generateSubnetsYaml(params *networkAttachmentConfigParams) string {
	if params.topology == "layer3" {
		l3Subnets := generateLayer3Subnets(params.cidr)
		return fmt.Sprintf("[%s]", strings.Join(l3Subnets, ","))
	}
	return fmt.Sprintf("[%s]", params.cidr)
}

func generateLayer3Subnets(cidrs string) []string {
	cidrList := strings.Split(cidrs, ",")
	var subnets []string
	for _, cidr := range cidrList {
		cidrSplit := strings.Split(cidr, "/")
		switch len(cidrSplit) {
		case 2:
			subnets = append(subnets, fmt.Sprintf(`{cidr: "%s/%s"}`, cidrSplit[0], cidrSplit[1]))
		case 3:
			subnets = append(subnets, fmt.Sprintf(`{cidr: "%s/%s", hostSubnet: %q }`, cidrSplit[0], cidrSplit[1], cidrSplit[2]))
		default:
			panic(fmt.Sprintf("invalid layer3 subnet: %v", cidr))
		}
	}
	return subnets
}

func generateIPAMLifecycle(params *networkAttachmentConfigParams) string {
	if !params.allowPersistentIPs {
		return ""
	}
	return `ipam:
      lifecycle: Persistent`
}

func createManifest(namespace, manifest string) (func(), error) {
	tmpDir, err := os.MkdirTemp("", "udn-test")
	if err != nil {
		return nil, err
	}
	cleanup := func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			framework.Logf("Unable to remove udn test yaml files from disk %s: %v", tmpDir, err)
		}
	}

	path := filepath.Join(tmpDir, "test-ovn-k-udn-"+rand.String(5)+".yaml")
	if err := os.WriteFile(path, []byte(manifest), 0644); err != nil {
		framework.Failf("Unable to write udn yaml to disk: %v", err)
	}

	_, err = e2ekubectl.RunKubectl(namespace, "create", "-f", path)
	if err != nil {
		return cleanup, err
	}
	return cleanup, nil
}

func applyManifest(namespace, manifest string) error {
	_, err := e2ekubectl.RunKubectlInput(namespace, manifest, "apply", "-f", "-")
	return err
}

var clusterUDNGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "clusteruserdefinednetworks",
}

var udnGVR = schema.GroupVersionResource{
	Group:    "k8s.ovn.org",
	Version:  "v1",
	Resource: "userdefinednetworks",
}

// getConditions extracts metav1 conditions from .status.conditions of an unstructured object
func getConditions(uns *unstructured.Unstructured) ([]metav1.Condition, error) {
	var conditions []metav1.Condition
	conditionsRaw, found, err := unstructured.NestedFieldNoCopy(uns.Object, "status", "conditions")
	if err != nil {
		return nil, fmt.Errorf("failed getting conditions in %s: %v", uns.GetName(), err)
	}
	if !found {
		return nil, fmt.Errorf("conditions not found in %v", uns)
	}

	conditionsJSON, err := json.Marshal(conditionsRaw)
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(conditionsJSON, &conditions); err != nil {
		return nil, err
	}

	return conditions, nil
}

// userDefinedNetworkReadyFunc returns a function that checks for the NetworkCreated/NetworkReady condition in the provided udn
func userDefinedNetworkReadyFunc(client dynamic.Interface, namespace, name string) func() error {
	return func() error {
		udn, err := client.Resource(udnGVR).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{}, "status")
		if err != nil {
			return err
		}
		conditions, err := getConditions(udn)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("no conditions found in: %v", udn)
		}
		for _, udnCondition := range conditions {
			if (udnCondition.Type == "NetworkCreated" || udnCondition.Type == "NetworkReady") && udnCondition.Status == metav1.ConditionTrue {
				return nil
			}

		}
		return fmt.Errorf("no NetworkCreated/NetworkReady condition found in: %v", udn)
	}
}

// userDefinedNetworkReadyFunc returns a function that checks for the NetworkCreated/NetworkReady condition in the provided cluster udn
func clusterUserDefinedNetworkReadyFunc(client dynamic.Interface, name string) func() error {
	return func() error {
		cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), name, metav1.GetOptions{}, "status")
		if err != nil {
			return err
		}
		conditions, err := getConditions(cUDN)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("no conditions found in: %v", cUDN)
		}
		for _, cUDNCondition := range conditions {
			if (cUDNCondition.Type == "NetworkCreated" || cUDNCondition.Type == "NetworkReady") && cUDNCondition.Status == metav1.ConditionTrue {
				return nil
			}
		}
		return fmt.Errorf("no NetworkCreated/NetworkReady condition found in: %v", cUDN)
	}
}

func newPrimaryUserDefinedNetworkManifest(oc *exutil.CLI, name string) string {
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  topology: Layer3
  layer3:
    role: Primary
    subnets: ` + generateCIDRforUDN(oc)
}

func generateCIDRforUDN(oc *exutil.CLI) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	cidr := `
    - cidr: 10.20.100.0/16
`
	if hasIPv6 && hasIPv4 {
		cidr = `
    - cidr: 10.20.100.0/16
    - cidr: 2014:100:200::0/60
`
	} else if hasIPv6 {
		cidr = `
    - cidr: 2014:100:200::0/60
`
	}
	return cidr
}

func newUserDefinedNetworkManifest(name string) string {
	return `
apiVersion: k8s.ovn.org/v1
kind: UserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  topology: "Layer2"
  layer2:
    role: Secondary
    subnets: ["10.100.0.0/16"]
`
}

func assertNetAttachDefManifest(nadClient nadclient.K8sCniCncfIoV1Interface, namespace, udnName, udnUID string) {
	nad, err := nadClient.NetworkAttachmentDefinitions(namespace).Get(context.Background(), udnName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	ExpectWithOffset(1, nad.Name).To(Equal(udnName))
	ExpectWithOffset(1, nad.Namespace).To(Equal(namespace))
	ExpectWithOffset(1, nad.OwnerReferences).To(Equal([]metav1.OwnerReference{{
		APIVersion:         "k8s.ovn.org/v1",
		Kind:               "UserDefinedNetwork",
		Name:               "test-net",
		UID:                types.UID(udnUID),
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}}))

	jsonTemplate := `{
		"cniVersion":"1.0.0",
		"type": "ovn-k8s-cni-overlay",
		"name": "%s",
		"netAttachDefName": "%s",
		"topology": "layer2",
		"role": "secondary",
		"subnets": "10.100.0.0/16"
	}`

	// REMOVEME(trozet): after network name has been updated to use underscores in OVNK
	expectedLegacyNetworkName := namespace + "." + udnName
	expectedNetworkName := namespace + "_" + udnName
	expectedNadName := namespace + "/" + udnName

	nadJSONLegacy := fmt.Sprintf(jsonTemplate, expectedLegacyNetworkName, expectedNadName)
	nadJSON := fmt.Sprintf(jsonTemplate, expectedNetworkName, expectedNadName)

	ExpectWithOffset(1, nad.Spec.Config).To(SatisfyAny(
		MatchJSON(nadJSONLegacy),
		MatchJSON(nadJSON),
	))
}

func validateUDNStatusReportsConsumers(client dynamic.Interface, udnNamesapce, udnName, expectedPodName string) error {
	udn, err := client.Resource(udnGVR).Namespace(udnNamesapce).Get(context.Background(), udnName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	conditions, err := getConditions(udn)
	if err != nil {
		return err
	}
	conditions = normalizeConditions(conditions)
	expectedMsg := fmt.Sprintf("failed to delete NetworkAttachmentDefinition [%[1]s/%[2]s]: network in use by the following pods: [%[1]s/%[3]s]",
		udnNamesapce, udnName, expectedPodName)
	networkReadyCondition := metav1.Condition{
		Type:    "NetworkReady",
		Status:  metav1.ConditionFalse,
		Reason:  "SyncError",
		Message: expectedMsg,
	}
	networkCreatedCondition := metav1.Condition{
		Type:    "NetworkCreated",
		Status:  metav1.ConditionFalse,
		Reason:  "SyncError",
		Message: expectedMsg,
	}
	for _, udnCondition := range conditions {
		if udnCondition == networkReadyCondition || udnCondition == networkCreatedCondition {
			return nil
		}
	}
	return fmt.Errorf("failed to find NetworkCreated/NetworkReady condition in %v", conditions)
}

func normalizeConditions(conditions []metav1.Condition) []metav1.Condition {
	for i := range conditions {
		t := metav1.NewTime(time.Time{})
		conditions[i].LastTransitionTime = t
	}
	return conditions
}

func assertClusterNADManifest(nadClient nadclient.K8sCniCncfIoV1Interface, namespace, udnName, udnUID string) {
	nad, err := nadClient.NetworkAttachmentDefinitions(namespace).Get(context.Background(), udnName, metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	ExpectWithOffset(1, nad.Name).To(Equal(udnName))
	ExpectWithOffset(1, nad.Namespace).To(Equal(namespace))
	ExpectWithOffset(1, nad.OwnerReferences).To(Equal([]metav1.OwnerReference{{
		APIVersion:         "k8s.ovn.org/v1",
		Kind:               "ClusterUserDefinedNetwork",
		Name:               udnName,
		UID:                types.UID(udnUID),
		BlockOwnerDeletion: pointer.Bool(true),
		Controller:         pointer.Bool(true),
	}}))
	ExpectWithOffset(1, nad.Labels).To(Equal(map[string]string{"k8s.ovn.org/user-defined-network": ""}))
	ExpectWithOffset(1, nad.Finalizers).To(Equal([]string{"k8s.ovn.org/user-defined-network-protection"}))

	// REMOVEME(trozet): after network name has been updated to use underscores in OVNK
	expectedLegacyNetworkName := "cluster.udn." + udnName

	expectedNetworkName := "cluster_udn_" + udnName
	expectedNadName := namespace + "/" + udnName

	jsonTemplate := `{
		"cniVersion":"1.0.0",
		"type": "ovn-k8s-cni-overlay",
		"name": "%s",
		"netAttachDefName": "%s",
		"topology": "layer2",
		"role": "secondary",
		"subnets": "10.100.0.0/16"
	}`

	nadJSONLegacy := fmt.Sprintf(jsonTemplate, expectedLegacyNetworkName, expectedNadName)
	nadJSON := fmt.Sprintf(jsonTemplate, expectedNetworkName, expectedNadName)

	ExpectWithOffset(1, nad.Spec.Config).To(SatisfyAny(
		MatchJSON(nadJSONLegacy),
		MatchJSON(nadJSON),
	))
}

func validateClusterUDNStatusReportsActiveNamespacesFunc(client dynamic.Interface, cUDNName string, expectedActiveNsNames ...string) func() error {
	return func() error {
		cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), cUDNName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		conditions, err := getConditions(cUDN)
		if err != nil {
			return err
		}
		if len(conditions) == 0 {
			return fmt.Errorf("expected at least one condition in %v", cUDN)
		}

		c := conditions[0]
		if c.Type != "NetworkCreated" && c.Type != "NetworkReady" {
			return fmt.Errorf("expected NetworkCreated/NetworkReady type in %v", c)
		}
		if c.Status != metav1.ConditionTrue {
			return fmt.Errorf("expected True status in %v", c)
		}
		if c.Reason != "NetworkAttachmentDefinitionCreated" && c.Reason != "NetworkAttachmentDefinitionReady" {
			return fmt.Errorf("expected NetworkAttachmentDefinitionCreated/NetworkAttachmentDefinitionReady reason in %v", c)
		}
		if !strings.Contains(c.Message, "NetworkAttachmentDefinition has been created in following namespaces:") {
			return fmt.Errorf("expected \"NetworkAttachmentDefinition has been created in following namespaces:\" in %s", c.Message)
		}

		for _, ns := range expectedActiveNsNames {
			if !strings.Contains(c.Message, ns) {
				return fmt.Errorf("expected to find %q namespace in %s", ns, c.Message)
			}
		}
		return nil
	}
}

func validateClusterUDNStatusReportConsumers(client dynamic.Interface, cUDNName, udnNamespace, expectedPodName string) error {
	cUDN, err := client.Resource(clusterUDNGVR).Get(context.Background(), cUDNName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	conditions, err := getConditions(cUDN)
	if err != nil {
		return err
	}
	conditions = normalizeConditions(conditions)
	expectedMsg := fmt.Sprintf("failed to delete NetworkAttachmentDefinition [%[1]s/%[2]s]: network in use by the following pods: [%[1]s/%[3]s]",
		udnNamespace, cUDNName, expectedPodName)
	networkCreatedCondition := metav1.Condition{
		Type:    "NetworkCreated",
		Status:  metav1.ConditionFalse,
		Reason:  "NetworkAttachmentDefinitionSyncError",
		Message: expectedMsg,
	}
	networkReadyCondition := metav1.Condition{
		Type:    "NetworkReady",
		Status:  metav1.ConditionFalse,
		Reason:  "NetworkAttachmentDefinitionSyncError",
		Message: expectedMsg,
	}
	for _, clusterUDNCondition := range conditions {
		if clusterUDNCondition == networkCreatedCondition || clusterUDNCondition == networkReadyCondition {
			return nil
		}
	}
	return fmt.Errorf("failed to find NetworkCreated/NetworkReady condition in %v", conditions)
}

func newClusterUDNManifest(name string, targetNamespaces ...string) string {
	targetNs := strings.Join(targetNamespaces, ",")
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ ` + targetNs + ` ]
  network:
    topology: Layer2
    layer2:
      role: Secondary
      subnets: ["10.100.0.0/16"]
`
}

func newPrimaryClusterUDNManifest(oc *exutil.CLI, name string, targetNamespaces ...string) string {
	targetNs := strings.Join(targetNamespaces, ",")
	return `
apiVersion: k8s.ovn.org/v1
kind: ClusterUserDefinedNetwork
metadata:
  name: ` + name + `
spec:
  namespaceSelector:
    matchExpressions:
    - key: kubernetes.io/metadata.name
      operator: In
      values: [ ` + targetNs + ` ]
  network:
    topology: Layer3
    layer3:
      role: Primary
      subnets: ` + generateCIDRforClusterUDN(oc)
}

func generateCIDRforClusterUDN(oc *exutil.CLI) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	cidr := `[{cidr: "203.203.0.0/16"}]`
	if hasIPv6 && hasIPv4 {
		cidr = `[{cidr: "203.203.0.0/16"},{cidr: "2014:100:200::0/60"}]`
	} else if hasIPv6 {
		cidr = `[{cidr: "2014:100:200::0/60"}]`
	}
	return cidr
}

func setRuntimeDefaultPSA(pod *v1.Pod) {
	dontEscape := false
	noRoot := true
	pod.Spec.SecurityContext = &v1.PodSecurityContext{
		RunAsNonRoot: &noRoot,
		SeccompProfile: &v1.SeccompProfile{
			Type: v1.SeccompProfileTypeRuntimeDefault,
		},
	}
	pod.Spec.Containers[0].SecurityContext = &v1.SecurityContext{
		AllowPrivilegeEscalation: &dontEscape,
		Capabilities: &v1.Capabilities{
			Drop: []v1.Capability{"ALL"},
		},
	}
}

type podOption func(*podConfiguration)

func podConfig(podName string, opts ...podOption) *podConfiguration {
	pod := &podConfiguration{
		name: podName,
	}
	for _, opt := range opts {
		opt(pod)
	}
	return pod
}

func withCommand(cmdGenerationFn func() []string) podOption {
	return func(pod *podConfiguration) {
		pod.containerCmd = cmdGenerationFn()
	}
}

func withLabels(labels map[string]string) podOption {
	return func(pod *podConfiguration) {
		pod.labels = labels
	}
}

func withNetworkAttachment(networks []nadapi.NetworkSelectionElement) podOption {
	return func(pod *podConfiguration) {
		pod.attachments = networks
	}
}

// podIPsForUserDefinedPrimaryNetwork returns the v4 or v6 IPs for a pod on the UDN
func podIPsForUserDefinedPrimaryNetwork(k8sClient clientset.Interface, podNamespace string, podName string, attachmentName string, index int) (string, error) {
	pod, err := k8sClient.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	netStatus, err := userDefinedNetworkStatus(pod, attachmentName)
	if err != nil {
		return "", err
	}

	if len(netStatus.IPs) == 0 {
		return "", fmt.Errorf("attachment for network %q without IPs", attachmentName)
	}
	if len(netStatus.IPs) > 2 {
		return "", fmt.Errorf("attachment for network %q with more than two IPs", attachmentName)
	}
	return netStatus.IPs[index].IP.String(), nil
}

func podIPsForDefaultNetwork(k8sClient clientset.Interface, podNamespace string, podName string) (string, string, error) {
	pod, err := k8sClient.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return "", "", err
	}
	ipv4, ipv6 := getPodAddresses(pod)
	return ipv4, ipv6, nil
}

func userDefinedNetworkStatus(pod *v1.Pod, networkName string) (PodAnnotation, error) {
	netStatus, err := unmarshalPodAnnotation(pod.Annotations, networkName)
	if err != nil {
		return PodAnnotation{}, fmt.Errorf("failed to unmarshall annotations for pod %q: %v", pod.Name, err)
	}

	return *netStatus, nil
}

func runUDNPod(cs clientset.Interface, namespace string, podConfig podConfiguration, podSpecTweak func(*v1.Pod)) *v1.Pod {
	By(fmt.Sprintf("instantiating the UDN pod %s", podConfig.name))
	podSpec := generatePodSpec(podConfig)
	if podSpecTweak != nil {
		podSpecTweak(podSpec)
	}
	serverPod, err := cs.CoreV1().Pods(podConfig.namespace).Create(
		context.Background(),
		podSpec,
		metav1.CreateOptions{},
	)
	Expect(err).NotTo(HaveOccurred())
	Expect(serverPod).NotTo(BeNil())

	By(fmt.Sprintf("asserting the UDN pod %s reaches the `Ready` state", podConfig.name))
	var updatedPod *v1.Pod
	Eventually(func() v1.PodPhase {
		updatedPod, err = cs.CoreV1().Pods(namespace).Get(context.Background(), serverPod.GetName(), metav1.GetOptions{})
		if err != nil {
			return v1.PodFailed
		}
		return updatedPod.Status.Phase
	}, podReadyPollTimeout, podReadyPollInterval).Should(Equal(v1.PodRunning))
	return updatedPod
}

type networkAttachmentConfigParams struct {
	cidr               string
	excludeCIDRs       []string
	namespace          string
	name               string
	topology           string
	networkName        string
	vlanID             int
	allowPersistentIPs bool
	role               string
}

// workloadNetworkConfig contains workload-specific network customizations
type workloadNetworkConfig struct {
	preconfiguredIPs []string
	preconfiguredMAC string
}

type networkAttachmentConfig struct {
	networkAttachmentConfigParams
}

func newNetworkAttachmentConfig(params networkAttachmentConfigParams) networkAttachmentConfig {
	networkAttachmentConfig := networkAttachmentConfig{
		networkAttachmentConfigParams: params,
	}
	if networkAttachmentConfig.networkName == "" {
		networkAttachmentConfig.networkName = uniqueNadName(networkAttachmentConfig.name)
	}
	return networkAttachmentConfig
}

func uniqueNadName(originalNetName string) string {
	const randomStringLength = 5
	return fmt.Sprintf("%s_%s", rand.String(randomStringLength), originalNetName)
}

func generateNAD(config networkAttachmentConfig) *nadapi.NetworkAttachmentDefinition {
	nadSpec := fmt.Sprintf(
		`
{
        "cniVersion": "0.3.0",
        "name": %q,
        "type": "ovn-k8s-cni-overlay",
        "topology":%q,
        "subnets": %q,
        "excludeSubnets": %q,
        "netAttachDefName": %q,
        "vlanID": %d,
        "allowPersistentIPs": %t,
        "role": %q
}
`,
		config.networkName,
		config.topology,
		config.cidr,
		strings.Join(config.excludeCIDRs, ","),
		namespacedName(config.namespace, config.name),
		config.vlanID,
		config.allowPersistentIPs,
		config.role,
	)
	return generateNetAttachDef(config.namespace, config.name, nadSpec)
}

func generateNetAttachDef(namespace, nadName, nadSpec string) *nadapi.NetworkAttachmentDefinition {
	return &nadapi.NetworkAttachmentDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      nadName,
			Namespace: namespace,
		},
		Spec: nadapi.NetworkAttachmentDefinitionSpec{Config: nadSpec},
	}
}

func namespacedName(namespace, name string) string {
	return fmt.Sprintf("%s/%s", namespace, name)
}

type podConfiguration struct {
	attachments            []nadapi.NetworkSelectionElement
	containerCmd           []string
	name                   string
	namespace              string
	nodeSelector           map[string]string
	isPrivileged           bool
	labels                 map[string]string
	requiresExtraNamespace bool
}

func generatePodSpec(config podConfiguration) *v1.Pod {
	podSpec := frameworkpod.NewAgnhostPod(config.namespace, config.name, nil, nil, nil, config.containerCmd...)
	if len(config.attachments) > 0 {
		podSpec.Annotations = networkSelectionElements(config.attachments...)
	}
	podSpec.Spec.NodeSelector = config.nodeSelector
	podSpec.Labels = config.labels
	if config.isPrivileged {
		privileged := true
		podSpec.Spec.Containers[0].SecurityContext.Privileged = &privileged
	}
	return podSpec
}

func networkSelectionElements(elements ...nadapi.NetworkSelectionElement) map[string]string {
	marshalledElements, err := json.Marshal(elements)
	if err != nil {
		panic(fmt.Errorf("programmer error: you've provided wrong input to the test data: %v", err))
	}
	return map[string]string{
		nadapi.NetworkAttachmentAnnot: string(marshalledElements),
	}
}

func httpServerContainerCmd(port uint16) []string {
	return []string{"netexec", "--http-port", fmt.Sprintf("%d", port)}
}

// takes the CLI, potential ipv4 and ipv6 cidrs and returns the correct cidr family for the cluster under test
func correctCIDRFamily(oc *exutil.CLI, ipv4CIDR, ipv6CIDR string) string {
	hasIPv4, hasIPv6, err := GetIPAddressFamily(oc)
	Expect(err).NotTo(HaveOccurred())
	// dual stack cluster
	if hasIPv6 && hasIPv4 {
		return strings.Join([]string{ipv4CIDR, ipv6CIDR}, ",")
	}
	// single stack ipv6 cluster
	if hasIPv6 {
		return ipv6CIDR
	}
	// single stack ipv4 cluster
	return ipv4CIDR
}

func getNetCIDRSubnet(netCIDR string) (string, error) {
	subStrings := strings.Split(netCIDR, "/")
	if len(subStrings) == 3 {
		return subStrings[0] + "/" + subStrings[1], nil
	} else if len(subStrings) == 2 {
		return netCIDR, nil
	}
	return "", fmt.Errorf("invalid network cidr: %q", netCIDR)
}

func inRange(cidr string, ip string) error {
	_, cidrRange, err := net.ParseCIDR(cidr)
	if err != nil {
		return err
	}

	if cidrRange.Contains(net.ParseIP(ip)) {
		return nil
	}

	return fmt.Errorf("ip [%s] is NOT in range %s", ip, cidr)
}

func connectToServer(clientPodConfig podConfiguration, serverIP string, port int) error {
	_, err := connectToServerWithPath(clientPodConfig.namespace, clientPodConfig.name, serverIP, "" /* no path */, port)
	return err
}

func connectToServerWithPath(podNamespace, podName, serverIP, path string, port int) (string, error) {
	return e2ekubectl.RunKubectl(
		podNamespace,
		"exec",
		podName,
		"--",
		"curl",
		"--connect-timeout",
		"2",
		net.JoinHostPort(serverIP, fmt.Sprintf("%d", port))+path,
	)
}

// Returns pod's ipv4 and ipv6 addresses IN ORDER
func getPodAddresses(pod *v1.Pod) (string, string) {
	var ipv4Res, ipv6Res string
	for _, a := range pod.Status.PodIPs {
		if utilnet.IsIPv4String(a.IP) {
			ipv4Res = a.IP
		}
		if utilnet.IsIPv6String(a.IP) {
			ipv6Res = a.IP
		}
	}
	return ipv4Res, ipv6Res
}

// PodAnnotation describes the assigned network details for a single pod network. (The
// actual annotation may include the equivalent of multiple PodAnnotations.)
type PodAnnotation struct {
	// IPs are the pod's assigned IP addresses/prefixes
	IPs []*net.IPNet
	// MAC is the pod's assigned MAC address
	MAC net.HardwareAddr
	// Gateways are the pod's gateway IP addresses; note that there may be
	// fewer Gateways than IPs.
	Gateways []net.IP
	// Routes are additional routes to add to the pod's network namespace
	Routes []PodRoute
	// Primary reveals if this network is the primary network of the pod or not
	Primary bool
}

// PodRoute describes any routes to be added to the pod's network namespace
type PodRoute struct {
	// Dest is the route destination
	Dest *net.IPNet
	// NextHop is the IP address of the next hop for traffic destined for Dest
	NextHop net.IP
}

type annotationNotSetError struct {
	msg string
}

func (anse annotationNotSetError) Error() string {
	return anse.msg
}

// newAnnotationNotSetError returns an error for an annotation that is not set
func newAnnotationNotSetError(format string, args ...interface{}) error {
	return annotationNotSetError{msg: fmt.Sprintf(format, args...)}
}

type podAnnotation struct {
	IPs      []string   `json:"ip_addresses"`
	MAC      string     `json:"mac_address"`
	Gateways []string   `json:"gateway_ips,omitempty"`
	Routes   []podRoute `json:"routes,omitempty"`

	IP      string `json:"ip_address,omitempty"`
	Gateway string `json:"gateway_ip,omitempty"`
	Primary bool   `json:"primary"`
}

type podRoute struct {
	Dest    string `json:"dest"`
	NextHop string `json:"nextHop"`
}

// UnmarshalPodAnnotation returns the default network info from pod.Annotations
func unmarshalPodAnnotation(annotations map[string]string, networkName string) (*PodAnnotation, error) {
	const podNetworkAnnotation = "k8s.ovn.org/pod-networks"
	ovnAnnotation, ok := annotations[podNetworkAnnotation]
	if !ok {
		return nil, newAnnotationNotSetError("could not find OVN pod annotation in %v", annotations)
	}

	podNetworks := make(map[string]podAnnotation)
	if err := json.Unmarshal([]byte(ovnAnnotation), &podNetworks); err != nil {
		return nil, fmt.Errorf("failed to unmarshal ovn pod annotation %q: %v",
			ovnAnnotation, err)
	}
	tempA := podNetworks[networkName]
	a := &tempA

	podAnnotation := &PodAnnotation{Primary: a.Primary}
	var err error

	podAnnotation.MAC, err = net.ParseMAC(a.MAC)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pod MAC %q: %v", a.MAC, err)
	}

	if len(a.IPs) == 0 {
		if a.IP == "" {
			return nil, fmt.Errorf("bad annotation data (neither ip_address nor ip_addresses is set)")
		}
		a.IPs = append(a.IPs, a.IP)
	} else if a.IP != "" && a.IP != a.IPs[0] {
		return nil, fmt.Errorf("bad annotation data (ip_address and ip_addresses conflict)")
	}
	for _, ipstr := range a.IPs {
		ip, ipnet, err := net.ParseCIDR(ipstr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod IP %q: %v", ipstr, err)
		}
		ipnet.IP = ip
		podAnnotation.IPs = append(podAnnotation.IPs, ipnet)
	}

	if len(a.Gateways) == 0 {
		if a.Gateway != "" {
			a.Gateways = append(a.Gateways, a.Gateway)
		}
	} else if a.Gateway != "" && a.Gateway != a.Gateways[0] {
		return nil, fmt.Errorf("bad annotation data (gateway_ip and gateway_ips conflict)")
	}
	for _, gwstr := range a.Gateways {
		gw := net.ParseIP(gwstr)
		if gw == nil {
			return nil, fmt.Errorf("failed to parse pod gateway %q", gwstr)
		}
		podAnnotation.Gateways = append(podAnnotation.Gateways, gw)
	}

	for _, r := range a.Routes {
		route := PodRoute{}
		_, route.Dest, err = net.ParseCIDR(r.Dest)
		if err != nil {
			return nil, fmt.Errorf("failed to parse pod route dest %q: %v", r.Dest, err)
		}
		if route.Dest.IP.IsUnspecified() {
			return nil, fmt.Errorf("bad podNetwork data: default route %v should be specified as gateway", route)
		}
		if r.NextHop != "" {
			route.NextHop = net.ParseIP(r.NextHop)
			if route.NextHop == nil {
				return nil, fmt.Errorf("failed to parse pod route next hop %q", r.NextHop)
			} else if utilnet.IsIPv6(route.NextHop) != utilnet.IsIPv6CIDR(route.Dest) {
				return nil, fmt.Errorf("pod route %s has next hop %s of different family", r.Dest, r.NextHop)
			}
		}
		podAnnotation.Routes = append(podAnnotation.Routes, route)
	}

	return podAnnotation, nil
}

// Copyright 2015 CNI authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// NextIP returns IP incremented by 1, if IP is invalid, return nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L23
func NextIP(ip net.IP) net.IP {
	normalizedIP := normalizeIP(ip)
	if normalizedIP == nil {
		return nil
	}

	i := ipToInt(normalizedIP)
	return intToIP(i.Add(i, big.NewInt(1)), len(normalizedIP) == net.IPv6len)
}

// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L60
func ipToInt(ip net.IP) *big.Int {
	return big.NewInt(0).SetBytes(ip)
}

// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L64
func intToIP(i *big.Int, isIPv6 bool) net.IP {
	intBytes := i.Bytes()

	if len(intBytes) == net.IPv4len || len(intBytes) == net.IPv6len {
		return intBytes
	}

	if isIPv6 {
		return append(make([]byte, net.IPv6len-len(intBytes)), intBytes...)
	}

	return append(make([]byte, net.IPv4len-len(intBytes)), intBytes...)
}

// normalizeIP will normalize IP by family,
// IPv4 : 4-byte form
// IPv6 : 16-byte form
// others : nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L82
func normalizeIP(ip net.IP) net.IP {
	if ipTo4 := ip.To4(); ipTo4 != nil {
		return ipTo4
	}
	return ip.To16()
}

// Network masks off the host portion of the IP, if IPNet is invalid,
// return nil
// copied from https://github.com/containernetworking/plugins/blob/acf8ddc8e1128e6f68a34f7fe91122afeb1fa93d/pkg/ip/cidr.go#L89C1-L105C2
func Network(ipn *net.IPNet) *net.IPNet {
	if ipn == nil {
		return nil
	}

	maskedIP := ipn.IP.Mask(ipn.Mask)
	if maskedIP == nil {
		return nil
	}

	return &net.IPNet{
		IP:   maskedIP,
		Mask: ipn.Mask,
	}
}

func udnWaitForOpenShift(oc *exutil.CLI, namespace string) error {
	serviceAccountName := "default"
	framework.Logf("Waiting for ServiceAccount %q to be provisioned...", serviceAccountName)
	err := exutil.WaitForServiceAccount(oc.AdminKubeClient().CoreV1().ServiceAccounts(namespace), serviceAccountName)
	if err != nil {
		return err
	}

	framework.Logf("Waiting on permissions in namespace %q ...", namespace)
	err = exutil.WaitForSelfSAR(1*time.Second, 60*time.Second, oc.AdminKubeClient(), kubeauthorizationv1.SelfSubjectAccessReviewSpec{
		ResourceAttributes: &kubeauthorizationv1.ResourceAttributes{
			Namespace: namespace,
			Verb:      "create",
			Group:     "",
			Resource:  "pods",
		},
	})
	if err != nil {
		return err
	}

	framework.Logf("Waiting on SCC annotations in namespace %q ...", namespace)
	err = exutil.WaitForNamespaceSCCAnnotations(oc.AdminKubeClient().CoreV1(), namespace)
	if err != nil {
		return err
	}
	return nil
}
