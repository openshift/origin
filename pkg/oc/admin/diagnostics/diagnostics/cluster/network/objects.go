package network

// Resource objects used by network diagnostics
import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
)

const (
	networkDiagTestPodSelector = "network-diag-pod-name"

	testServicePort = 9876
)

func GetNetworkDiagnosticsPod(diagnosticsImage, command, podName, nodeName string) *kapi.Pod {
	privileged := true
	hostRootVolName := "host-root-dir"
	secretVolName := "kconfig-secret"
	secretDirBaseName := "secrets"
	gracePeriod := int64(0)

	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
		Spec: kapi.PodSpec{
			RestartPolicy:                 kapi.RestartPolicyNever,
			TerminationGracePeriodSeconds: &gracePeriod,
			SecurityContext: &kapi.PodSecurityContext{
				HostPID:     true,
				HostIPC:     true,
				HostNetwork: true,
			},
			NodeName: nodeName,
			Containers: []kapi.Container{
				{
					Name:            podName,
					Image:           diagnosticsImage,
					ImagePullPolicy: kapi.PullIfNotPresent,
					SecurityContext: &kapi.SecurityContext{
						Privileged: &privileged,
					},
					Env: []kapi.EnvVar{
						{
							Name:  kclientcmd.RecommendedConfigPathEnvVar,
							Value: fmt.Sprintf("/%s/%s", secretDirBaseName, strings.ToLower(kclientcmd.RecommendedConfigPathEnvVar)),
						},
					},
					VolumeMounts: []kapi.VolumeMount{
						{
							Name:      hostRootVolName,
							MountPath: util.NetworkDiagContainerMountPath,
						},
						{
							Name:      secretVolName,
							MountPath: fmt.Sprintf("%s/%s", util.NetworkDiagContainerMountPath, secretDirBaseName),
							ReadOnly:  true,
						},
					},
					Command: []string{"/bin/bash", "-c"},
					Args:    []string{getNetworkDebugScript(util.NetworkDiagContainerMountPath, command)},
				},
			},
			Volumes: []kapi.Volume{
				{
					Name: hostRootVolName,
					VolumeSource: kapi.VolumeSource{
						HostPath: &kapi.HostPathVolumeSource{
							Path: "/",
						},
					},
				},
				{
					Name: secretVolName,
					VolumeSource: kapi.VolumeSource{
						Secret: &kapi.SecretVolumeSource{
							SecretName: util.NetworkDiagSecretName,
						},
					},
				},
			},
		},
	}
	return pod
}

func GetTestPod(testPodImage, testPodProtocol, podName, nodeName string, testPodPort int) *kapi.Pod {
	gracePeriod := int64(0)

	pod := &kapi.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				networkDiagTestPodSelector: podName,
			},
		},
		Spec: kapi.PodSpec{
			RestartPolicy:                 kapi.RestartPolicyNever,
			TerminationGracePeriodSeconds: &gracePeriod,
			NodeName:                      nodeName,
			Containers: []kapi.Container{
				{
					Name:            podName,
					Image:           testPodImage,
					ImagePullPolicy: kapi.PullIfNotPresent,
				},
			},
		},
	}

	if getTrimmedImage(testPodImage) == getTrimmedImage(util.GetNetworkDiagDefaultTestPodImage()) {
		pod.Spec.Containers[0].Command = []string{
			"socat", "-T", "1", "-d",
			fmt.Sprintf("%s-l:%d,reuseaddr,fork,crlf", testPodProtocol, testPodPort),
			"system:\"echo 'HTTP/1.0 200 OK'; echo 'Content-Type: text/plain'; echo; echo 'Hello OpenShift'\"",
		}
	}
	return pod
}

func getTrimmedImage(image string) string {
	// Image format could be: [<dns-name>/]openshift/origin-deployer[:<tag>]
	tokens := strings.Split(image, "/")
	trimImageWithTag := tokens[len(tokens)-1]

	return strings.Split(trimImageWithTag, ":")[0]
}

func GetTestService(serviceName, podName, podProtocol, nodeName string, podPort int) *kapi.Service {
	return &kapi.Service{
		ObjectMeta: metav1.ObjectMeta{Name: serviceName},
		Spec: kapi.ServiceSpec{
			Type: kapi.ServiceTypeClusterIP,
			Selector: map[string]string{
				networkDiagTestPodSelector: podName,
			},
			Ports: []kapi.ServicePort{
				{
					Protocol:   kapi.Protocol(podProtocol),
					Port:       testServicePort,
					TargetPort: intstr.FromInt(podPort),
				},
			},
		},
	}
}

func getNetworkDebugScript(nodeRootFS, command string) string {
	return fmt.Sprintf(`
#!/bin/bash
#
# Based on containerized/non-containerized openshift install,
# this script sets the environment so that docker, openshift, iptables, etc.
# binaries are availble for network diagnostics.
#
set -o nounset
set -o pipefail

node_rootfs=%s
cmd="%s"

# Origin image: openshift/node, OSE image: openshift3/node
node_image_regex="^openshift.*/node"

node_container_id="$(chroot "${node_rootfs}" docker ps --format='{{.Image}} {{.ID}}' | grep "${node_image_regex}" | cut -d' ' -f2)"

if [[ -z "${node_container_id}" ]]; then # non-containerized openshift env

    chroot "${node_rootfs}" ${cmd}

else # containerized env

    # On containerized install, docker on the host is used by node container,
    # For the privileged network diagnostics pod to use all the binaries on the node:
    # - Copy kubeconfig secret to node mount namespace
    # - Run openshift under the mount namespace of node

    node_docker_pid="$(chroot "${node_rootfs}" docker inspect --format='{{.State.Pid}}' "${node_container_id}")"
    kubeconfig="/etc/origin/node/kubeconfig"
    cp "${node_rootfs}/secrets/kubeconfig" "${node_rootfs}/${kubeconfig}"

    chroot "${node_rootfs}" nsenter -m -t "${node_docker_pid}" -- /bin/bash -c 'KUBECONFIG='"${kubeconfig} ${cmd}"''

fi`, nodeRootFS, command)
}
