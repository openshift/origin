package registry

import (
	"fmt"
	"os"
	"path"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/openshift"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
	securityclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

const (
	DefaultNamespace  = "default"
	SvcDockerRegistry = "docker-registry"
	masterConfigDir   = "/var/lib/origin/openshift.local.config/master"
	// This is needed because of NO_PROXY cannot handle the CIDR range
	RegistryServiceClusterIP = "172.30.1.1"
)

type RegistryComponentOptions struct {
	ClusterAdminKubeConfig *rest.Config

	OCImage         string
	MasterConfigDir string
	Images          string
	PVDir           string
}

func (r *RegistryComponentOptions) Name() string {
	return "openshift-image-registry"
}

func (r *RegistryComponentOptions) Install(dockerClient dockerhelper.Interface, logdir string) error {
	kubeClient, err := kubernetes.NewForConfig(r.ClusterAdminKubeConfig)
	_, err = kubeClient.Core().Services(DefaultNamespace).Get(SvcDockerRegistry, metav1.GetOptions{})
	if err == nil {
		// If there's no error, the registry already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving docker registry service").WithCause(err)
	}

	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", r.Name())

	securityClient, err := securityclient.NewForConfig(r.ClusterAdminKubeConfig)
	if err != nil {
		return err
	}
	err = openshift.AddSCCToServiceAccount(securityClient, "privileged", "registry", "default", os.Stdout)
	if err != nil {
		return errors.NewError("cannot add privileged SCC to registry service account").WithCause(err)
	}

	// Obtain registry markup. The reason it is not created outright is because
	// we need to modify the ClusterIP of the registry service. The command doesn't
	// have an option to set it.
	flags := []string{
		"adm",
		"registry",
		"--loglevel=8",
		// We need to set the ClusterIP for registry in order to be able to set the NO_PROXY no predicable
		// IP address as NO_PROXY does not support CIDR format.
		// TODO: We should switch the cluster up registry to use DNS.
		"--cluster-ip=" + RegistryServiceClusterIP,
		"--config=" + masterConfigDir + "/admin.kubeconfig",
		fmt.Sprintf("--images=%s", r.Images),
		fmt.Sprintf("--mount-host=%s", path.Join(r.PVDir, "registry")),
	}
	_, stdout, stderr, rc, err := imageRunHelper.Image(r.OCImage).
		Privileged().
		DiscardContainer().
		HostNetwork().
		HostPid().
		Bind(r.MasterConfigDir + ":" + masterConfigDir).
		Entrypoint("oc").
		Command(flags...).Output()

	if err := componentinstall.LogContainer(logdir, r.Name(), stdout, stderr); err != nil {
		glog.Errorf("error logging %q: %v", r.Name(), err)
	}
	if err != nil {
		return errors.NewError("could not run %q: %v", r.Name(), err).WithCause(err)
	}
	if rc != 0 {
		return errors.NewError("could not run %q: rc==%v", r.Name(), rc)
	}

	return nil
}
