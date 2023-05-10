package util

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"
)

// DetermineImageFromRelease will get the image and tag for imageTagName from the release image.
// For example, you can specify oauth-server for the oauth-server image or network-tools to get the image
// and tag for that image.
func DetermineImageFromRelease(oc *CLI, imageTagName string) (string, error) {
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	releaseImage := cv.Status.Desired.Image
	if len(releaseImage) == 0 {
		return "", fmt.Errorf("cannot determine release image from ClusterVersion resource")
	}
	podClient := e2epod.PodClientNS(oc.KubeFramework(), oc.Namespace())
	podClient.CreateSync(context.TODO(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "extract-release-imagerefs"},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:    "imagerefs",
					Image:   releaseImage,
					Command: []string{"/bin/sleep", "10000"},
				},
			},
		},
	})
	defer podClient.Delete(context.Background(), "extract-release-imagerefs", metav1.DeleteOptions{})
	imageRefsString := e2epod.ExecShellInContainer(oc.KubeFramework(), "extract-release-imagerefs", "imagerefs", "cat /release-manifests/image-references")
	imageRefs := struct {
		Spec struct {
			Tags []struct {
				Name string `json:"name"`
				From struct {
					Name string `json:"name"`
				} `json:"from"`
			} `json:"tags"`
		} `json:"spec"`
	}{}
	err = json.Unmarshal([]byte(imageRefsString), &imageRefs)
	if err != nil {
		return "", err
	}
	for _, t := range imageRefs.Spec.Tags {
		if t.Name == imageTagName {
			return t.From.Name, nil
		}
	}
	return "", fmt.Errorf("Could not find image: %s", imageTagName)
}
