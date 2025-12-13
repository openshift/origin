package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"

	nadtypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	"github.com/lithammer/dedent"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("[sig-network][Feature:bond]", func() {
	oc := exutil.NewCLI("bond")
	f := oc.KubeFramework()

	g.It("should create a pod with bond interface [apigroup:k8s.cni.cncf.io]", g.Label("Size:M"), func() {
		namespace := f.Namespace.Name
		podName1, podName2 := "pod1", "pod2"
		bondnad1, bondnad2 := "bondnad1", "bondnad2"

		g.By("creating network attachment definitions")
		err := createNetworkAttachmentDefinition(
			oc.AdminConfig(),
			namespace,
			"macvlannad",
			`{"cniVersion":"0.4.0","name":"macvlan-nad","plugins":[{ "type": "macvlan"}]}`,
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create macvlan network-attachment-definition")

		networkConfig, err := oc.AdminConfigClient().ConfigV1().Networks().Get(context.Background(), "cluster", metav1.GetOptions{})
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to get network config")

		err = createBondNAD(
			oc.AdminConfig(),
			namespace,
			bondnad1,
			"192.0.2.10/24",
			networkConfig.Status.ClusterNetworkMTU,
			"net1",
			"net2",
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create bond network-attachment-definition")

		err = createBondNAD(
			oc.AdminConfig(),
			namespace,
			bondnad2,
			"192.0.2.11/24",
			networkConfig.Status.ClusterNetworkMTU,
			"net1",
			"net2",
		)
		o.Expect(err).NotTo(o.HaveOccurred(), "unable to create bond network-attachment-definition")

		g.By("creating first pod and checking results")
		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName1, func(pod *kapiv1.Pod) {
			macVlanAnnotation := fmt.Sprintf("%s/%s", namespace, "macvlannad")
			bondAnnotation := fmt.Sprintf("%s/%s@%s", namespace, bondnad1, bondnad1)
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s, %s, %s", macVlanAnnotation, macVlanAnnotation, bondAnnotation)}
		})
		pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName1, metav1.GetOptions{})
		o.Expect(err).ToNot(o.HaveOccurred())

		networkStatusString, ok := pod.Annotations["k8s.v1.cni.cncf.io/network-status"]
		o.Expect(ok).To(o.BeTrue())
		o.Expect(networkStatusString).ToNot(o.BeNil())
		networkStatuses := []nadtypes.NetworkStatus{}
		o.Expect(json.Unmarshal([]byte(networkStatusString), &networkStatuses)).ToNot(o.HaveOccurred())
		o.Expect(networkStatuses).To(o.HaveLen(4))
		o.Expect(networkStatuses[3].Interface).To(o.Equal(bondnad1))
		o.Expect(networkStatuses[3].Name).To(o.Equal(fmt.Sprintf("%s/%s", namespace, bondnad1)))

		g.By("having a second pod pinging the first pod")
		exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName2, func(pod *kapiv1.Pod) {
			macVlanAnnotation := fmt.Sprintf("%s/%s", namespace, "macvlannad")
			bondAnnotation := fmt.Sprintf("%s/%s@%s", namespace, bondnad2, bondnad2)
			pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s, %s, %s", macVlanAnnotation, macVlanAnnotation, bondAnnotation)}
			pod.Spec.Containers[0].Command = []string{"/bin/bash", "-c", fmt.Sprintf("ping -c 1 %s", "192.0.2.10")}
		})
	})
})

func createBondNAD(config *rest.Config, namespace string, nadName string, ip string, mtu int, slaveNames ...string) error {
	slaves := []string{}
	for _, name := range slaveNames {
		slaves = append(slaves, fmt.Sprintf("{\"name\": \"%s\"}", name))
	}
	links := strings.Join(slaves, ",")
	nadConfig := dedent.Dedent(fmt.Sprintf(`{
		"type": "bond",
		"cniVersion": "0.3.1",
		"name": "%s",
		"mode": "active-backup",
		"failOverMac": 1,
		"linksInContainer": true,
		"miimon": "100",
		"mtu": %d,
		"links": [%s],
		"ipam": {"type":"static","addresses":[{"address":"%s"}]}
	}`, nadName, mtu, links, ip))
	return createNetworkAttachmentDefinition(config, namespace, nadName, nadConfig)
}
