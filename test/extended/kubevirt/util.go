package kubevirt

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/storage/names"
	k8sclient "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2enode "k8s.io/kubernetes/test/e2e/framework/node"

	e2eoutput "k8s.io/kubernetes/test/e2e/framework/pod/output"
	svc "k8s.io/kubernetes/test/e2e/framework/service"
	e2eskipper "k8s.io/kubernetes/test/e2e/framework/skipper"
)

type NodeType int

type IPFamily string

func launchWebserverNodePortService(client k8sclient.Interface, namespace, serviceName string, nodeName string) int32 {
	labelSelector := make(map[string]string)
	labelSelector["name"] = "web"
	createPodForService(client, namespace, serviceName, nodeName, labelSelector)

	servicePort := 8080
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     int32(servicePort),
				},
			},
			Selector: labelSelector,
		},
	}
	serviceClient := client.CoreV1().Services(namespace)
	_, err := serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	expectNoError(err)
	expectNoError(exutil.WaitForEndpoint(client, namespace, serviceName))
	service, err = serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	expectNoError(err)

	Expect(service.Spec.Ports[0].NodePort).ToNot(Equal(0))
	return service.Spec.Ports[0].NodePort

}

func launchWebserverLoadBalancerService(client k8sclient.Interface, namespace, serviceName string, nodeName string) string {
	labelSelector := make(map[string]string)
	labelSelector["name"] = "web"
	createPodForService(client, namespace, serviceName, nodeName, labelSelector)

	servicePort := 8080
	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: serviceName,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeLoadBalancer,
			Ports: []corev1.ServicePort{
				{
					Protocol: corev1.ProtocolTCP,
					Port:     int32(servicePort),
				},
			},
			Selector: labelSelector,
		},
	}
	serviceClient := client.CoreV1().Services(namespace)
	_, err := serviceClient.Create(context.Background(), service, metav1.CreateOptions{})
	expectNoError(err)
	expectNoError(exutil.WaitForEndpoint(client, namespace, serviceName))
	service, err = serviceClient.Get(context.Background(), serviceName, metav1.GetOptions{})
	expectNoError(err)

	jig := svc.NewTestJig(client, namespace, serviceName)
	service, err = jig.WaitForLoadBalancer(context.Background(), svc.GetServiceLoadBalancerCreationTimeout(context.Background(), client))
	expectNoError(err)

	host := ""
	if service.Status.LoadBalancer.Ingress[0].IP != "" {
		host = service.Status.LoadBalancer.Ingress[0].IP
	} else if service.Status.LoadBalancer.Ingress[0].Hostname != "" {
		host = service.Status.LoadBalancer.Ingress[0].Hostname
	}
	Expect(host).NotTo(Equal(""))

	return net.JoinHostPort(host, strconv.Itoa(servicePort))
}

func checkConnectivityToHostWithCLI(f *e2e.Framework, oc *exutil.CLI, nodeName string, podName string, host string, timeout time.Duration, hostNetwork bool) error {
	namespace := f.Namespace.Name

	e2e.Logf("Creating an exec pod on node %v", nodeName)
	execPod := exutil.CreateExecPodOrFail(f.ClientSet, namespace, fmt.Sprintf("execpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		if pod.GenerateName == "" {
			pod.GenerateName = pod.Name
			pod.Name = ""
		}
		pod.Spec.NodeName = nodeName
		pod.Spec.HostNetwork = hostNetwork
	})
	defer func() {
		e2e.Logf("Cleaning up the exec pod")
		err := f.ClientSet.CoreV1().Pods(namespace).Delete(context.Background(), execPod.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}()

	var stdout string
	e2e.Logf("Waiting up to %v to wget %s", timeout, host)
	cmd := fmt.Sprintf("wget -T 30 -qO- %s", host)
	var err error
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(10 * time.Second) {
		stdout, err = oc.Run("exec").Args(execPod.Name, "-n", namespace, "--", "/bin/sh", "-x", "-c", cmd).Output()
		if err != nil {
			e2e.Logf("got err: %v, retry until timeout", err)
			continue
		}
		// Need to check output because wget -q might omit the error.
		if strings.TrimSpace(stdout) == "" {
			e2e.Logf("got empty stdout, retry until timeout")
			continue
		}
		break
	}
	return err
}

