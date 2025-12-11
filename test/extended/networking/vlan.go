package networking

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	nadtypes "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/apis/k8s.cni.cncf.io/v1"
	nadclient "github.com/k8snetworkplumbingwg/network-attachment-definition-client/pkg/client/clientset/versioned"
	g "github.com/onsi/ginkgo/v2"
	o "github.com/onsi/gomega"
	exutil "github.com/openshift/origin/test/extended/util"

	corev1 "k8s.io/api/core/v1"
	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	frameworkpod "k8s.io/kubernetes/test/e2e/framework/pod"
	podframework "k8s.io/kubernetes/test/e2e/framework/pod"
)

var _ = g.Describe("[sig-network][Feature:vlan]", func() {
	oc := exutil.NewCLI("vlan")
	f := oc.KubeFramework()

	var podName1 string
	var podName2 string
	var podName3 string
	var vlan1 string
	var vlan2 string
	var otherVlan string
	var ip1 string
	var ip2 string
	var otherIp string
	var bridge string
	var namespace string
	var vlanNadConfig string

	g.BeforeEach(func() {
		namespace = f.Namespace.Name
		vlanNadConfig = `{
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

		podName1 = fmt.Sprintf("pod1-%s", utilrand.String(4))
		podName2 = fmt.Sprintf("pod2-%s", utilrand.String(4))
		podName3 = fmt.Sprintf("pod3-%s", utilrand.String(4))

		vlan1 = fmt.Sprintf("vlan01-%s", utilrand.String(4))
		vlan2 = fmt.Sprintf("vlan02-%s", utilrand.String(4))
		otherVlan = fmt.Sprintf("vlan03-%s", utilrand.String(4))

		bridge = fmt.Sprintf("br0-%s", utilrand.String(4))

		g.By("creating bridge network attachment definition")
		err := createNetworkAttachmentDefinition(
			oc.AdminConfig(),
			namespace,
			bridge,
			fmt.Sprintf(`{"cniVersion":"0.4.0","name":"%s","plugins":[{ "type": "bridge", "bridge": "%s"}]}`, bridge, bridge),
		)
		o.Expect(err).NotTo(o.HaveOccurred(), fmt.Sprintf("unable to create bridge network-attachment-definition"))
	})

	g.AfterEach(func() {
		for _, pod := range []string{podName1, podName2, podName3} {
			_ = f.ClientSet.CoreV1().Pods(namespace).Delete(context.TODO(), pod, metav1.DeleteOptions{})
		}

		c, err := nadclient.NewForConfig(oc.AdminConfig())
		o.Expect(err).NotTo(o.HaveOccurred())

		for _, nad := range []string{vlan1, vlan2, otherVlan, bridge} {
			err = c.K8sCniCncfIoV1().NetworkAttachmentDefinitions(namespace).Delete(context.TODO(), nad, metav1.DeleteOptions{})
			o.Expect(err).NotTo(o.HaveOccurred())
		}
	})

	for _, pluginType := range []string{"vlan", "ipvlan", "macvlan"} {
		g.It(fmt.Sprintf("should create pingable pods with %s interface on an in-container master [apigroup:k8s.cni.cncf.io]", pluginType), g.Label("Size:M"), func() {
			vlanId, otherVlanId := "73", "37"
			ip1, ip2, otherIp = "10.10.0.2", "10.10.0.3", "10.10.0.4"
			bridgeAnnotation := fmt.Sprintf("%s/%s@%s", namespace, bridge, bridge)

			g.By("creating a vlan network attachment definition")
			err := createNetworkAttachmentDefinition(
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
			var networkStatuses []nadtypes.NetworkStatus
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
