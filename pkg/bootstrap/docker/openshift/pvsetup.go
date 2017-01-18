package openshift

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/client"
)

const (
	pvCount          = 100
	pvSetupJobName   = "persistent-volume-setup"
	pvInstallerSA    = "pvinstaller"
	pvSetupNamespace = "default"
)

const createPVScript = `#/bin/bash

set -e

function generate_pv() {
  local basedir="${1}"
  local name="${2}"
cat <<EOF
apiVersion: v1
kind: PersistentVolume
metadata:
  name: ${name}
  labels:
    volume: ${name}
spec:
  capacity:
    storage: 100Gi
  accessModes:
    - ReadWriteOnce
    - ReadWriteMany
    - ReadOnlyMany
  hostPath:
    path: ${basedir}/${name}
  persistentVolumeReclaimPolicy: Recycle
EOF
}

function setup_pv_dir() {
  local dir="${1}"
  if [[ ! -d "${dir}" ]]; then
    mkdir -p "${dir}"
  fi
  if ! chcon -t svirt_sandbox_file_t "${dir}" &> /dev/null; then
    echo "Not setting SELinux content for ${dir}"
  fi
  chmod 770 "${dir}"
}

function create_pv() {
  local basedir="${1}"
  local name="${2}"

  setup_pv_dir "${basedir}/${name}"
  if ! oc get pv "${name}" &> /dev/null; then 
    generate_pv "${basedir}" "${name}" | oc create -f -
  else
    echo "persistentvolume ${name} already exists"
  fi
}

basedir="%[2]s"
setup_pv_dir "${basedir}/registry"

for i in $(seq -f "%%04g" 1 %[1]d); do
  create_pv "${basedir}" "pv${i}"
done
`

func (h *Helper) SetupPersistentStorage(osclient client.Interface, kclient kclientset.Interface, dir string) error {

	err := h.ensurePVInstallerSA(osclient, kclient)
	if err != nil {
		return err
	}

	_, err = kclient.Batch().Jobs(pvSetupNamespace).Get(pvSetupJobName)
	if err == nil {
		// Job exists, it should run to completion
		return nil
	}
	if !kerrors.IsNotFound(err) {
		return errors.NewError("error retrieving job to setup persistent volumes (%s/%s)", pvSetupNamespace, pvSetupJobName).WithCause(err).WithDetails(h.OriginLog())
	}

	setupJob := persistentStorageSetupJob(pvSetupJobName, dir, h.image)
	if _, err = kclient.Batch().Jobs(pvSetupNamespace).Create(setupJob); err != nil {
		return errors.NewError("cannot create job to setup persistent volumes (%s/%s)", pvSetupNamespace, pvSetupJobName).WithCause(err).WithDetails(h.OriginLog())
	}

	return nil
}

func (h *Helper) ensurePVInstallerSA(osclient client.Interface, kclient kclientset.Interface) error {
	createSA := false
	sa, err := kclient.Core().ServiceAccounts(pvSetupNamespace).Get(pvInstallerSA)
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return errors.NewError("error retrieving installer service account (%s/%s)", pvSetupNamespace, pvInstallerSA).WithCause(err).WithDetails(h.OriginLog())
		}
		createSA = true
	}

	// Create installer SA
	if createSA {
		sa = &kapi.ServiceAccount{}
		sa.Name = pvInstallerSA
		_, err = kclient.Core().ServiceAccounts(pvSetupNamespace).Create(sa)
		if err != nil {
			return errors.NewError("cannot create pvinstaller service account").WithCause(err).WithDetails(h.OriginLog())
		}
	}

	err = AddSCCToServiceAccount(kclient, "privileged", "pvinstaller", "default")
	if err != nil {
		return errors.NewError("cannot add privileged SCC to pvinstaller service account").WithCause(err).WithDetails(h.OriginLog())
	}

	saUser := serviceaccount.MakeUsername(pvSetupNamespace, pvInstallerSA)
	err = AddClusterRole(osclient, "cluster-admin", saUser)
	if err != nil {
		return errors.NewError("cannot add cluster role to service account (%s/%s)", pvSetupNamespace, pvInstallerSA).WithCause(err).WithDetails(h.OriginLog())
	}

	return nil
}

func persistentStorageSetupJob(name, dir, image string) *kbatch.Job {
	// Job volume
	volume := kapi.Volume{}
	volume.Name = "pvdir"
	volume.HostPath = &kapi.HostPathVolumeSource{Path: dir}

	// Volume mount
	mount := kapi.VolumeMount{}
	mount.Name = "pvdir"
	mount.MountPath = dir

	// Job container
	container := kapi.Container{}
	container.Name = "storage-setup-job"
	container.Image = image
	container.Command = []string{"/bin/bash", "-c", fmt.Sprintf(createPVScript, pvCount, dir)}
	privileged := true
	container.SecurityContext = &kapi.SecurityContext{
		Privileged: &privileged,
	}
	container.VolumeMounts = []kapi.VolumeMount{mount}

	// Job
	completions := int32(1)
	deadline := int64(60 * 20)
	job := &kbatch.Job{}
	job.Name = name
	job.Spec.Completions = &completions
	job.Spec.ActiveDeadlineSeconds = &deadline
	job.Spec.Template.Spec.Volumes = []kapi.Volume{volume}
	job.Spec.Template.Spec.RestartPolicy = kapi.RestartPolicyNever
	job.Spec.Template.Spec.ServiceAccountName = "pvinstaller"
	job.Spec.Template.Spec.Containers = []kapi.Container{container}
	return job
}