func createPodForService(client k8sclient.Interface, namespace, serviceName string, nodeName string, labelMap map[string]string) {
	exutil.LaunchWebserverPod(client, namespace, serviceName, nodeName)
	// FIXME: make e2e.LaunchWebserverPod() set the label when creating the pod
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		podClient := client.CoreV1().Pods(namespace)
		pod, err := podClient.Get(context.Background(), serviceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.ObjectMeta.Labels == nil {
			pod.ObjectMeta.Labels = labelMap
		} else {
			for key, value := range labelMap {
				pod.ObjectMeta.Labels[key] = value
			}
		}
		_, err = podClient.Update(context.Background(), pod, metav1.UpdateOptions{})
		return err
	})
	expectNoError(err)
}

func platformType(configClient configv1client.Interface) (configv1.PlatformType, error) {
	infrastructure, err := configClient.ConfigV1().Infrastructures().Get(context.Background(), "cluster", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	return infrastructure.Status.PlatformStatus.Type, nil
}

func checkConnectivityToHost(f *e2e.Framework, nodeName string, podName string, host string, timeout time.Duration, hostNetwork bool) error {
	e2e.Logf("Creating an exec pod on node %v", nodeName)
	execPod := exutil.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, fmt.Sprintf("execpod-sourceip-%s", nodeName), func(pod *corev1.Pod) {
		if pod.GenerateName == "" {
			pod.GenerateName = pod.Name
			pod.Name = ""
		}
		pod.Spec.NodeName = nodeName
		pod.Spec.HostNetwork = hostNetwork
	})
	defer func() {
		e2e.Logf("Cleaning up the exec pod")
		err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Delete(context.Background(), execPod.Name, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())
	}()

	var stdout string
	e2e.Logf("Waiting up to %v to wget %s", timeout, host)
	cmd := fmt.Sprintf("wget -T 30 -qO- %s", host)
	var err error
	for start := time.Now(); time.Since(start) < timeout; time.Sleep(10 * time.Second) {
		stdout, err = e2eoutput.RunHostCmd(execPod.Namespace, execPod.Name, cmd)
		if err != nil {
			e2e.Logf("got err: %v, retry until timeout", err)
			continue
		}
		// Need to check output because wget -q might omit the error.
		if strings.TrimSpace(stdout) == "" {
			e2e.Logf("got empty stdout, retry until timeout")
			continue
		}
		break
	}
	return err
}

func getKubeVirtPodFromGuestNode(framework *e2e.Framework, node v1.Node) (*corev1.Pod, error) {
	podList, err := framework.ClientSet.CoreV1().Pods(framework.Namespace.Name).List(context.Background(), metav1.ListOptions{
		LabelSelector: "kubevirt.io=virt-launcher",
	})
	Expect(err).NotTo(HaveOccurred())
	for _, pod := range podList.Items {
		if strings.Contains(pod.Name, node.Name) {
			return &pod, nil
		}
	}

	return nil, fmt.Errorf("kubevirt node's infra pod not found")
}

func getSchedulableInfraNode(framework *e2e.Framework, serverInfraNode string) (string, error) {

	// find an infra node to schedule the client on
	nodes, err := e2enode.GetReadySchedulableNodes(context.TODO(), framework.ClientSet)
	if err != nil {
		e2e.Logf("Unable to get schedulable nodes due to %v", err)
		return "", err
	}

	for _, node := range nodes.Items {
		if node.Name != serverInfraNode {
			return node.Name, nil
		}
	}
	return "", fmt.Errorf("infra node not found")
}

func getNodeInternalAddress(node *v1.Node) (string, error) {

	for _, addr := range node.Status.Addresses {

		if addr.Type == v1.NodeInternalIP {
			return addr.Address, nil
		}
	}
	return "", fmt.Errorf("no internal node ip found for node %s", node.Name)
}

func checkKubeVirtGuestClusterHostNetworkNodePortConnectivity(serverFramework, clientFramework *e2e.Framework) error {
	return checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework, true, v1.ServiceTypeNodePort)
}

func checkKubeVirtGuestClusterPodNetworkNodePortConnectivity(serverFramework, clientFramework *e2e.Framework) error {
	return checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework, false, v1.ServiceTypeNodePort)
}

