package registry

import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/host"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	DefaultNamespace  = "default"
	SvcDockerRegistry = "docker-registry"
	// This is needed because of NO_PROXY cannot handle the CIDR range
	RegistryServiceClusterIP = "172.30.1.1"
)

type RegistryComponentOptions struct {
	InstallContext componentinstall.Context
}

func (r *RegistryComponentOptions) Name() string {
	return "openshift-image-registry"
}

// ensureRemoteRegistryStoragePermissions ensures the remote host directory for registry storage have write access permissions
// so the registry can successfully write data into it.
// TODO: This is a remote docker snowflake
func (r *RegistryComponentOptions) ensureRemoteRegistryStoragePermissions(dir string, dockerClient dockerhelper.Interface) error {
	glog.V(5).Infof("Ensuring the write permissions in remote directory %s", dir)
	_, rc, err := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New().
		Image(r.InstallContext.ClientImage()).
		DiscardContainer().
		Privileged().
		Bind(fmt.Sprintf("%s:/pv", dir)).
		Entrypoint("/bin/bash").
		Command("-c", "mkdir -p /pv/registry && chmod 0777 /pv/registry").Run()
	if rc != 0 {
		return fmt.Errorf("command returning non-zero exit code: %d", rc)
	}
	return err
}

func (r *RegistryComponentOptions) Install(dockerClient dockerhelper.Interface) error {
	kubeAdminClient, err := kubernetes.NewForConfig(r.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	_, err = kubeAdminClient.Core().Services(DefaultNamespace).Get(SvcDockerRegistry, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the registry already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving docker registry service").WithCause(err)
	}

	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	if err := componentinstall.AddPrivilegedUser(r.InstallContext.ClusterAdminClientConfig(), DefaultNamespace, "registry"); err != nil {
		return err
	}

	// If docker is on remote host, the base dir is different.
	baseDir := r.InstallContext.BaseDir()
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		baseDir = path.Join(host.RemoteHostOriginDir, r.InstallContext.BaseDir())
	}

	registryStorageDir := path.Join(baseDir, "openshift.local.pv")

	// If docker is on remote host, ensure the permissions for registry
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		if err := r.ensureRemoteRegistryStoragePermissions(registryStorageDir, dockerClient); err != nil {
			return errors.NewError("error ensuring remote host registry directory permissions").WithCause(err)
		}
	}
	registryStorageDir = path.Join(registryStorageDir, "registry")

	masterConfigDir := path.Join(baseDir, kubeapiserver.KubeAPIServerDirName)
	flags := []string{
		"adm",
		"registry",
		fmt.Sprintf("--loglevel=%d", r.InstallContext.ComponentLogLevel()),
		// We need to set the ClusterIP for registry in order to be able to set the NO_PROXY no predicable
		// IP address as NO_PROXY does not support CIDR format.
		// TODO: We should switch the cluster up registry to use DNS.
		fmt.Sprintf("--cluster-ip=%s", RegistryServiceClusterIP),
		fmt.Sprintf("--config=%s", path.Join(masterConfigDir, "admin.kubeconfig")),
		fmt.Sprintf("--images=%s", r.InstallContext.ImageFormat()),
		fmt.Sprintf("--mount-host=%s", registryStorageDir),
	}
	_, rc, err := imageRunHelper.Image(r.InstallContext.ClientImage()).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		SaveContainerLogs(r.Name(), filepath.Join(r.InstallContext.BaseDir(), "logs")).
		Bind(masterConfigDir + ":" + masterConfigDir).
		Entrypoint("oc").
		Command(flags...).Run()

	if rc != 0 {
		return errors.NewError("could not run %q: rc=%d", r.Name(), rc)
	}

	return nil
}
