package util

import (
	"context"
	"encoding/json"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	e2epod "k8s.io/kubernetes/test/e2e/framework/pod"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
)

// GetClusterVersion returns the ClusterVersion object.
func GetClusterVersion(ctx context.Context, config *restclient.Config) (*configv1.ClusterVersion, error) {
	c, err := configv1client.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	cv, err := c.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return cv, nil
}

// GetCurrentVersion determines and returns the cluster's current version by iterating through the
// provided update history until it finds the first version with update State of Completed. If a
// Completed version is not found the version of the oldest history entry, which is the originally
// installed version, is returned. If history is empty the empty string is returned.
func GetCurrentVersion(ctx context.Context, config *restclient.Config) (string, error) {
	cv, err := GetClusterVersion(ctx, config)
	if err != nil {
		return "", err
	}
	for _, h := range cv.Status.History {
		if h.State == configv1.CompletedUpdate {
			return h.Version, nil
		}
	}
	// Empty history should only occur if method is called early in startup before history is populated.
	if len(cv.Status.History) != 0 {
		return cv.Status.History[len(cv.Status.History)-1].Version, nil
	}
	return "", nil
}

// GetReleaseImage returns ReleaseImage.
func GetReleaseImage(ctx context.Context, config *restclient.Config) (string, error) {
	cv, err := GetClusterVersion(ctx, config)
	if err != nil {
		return "", err
	}
	return cv.Status.Desired.Image, nil
}

// DetermineImageFromRelease will get the image and tag for imageTagName from the release image.
// For example, you can specify oauth-server for the oauth-server image or network-tools to get the image
// and tag for that image.
func DetermineImageFromRelease(ctx context.Context, oc *CLI, imageTagName string) (string, error) {
	releaseImage, err := GetReleaseImage(ctx, oc.AdminConfig())
	if err != nil {
		return "", err
	}
	if len(releaseImage) == 0 {
		return "", fmt.Errorf("cannot determine release image from ClusterVersion resource")
	}
	podClient := e2epod.PodClientNS(oc.KubeFramework(), oc.Namespace())
	podClient.CreateSync(ctx, &corev1.Pod{
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
	defer podClient.Delete(ctx, "extract-release-imagerefs", metav1.DeleteOptions{})
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
