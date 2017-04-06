package openshift

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	apierrors "k8s.io/kubernetes/pkg/api/errors"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/serviceaccount"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/admin/registry"
	"github.com/openshift/origin/pkg/cmd/admin/router"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const (
	DefaultNamespace  = "default"
	SvcDockerRegistry = "docker-registry"
	SvcRouter         = "router"
	masterConfigDir   = "/var/lib/origin/openshift.local.config/master"
	RegistryServiceIP = "172.30.1.1"
)

// InstallRegistry checks whether a registry is installed and installs one if not already installed
func (h *Helper) InstallRegistry(kubeClient kclientset.Interface, f *clientcmd.Factory, configDir, images, pvDir string, out, errout io.Writer) error {
	_, err := kubeClient.Core().Services(DefaultNamespace).Get(SvcDockerRegistry)
	if err == nil {
		// If there's no error, the registry already exists
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving docker registry service").WithCause(err).WithDetails(h.OriginLog())
	}

	err = AddSCCToServiceAccount(kubeClient, "privileged", "registry", "default")
	if err != nil {
		return errors.NewError("cannot add privileged SCC to registry service account").WithCause(err).WithDetails(h.OriginLog())
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = images
	opts := &registry.RegistryOptions{
		Config: &registry.RegistryConfig{
			Name:           "registry",
			Type:           "docker-registry",
			ImageTemplate:  imageTemplate,
			Ports:          "5000",
			Replicas:       1,
			Labels:         "docker-registry=default",
			Volume:         "/registry",
			ServiceAccount: "registry",
			HostMount:      path.Join(pvDir, "registry"),
			ClusterIP:      RegistryServiceIP,
		},
	}
	cmd := registry.NewCmdRegistry(f, "", "registry", out, errout)
	output := &bytes.Buffer{}
	err = opts.Complete(f, cmd, output, output, []string{})
	if err != nil {
		return errors.NewError("error completing the registry configuration").WithCause(err)
	}
	err = opts.RunCmdRegistry()
	glog.V(4).Infof("Registry command output:\n%s", output.String())
	if err != nil {
		return errors.NewError("cannot install registry").WithCause(err).WithDetails(h.OriginLog())
	}
	return nil
}

// InstallRouter installs a default router on the OpenShift server
func (h *Helper) InstallRouter(kubeClient kclientset.Interface, f *clientcmd.Factory, configDir, images, hostIP string, portForwarding bool, out, errout io.Writer) error {
	_, err := kubeClient.Core().Services(DefaultNamespace).Get(SvcRouter)
	if err == nil {
		// Router service already exists, nothing to do
		return nil
	}
	if !apierrors.IsNotFound(err) {
		return errors.NewError("error retrieving router service").WithCause(err).WithDetails(h.OriginLog())
	}

	masterDir := filepath.Join(configDir, "master")

	// Create service account for router
	routerSA := &kapi.ServiceAccount{}
	routerSA.Name = "router"
	_, err = kubeClient.Core().ServiceAccounts("default").Create(routerSA)
	if err != nil {
		return errors.NewError("cannot create router service account").WithCause(err).WithDetails(h.OriginLog())
	}

	// Add router SA to privileged SCC
	privilegedSCC, err := kubeClient.Core().SecurityContextConstraints().Get("privileged")
	if err != nil {
		return errors.NewError("cannot retrieve privileged SCC").WithCause(err).WithDetails(h.OriginLog())
	}
	privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername("default", "router"))
	_, err = kubeClient.Core().SecurityContextConstraints().Update(privilegedSCC)
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
		return err
	}

	imageTemplate := variable.NewDefaultImageTemplate()
	imageTemplate.Format = images
	cfg := &router.RouterConfig{
		Name:               "router",
		Type:               "haproxy-router",
		ImageTemplate:      imageTemplate,
		Ports:              "80:80,443:443",
		Replicas:           1,
		Labels:             "router=<name>",
		DefaultCertificate: filepath.Join(masterDir, "router.pem"),
		StatsPort:          1936,
		StatsUsername:      "admin",
		HostNetwork:        !portForwarding,
		HostPorts:          true,
		ServiceAccount:     "router",
	}
	output := &bytes.Buffer{}
	cmd := router.NewCmdRouter(f, "", "router", out, errout)
	cmd.SetOutput(output)
	err = router.RunCmdRouter(f, cmd, output, output, cfg, []string{})
	glog.V(4).Infof("Router command output:\n%s", output.String())
	if err != nil {
		return errors.NewError("cannot install router").WithCause(err).WithDetails(h.OriginLog())
	}
	return nil
}

func AddClusterRole(osClient client.Interface, role, user string) error {
	clusterRoleBindingAccessor := policy.NewClusterRoleBindingAccessor(osClient)
	addClusterReaderRole := policy.RoleModificationOptions{
		RoleName:            role,
		RoleBindingAccessor: clusterRoleBindingAccessor,
		Users:               []string{user},
	}
	return addClusterReaderRole.AddRole()
}

func AddRoleToServiceAccount(osClient client.Interface, role, sa, namespace string) error {
	roleBindingAccessor := policy.NewLocalRoleBindingAccessor(namespace, osClient)
	addRole := policy.RoleModificationOptions{
		RoleName:            role,
		RoleBindingAccessor: roleBindingAccessor,
		Subjects: []kapi.ObjectReference{
			{
				Namespace: namespace,
				Name:      sa,
				Kind:      "ServiceAccount",
			},
		},
	}
	return addRole.AddRole()
}

func AddSCCToServiceAccount(kubeClient kclientset.Interface, scc, sa, namespace string) error {
	modifySCC := policy.SCCModificationOptions{
		SCCName:      scc,
		SCCInterface: kubeClient.Core(),
		Subjects: []kapi.ObjectReference{
			{
				Namespace: namespace,
				Name:      sa,
				Kind:      "ServiceAccount",
			},
		},
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
