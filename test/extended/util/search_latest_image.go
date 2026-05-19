package util

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const openshiftPayloadImageNamespace = "openshift"

// SearchLatestImage returns the resolved docker pull spec for imageName:latest in the openshift
// namespace (payload imagestreams such as cli, tools, must-gather). The cluster serves the
// architecture-appropriate image; callers must not hardcode digests.
func SearchLatestImage(oc *CLI, imageName string) (string, error) {
	if imageName == "" {
		return "", fmt.Errorf("imageName is empty")
	}
	ctx := context.Background()
	istag, err := oc.AdminImageClient().ImageV1().ImageStreamTags(openshiftPayloadImageNamespace).Get(ctx, imageName+":latest", metav1.GetOptions{})
	if err != nil {
		return "", err
	}
	ref := istag.Image.DockerImageReference
	if ref == "" {
		return "", fmt.Errorf("empty DockerImageReference for %s/%s:latest", openshiftPayloadImageNamespace, imageName)
	}
	return ref, nil
}
