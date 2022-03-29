package networking

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	"strings"
	"time"
)

const (
	tuningNADName = "tuningnad"
	baseNAD       = "basenad"
	podName       = "testpod"

	sysctlKey  = "net.ipv4.conf.IFNAME.send_redirects"
	sysctlPath = "/proc/sys/net/ipv4/conf/%s/send_redirects"
)

type SysctlVariant struct {
	Sysctl string
	Value  string
	Path   string
}

var whitelistedSysctls = []SysctlVariant{
	{Sysctl: "net.ipv4.conf.IFNAME.send_redirects", Value: "1", Path: "/proc/sys/net/ipv4/conf/net1/send_redirects"},
	{Sysctl: "net.ipv4.conf.IFNAME.accept_source_route", Value: "1", Path: "/proc/sys/net/ipv4/conf/net1/accept_source_route"},
	{Sysctl: "net.ipv6.conf.IFNAME.accept_source_route", Value: "1", Path: "/proc/sys/net/ipv6/conf/net1/accept_source_route"},
	{Sysctl: "net.ipv4.conf.IFNAME.accept_redirects", Value: "1", Path: "/proc/sys/net/ipv4/conf/net1/accept_redirects"},
	{Sysctl: "net.ipv6.conf.IFNAME.accept_redirects", Value: "1", Path: "/proc/sys/net/ipv6/conf/net1/accept_redirects"},
	{Sysctl: "net.ipv4.conf.IFNAME.secure_redirects", Value: "1", Path: "/proc/sys/net/ipv4/conf/net1/secure_redirects"},
	{Sysctl: "net.ipv6.neigh.IFNAME.base_reachable_time_ms", Value: "30005", Path: "/proc/sys/net/ipv6/neigh/net1/base_reachable_time_ms"},
	{Sysctl: "net.ipv6.neigh.IFNAME.retrans_time_ms", Value: "1005", Path: "/proc/sys/net/ipv6/neigh/net1/retrans_time_ms"},
}

