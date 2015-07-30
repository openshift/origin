package util

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/util/namer"
)

// OsFramework provides helper functions and watchers for OpenShift
type OsFramework struct {
	Namespace *kapi.Namespace
	Client    *client.Client
}

// NewOsFramework initialize OpenShift testing framework
func NewOsFramework(ns *kapi.Namespace, c *client.Client) *OsFramework {
	return &OsFramework{Namespace: ns, Client: c}
}

// WaitForABuild waits for a build to become Completed
func (f *OsFramework) WaitForABuild(buildName string) error {
	for {
		list, err := f.Client.Builds(f.Namespace.Name).List(labels.Everything(), fields.Everything())
		if err != nil {
			return err
		}
		rv := list.ResourceVersion

		isOK := func(e *buildapi.Build) bool {
			return e.Name == buildName && e.Status.Phase == buildapi.BuildPhaseComplete
		}

		isFailed := func(e *buildapi.Build) bool {
			return e.Status.Phase == buildapi.BuildPhaseFailed || e.Status.Phase == buildapi.BuildPhaseError
		}

		for i := range list.Items {
			if isOK(&list.Items[i]) {
				return nil
			}
			if isFailed(&list.Items[i]) {
				return fmt.Errorf("The build %q status is %q", buildName, &list.Items[i].Status.Phase)
			}
		}

		w, err := f.Client.Builds(f.Namespace.Name).Watch(
			labels.Everything(),
			fields.Set{"name": buildName}.AsSelector(),
			rv,
		)
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
					return fmt.Errorf("The build %q status is %q", buildName, e.Status.Phase)
				}
			}
		}
	}
}

// CreatePodForImageStream creates a pod object from given imageStream
func (f *OsFramework) CreatePodForImageStream(imageStreamName string) (*kapi.Pod, error) {
	imageStream, err := f.Client.ImageStreams(f.Namespace.Name).Get(imageStreamName)
	if err != nil {
		return nil, err
	}

	tags := []string{}
	for tag := range imageStream.Status.Tags {
		tags = append(tags, tag)
	}

	imageName := imageStream.Status.Tags[tags[0]].Items[0].DockerImageReference
	podName := namer.GetPodName("test-pod", string(kutil.NewUUID()))
	return &kapi.Pod{
		TypeMeta: kapi.TypeMeta{
			Kind:       "Pod",
			APIVersion: "v1",
		},
		ObjectMeta: kapi.ObjectMeta{
			Name:   podName,
			Labels: map[string]string{"name": podName},
		},
		Spec: kapi.PodSpec{
			ServiceAccountName: "builder",
			Containers: []kapi.Container{
				{
					Name:  "test",
					Image: imageName,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}, nil
}
