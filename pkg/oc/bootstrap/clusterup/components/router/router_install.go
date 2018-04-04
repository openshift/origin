package router

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
)

const (
	DefaultNamespace         = "default"
	RouterServiceAccountName = "router"
	RouterServiceName        = "router"
)

type RouterComponentOptions struct {
	PublicMasterURL string
	RoutingSuffix   string
	PortForwarding  bool

	InstallContext componentinstall.Context
}

func (c *RouterComponentOptions) Name() string {
	return "openshift-router"
}

func (c *RouterComponentOptions) Install(dockerClient dockerhelper.Interface, logdir string) error {
	kubeAdminClient, err := kclientset.NewForConfig(c.InstallContext.ClusterAdminClientConfig())
	if err != nil {
		return err
	}
	_, err = kubeAdminClient.Core().Services(DefaultNamespace).Get(RouterServiceName, metav1.GetOptions{})
	if err == nil {
		glog.V(3).Infof("The %q service is already present, skipping installation", RouterServiceName)
		// Router service already exists, nothing to do
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving router service").WithCause(err)
	}

	componentName := "install-router"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()

	// Create service account for router
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = RouterServiceAccountName
	_, err = kubeAdminClient.Core().ServiceAccounts(DefaultNamespace).Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create router service account").WithCause(err)
	}

	if err := componentinstall.AddPrivilegedUser(c.InstallContext.ClusterAdminClientConfig(), DefaultNamespace, RouterServiceAccountName); err != nil {
		return err
	}

	masterConfigDir := path.Join(c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName)
	// Create router cert
	cmdOutput := &bytes.Buffer{}
	createCertOptions := &admin.CreateServerCertOptions{
		SignerCertOptions: &admin.SignerCertOptions{
			CertFile:   filepath.Join(masterConfigDir, "ca.crt"),
			KeyFile:    filepath.Join(masterConfigDir, "ca.key"),
			SerialFile: filepath.Join(masterConfigDir, "ca.serial.txt"),
		},
		Overwrite: true,
		Hostnames: []string{
			c.RoutingSuffix,
			// This will ensure that routes using edge termination and the default
			// certs will use certs valid for their arbitrary subdomain names.
			"*." + c.RoutingSuffix,
		},
		CertFile: filepath.Join(masterConfigDir, "router.crt"),
		KeyFile:  filepath.Join(masterConfigDir, "router.key"),
		Output:   cmdOutput,
	}
	_, err = createCertOptions.CreateServerCert()
	if err != nil {
		return errors.NewError("cannot create router cert").WithCause(err)
	}

	err = catFiles(filepath.Join(masterConfigDir, "router.pem"),
		filepath.Join(masterConfigDir, "router.crt"),
		filepath.Join(masterConfigDir, "router.key"),
		filepath.Join(masterConfigDir, "ca.crt"))
	if err != nil {
		return errors.NewError("cannot create aggregate router cert").WithCause(err)
	}

	routerCertPath := masterConfigDir + "/router.pem"
	flags := []string{
		"adm", "router",
		"--host-ports=true",
		fmt.Sprintf("--loglevel=%d", c.InstallContext.LogLevel()),
		"--config=" + masterConfigDir + "/admin.kubeconfig",
		fmt.Sprintf("--host-network=%v", !c.PortForwarding),
		fmt.Sprintf("--images=%s", c.InstallContext.ImageFormat()),
		fmt.Sprintf("--default-cert=%s", routerCertPath),
	}
	_, rc, err := imageRunHelper.Image(c.InstallContext.ClientImage()).
		Privileged().
		DiscardContainer().
		HostNetwork().
		SaveContainerLogs(componentName, logdir).
		Bind(masterConfigDir + ":" + masterConfigDir).
		Entrypoint("oc").
		Command(flags...).Run()
	if rc != 0 {
		return errors.NewError("could not run %q: rc=%d", componentName, rc)
	}
	return nil
}

// catFiles concatenates multiple source files into a single destination file
func catFiles(dest string, src ...string) error {
	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()
	for _, f := range src {
		in, oerr := os.Open(f)
		if oerr != nil {
			return err
		}
		_, err = io.Copy(out, in)
		in.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
