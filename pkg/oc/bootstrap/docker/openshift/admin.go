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
	"k8s.io/client-go/rest"
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
	securityclientinternal "github.com/openshift/origin/pkg/security/generated/internalclientset"
	securitytypedclient "github.com/openshift/origin/pkg/security/generated/internalclientset/typed/security/internalversion"
)

const (
	DefaultNamespace         = "default"
	RouterServiceAccountName = "router"
	RouterServiceName        = "router"
)

// InstallRouter installs a default router on the OpenShift server
func (h *Helper) InstallRouter(dockerClient dockerhelper.Interface, ocImage string, restConfig *rest.Config, configDir, logdir, images, hostIP string, portForwarding bool, out, errout io.Writer) error {
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
		return errors.NewError("error retrieving router service").WithCause(err).WithDetails(h.OriginLog())
	}

	componentName := "install-router"
	imageRunHelper := run.NewRunHelper(dockerhelper.NewHelper(dockerClient)).New()
	glog.Infof("Running %q", componentName)

	// Create service account for router
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = RouterServiceAccountName
	_, err = kubeClient.Core().ServiceAccounts("default").Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create router service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add router SA to privileged SCC
	securityClient, err := securityclientinternal.NewForConfig(restConfig)
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
			CertFile:   filepath.Join(configDir, "ca.crt"),
			KeyFile:    filepath.Join(configDir, "ca.key"),
			SerialFile: filepath.Join(configDir, "ca.serial.txt"),
		},
		Overwrite: true,
		Hostnames: []string{
			routingSuffix,
			// This will ensure that routes using edge termination and the default
			// certs will use certs valid for their arbitrary subdomain names.
			fmt.Sprintf("*.%s", routingSuffix),
		},
		CertFile: filepath.Join(configDir, "router.crt"),
		KeyFile:  filepath.Join(configDir, "router.key"),
		Output:   cmdOutput,
	}
	_, err = createCertOptions.CreateServerCert()
	if err != nil {
		return errors.NewError("cannot create router cert").WithCause(err)
	}

	err = catFiles(filepath.Join(configDir, "router.pem"),
		filepath.Join(configDir, "router.crt"),
		filepath.Join(configDir, "router.key"),
		filepath.Join(configDir, "ca.crt"))
	if err != nil {
		return errors.NewError("cannot create aggregate router cert").WithCause(err)
	}

	routerCertPath := configDir + "/router.pem"
	flags := []string{
		"adm", "router",
		"--host-ports=true",
		"--loglevel=8",
		"--config=" + configDir + "/admin.kubeconfig",
		fmt.Sprintf("--host-network=%v", !portForwarding),
		fmt.Sprintf("--images=%s", images),
		fmt.Sprintf("--default-cert=%s", routerCertPath),
	}
	_, stdout, stderr, rc, err := imageRunHelper.Image(ocImage).
		Privileged().
		DiscardContainer().
		HostNetwork().
		Bind(configDir + ":" + configDir).
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
