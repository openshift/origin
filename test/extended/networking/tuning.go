package networking

import (
	"context"
	"fmt"
	g "github.com/onsi/ginkgo"
	t "github.com/onsi/ginkgo/extensions/table"
	o "github.com/onsi/gomega"
	"github.com/openshift/origin/test/extended/util"
	exutil "github.com/openshift/origin/test/extended/util"
	kappsv1 "k8s.io/api/apps/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"strings"
)

const (
	AllowlistDsName        = "allowlist-ds"
	AllowlistCleanupDsName = "allowlistcleanup-ds"
	AllowlistConfigMapName = "allowlist-cm"
	TuningNadName          = "tuningnad"
	AllowlistPodName       = "allowlist-pod"
	RbacName               = "allowlistrbac"
)

var _ = g.Describe("[sig-network][Feature:tuning]", func() {
	oc := exutil.NewCLI("tuning")
	f := oc.KubeFramework()

	t.DescribeTable("pod should start for each sysctl on whitelist", func(sysctl, value, path string) {
		namespace := f.Namespace.Name
		err := configureRbac(f.ClientSet, namespace)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create config map")
		err = configureAllowlistOnNodes(f.ClientSet, namespace, []string{"^net\\.ipv4\\.conf\\.IFNAME\\.[a-z_]*$\n"})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to configure allowlists on nodes")

		err = createTuningNad(oc.AdminConfig(), namespace, TuningNadName, map[string]string{sysctl: value})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create nad")

		util.CreateExecPodOrFail(f.ClientSet, namespace, AllowlistPodName, func(pod *kapiv1.Pod) {
			pod.ObjectMeta.Annotations =  map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s/%s", namespace, TuningNadName)}
		})
		result, err := isPodSysctlApplied(oc, namespace, AllowlistPodName, []string{"/bin/bash", "-c", fmt.Sprintf("cat %s", path)}, value)
		o.Expect(err).NotTo(o.HaveOccurred(), "error checking pod sysctls")
		o.Expect(result).To(o.BeTrue(), "unable to create daemonset")

		err = cleanupAllowlistsOnNodes(f.ClientSet, namespace)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create daemonset")
	},
		t.Entry("net.ipv4.conf.IFNAME.arp_filter", "net.ipv4.conf.IFNAME.arp_filter", "1", "/proc/sys/net/ipv4/conf/net1/arp_filter"),
	)
})

func configureRbac(c kclientset.Interface, namespace string) error {
	err := util.CreateRole(c, namespace, RbacName)
	if err != nil {
		return err
	}
	return util.CreateRoleBinding(c, namespace, RbacName)
}

func configureAllowlistOnNodes(c kclientset.Interface, namespace string, sysctls []string) error {
	err := createConfigMap(c, namespace, AllowlistConfigMapName, sysctls)
	if err != nil {
		return err
	}
	_, err = createDS(c, namespace, AllowlistDsName, "cp /allowlist/allowlist.conf /host/ && sleep INF", "test -f /host/allowlist.conf", AllowlistConfigMapName)
	if err != nil {
		return err
	}
	return util.WaitForDSRunning(c, namespace, AllowlistDsName)
}

func cleanupAllowlistsOnNodes(c kclientset.Interface, namespace string) error {
	_, err := createDS(c, namespace, AllowlistCleanupDsName, "rm -f /host/allowlist.conf && sleep INF", "! test -f /host/allowlist.conf", AllowlistConfigMapName)
	if err != nil {
		return err
	}
	return util.WaitForDSRunning(c, namespace, AllowlistCleanupDsName)
}

func isPodSysctlApplied(oc *exutil.CLI, namespace string, name string, command []string, expectedResult string) (bool, error) {
	pod, err := oc.KubeFramework().ClientSet.CoreV1().Pods(namespace).Get(context.Background(), AllowlistPodName, metav1.GetOptions{})
	if err != nil {
		return true, err
	}
	output, err := util.ExecCommandOnPod(oc, *pod, command)
	if err != nil {
		return true, err
	}
	return strings.TrimSpace(output.String()) == expectedResult, nil
}

func createConfigMap(c kclientset.Interface, namespace string, name string, sysctls []string) error {
	allowList := ""
	for _, sysctl := range sysctls {
		allowList = allowList + sysctl + "\n"
	}
	configMap := &kapiv1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: map[string]string{"allowlist.conf": allowList},
	}
	_, err := c.CoreV1().ConfigMaps(namespace).Create(context.Background(), configMap, metav1.CreateOptions{})
	return err
}

func createDS(c kclientset.Interface, namespace string, name string, command string, readinessCommand string, mountCM string) (*kappsv1.DaemonSet, error) {
	var volumesDefaultMode int32 = 444
	var hostPathType kapiv1.HostPathType = kapiv1.HostPathDirectoryOrCreate
	return util.CreateDS(c, namespace, name, command, readinessCommand,
		[]kapiv1.VolumeMount{
			{Name: "allowlist", MountPath: "/allowlist"},
			{Name: "tuningdir", MountPath: "/host", ReadOnly: false},
		},
		[]kapiv1.Volume{
			{Name: "allowlist", VolumeSource: kapiv1.VolumeSource{ConfigMap: &kapiv1.ConfigMapVolumeSource{LocalObjectReference: kapiv1.LocalObjectReference{Name: mountCM}, DefaultMode: &volumesDefaultMode}}},
			{Name: "tuningdir", VolumeSource: kapiv1.VolumeSource{HostPath: &kapiv1.HostPathVolumeSource{Path: "/etc/cni/tuning/", Type: &hostPathType}}},
		},
	)
}

func createTuningNad(config *rest.Config, namespace string, nadName string, sysctls map[string]string) error {
	sysctlString := ""
	first := true
	for sysctl, value := range sysctls {
		if first {
			first = false
		} else {
			sysctlString = sysctlString + ","
		}
		sysctlString = sysctlString + fmt.Sprintf("\"%s\":\"%s\"", sysctl, value)
	}
	nadConfig := fmt.Sprintf(`{"cniVersion":"0.4.0","name":"%s","plugins":[{"type":"bridge","bridge":"tunbr","ipam":{"type":"static","addresses":[{"address":"10.10.0.1/24"}]}},{"type":"tuning","sysctl":{%s}}]}`, nadName, sysctlString)
	return util.CreateNetworkAttachmentDefinition(config, namespace, TuningNadName, nadConfig)
}
