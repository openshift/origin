package util

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	kapiv1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// CreatePersistentVolume creates a HostPath Persistent Volume.
func CreatePersistentVolume(name, capacity, hostPath string) *kapiv1.PersistentVolume {
	return &kapiv1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.PersistentVolumeSpec{
			PersistentVolumeSource: kapiv1.PersistentVolumeSource{
				HostPath: &kapiv1.HostPathVolumeSource{
					Path: hostPath,
				},
			},
			Capacity: kapiv1.ResourceList{
				kapiv1.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []kapiv1.PersistentVolumeAccessMode{
				kapiv1.ReadWriteOnce,
				kapiv1.ReadOnlyMany,
				kapiv1.ReadWriteMany,
			},
		},
	}
}

// SetupHostPathVolumes will create the requested number of persistent volumes with the given capacity
func SetupHostPathVolumes(oc *CLI, capacity string, count int) (volumes []*kapiv1.PersistentVolume, err error) {
	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	rootDir, err := ioutil.TempDir(TestContext.OutputDir, "persistent-volumes")
	if err != nil {
		e2e.Logf("Error creating pv dir %s: %v\n", TestContext.OutputDir, err)
		return volumes, err
	}
	e2e.Logf("Created pv dir %s\n", rootDir)
	for i := 0; i < count; i++ {
		dir, err := ioutil.TempDir(rootDir, fmt.Sprintf("%0.4d", i))
		if err != nil {
			e2e.Logf("Error creating pv subdir %s: %v\n", rootDir, err)
			return volumes, err
		}
		e2e.Logf("Created pv subdir %s\n", dir)
		if _, err = exec.LookPath("chcon"); err == nil {
			e2e.Logf("Found chcon in path\n")
			out, err := exec.Command("chcon", "-t", "svirt_sandbox_file_t", dir).CombinedOutput()
			if err != nil {
				e2e.Logf("Error running chcon on %s, %s, %v\n", dir, string(out), err)
				return volumes, err
			}
			e2e.Logf("Ran chcon on %s\n", dir)
		}
		if err != nil {
			e2e.Logf("Error finding chcon in path: %v\n", err)
			return volumes, err
		}
		if err = os.Chmod(dir, 0777); err != nil {
			e2e.Logf("Error running chmod on %s, %v\n", dir, err)
			return volumes, err
		}
		e2e.Logf("Ran chmod on %s\n", dir)
		pv, err := c.Create(CreatePersistentVolume(fmt.Sprintf("%s%s-%0.4d", pvPrefix, prefix, i), capacity, dir))
		if err != nil {
			e2e.Logf("Error defining PV %v\n", err)
			return volumes, err
		}
		e2e.Logf("Created PVs\n")
		volumes = append(volumes, pv)
	}
	return volumes, err
}

// RemoveHostPathVolumes removes all persistent volumes created by SetupHostPathVolumes
func RemoveHostPathVolumes(oc *CLI) error {
	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	pvs, err := c.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	prefix = fmt.Sprintf("%s%s-", pvPrefix, prefix)
	for _, pv := range pvs.Items {
		if !strings.HasPrefix(pv.Name, prefix) {
			continue
		}

		pvInfo, err := c.Get(pv.Name, metav1.GetOptions{})
		if err != nil {
			e2e.Logf("WARNING: couldn't get meta info for PV %s: %v\n", pv.Name, err)
			continue
		}

		if err = c.Delete(pv.Name, nil); err != nil {
			e2e.Logf("WARNING: couldn't remove PV %s: %v\n", pv.Name, err)
			continue
		}

		volumeDir := pvInfo.Spec.HostPath.Path
		if err = os.RemoveAll(volumeDir); err != nil {
			e2e.Logf("WARNING: couldn't remove directory %q: %v\n", volumeDir, err)
			continue
		}

		parentDir := filepath.Dir(volumeDir)
		if parentDir == "." || parentDir == "/" {
			continue
		}

		if err = os.Remove(parentDir); err != nil {
			e2e.Logf("WARNING: couldn't remove directory %q: %v\n", parentDir, err)
			continue
		}
	}
	return nil
}

// CreateNFSBackedPersistentVolume creates a persistent volume backed by a pod
// running nfs
func CreateNFSBackedPersistentVolume(name, capacity, server string) *kapiv1.PersistentVolume {
	e2e.Logf("Creating persistent volume")
	return &kapiv1.PersistentVolume{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PersistentVolume",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"name": name},
		},
		Spec: kapiv1.PersistentVolumeSpec{
			PersistentVolumeSource: kapiv1.PersistentVolumeSource{
				NFS: &kapiv1.NFSVolumeSource{
					Server:   server,
					Path:     "/exports/data-0",
					ReadOnly: false,
				},
			},
			Capacity: kapiv1.ResourceList{
				kapiv1.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []kapiv1.PersistentVolumeAccessMode{
				kapiv1.ReadWriteMany,
				kapiv1.ReadWriteOnce,
			},
		},
	}
}

// SetupNFSBackedPersistentVolume sets up an NFS backed persistent volume
func SetupNFSBackedPersistentVolume(oc *CLI, capacity string) (*kapiv1.PersistentVolume, error) {
	e2e.Logf("Setting up nfs backed persistent volume")
	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	_, svc, err := SetupNFSServer(oc, capacity)
	if err != nil {
		return nil, err
	}
	server, err := oc.Run("get").Args("service", svc.Name, "--template", "{{.spec.clusterIP}}").Output()
	if err != nil {
		return nil, err
	}

	pv, err := c.Create(CreateNFSBackedPersistentVolume(fmt.Sprintf("%s%s", pvPrefix, prefix), capacity, server))
	if err != nil {
		e2e.Logf("Error creating persistent volume %v\n", err)
		return nil, err
	}
	return pv, nil
}

// RemoveNFSBackedPersistentVolume removes persistent volumes created by SetupNFSBackedPersistentVolume
func RemoveNFSBackedPersistentVolume(oc *CLI) error {
	c := oc.AdminKubeClient().Core().PersistentVolumes()
	prefix := oc.Namespace()

	pvs, err := c.List(metav1.ListOptions{})
	if err != nil {
		return err
	}
	for _, pv := range pvs.Items {
		if !strings.HasPrefix(pv.Name, prefix) {
			e2e.Logf("Skipping removing persistent volume: %s", pv.Name)
			continue
		}

		if err = c.Delete(pv.Name, nil); err != nil {
			e2e.Logf("WARNING: couldn't remove PV %s: %v\n", pv.Name, err)
			continue
		}
		e2e.Logf("Removed persistent volume: %s", pv.Name)
	}

	if err = RemoveNFSServer(oc); err != nil {
		return err
	}

	return nil
}
