package registry

import (
	"fmt"
	"path"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	DefaultNamespace  = "default"
	SvcDockerRegistry = "docker-registry"
	masterConfigDir   = "/var/lib/origin/openshift.local.config/master"
	// This is needed because of NO_PROXY cannot handle the CIDR range
	RegistryServiceClusterIP = "172.30.1.1"
)

type RegistryComponentOptions struct {
	PVDir string

	InstallContext componentinstall.Context
}

func (r *RegistryComponentOptions) Name() string {
	return "openshift-image-registry"
}

func (r *RegistryComponentOptions) Install(dockerClient dockerhelper.Interface, logdir string) error {
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

	masterConfigDir := path.Join(r.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName)
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
		fmt.Sprintf("--mount-host=%s", path.Join(r.PVDir, "registry")),
	}
	_, rc, err := imageRunHelper.Image(r.InstallContext.ClientImage()).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		SaveContainerLogs(r.Name(), logdir).
		Bind(masterConfigDir + ":" + masterConfigDir).
		Entrypoint("oc").
		Command(flags...).Run()

	if rc != 0 {
		return errors.NewError("could not run %q: rc=%d", r.Name(), rc)
	}

	return nil
}
