package compat_otp

import (
	"context"
	"fmt"
	"time"

	exutil "github.com/openshift/origin/test/extended/util"

	kapiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	e2e "k8s.io/kubernetes/test/e2e/framework"
	e2epv "k8s.io/kubernetes/test/e2e/framework/pv"
	"k8s.io/kubernetes/test/e2e/framework/volume"
)

// SetupK8SNFSServerAndVolume sets up an nfs server pod with count number of persistent volumes
func SetupK8SNFSServerAndVolume(oc *exutil.CLI, count int) (*kapiv1.Pod, []*kapiv1.PersistentVolume, error) {
	e2e.Logf("Adding privileged scc from system:serviceaccount:%s:default", oc.Namespace())
	_, err := oc.AsAdmin().Run("adm").Args("policy", "add-scc-to-user", "privileged", fmt.Sprintf("system:serviceaccount:%s:default", oc.Namespace())).Output()
	if err != nil {
		return nil, nil, err
	}

	e2e.Logf("Creating NFS server")
	config := volume.TestConfig{
		Namespace: oc.Namespace(),
		Prefix:    "nfs",
		// this image is an extension of k8s.gcr.io/volume-nfs:0.8 that adds
		// additional nfs mounts to allow for openshift extended tests with
		// replicas and shared state (formerly mongo, postgresql, mysql, etc., now only jenkins); defined
		// in repo https://github.com/redhat-developer/nfs-server
		ServerImage:   "quay.io/redhat-developer/nfs-server:1.1",
		ServerPorts:   []int{2049},
		ServerVolumes: map[string]string{"": "/exports"},
	}
	pod, ip := volume.CreateStorageServer(context.TODO(), oc.AsAdmin().KubeFramework().ClientSet, config)
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
	volLabel := labels.Set{e2epv.VolumeSelectorKey: oc.Namespace()}
	for i := 0; i < count; i++ {
		e2e.Logf(fmt.Sprintf("Creating persistent volume %d", i))
		pvConfig := e2epv.PersistentVolumeConfig{
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
		pvTemplate := e2epv.MakePersistentVolume(pvConfig)
		pv, err := oc.AdminKubeClient().CoreV1().PersistentVolumes().Create(context.Background(), pvTemplate, metav1.CreateOptions{})
		if err != nil {
			e2e.Logf("error creating persistent volume %#v", err)
		}
		e2e.Logf("Created persistent volume %#v", pv)
		pvs = append(pvs, pv)
	}
	return pod, pvs, err
}
