package util

import (
	"fmt"
	"net"
	"strconv"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubeutil "github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

type ValidateFunc func(string) error

// VerifyImage verifies if the latest image in given ImageRepository is valid
func VerifyImage(repo *imageapi.ImageRepository, ns string, validator ValidateFunc) error {
	pod := CreatePodFromImage(repo, ns)
	if pod == nil {
		return fmt.Errorf("Unable to create Pod for %+v", repo.Status.DockerImageRepository)
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
	// TODO: There is no Pods.Watch() (yet) in OpenShift. The Watch() is already
	// implemented in upstream, so uncomment this once that happens.
	/*
		podWatch, err := client.Pods(ns).Watch(labels.Everything(), labels.Everything(), "0")
		if err != nil {
			return "", fmt.Errorf("Unable to create watcher for Pod: %v", err)
		}
		defer podWatch.Stop()

		running := false
		for event := range podWatch.ResultChan() {
			currentPod, ok := event.Object.(*kapi.Pod)
			if !ok {
				return "", fmt.Errorf("Unable to convert event object to Pod")
			}
			if pod.Name == currentPod.Name {
				switch pod.Status.Phase {
				case kapi.PodFailed:
					return "", fmt.Errorf("Pod failed to run")
				case kapi.PodRunning:
					running = true
				}
			}
			if running == true {
				break
			}
		}
		fmt.Printf("The Pod %s is now running.", pod.Name)
	*/

	// Now wait for the service to get the endpoint
	// TODO: Endpoints() have no Watch in upstream, once they do, replace this
	// code to use Watch()
	for retries := 240; retries != 0; retries-- {
		endpoints, err := client.Endpoints(ns).Get(service.Name)
		if err != nil {
			fmt.Printf("%v\n", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if len(endpoints.Endpoints) == 0 {
			fmt.Printf("Waiting for Service %s endpoints...\n", service.Name)
			time.Sleep(1 * time.Second)
			continue
		}
		for i := range endpoints.Endpoints {
			ep := &endpoints.Endpoints[i]
			addr := net.JoinHostPort(ep.IP, strconv.Itoa(ep.Port))
			fmt.Printf("The Service %s has endpoint: %s", pod.Name, addr)
			return addr, nil
		}
	}

	return "", fmt.Errorf("Service does not get any endpoints")
}

// CreatePodFromImage creates a Pod from the latest image available in the Image
// Repository
func CreatePodFromImage(repo *imageapi.ImageRepository, ns string) *kapi.Pod {
	client, err := GetClusterAdminKubeClient(KubeConfigPath())
	if err != nil {
		return nil
	}
	pod := &kapi.Pod{
		ObjectMeta: kapi.ObjectMeta{
			Name:   ns + "pod",
			Labels: map[string]string{"name": ns + "pod"},
		},
		Spec: kapi.PodSpec{
			Containers: []kapi.Container{
				{
					Name:  "sample",
					Image: repo.Status.DockerImageRepository,
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
			Selector:   map[string]string{"name": pod.Name},
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
