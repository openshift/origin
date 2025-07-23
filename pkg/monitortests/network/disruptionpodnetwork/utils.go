package disruptionpodnetwork

import (
	"context"
	"errors"
	"fmt"
	"github.com/openshift/origin/pkg/test/extensions"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	exutil "github.com/openshift/origin/test/extended/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// GetOpenshiftTestsImagePullSpec returns the pull spec or an error.
// IN ginkgo environment, oc needs to be created before BeforeEach and passed in
func GetOpenshiftTestsImagePullSpec(ctx context.Context, adminRESTConfig *rest.Config, suggestedPayloadImage string, oc *exutil.CLI) (string, error) {
	if len(suggestedPayloadImage) == 0 {
		var err error
		suggestedPayloadImage, err = extensions.DetermineReleasePayloadImage()
		if err != nil {
			return "", err
		}
	}

	logrus.Infof("payload image reported by CV: %v\n", suggestedPayloadImage)
	// runImageExtract extracts src from specified image to dst

	// Extract the openshift-tests image from the release payload
	openshiftTestsImagePullSpec, err := extensions.ExtractImageFromReleasePayload(suggestedPayloadImage, "tests")
	if err != nil {
		logrus.WithError(err).Errorf("unable to determine openshift-tests image through ExtractImageFromReleasePayload: %v", err)
		// Now try the wrapper to see if it makes a difference
		if oc == nil {
			oc = exutil.NewCLIWithoutNamespace("openshift-tests")
		}
		outStr, err := oc.Run("adm", "release", "info", suggestedPayloadImage).Args("--image-for=tests").Output()
		if err != nil {
			logrus.WithError(err).Errorf("unable to determine openshift-tests image through oc wrapper with default ps: %v", outStr)

			kubeClient := oc.AdminKubeClient()
			// Try to use the same pull secret as the cluster under test
			imagePullSecret, err := kubeClient.CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
			if err != nil {
				logrus.WithError(err).Errorf("unable to get pull secret from cluster: %v", err)
				return "", fmt.Errorf("unable to get pull secret from cluster: %v", err)
			}

			// cache file to local temp location
			imagePullFile, err := ioutil.TempFile("", "image-pull-secret")
			if err != nil {
				logrus.WithError(err).Errorf("unable to create a temporary file: %v", err)
				return "", fmt.Errorf("unable to create a temporary file: %v", err)
			}
			defer os.Remove(imagePullFile.Name())

			// write the content
			imagePullSecretBytes := imagePullSecret.Data[".dockerconfigjson"]
			if _, err = imagePullFile.Write(imagePullSecretBytes); err != nil {
				logrus.WithError(err).Errorf("unable to write pull secret to temp file: %v", err)
				return "", fmt.Errorf("unable to write pull secret to temp file: %v", err)
			}
			if err = imagePullFile.Close(); err != nil {
				logrus.WithError(err).Errorf("unable to close file: %v", err)
				return "", fmt.Errorf("unable to close file: %v", err)
			}

			cmdArgs := []string{"--image-for=tests", "--registry-config", imagePullFile.Name()}

			// Trust also user trusted CA from the cluster under test
			userCaBundle, err := kubeClient.CoreV1().ConfigMaps("openshift-config").Get(context.Background(), "user-ca-bundle", metav1.GetOptions{})
			if err != nil {
				logrus.WithError(err).Errorf("unable to get user-ca-bundle configmap from cluster: %v", err)
				if !apierrors.IsNotFound(err) {
					return "", fmt.Errorf("unable to get user-ca-bundle configmap from cluster: %v", err)
				}
			}

			if userCaBundle != nil {
				// cache file to local temp location
				userCaBundleFile, err := ioutil.TempFile("", "user-ca-bundle")
				if err != nil {
					logrus.WithError(err).Errorf("unable to create a temporary file: %v", err)
					return "", fmt.Errorf("unable to create a temporary file: %v", err)
				}
				defer os.Remove(userCaBundleFile.Name())

				// write the content
				userCaBundleString := userCaBundle.Data["ca-bundle.crt"]
				if _, err = userCaBundleFile.WriteString(userCaBundleString); err != nil {
					logrus.WithError(err).Errorf("unable to write user CA bundle to temp file: %v", err)
					return "", fmt.Errorf("unable to write user CA bundle to temp file: %v", err)
				}
				if err = userCaBundleFile.Close(); err != nil {
					logrus.WithError(err).Errorf("unable to close file: %v", err)
					return "", fmt.Errorf("unable to close file: %v", err)
				}

				cmdArgs = append(cmdArgs, "--certificate-authority", userCaBundleFile.Name())
			}

			outStr, err = oc.Run("adm", "release", "info", suggestedPayloadImage).Args(cmdArgs...).Output()
			if err != nil {
				logrus.WithError(err).Errorf("unable to determine openshift-tests image through oc wrapper with cluster ps")

				// What is the mirror mode

				return "", fmt.Errorf("unable to determine openshift-tests image oc wrapper with cluster ps: %v", err)
			} else {
				logrus.Infof("successfully getting image for test with oc wrapper with cluster ps: %s\n", outStr)
			}
		} else {
			logrus.Infof("successfully getting image for test with oc wrapper with default ps: %s\n", outStr)
		}

		openshiftTestsImagePullSpec = strings.TrimSpace(outStr)
	}
	fmt.Printf("openshift-tests image pull spec is %v\n", openshiftTestsImagePullSpec)

	return openshiftTestsImagePullSpec, nil
}

func GetOpenshiftTestsImagePullSpecWithRetries(ctx context.Context, adminRESTConfig *rest.Config, suggestedPayloadImage string, oc *exutil.CLI, retries int) (string, error) {
	var errs []error

	for i := 0; i < retries; i++ {
		result, err := GetOpenshiftTestsImagePullSpec(ctx, adminRESTConfig, suggestedPayloadImage, oc)
		if err == nil {
			return result, nil
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(30 * time.Second):
		}
		errs = append(errs, err)
	}
	return "", errors.Join(errs...)
}
