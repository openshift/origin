package resolve

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	triggerapi "github.com/openshift/origin/pkg/image/apis/image/v1/trigger"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
)

// LatestTriggerImagePullSpec resolves the latest image pull spec for given
// object reference and returns the docker image reference.
func LatestTriggerImagePullSpec(c imageclient.ImageStreamTagsGetter, namespace string, ref triggerapi.ObjectReference) (*imageapi.DockerImageReference, error) {
	if len(ref.Namespace) > 0 {
		namespace = ref.Namespace
	}
	tag, err := c.ImageStreamTags(namespace).Get(ref.Name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("unable to get latest pull spec for %s/%s: %v", namespace, ref.Name, err)
	}
	if len(tag.Image.DockerImageReference) == 0 {
		return nil, fmt.Errorf("image pull spec for %s/%s is empty", namespace, ref.Name)
	}
	dockerImageReference, err := imageapi.ParseDockerImageReference(tag.Image.DockerImageReference)
	if err != nil {
		return nil, err
	}
	return &dockerImageReference, nil
}
