package util

import (
	"fmt"
	"io/ioutil"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/api/latest"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/util/namer"
)

// WriteObjectToFile writes the JSON representation of runtime.Object into a temporary
// file.
func WriteObjectToFile(obj runtime.Object, filename string) error {
	content, err := latest.Codec.Encode(obj)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(filename, []byte(content), 0644)
}

// WaitForABuild waits for a Build object to match either isOK or isFailed conditions
func WaitForABuild(c client.BuildInterface, name string, isOK, isFailed func(*buildapi.Build) bool) error {
	for {
		list, err := c.List(labels.Everything(), fields.Set{"name": name}.AsSelector())
		if err != nil {
			return err
		}
		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The build %q status is %q", name, &list.Items[i].Status.Phase)
			}
		}

		rv := list.ResourceVersion
		w, err := c.Watch(labels.Everything(), fields.Set{"name": name}.AsSelector(), rv)
		if err != nil {
			return err
		}
		defer w.Stop()

		for {
			val, ok := <-w.ResultChan()
			if !ok {
				// reget and re-watch
				break
			}
			if e, ok := val.Object.(*buildapi.Build); ok {
				if isOK(e) {
					return nil
				}
				if isFailed(e) {
					return fmt.Errorf("The build %q status is %q", name, e.Status.Phase)
				}
			}
		}
	}
}

// GetDockerImageReference retrieves the full Docker pull spec from the given ImageStream
// and tag
func GetDockerImageReference(c client.ImageStreamInterface, name, tag string) (string, error) {
	imageStream, err := c.Get(name)
	if err != nil {
		return "", err
	}
	isTag, ok := imageStream.Status.Tags[tag]
	if !ok {
		return "", fmt.Errorf("ImageStream %q does not have tag %q", name, tag)
	}
	if len(isTag.Items) == 0 {
		return "", fmt.Errorf("ImageStreamTag %q is empty", tag)
	}
	return isTag.Items[0].DockerImageReference, nil
}

// GetPodForContainer creates a new Pod that runs specified container
func GetPodForContainer(container kapi.Container) *kapi.Pod {
	name := namer.GetPodName("test-pod", string(kutil.NewUUID()))
	return &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapi.PodSpec{
			Containers:    []kapi.Container{container},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
}

// GetPodForImage creates a new Pod that runs the containers from specified
// Docker image reference
func GetPodForImage(dockerImageReference string) *kapi.Pod {
	return GetPodForContainer(kapi.Container{
		Name:  "test",
		Image: dockerImageReference,
	})
}