var _ = g.Describe("[sig-network][Feature:tuning]", func() {
	oc := exutil.NewCLI("tuning")
	f := oc.KubeFramework()

	g.It("pod should start with all sysctl on whitelist", func() {
		namespace := f.Namespace.Name
		sysctls := map[string]string{}
		for _, sysctl := range whitelistedSysctls {
			sysctls[sysctl.Sysctl] = sysctl.Value
		}
		err := createTuningNAD(oc.AdminConfig(), namespace, tuningNADName, sysctls)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName, func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, tuningNADName)}
		})
		for _, sysctl := range whitelistedSysctls {
			result, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", sysctl.Path).Output()
			o.Expect(err).NotTo(o.HaveOccurred(), "error checking pod sysctl")
			o.Expect(result).To(o.Equal(sysctl.Value), "incorrect sysctl value")
		}
	})
	t.DescribeTable("pod should not start for sysctls not on whitelist", func(sysctl, value string) {
		namespace := f.Namespace.Name
		tuningNADName := "tuningnadwithdisallowedsysctls"
		err := createTuningNAD(oc.AdminConfig(), namespace, tuningNADName, map[string]string{sysctl: value})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		podDefinition := frameworkpod.NewAgnhostPod(namespace, podName, nil, nil, nil)
		podDefinition.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, tuningNADName)}
		pod := f.PodClient().Create(podDefinition)
		err = frameworkpod.WaitForPodCondition(f.ClientSet, namespace, pod.Name, "Failed", 30*time.Second, func(pod *kapiv1.Pod) (bool, error) {
			if pod.Status.Phase == kapiv1.PodPending {
				return true, nil
			}
			return false, nil
		})
		o.Expect(err).NotTo(o.HaveOccurred(), "incorrect pod status")
		o.Consistently(func() kapiv1.PodPhase {
			pod, err := f.ClientSet.CoreV1().Pods(namespace).Get(context.Background(), podName, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())
			return pod.Status.Phase
		}, 15*time.Second, 3*time.Second).Should(o.Equal(kapiv1.PodPending))
	},
		t.Entry("net.ipv4.conf.all.send_redirects", "net.ipv4.conf.all.send_redirects", "1"),
		t.Entry("net.ipv4.conf.IFNAME.arp_filter", "net.ipv4.conf.IFNAME.arp_filter", "1"),
	)

	g.It("pod sysctls should not affect node", func() {
		namespace := f.Namespace.Name
		g.By("creating a preexisting pod to check node sysctl")
		nodePod := frameworkpod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, "nodeaccess-pod-", func(pod *kapiv1.Pod) {
			pod.Spec.Volumes = []kapiv1.Volume{
				{Name: "sysvolume", VolumeSource: kapiv1.VolumeSource{HostPath: &kapiv1.HostPathVolumeSource{Path: "/sys/class/net"}}},
				{Name: "procvolume", VolumeSource: kapiv1.VolumeSource{HostPath: &kapiv1.HostPathVolumeSource{Path: "/proc"}}},
			}
			pod.Spec.Containers[0].VolumeMounts = []kapiv1.VolumeMount{{Name: "sysvolume", MountPath: "/host/net"}, {Name: "procvolume", MountPath: "/host/proc"}}
			pod.Spec.HostNetwork = true
		})

		g.By("retrieving node interface name to create sysctl pod with interface of the same name")
		nodeInterfaceNames, err := oc.AsAdmin().Run("exec").Args(nodePod.Name, "--", "ls", "/host/net").Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get interface names")
		nodeInterfaceName := strings.Fields(nodeInterfaceNames)[0]
		if nodeInterfaceName == "lo" {
			nodeInterfaceName = strings.Fields(nodeInterfaceNames)[1]
		}

		g.By("getting the value of the node sysctl")
		nodeSysctlValue, err := oc.AsAdmin().Run("exec").Args(nodePod.Name, "--", "cat", "/host"+fmt.Sprintf(sysctlPath, nodeInterfaceName)).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl")
		sysctlValue := "1"
		if sysctlValue == nodeSysctlValue {
			sysctlValue = "0"
		}
		g.By("creating a network-attachment-definition with sysctl of value other than the node sysctl value")
		err = createTuningNAD(oc.AdminConfig(), namespace, tuningNADName, map[string]string{sysctlKey: sysctlValue})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		g.By("creating a pod with a sysctl set")
		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName, func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s@%s", namespace, tuningNADName, nodeInterfaceName)}
			pod.Spec.NodeName = nodePod.Spec.NodeName
		})

		g.By("checking the value of the sysctl on the pod is that specified in the network-attachment-defintion")
		result, err := oc.AsAdmin().Run("exec").Args(podName, "-n", namespace, "--", "cat", fmt.Sprintf(sysctlPath, nodeInterfaceName)).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error checking pod sysctls")
		o.Expect(result).To(o.Equal(sysctlValue), "incorrect sysctl value")

		g.By("checking that the value of the node sysctl did not change")
		nodeSysctlValue2, err := oc.AsAdmin().Run("exec").Args(nodePod.Name, "--", "cat", "/host"+fmt.Sprintf(sysctlPath, nodeInterfaceName)).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "error checking pod sysctls")
		o.Expect(nodeSysctlValue).Should(o.Equal(nodeSysctlValue2))
	})

	g.It("pod sysctl should not affect existing pods", func() {
		namespace := f.Namespace.Name
		path := fmt.Sprintf(sysctlPath, "net1")
		err := createNAD(oc.AdminConfig(), namespace, baseNAD)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		previousPod := frameworkpod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, "previous-pod-", func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, baseNAD)}
		})
		podOutputBeforeSysctlAplied, err := oc.AsAdmin().Run("exec").Args(previousPod.Name, "--", "cat", path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		sysctlValue := "1"
		if sysctlValue == podOutputBeforeSysctlAplied {
			sysctlValue = "0"
		}
		err = createTuningNAD(oc.AdminConfig(), namespace, tuningNADName, map[string]string{sysctlKey: sysctlValue})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName, func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, tuningNADName)}
			pod.Spec.NodeName = previousPod.Spec.NodeName
		})
		podOutputAfterSysctlAplied, err := oc.AsAdmin().Run("exec").Args(previousPod.Name, "--", "cat", path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		o.Expect(podOutputBeforeSysctlAplied).Should(o.Equal(podOutputAfterSysctlAplied))
	})

	g.It("pod sysctl should not affect newly created pods", func() {
		namespace := f.Namespace.Name
		path := fmt.Sprintf(sysctlPath, "net1")

		err := createNAD(oc.AdminConfig(), namespace, baseNAD)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		previousPod := frameworkpod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, baseNAD)}
		})
		podOutputBeforeSysctlAplied, err := oc.AsAdmin().Run("exec").Args(previousPod.Name, "--", "cat", path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		sysctlValue := "1"
		if sysctlValue == podOutputBeforeSysctlAplied {
			sysctlValue = "0"
		}
		err = createTuningNAD(oc.AdminConfig(), namespace, tuningNADName, map[string]string{sysctlKey: sysctlValue})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create network-attachment-definition")

		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName, func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, tuningNADName)}
			pod.Spec.NodeName = previousPod.Spec.NodeName

		})
		podOutputAfterSysctlAplied, err := oc.AsAdmin().Run("exec").Args(previousPod.Name, "--", "cat", path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		o.Expect(podOutputBeforeSysctlAplied).Should(o.Equal(podOutputAfterSysctlAplied))

		nextPod := frameworkpod.CreateExecPodOrFail(f.ClientSet, f.Namespace.Name, "sysctl-pod-", func(pod *kapiv1.Pod) {
			pod.Spec.NodeName = previousPod.Spec.NodeName
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, baseNAD)}

		})
		podOutput, err := oc.AsAdmin().Run("exec").Args(nextPod.Name, "--", "cat", path).Output()
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to check sysctl value")
		o.Expect(podOutput).Should(o.Equal(podOutputBeforeSysctlAplied))
	})

})

func createNAD(config *rest.Config, namespace string, nadName string) error {
	nadConfig := fmt.Sprintf(`{"cniVersion":"0.4.0","name":"%s","plugins":[{"type":"bridge","bridge":"tunbr","ipam":{"type":"static","addresses":[{"address":"10.10.0.1/24"}]}}]}`, nadName)
	return createNetworkAttachmentDefinition(config, namespace, nadName, nadConfig)
}

func createTuningNAD(config *rest.Config, namespace string, nadName string, sysctls map[string]string) error {
	sysctlString := ""
	for sysctl, value := range sysctls {
		if len(sysctlString) > 0 {
			sysctlString = sysctlString + ","
		}
		sysctlString = sysctlString + fmt.Sprintf("\"%s\":\"%s\"", sysctl, value)
	}
	nadConfig := fmt.Sprintf(`{"cniVersion":"0.4.0","name":"%s","plugins":[{"type":"bridge","bridge":"tunbr","ipam":{"type":"static","addresses":[{"address":"10.10.0.1/24"}]}},{"type":"tuning","sysctl":{%s}}]}`, nadName, sysctlString)
	return createNetworkAttachmentDefinition(config, namespace, nadName, nadConfig)
}