func checkKubeVirtInfraClusterConnectivity(serverFramework, clientFramework *e2e.Framework, oc *exutil.CLI, serviceType v1.ServiceType) error {
	serverGuestNode, err := e2enode.GetRandomReadySchedulableNode(context.TODO(), serverFramework.ClientSet)
	Expect(err).NotTo(HaveOccurred())

	serverVMPod, err := getKubeVirtPodFromGuestNode(clientFramework, *serverGuestNode)
	Expect(err).NotTo(HaveOccurred())

	serverInfraNode := serverVMPod.Spec.NodeName
	serverGuestNodeIP := serverVMPod.Status.PodIP

	infraClientNode, err := getSchedulableInfraNode(clientFramework, serverInfraNode)
	Expect(err).NotTo(HaveOccurred())

	podName := names.SimpleNameGenerator.GenerateName("service-")

	serviceAddr := ""
	if serviceType == v1.ServiceTypeNodePort {
		serverGuestNodePort := launchWebserverNodePortService(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverGuestNode.Name)
		serviceAddr = net.JoinHostPort(serverGuestNodeIP, strconv.Itoa(int(serverGuestNodePort)))
	}
	if serviceType == v1.ServiceTypeLoadBalancer {
		serviceAddr = launchWebserverLoadBalancerService(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverGuestNode.Name)
	}

	return checkConnectivityToHostWithCLI(clientFramework, oc, infraClientNode, "service-wget", serviceAddr, 10*time.Minute, false)
}

func checkKubeVirtInfraClusterNodePortConnectivity(serverFramework, clientFramework *e2e.Framework, oc *exutil.CLI) error {
	return checkKubeVirtInfraClusterConnectivity(serverFramework, clientFramework, oc, v1.ServiceTypeNodePort)
}

func checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework *e2e.Framework, hostNetwork bool, serviceType v1.ServiceType) error {
	nodes, err := e2enode.GetBoundedReadySchedulableNodes(context.TODO(), serverFramework.ClientSet, 2)
	if err != nil {
		return err
	}

	clientNode := nodes.Items[0]
	serverNode := nodes.Items[1]
	podName := names.SimpleNameGenerator.GenerateName("service-")
	serviceAddr := ""
	if serviceType == v1.ServiceTypeNodePort {
		serverNodePort := launchWebserverNodePortService(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverNode.Name)

		ip, err := getNodeInternalAddress(&clientNode)
		Expect(err).NotTo(HaveOccurred())

		serviceAddr = net.JoinHostPort(ip, strconv.Itoa(int(serverNodePort)))

	}
	if serviceType == v1.ServiceTypeClusterIP {
		serviceAddr = exutil.LaunchWebserverPod(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverNode.Name)
	}
	if serviceType == v1.ServiceTypeLoadBalancer {
		serviceAddr = launchWebserverLoadBalancerService(serverFramework.ClientSet, serverFramework.Namespace.Name, podName, serverNode.Name)
	}
	return checkConnectivityToHost(clientFramework, clientNode.Name, "service-wget", serviceAddr, 10*time.Minute, hostNetwork)
}

func checkKubeVirtGuestClusterPodNetworkConnectivity(serverFramework, clientFramework *e2e.Framework) error {
	return checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework, false, v1.ServiceTypeClusterIP)

}

func checkKubeVirtGuestClusterHostNetworkConnectivity(serverFramework, clientFramework *e2e.Framework) error {
	return checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework, true, v1.ServiceTypeClusterIP)
}

func checkKubeVirtInfraClusterLoadBalancerConnectivity(serverFramework, clientFramework *e2e.Framework, oc *exutil.CLI) error {
	return checkKubeVirtInfraClusterConnectivity(serverFramework, clientFramework, oc, v1.ServiceTypeLoadBalancer)
}

func checkKubeVirtGuestClusterLoadBalancerConnectivity(serverFramework, clientFramework *e2e.Framework) error {
	return checkKubeVirtGuestClusterConnectivity(serverFramework, clientFramework, false, v1.ServiceTypeLoadBalancer)
}

func InKubeVirtClusterContext(oc *exutil.CLI, body func()) {
	Context("when running openshift cluster on KubeVirt virtual machines",
		func() {
			BeforeEach(func() {
				pType, err := platformType(oc.AdminConfigClient())
				expectNoError(err)
				if pType != configv1.KubevirtPlatformType {
					e2eskipper.Skipf("Not running in KubeVirt cluster")
				}
			})

			body()
		},
	)
}

func setMgmtFramework(mgmtFramework *e2e.Framework) *exutil.CLI {
	_, hcpNamespace, err := exutil.GetHypershiftManagementClusterConfigAndNamespace()
	Expect(err).NotTo(HaveOccurred())

	oc := exutil.NewHypershiftManagementCLI(hcpNamespace).AsAdmin()

	mgmtClientSet := oc.KubeClient()
	mgmtFramework.Namespace = &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: hcpNamespace,
		},
	}
	mgmtFramework.ClientSet = mgmtClientSet

	return oc
}

func expectNoError(err error, explain ...interface{}) {
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), explain...)
}
