package support

import (
	"fmt"
	"io"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	imagev1 "github.com/openshift/api/image/v1"
	imageclientv1 "github.com/openshift/client-go/image/clientset/versioned"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

type lifecycleTagHook struct {
	imageClient imageclientv1.Interface
	hookType    string
	out         io.Writer
}

func (h *lifecycleTagHook) run(hook *appsapi.LifecycleHook, rc *corev1.ReplicationController) error {
	var errs []error
	for _, action := range hook.TagImages {
		value, ok := findContainerImage(rc, action.ContainerName)
		if !ok {
			errs = append(errs, fmt.Errorf("unable to find image for container %q, container could not be found", action.ContainerName))
			continue
		}
		namespace := action.To.Namespace
		if len(namespace) == 0 {
			namespace = rc.Namespace
		}
		if _, err := h.imageClient.ImageV1().ImageStreamTags(namespace).Update(&imagev1.ImageStreamTag{
			ObjectMeta: metav1.ObjectMeta{
				Name:      action.To.Name,
				Namespace: namespace,
			},
			Tag: &imagev1.TagReference{
				From: &corev1.ObjectReference{
					Kind: "DockerImage",
					Name: value,
				},
			},
		}); err != nil {
			errs = append(errs, err)
			continue
		}
		fmt.Fprintf(h.out, "--> %s: Tagged %q into %s/%s\n", h.hookType, value, action.To.Namespace, action.To.Name)
	}

	return utilerrors.NewAggregate(errs)
}

// findContainerImage returns the image with the given container name from a replication controller.
func findContainerImage(rc *corev1.ReplicationController, containerName string) (string, bool) {
	if rc.Spec.Template == nil {
		return "", false
	}
	for _, container := range rc.Spec.Template.Spec.Containers {
		if container.Name == containerName {
			return container.Image, true
		}
	}
	return "", false
}
