package openshift

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/golang/glog"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	"github.com/openshift/origin/pkg/oc/bootstrap/clusterup/componentinstall"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/dockerhelper"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/oc/bootstrap/docker/run"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

const (
	DefaultNamespace         = "default"
	RouterServiceAccountName = "router"
	RouterServiceName        = "router"

	masterConfigDir = "/var/lib/origin/openshift.local.config/master"
	routerCertPath  = masterConfigDir + "/router.pem"
)

// InstallRouter installs a default router on the OpenShift server
func (h *Helper) InstallRouter(dockerClient dockerhelper.Interface, ocImage string, kubeClient kclientset.Interface, f *clientcmd.Factory, configDir, logdir, images, hostIP string, portForwarding bool, out, errout io.Writer) error {
	_, err := kubeClient.Core().Services(DefaultNamespace).Get(RouterServiceName, metav1.GetOptions{})
	if err == nil {
		glog.V(3).Infof("The %q service is already present, skipping installation", RouterServiceName)
		// Router service already exists, nothing to do
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving router service").WithCause(err).WithDetails(h.OriginLog())
	}

	componentName := "install-router"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

	masterDir := filepath.Join(configDir, "master")

	// Create service account for router
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = RouterServiceAccountName
	_, err = kubeClient.Core().ServiceAccounts("default").Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create router service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add router SA to privileged SCC
	securityClient, err := f.OpenshiftInternalSecurityClient()
	if err != nil {
		return err
	}
	privilegedSCC, err := securityClient.Security().SecurityContextConstraints().Get("privileged", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve privileged SCC").WithCause(err).WithDetails(h.OriginLog())
	}
	privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername("default", RouterServiceAccountName))
	_, err = securityClient.Security().SecurityContextConstraints().Update(privilegedSCC)
	if err != nil {
		return errors.NewError("cannot update privileged SCC").WithCause(err).WithDetails(h.OriginLog())
	}

	routingSuffix := h.routingSuffix
	if len(routingSuffix) == 0 {
		routingSuffix = fmt.Sprintf("%s.nip.io", hostIP)
	}

	// Create router cert
	cmdOutput := &bytes.Buffer{}
	createCertOptions := &admin.CreateServerCertOptions{
		SignerCertOptions: &admin.SignerCertOptions{
			CertFile:   filepath.Join(masterDir, "ca.crt"),
			KeyFile:    filepath.Join(masterDir, "ca.key"),
			SerialFile: filepath.Join(masterDir, "ca.serial.txt"),
		},
		Overwrite: true,
		Hostnames: []string{
			routingSuffix,
			// This will ensure that routes using edge termination and the default
			// certs will use certs valid for their arbitrary subdomain names.
			fmt.Sprintf("*.%s", routingSuffix),
		},
		CertFile: filepath.Join(masterDir, "router.crt"),
		KeyFile:  filepath.Join(masterDir, "router.key"),
		Output:   cmdOutput,
	}
	_, err = createCertOptions.CreateServerCert()
	if err != nil {
		return errors.NewError("cannot create router cert").WithCause(err)
	}

	err = catFiles(filepath.Join(masterDir, "router.pem"),
		filepath.Join(masterDir, "router.crt"),
		filepath.Join(masterDir, "router.key"),
		filepath.Join(masterDir, "ca.crt"))
	if err != nil {
		return errors.NewError("cannot create aggregate router cert").WithCause(err)
	}

	flags := []string{
		"adm", "router",
		"--host-ports=true",
		"--loglevel=8",
		"--config=" + masterConfigDir + "/admin.kubeconfig",
		fmt.Sprintf("--host-network=%v", !portForwarding),
		fmt.Sprintf("--images=%s", images),
		fmt.Sprintf("--default-cert=%s", routerCertPath),
	}
	_, stdout, stderr, rc, err := imageRunHelper.Image(ocImage).
		DiscardContainer().
		HostNetwork().
		Bind(masterDir + ":" + masterConfigDir).
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

func AddClusterRole(authorizationClient authorizationtypedclient.ClusterRoleBindingsGetter, role, user string) error {
	clusterRoleBindingAccessor := policy.NewClusterRoleBindingAccessor(authorizationClient)
	addClusterReaderRole := policy.RoleModificationOptions{
		RoleName:            role,
		RoleBindingAccessor: clusterRoleBindingAccessor,
		Users:               []string{user},
	}
	return addClusterReaderRole.AddRole()
}

func AddSCCToServiceAccount(securityClient securitytypedclient.SecurityContextConstraintsGetter, scc, sa, namespace string, out io.Writer) error {
	modifySCC := policy.SCCModificationOptions{
		SCCName:      scc,
		SCCInterface: securityClient.SecurityContextConstraints(),
		Subjects: []kapi.ObjectReference{
			{
				Namespace: namespace,
				Name:      sa,
				Kind:      "ServiceAccount",
			},
		},

		Out: out,
	}
	return modifySCC.AddSCC()
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
