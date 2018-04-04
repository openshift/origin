package router

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"k8s.io/client-go/util/retry"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/errors"
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
)

const (
	DefaultNamespace         = "default"
	RouterServiceAccountName = "router"
	RouterServiceName        = "router"
)

type RouterComponentOptions struct {
	OCImage         string
	MasterConfigDir string
	ImageFormat     string
	PublicMasterURL string
	RoutingSuffix   string
	PortForwarding  bool
}

func (c *RouterComponentOptions) Name() string {
	return "openshift-router"
}

func (c *RouterComponentOptions) Install(dockerClient dockerhelper.Interface, logdir string) error {
	clusterAdminKubeConfigBytes, err := ioutil.ReadFile(path.Join(c.MasterConfigDir, "admin.kubeconfig"))
	if err != nil {
		return err
	}
	restConfig, err := kclientcmd.RESTConfigFromKubeConfig(clusterAdminKubeConfigBytes)
	if err != nil {
		return err
	}
	kubeClient, err := kclientset.NewForConfig(restConfig)
	if err != nil {
		return err
	}
	_, err = kubeClient.Core().Services(DefaultNamespace).Get(RouterServiceName, metav1.GetOptions{})
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
	glog.Infof("Running %q", componentName)

	// Create service account for router
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = RouterServiceAccountName
	_, err = kubeClient.Core().ServiceAccounts("default").Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create router service account").WithCause(err)
	}

	// Add router SA to privileged SCC
	err = retry.RetryOnConflict(retry.DefaultRetry, func() error {
		securityClient, err := securityclientinternal.NewForConfig(restConfig)
		if err != nil {
			return err
		}
		privilegedSCC, err := securityClient.Security().SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
		if err != nil {
			return err
		}
		privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername("default", RouterServiceAccountName))
		_, err = securityClient.Security().SecurityContextConstraints().Update(privilegedSCC)
		if err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return errors.NewError("cannot update privileged SCC").WithCause(err)
	}

	// Create router cert
	cmdOutput := &bytes.Buffer{}
	createCertOptions := &admin.CreateServerCertOptions{
		SignerCertOptions: &admin.SignerCertOptions{
			CertFile:   filepath.Join(c.MasterConfigDir, "ca.crt"),
			KeyFile:    filepath.Join(c.MasterConfigDir, "ca.key"),
			SerialFile: filepath.Join(c.MasterConfigDir, "ca.serial.txt"),
		},
		Overwrite: true,
		Hostnames: []string{
			c.RoutingSuffix,
			// This will ensure that routes using edge termination and the default
			// certs will use certs valid for their arbitrary subdomain names.
			fmt.Sprintf("*.%s", c.RoutingSuffix),
		},
		CertFile: filepath.Join(c.MasterConfigDir, "router.crt"),
		KeyFile:  filepath.Join(c.MasterConfigDir, "router.key"),
		Output:   cmdOutput,
	}
	_, err = createCertOptions.CreateServerCert()
	if err != nil {
		return errors.NewError("cannot create router cert").WithCause(err)
	}

	err = catFiles(filepath.Join(c.MasterConfigDir, "router.pem"),
		filepath.Join(c.MasterConfigDir, "router.crt"),
		filepath.Join(c.MasterConfigDir, "router.key"),
		filepath.Join(c.MasterConfigDir, "ca.crt"))
	if err != nil {
		return errors.NewError("cannot create aggregate router cert").WithCause(err)
	}

	routerCertPath := c.MasterConfigDir + "/router.pem"
	flags := []string{
		"adm", "router",
		"--host-ports=true",
		"--loglevel=8",
		"--config=" + c.MasterConfigDir + "/admin.kubeconfig",
		fmt.Sprintf("--host-network=%v", !c.PortForwarding),
		fmt.Sprintf("--images=%s", c.ImageFormat),
		fmt.Sprintf("--default-cert=%s", routerCertPath),
	}
	_, stdout, stderr, rc, err := imageRunHelper.Image(c.OCImage).
		Privileged().
		DiscardContainer().
		HostNetwork().
		Bind(c.MasterConfigDir + ":" + c.MasterConfigDir).
		Entrypoint("oc").
		Command(flags...).Output()

	if err := componentinstall.LogContainer(logdir, componentName, stdout, stderr); err != nil {
		glog.Errorf("error logging %q: %v", componentName, err)
	}
	if err != nil {
		return errors.NewError("could not run %q: %v", componentName, err).WithCause(err)
	}
	if rc != 0 {
		return errors.NewError("could not run %q: rc==%v", componentName, rc)
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
