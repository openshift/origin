package util

import (
	"fmt"
	"net"
	"strconv"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	kubeutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ValidateFunc func(string) error

// VerifyImage verifies if the latest image in given ImageStream is valid
func VerifyImage(stream *imageapi.ImageStream, tag, ns string, validator ValidateFunc) error {
	pod := CreatePodFromImage(stream, tag, ns)
	if pod == nil {
		return fmt.Errorf("Unable to create Pod for %+v", stream.Status.DockerImageRepository)
	}
	service := CreateServiceForPod(pod, ns)
	if service == nil {
		return fmt.Errorf("Unable to create Service for %+v", service)
	}

	defer CleanupServiceAndPod(pod, service, ns)

	address, err := WaitForAddress(pod, service, ns)
	if err != nil {
		return fmt.Errorf("Failed to obtain address: %v", err)
	}

	return validator(address)
}

// WaitForAddress waits for the Pod to be running and then for the Service to
// get the endpoint.
func WaitForAddress(pod *kapi.Pod, service *kapi.Service, ns string) (string, error) {
	client, err := GetClusterAdminKubeClient(KubeConfigPath())
	if err != nil {
		return "", err
	}
	watcher, err := client.Endpoints(ns).Watch(labels.Everything(), fields.Everything(), "0")
	if err != nil {
		return "", fmt.Errorf("Unexpected error: %v", err)
	}
	defer watcher.Stop()
	for event := range watcher.ResultChan() {
		eventEndpoint, ok := event.Object.(*kapi.Endpoints)
		if !ok {
			return "", fmt.Errorf("Unable to convert object %+v to Endpoints", eventEndpoint)
		}
		if eventEndpoint.Name != service.Name {
			continue
		}
		if len(eventEndpoint.Endpoints) == 0 {
			fmt.Printf("Waiting for %s address\n", eventEndpoint.Name)
			continue
		}
		for i := range eventEndpoint.Endpoints {
			e := &eventEndpoint.Endpoints[i]
			addr := net.JoinHostPort(e.IP, strconv.Itoa(e.Port))
			fmt.Printf("Discovered new %s endpoint: %s\n", service.Name, addr)
			return addr, nil
		}
	}
	return "", fmt.Errorf("Service does not get any endpoints")
}

// CreatePodFromImage creates a Pod from the latest image available in the Image
// Stream
func CreatePodFromImage(stream *imageapi.ImageStream, tag, ns string) *kapi.Pod {
	client, err := GetClusterAdminKubeClient(KubeConfigPath())
	if err != nil {
		return nil
	}
	imageName := stream.Status.DockerImageRepository
	if len(tag) > 0 {
		imageName += ":" + tag
	}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:   ns,
			Labels: map[string]string{"name": ns},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "sample",
					Image: imageName,
				},
			},
			RestartPolicy: kapi.RestartPolicyNever,
		},
	}
	if pod, err := client.Pods(ns).Create(pod); err != nil {
		fmt.Printf("%v\n", err)
		return nil
	} else {
		return pod
	}
}

// CreateServiceForPod creates a service to serve the provided Pod
func CreateServiceForPod(pod *kapi.Pod, ns string) *kapi.Service {
	client, err := GetClusterAdminKubeClient(KubeConfigPath())
	if err != nil {
		return nil
	}
	service := &kapi.Service{
		ObjectMeta: kapi.ObjectMeta{
			Name: ns,
		},
		Spec: kapi.ServiceSpec{
			Selector:   map[string]string{"name": ns},
			TargetPort: kubeutil.IntOrString{Kind: kubeutil.IntstrInt, IntVal: 8080},
			Port:       8080,
		},
	}
	if service, err := client.Services(ns).Create(service); err != nil {
		fmt.Printf("%v\n", err)
		return nil
	} else {
		return service
	}
}

// CleanupServiceAndPod removes the Service and the Pod
func CleanupServiceAndPod(pod *kapi.Pod, service *kapi.Service, ns string) {
	client, err := GetClusterAdminKubeClient(KubeConfigPath())
	if err != nil {
		return
	}
	client.Pods(ns).Delete(pod.Name)
	client.Services(ns).Delete(service.Name)
}
