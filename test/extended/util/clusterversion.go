package util

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	"k8s.io/kubernetes/test/e2e/framework"
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
	podName := "extract-release-imagerefs"
	podClient := e2epod.PodClientNS(oc.KubeFramework(), oc.Namespace())
	podClient.CreateSync(ctx, &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: podName},
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
	defer func() {
		podClient.Delete(ctx, podName, metav1.DeleteOptions{})
		err = e2epod.WaitForPodNotFoundInNamespace(ctx, oc.kubeFramework.ClientSet,
			podName, oc.Namespace(), e2epod.PodDeleteTimeout)
		if err != nil {
			framework.Logf("pod %q is still found in namespace %q", podName, oc.Namespace())
		}
	}()
	imageRefsString := e2epod.ExecShellInContainer(oc.KubeFramework(), podName, "imagerefs", "cat /release-manifests/image-references")
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

// prepareImagePullSecretAndCABundle prepares the image pull secret and optional user CA bundle for use,
// returning the necessary command-line arguments, or an error if setup fails.
func PrepareImagePullSecretAndCABundle(oc *CLI) (func(), []string, error) {
	kubeClient := oc.AdminKubeClient()
	// Try to use the same pull secret as the cluster under test
	imagePullSecret, err := kubeClient.CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get pull secret from cluster: %v", err)
	}

	// cache file to local temp location
	imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create a temporary file: %v", err)
	}
	cleanup := func() {
		os.Remove(imagePullFile.Name())
	}

	// write the content
	imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
	if _, err = imagePullFile.Write(imagePullSecretBytes); err != nil {
		return cleanup, nil, fmt.Errorf("unable to write pull secret to temp file: %v", err)
	}
	if err = imagePullFile.Close(); err != nil {
		return cleanup, nil, fmt.Errorf("unable to close file: %v", err)
	}

	cmdArgs := []string{"--registry-config", imagePullFile.Name()}

	// Trust also user trusted CA from the cluster under test
	userCaBundle, err := kubeClient.CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "user-ca-bundle", metav1.GetOptions{})
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return cleanup, nil, fmt.Errorf("unable to get user-ca-bundle configmap from cluster: %v", err)
		}
	}

	if userCaBundle != nil {
		// cache file to local temp location
		userCaBundleFile, err := ioutil.TempFile("", "user-ca-bundle")
		if err != nil {
			return cleanup, nil, fmt.Errorf("unable to create a temporary file: %v", err)
		}

		cleanup = func() {
			os.Remove(imagePullFile.Name())
			os.Remove(userCaBundleFile.Name())
		}

		// write the content
		userCaBundleString := userCaBundle.Data["ca-bundle.crt"]
		if _, err = userCaBundleFile.WriteString(userCaBundleString); err != nil {
			return cleanup, nil, fmt.Errorf("unable to write user CA bundle to temp file: %v", err)
		}
		if err = userCaBundleFile.Close(); err != nil {
			return cleanup, nil, fmt.Errorf("unable to close file: %v", err)
		}
		cmdArgs = append(cmdArgs, "--certificate-authority", userCaBundleFile.Name())
	}
	return cleanup, cmdArgs, nil
}
