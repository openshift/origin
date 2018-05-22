package util

import (
	"fmt"
	"time"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// SetupK8SNFSServerAndVolume sets up an nfs server pod with count number of persistent volumes
func SetupK8SNFSServerAndVolume(oc *CLI, count int) (*kapiv1.Pod, []*kapiv1.PersistentVolume, error) {
	e2e.Logf("Adding privileged scc from system:serviceaccount:%s:default", oc.Namespace())
	_, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", oc.Namespace())).Output()
	if err != nil {
		return nil, nil, err
	}

	e2e.Logf(fmt.Sprintf("Creating NFS server"))
	config := e2e.VolumeTestConfig{
		Namespace: oc.Namespace(),
		Prefix:    "nfs",
		// this image is an extension of k8s.gcr.io/volume-nfs:0.8 that adds
		// additional nfs mounts to allow for openshift extended tests with
		// replicas and shared state (mongo, postgresql, mysql, etc.); defined
		// in repo https://github.com/gmontero/nfs-server
		ServerImage:   "docker.io/gmontero/nfs-server:latest",
		ServerPorts:   []int{2049},
		ServerVolumes: map[string]string{"": "/exports"},
	}
	pod, ip := e2e.CreateStorageServer(oc.AsAdmin().KubeFramework().ClientSet, config)
	e2e.Logf("Waiting for pod running")
	err = wait.PollImmediate(5*time.Second, 1*time.Minute, func() (bool, error) {
		phase, err := oc.AsAdmin().Run("get").Args("pods", pod.Name, "--template", "{{.status.phase}}").Output()
		if err != nil {
			return false, nil
		}
		if phase != "Running" {
			return false, nil
		}
		return true, nil
	})

	pvs := []*kapiv1.PersistentVolume{}
	volLabel := labels.Set{e2e.VolumeSelectorKey: oc.Namespace()}
	for i := 0; i < count; i++ {
		e2e.Logf(fmt.Sprintf("Creating persistent volume %d", i))
		pvConfig := e2e.PersistentVolumeConfig{
			NamePrefix: "nfs-",
			Labels:     volLabel,
			PVSource: kapiv1.PersistentVolumeSource{
				NFS: &kapiv1.NFSVolumeSource{
					Server:   ip,
					Path:     fmt.Sprintf("/exports/data-%d", i),
					ReadOnly: false,
				},
			},
		}
		pvTemplate := e2e.MakePersistentVolume(pvConfig)
		pv, err := oc.AdminKubeClient().Core().PersistentVolumes().Create(pvTemplate)
		if err != nil {
			e2e.Logf("error creating persistent volume %#v", err)
		}
		e2e.Logf("Created persistent volume %#v", pv)
		pvs = append(pvs, pv)
	}
	return pod, pvs, err
}
