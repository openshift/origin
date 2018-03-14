package openshift

import (
	"bytes"
	"fmt"
	"os"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kbatch "k8s.io/kubernetes/pkg/apis/batch"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset"
)

const (
	pvCount            = 100
	pvSetupJobName     = "persistent-volume-setup"
	pvInstallerSA      = "pvinstaller"
	pvSetupNamespace   = "default"
	pvIgnoreMarkerFile = ".skip_pv"
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

// SetupPersistentStorage sets up persistent storage
func (h *Helper) SetupPersistentStorage(authorizationClient authorizationtypedclient.ClusterRoleBindingsGetter, kclient kclientset.Interface, securityClient securityclient.Interface, dir, HostPersistentVolumesDir string) error {
	err := h.ensurePVInstallerSA(authorizationClient, kclient, securityClient)
	if err != nil {
		return err
	}

	_, err = kclient.Batch().Jobs(pvSetupNamespace).Get(pvSetupJobName, metav1.GetOptions{})
	if err == nil {
		// Job exists, it should run to completion
		return nil
	}
	if !kerrors.IsNotFound(err) {
		return errors.NewError("error retrieving job to setup persistent volumes (%s/%s)", pvSetupNamespace, pvSetupJobName).WithCause(err).WithDetails(h.OriginLog())
	}

	// check if we need to create pv's
	_, err = os.Stat(fmt.Sprintf("%s/%s", HostPersistentVolumesDir, pvIgnoreMarkerFile))
	if !os.IsNotExist(err) {
		fmt.Printf("Skip persistent volume creation \n")
	} else {
		setupJob := persistentStorageSetupJob(pvSetupJobName, dir, h.image, pvCount)
		if _, err = kclient.Batch().Jobs(pvSetupNamespace).Create(setupJob); err != nil {
			return errors.NewError("cannot create job to setup persistent volumes (%s/%s)", pvSetupNamespace, pvSetupJobName).WithCause(err).WithDetails(h.OriginLog())
		}
	}
	return nil
}

func (h *Helper) ensurePVInstallerSA(authorizationClient authorizationtypedclient.ClusterRoleBindingsGetter, kclient kclientset.Interface, securityClient securityclient.Interface) error {
	createSA := false
	sa, err := kclient.Core().ServiceAccounts(pvSetupNamespace).Get(pvInstallerSA, metav1.GetOptions{})
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

	err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		err = AddSCCToServiceAccount(securityClient.Security(), "privileged", "pvinstaller", "default", &bytes.Buffer{})
		// TODO this eventually becomes a component, but we do need to figure out why this is sometimes giving a 404. on SCC get
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, errors.NewError("cannot add privileged SCC to pvinstaller service account").WithCause(err).WithDetails(h.OriginLog())
		}

		saUser := serviceaccount.MakeUsername(pvSetupNamespace, pvInstallerSA)
		err = AddClusterRole(authorizationClient, "cluster-admin", saUser)

		// TODO this eventually becomes a component, but we do need to figure out why this is sometimes giving a 404. on GET https://127.0.0.1:8443/apis/authorization.openshift.io/v1/clusterrolebindings
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, errors.NewError("cannot add cluster role to service account (%s/%s)", pvSetupNamespace, pvInstallerSA).WithCause(err).WithDetails(h.OriginLog())
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return nil
}

func persistentStorageSetupJob(name, dir, image string, pvCount int) *kbatch.Job {
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
