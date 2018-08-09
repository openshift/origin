package persistent_volumes

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/golang/glog"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	"k8s.io/client-go/kubernetes"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	"k8s.io/client-go/util/retry"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	securityclient "github.com/openshift/client-go/security/clientset/versioned"
	securitytypedclient "github.com/openshift/client-go/security/clientset/versioned/typed/security/v1"
	"github.com/openshift/origin/pkg/oc/cli/admin/policy"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/docker/dockerhelper"
)

const (
	pvCount            = 100
	pvSetupJobName     = "persistent-volume-setup"
	pvInstallerSA      = "pvinstaller"
	pvTargetNamespace  = "default"
	pvIgnoreMarkerFile = ".skip_pv"
)

// TODO: This really should be a Go template
const createPVScript = `#/bin/bash -xe
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

for i in $(seq -f "%%04g" 1 %[1]d); do
  create_pv "${basedir}" "pv${i}"
done
`

type SetupPersistentVolumesOptions struct {
	InstallContext componentinstall.Context
}

func (c *SetupPersistentVolumesOptions) Name() string {
	return "persistent-volumes"
}

func (c *SetupPersistentVolumesOptions) Install(dockerClient dockerhelper.Interface) error {
	kclient, err := kubernetes.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	securityClient, err := securityclient.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	rbacClient, err := rbacv1client.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	if err := ensurePVInstallerSA(rbacClient, kclient, securityClient); err != nil {
		return err
	}

	// TODO: Make the job idempotent and non-failing
	_, err = kclient.BatchV1().Jobs(pvTargetNamespace).Get(pvSetupJobName, metav1.GetOptions{})
	if err == nil {
		return nil
	}
	if !kerrors.IsNotFound(err) {
		return fmt.Errorf("error retrieving job to setup persistent volumes (%s/%s): %v", pvTargetNamespace, pvSetupJobName, err)
	}
	targetDir := path.Join(c.InstallContext.BaseDir(), "openshift.local.pv")

	if _, err := os.Stat(path.Join(targetDir, pvIgnoreMarkerFile)); !os.IsNotExist(err) {
		glog.Infof("Found %q marker file, skipping persistent volume setup", path.Join(targetDir, pvIgnoreMarkerFile))
		return nil
	}

	setupJob := persistentStorageSetupJob(pvSetupJobName, targetDir, c.InstallContext.ClientImage(), pvCount)
	if _, err = kclient.BatchV1().Jobs(pvTargetNamespace).Create(setupJob); err != nil {
		return fmt.Errorf("cannot create job to setup persistent volumes (%s/%s): %v", pvTargetNamespace, pvSetupJobName, err)
	}

	return nil
}

func ensurePVInstallerSA(rbacClient rbacv1client.RbacV1Interface, kclient kubernetes.Interface, securityClient securityclient.Interface) error {
	sa, err := kclient.CoreV1().ServiceAccounts(pvTargetNamespace).Get(pvInstallerSA, metav1.GetOptions{})
	if err != nil {
		if !kerrors.IsNotFound(err) {
			return fmt.Errorf("error retrieving installer service account (%s/%s): %v", pvTargetNamespace, pvInstallerSA, err)
		}
		sa = &corev1.ServiceAccount{}
		sa.Name = pvInstallerSA
		if _, err := kclient.CoreV1().ServiceAccounts(pvTargetNamespace).Create(sa); err != nil {
			return fmt.Errorf("cannot create %q service account: %v", sa.Name, err)
		}
	}

	err = wait.PollImmediate(time.Second, 5*time.Minute, func() (bool, error) {
		err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return addSCCToServiceAccount(securityClient.SecurityV1(), "privileged", pvInstallerSA, pvTargetNamespace, &bytes.Buffer{})
		})
		// TODO: We do need to figure out why this is sometimes giving a 404. on SCC get
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("cannot add privileged SCC to %q SA: %v", sa.Name, err)
		}

		saUser := serviceaccount.MakeUsername(pvTargetNamespace, pvInstallerSA)
		err = retry.RetryOnConflict(retry.DefaultBackoff, func() error {
			return addClusterRole(rbacClient, "cluster-admin", saUser)
		})

		// TODO: we do need to figure out why this is sometimes giving a 404. on GET https://127.0.0.1:8443/apis/authorization.openshift.io/v1/clusterrolebindings
		if kerrors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, fmt.Errorf("cannot add cluster role to service account (%s/%s): %v", pvTargetNamespace, pvInstallerSA, err)
		}
		return true, nil
	})
	if err != nil {
		return err
	}

	return nil
}

func persistentStorageSetupJob(name, dir, image string, pvCount int) *batchv1.Job {
	// Job volume
	volume := corev1.Volume{}
	volume.Name = "pvdir"
	volume.HostPath = &corev1.HostPathVolumeSource{Path: dir}

	// Volume mount
	mount := corev1.VolumeMount{}
	mount.Name = "pvdir"
	mount.MountPath = dir

	// Job container
	container := corev1.Container{}
	container.Name = "setup-persistent-volumes"
	container.Image = image
	container.Command = []string{"/bin/bash", "-c", fmt.Sprintf(createPVScript, pvCount, dir)}
	privileged := true
	container.SecurityContext = &corev1.SecurityContext{
		Privileged: &privileged,
	}
	container.VolumeMounts = []corev1.VolumeMount{mount}

	// Job
	completions := int32(1)
	deadline := int64(60 * 20)
	job := &batchv1.Job{}
	job.Name = name
	job.Spec.Completions = &completions
	job.Spec.ActiveDeadlineSeconds = &deadline
	job.Spec.Template.Spec.Volumes = []corev1.Volume{volume}
	job.Spec.Template.Spec.RestartPolicy = corev1.RestartPolicyNever
	job.Spec.Template.Spec.ServiceAccountName = pvInstallerSA
	job.Spec.Template.Spec.Containers = []corev1.Container{container}
	return job
}

func addClusterRole(rbacClient rbacv1client.RbacV1Interface, role, user string) error {
	addClusterReaderRole := policy.RoleModificationOptions{
		RoleName:   role,
		RoleKind:   "ClusterRole",
		RbacClient: rbacClient,
		Users:      []string{user},
	}
	return addClusterReaderRole.AddRole()
}

func addSCCToServiceAccount(securityClient securitytypedclient.SecurityContextConstraintsGetter, scc, sa, namespace string, out io.Writer) error {
	modifySCC := policy.SCCModificationOptions{
		SCCName:      scc,
		SCCInterface: securityClient.SecurityContextConstraints(),
		Subjects: []corev1.ObjectReference{
			{
				Namespace: namespace,
				Name:      sa,
				Kind:      "ServiceAccount",
			},
		},

		IOStreams: genericclioptions.IOStreams{Out: out},
	}
	return modifySCC.AddSCC()
}
