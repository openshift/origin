package network

// Resource objects used by network diagnostics
import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/oc/cli/admin/diagnostics/diagnostics/cluster/network/in_pod/util"
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
					Args:    []string{fmt.Sprintf("chroot %s %s", util.NetworkDiagContainerMountPath, command)},
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

	if testPodImage == util.GetNetworkDiagDefaultTestPodImage() {
		pod.Spec.Containers[0].Command = []string{
			"socat", "-T", "1", "-d",
			fmt.Sprintf("%s-l:%d,reuseaddr,fork,crlf", testPodProtocol, testPodPort),
			"system:\"echo 'HTTP/1.0 200 OK'; echo 'Content-Type: text/plain'; echo; echo 'Hello OpenShift'\"",
		}
	}
	return pod
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
