package disruptionpodnetwork

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"

	configclient "github.com/openshift/client-go/config/clientset/versioned"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// GetOpenshiftTestsImagePullSpec returns the pull spec or an error.
func GetOpenshiftTestsImagePullSpec(ctx context.Context, adminRESTConfig *rest.Config, suggestedPayloadImage string) (string, error) {
	if len(suggestedPayloadImage) == 0 {
		configClient, err := configclient.NewForConfig(adminRESTConfig)
		if err != nil {
			return "", err
		}
		clusterVersion, err := configClient.ConfigV1().ClusterVersions().Get(ctx, "version", metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return "", fmt.Errorf("clusterversion/version not found and no image pull spec specified")
		}
		if err != nil {
			return "", err
		}
		suggestedPayloadImage = clusterVersion.Status.History[0].Image
	}

	// runImageExtract extracts src from specified image to dst
	cmd := exec.Command("oc", "adm", "release", "info", suggestedPayloadImage, "--image-for=tests")
	out := &bytes.Buffer{}
	errOut := &bytes.Buffer{}
	cmd.Stdout = out
	cmd.Stderr = errOut
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("unable to determine openshift-tests image: %v: %v", err, errOut.String())
	}
	openshiftTestsImagePullSpec := strings.TrimSpace(out.String())
	fmt.Printf("openshift-tests image pull spec is %v\n", openshiftTestsImagePullSpec)

	return openshiftTestsImagePullSpec, nil
}
