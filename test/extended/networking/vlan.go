package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nadtypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"
	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	podframework "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network][Feature:vlan]", func() {
	oc := exutil.NewCLI("vlan")
	f := oc.KubeFramework()
	for _, pluginType := range []string{"vlan", "ipvlan", "macvlan"} {
		vlanNadConfig := `{
		"cniVersion":"0.4.0","name":"%s",
		"plugins":[
			{ 
				"type": "%s", 
				"linkInContainer": true,
				"master": "%s", 
				"mtu": 1500, 
				"vlanId": %s,
				"ipam": { "type": "static", "addresses": [{ "address": "%s/24" }] }
			}]
		}`

		g.It(fmt.Sprintf("should create pingable pods with %s interface on an in-container master [apigroup:k8s.cni.cncf.io]", pluginType), func() {
			namespace := f.Namespace.Name
			podName1, podName2, podName3 := "pod1", "pod2", "pod3"
			vlan1, vlan2, otherVlan := "vlan01", "vlan02", "vlan03"
			ip1, ip2, otherIp := "10.10.0.2", "10.10.0.3", "10.10.0.4"
			bridge := "br0"
			vlanId, otherVlanId := "73", "37"
			bridgeAnnotation := fmt.Sprintf("%s/%s@%s", namespace, bridge, bridge)

			g.By("creating bridge network attachment definition")
			err := createNetworkAttachmentDefinition(
				oc.AdminConfig(),
				namespace,
				bridge,
				fmt.Sprintf(`{"cniVersion":"0.4.0","name":"%s","plugins":[{ "type": "bridge", "bridge": "%s"}]}`, bridge, bridge),
			)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to create bridge network-attachment-definition"))

			g.By("creating a vlan network attachment definition")
			err = createNetworkAttachmentDefinition(
				oc.AdminConfig(),
				namespace,
				vlan1,
				fmt.Sprintf(vlanNadConfig, vlan1, pluginType, bridge, vlanId, ip1),
			)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to create %s network-attachment-definition", pluginType))

			err = createNetworkAttachmentDefinition(
				oc.AdminConfig(),
				namespace,
				vlan2,
				fmt.Sprintf(vlanNadConfig, vlan2, pluginType, bridge, vlanId, ip2),
			)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to create %s network-attachment-definition", pluginType))

			err = createNetworkAttachmentDefinition(
				oc.AdminConfig(),
				namespace,
				otherVlan,
				fmt.Sprintf(vlanNadConfig, otherVlan, pluginType, bridge, otherVlanId, otherIp),
			)
			o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to create %s network-attachment-definition", pluginType))

			g.By("creating first pod and checking results")
			exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName1, func(pod *kapiv1.Pod) {
				vlanAnnotation := fmt.Sprintf("%s/%s@%s", namespace, vlan1, vlan1)
				pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s, %s", bridgeAnnotation, vlanAnnotation)}
			})
			pod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), podName1, metav1.GetOptions{})
			o.Expect(err).ToNot(o.HaveOccurred())

			networkStatusString, ok := pod.Annotations["k8s.v1.cni.cncf.io/network-status"]
			o.Expect(ok).To(o.BeTrue())
			o.Expect(networkStatusString).ToNot(o.BeNil())
			networkStatuses := []nadtypes.NetworkStatus{}
			o.Expect(json.Unmarshal([]byte(networkStatusString), &networkStatuses)).ToNot(o.HaveOccurred())
			o.Expect(networkStatuses).To(o.HaveLen(3))
			o.Expect(networkStatuses[2].Interface).To(o.Equal(vlan1))
			o.Expect(networkStatuses[2].Name).To(o.Equal(fmt.Sprintf("%s/%s", namespace, vlan1)))

			g.By("having a second pod pinging the first pod")
			exutil.CreateExecPodOrFail(f.ClientSet, namespace, podName2, func(pod *kapiv1.Pod) {
				vlanAnnotation := fmt.Sprintf("%s/%s@%s", namespace, vlan2, vlan2)
				pod.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s, %s", bridgeAnnotation, vlanAnnotation)}
				pod.Spec.Containers[0].Command = []string{"/bin/bash", "-c", fmt.Sprintf("ping -c 1 %s", ip1)}
			})

			g.By("having another pod on different vlan unable to ping the first pod")

			podDefinition := frameworkpod.NewAgnhostPod(f.Namespace.Name, podName3, nil, nil, nil)
			vlanAnnotation := fmt.Sprintf("%s/%s@%s", namespace, otherVlan, otherVlan)
			podDefinition.ObjectMeta.Annotations = map[string]string{"k8s.v1.cni.cncf.io/networks": fmt.Sprintf("%s, %s", bridgeAnnotation, vlanAnnotation)}
			podDefinition.Spec.Containers[0].Args = nil
			podDefinition.Spec.Containers[0].Command = []string{"/bin/bash", "-c", fmt.Sprintf("ping -c 1 %s && touch /tmp/ready", ip1)}
			podDefinition.Spec.Containers[0].ReadinessProbe = &corev1.Probe{
				ProbeHandler: corev1.ProbeHandler{
					Exec: &corev1.ExecAction{
						Command: []string{"/bin/bash", "-c", fmt.Sprintf("test -f /tmp/ready")},
					},
				},
			}

			podDefinition.Spec.Containers[0].SecurityContext = podframework.GetRestrictedContainerSecurityContext()
			podDefinition.Spec.SecurityContext = podframework.GetRestrictedPodSecurityContext()
			otherVlandPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Create(context.TODO(), podDefinition, metav1.CreateOptions{})
			o.Expect(err).NotTo(o.HaveOccurred(), "error creating pod")
			err = wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
				retrievedPod, err := f.ClientSet.CoreV1().Pods(f.Namespace.Name).Get(context.TODO(), otherVlandPod.Name, metav1.GetOptions{})
				if err != nil {
					return false, err
				}
				return len(retrievedPod.Status.ContainerStatuses) == 1 && retrievedPod.Status.ContainerStatuses[0].Ready == false, nil
			})
			o.Expect(err).NotTo(o.HaveOccurred(), "ping container should not run successfully")
		})
	}
})
