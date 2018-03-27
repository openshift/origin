package util

import (
	"fmt"
	"time"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CreateNFSServerReplicationController creates an nfs server replication controller
func CreateNFSServerReplicationController(name, capacity string) *kapiv1.ReplicationController {
	replicas := int32(1)
	privileged := true

	return &kapiv1.ReplicationController{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ReplicationController",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.ReplicationControllerSpec{
			Replicas: &replicas,
			Selector: map[string]string{
				"role": "nfs-server",
			},
			Template: &kapiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"role": "nfs-server"},
				},
				Spec: kapiv1.PodSpec{
					Containers: []kapiv1.Container{
						{
							Name: "nfs-server",
							// The code for this image is located at https://github.com/coreydaley/nfs-server
							// The image exports 10 mounts, /exports/data-0 through /exports/data-9
							Image: "docker.io/coreydaley/nfs-server",
							Ports: []kapiv1.ContainerPort{
								{ContainerPort: 2049, Name: "nfs"},
								{ContainerPort: 2049, Name: "nfs-udp", Protocol: "UDP"},
								{ContainerPort: 20048, Name: "mountd"},
								{ContainerPort: 111, Name: "rpcbind"},
								{ContainerPort: 111, Name: "rpcbind-udp", Protocol: "UDP"},
							},
							Resources: kapiv1.ResourceRequirements{
								Requests: kapiv1.ResourceList{
									kapiv1.ResourceMemory: resource.MustParse(capacity),
								},
							},
							SecurityContext: &kapiv1.SecurityContext{
								Privileged: &privileged,
							},
						},
					},
				},
			},
		},
	}
}

// CreateNFSServerService creates a service for the nfs replication controller
func CreateNFSServerService(name string) *kapiv1.Service {
	return &kapiv1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.ServiceSpec{
			Ports: []kapiv1.ServicePort{
				{Port: 2049, Name: "nfs"},
				{Port: 2049, Name: "nfs-udp", Protocol: "UDP"},
				{Port: 20048, Name: "mountd"},
				{Port: 111, Name: "rpcbind"},
				{Port: 111, Name: "rpcbind-udp", Protocol: "UDP"},
			},
			Selector: map[string]string{
				"role": "nfs-server",
			},
		},
	}
}

// SetupNFSServer sets up an nfs server replication controller with the given capacity
// and the nfs service
func SetupNFSServer(oc *CLI, capacity string) (*kapiv1.ReplicationController, *kapiv1.Service, error) {
	e2e.Logf("Setting up the nfs server")
	prefix := oc.Namespace()
	errs := []error{}

	e2e.Logf("Adding privileged scc from system:serviceaccount:%s:default", oc.Namespace())
	if _, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", oc.Namespace())).Output(); err != nil {
		return nil, nil, err
	}

	e2e.Logf("Setting up the nfs server replication controller")
	rc, err := oc.AdminKubeClient().Core().ReplicationControllers(oc.Namespace()).Create(CreateNFSServerReplicationController(fmt.Sprintf("%s%s", nfsPrefix, prefix), capacity))
	if err != nil {
		e2e.Logf("WARNING: unable to create replication controller %s%s: %v\n", nfsPrefix, prefix, err)
		errs = append(errs, err)
	}
	err = wait.PollImmediate(5*time.Second, 5*time.Minute, func() (bool, error) {
		e2e.Logf("Checking replication controller status")
		readyReplicas, err := oc.AsAdmin().Run("get").Args("replicationcontrollers", rc.Name, "--template", "{{.status.readyReplicas}}").Output()
		if err != nil {
			return false, nil
		}
		availableReplicas, err := oc.AsAdmin().Run("get").Args("replicationcontrollers", rc.Name, "--template", "{{.status.availableReplicas}}").Output()
		if err != nil {
			return false, nil
		}
		e2e.Logf("readyReplicas: %s, availableReplicas: %s", readyReplicas, availableReplicas)
		if readyReplicas != "1" || availableReplicas != "1" {
			return false, nil
		}
		return true, nil
	})
	describe, err := oc.AsAdmin().Run("describe").Args(fmt.Sprintf("rc/%s", rc.Name)).Output()
	e2e.Logf("Describing rc/%s\n%v", rc.Name, describe)
	if err != nil {
		return nil, nil, err
	}

	e2e.Logf("Setting up the nfs server service")
	svc, err := oc.AdminKubeClient().Core().Services(oc.Namespace()).Create(CreateNFSServerService(fmt.Sprintf("%s%s", nfsPrefix, prefix)))
	if err != nil {
		e2e.Logf("WARNING: unable to create service %s%s: %v\n", nfsPrefix, prefix, err)
		errs = append(errs, err)
	}
	describe, err = oc.AsAdmin().Run("describe").Args(fmt.Sprintf("svc/%s", svc.Name)).Output()
	e2e.Logf("Describing svc/%s\n%v", svc.Name, describe)

	e2e.Logf("Waiting for svc/%s endpoints to become available", svc.Name)
	if err = WaitForEndpointsAvailable(oc, svc.Name); err != nil {
		return nil, nil, err
	}

	return rc, svc, kutilerrors.NewAggregate(errs)
}

// RemoveNFSServer removes the nfs server replication controller and nfs service
func RemoveNFSServer(oc *CLI) error {
	e2e.Logf("Removing the nfs server")
	prefix := oc.Namespace()
	errs := []error{}

	e2e.Logf("Removing the nfs server service")
	if err := oc.AdminKubeClient().Core().Services(oc.Namespace()).Delete(fmt.Sprintf("%s%s", nfsPrefix, prefix), nil); err != nil {
		e2e.Logf("WARNING: unable to remove service %s%s: %v\n", nfsPrefix, prefix, err)
		errs = append(errs, err)
	}

	e2e.Logf("Removing the nfs server replication controller")
	if err := oc.AdminKubeClient().Core().ReplicationControllers(oc.Namespace()).Delete(fmt.Sprintf("%s%s", nfsPrefix, prefix), nil); err != nil {
		e2e.Logf("WARNING: unable to remove replication controller %s%s: %v\n", nfsPrefix, prefix, err)
		errs = append(errs, err)
	}

	e2e.Logf("Removing privileged scc from system:serviceaccount:%s:default", oc.Namespace())
	if _, err := oc.AsAdmin().Run("adm").Args("policy", "remove-scc-from-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", oc.Namespace())).Output(); err != nil {
		errs = append(errs, err)
	}

	return kutilerrors.NewAggregate(errs)
}
