package router

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
	goruntime "runtime"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/host"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/kubeapiserver"
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
	InstallContext componentinstall.Context
}

func (c *RouterComponentOptions) Name() string {
	return "openshift-router"
}

func (c *RouterComponentOptions) Install(dockerClient dockerhelper.Interface) error {
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

	masterConfig, err := getMasterConfig(c.InstallContext.BaseDir())
	if err != nil {
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
			masterConfig.RoutingConfig.Subdomain,
			// This will ensure that routes using edge termination and the default
			// certs will use certs valid for their arbitrary subdomain names.
			"*." + masterConfig.RoutingConfig.Subdomain,
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

	dockerHelper := dockerhelper.NewHelper(dockerClient)

	// This snowflake makes sure that when the Docker is on remote host, we copy the generated cert into
	// the Docker host master config dir.
	if len(os.Getenv("DOCKER_HOST")) > 0 {
		hostHelper := host.NewHostHelper(dockerHelper, c.InstallContext.ClientImage())
		remoteMasterConfigDir := path.Join(host.RemoteHostOriginDir, c.InstallContext.BaseDir(), kubeapiserver.KubeAPIServerDirName)
		if err := hostHelper.CopyToHost(masterConfigDir, remoteMasterConfigDir); err != nil {
			return err
		}
		masterConfigDir = remoteMasterConfigDir
	}

	routerCertPath := masterConfigDir + "/router.pem"

	flags := []string{
		"adm", "router",
		"--host-ports=true",
		fmt.Sprintf("--loglevel=%d", c.InstallContext.ComponentLogLevel()),
		"--config=" + masterConfigDir + "/admin.kubeconfig",
		fmt.Sprintf("--host-network=%v", !portForwarding()),
		fmt.Sprintf("--images=%s", c.InstallContext.ImageFormat()),
		fmt.Sprintf("--default-cert=%s", routerCertPath),
	}
	_, rc, err := imageRunHelper.Image(c.InstallContext.ClientImage()).
		Privileged().
		DiscardContainer().
		HostNetwork().
		SaveContainerLogs(componentName, filepath.Join(c.InstallContext.BaseDir(), "logs")).
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

func getMasterConfig(basedir string) (*configapi.MasterConfig, error) {
	configBytes, err := ioutil.ReadFile(path.Join(basedir, kubeapiserver.KubeAPIServerDirName, "master-config.yaml"))
	if err != nil {
		return nil, err
	}
	configObj, err := runtime.Decode(configapilatest.Codec, configBytes)
	if err != nil {
		return nil, err
	}
	masterConfig, ok := configObj.(*configapi.MasterConfig)
	if !ok {
		return nil, fmt.Errorf("the %#v is not MasterConfig", configObj)
	}
	return masterConfig, nil
}

func portForwarding() bool {
	// true if running on Mac, with no DOCKER_HOST defined
	return goruntime.GOOS == "darwin" && len(os.Getenv("DOCKER_HOST")) == 0
}
