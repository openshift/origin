package cli

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/openshift/origin/test/extended/util/image"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

func newHelloPod() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "hello-openshift",
			Labels: map[string]string{
				"name": "hello-openshift",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:  "hello-openshift",
					Image: k8simage.GetE2EImage(k8simage.Agnhost),
					Args:  []string{"netexec"},
					Ports: []corev1.ContainerPort{
						{
							ContainerPort: 8080,
							Protocol:      corev1.ProtocolTCP,
						},
					},
					VolumeMounts: []corev1.VolumeMount{
						{
							Name:      "tmp",
							MountPath: "/tmp",
						},
					},
					TerminationMessagePath: "/dev/termination-log",
					ImagePullPolicy:        corev1.PullIfNotPresent,
				},
			},
			Volumes: []corev1.Volume{
				{
					Name: "tmp",
					VolumeSource: corev1.VolumeSource{
						EmptyDir: &corev1.EmptyDirVolumeSource{},
					},
				},
			},
			RestartPolicy: corev1.RestartPolicyAlways,
			DNSPolicy:     corev1.DNSClusterFirst,
		},
	}
}

func newShellPod(shell string) *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "cli-test",
		},
		Spec: corev1.PodSpec{
			RestartPolicy: corev1.RestartPolicyNever,
			Containers: []corev1.Container{
				{
					Name:    "test",
					Image:   image.ShellImage(),
					Command: []string{"/bin/bash", "-c", shell},
					Env: []corev1.EnvVar{
						{
							Name:  "HOME",
							Value: "/tmp",
						},
					},
				},
			},
		},
	}
}

func newFrontendService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "frontend",
			Labels: map[string]string{
				"name": "frontend",
			},
		},
		Spec: corev1.ServiceSpec{
			Ports: []corev1.ServicePort{
				{
					Protocol:   corev1.ProtocolTCP,
					Port:       9998,
					TargetPort: intstr.FromInt(9998),
				},
			},
			Selector: map[string]string{
				"name": "frontend",
			},
			Type:            "ClusterIP",
			SessionAffinity: corev1.ServiceAffinityNone,
		},
	}
}

func newBusyBoxStatefulSet() *appsv1.StatefulSet {
	var replicas = int32(1)

	return &appsv1.StatefulSet{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "StatefulSet",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "testapp",
		},
		Spec: appsv1.StatefulSetSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "testapp",
				},
			},
			ServiceName: "frontend",
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "testapp",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "testapp",
							Image: k8simage.GetE2EImage(k8simage.Pause),
						},
					},
				},
			},
		},
	}
}

func writeObjectToFile(obj runtime.Object) (string, error) {
	dir, err := os.MkdirTemp("", "oc-test")
	if err != nil {
		return "", err
	}

	name := filepath.Join(dir, "testfile")
	f, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if err := json.NewEncoder(f).Encode(obj); err != nil {
		return "", err
	}

	return name, nil
}

// replaceImageInFile reads in a file, replaces the given image name, then writes to a new temporary file.
func replaceImageInFile(source, oldImage, newImage string) (string, error) {
	f, err := os.OpenFile(source, os.O_RDONLY, 0644)
	if err != nil {
		return "", err
	}
	defer f.Close()

	dir, err := os.MkdirTemp("", "oc-test")
	if err != nil {
		return "", err
	}

	name := filepath.Join(dir, "testfile")
	writer, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", err
	}
	defer writer.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		yamlImage := "image: " + oldImage
		jsonImage := `"image": "` + oldImage + `"`
		if strings.Contains(line, yamlImage) {
			line = strings.ReplaceAll(line, yamlImage, "image: "+newImage)
		} else if strings.Contains(line, jsonImage) {
			line = strings.ReplaceAll(line, jsonImage, `"image": "`+newImage+`"`)
		}

		if _, err := io.WriteString(writer, line+"\n"); err != nil {
			return "", err
		}
	}

	if scanner.Err() != nil {
		return "", err
	}

	return name, nil
}
