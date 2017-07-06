package openshift

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/golang/glog"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/authentication/serviceaccount"
	kapi "k8s.io/kubernetes/pkg/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"github.com/openshift/origin/pkg/bootstrap/docker/errors"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/admin/policy"
	"github.com/openshift/origin/pkg/cmd/server/admin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	"github.com/openshift/origin/pkg/security/legacyclient"
)

const (
	DefaultNamespace  = "default"
	SvcDockerRegistry = "docker-registry"
	SvcRouter         = "router"
	masterConfigDir   = "/var/lib/origin/openshift.local.config/master"
	RegistryServiceIP = "172.30.1.1"
	routerCertPath    = masterConfigDir + "/router.pem"
)

// InstallRegistry checks whether a registry is installed and installs one if not already installed
func (h *Helper) InstallRegistry(kubeClient kclientset.Interface, f *clientcmd.Factory, configDir, images, pvDir string, out, errout io.Writer) error {
	_, err := kubeClient.Core().Services(DefaultNamespace).Get(SvcDockerRegistry, metav1.GetOptions{})
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

	// Obtain registry markup. The reason it is not created outright is because
	// we need to modify the ClusterIP of the registry service. The command doesn't
	// have an option to set it.
	registryJSON, stdErr, err := h.execHelper.Command("oc", "adm", "registry",
		"--dry-run",
		"--output=json",
		fmt.Sprintf("--images=%s", images),
		fmt.Sprintf("--mount-host=%s", path.Join(pvDir, "registry"))).Output()

	if err != nil {
		return errors.NewError("cannot generate registry resources").WithCause(err).WithDetails(stdErr)
	}

	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), []byte(registryJSON))
	if err != nil {
		return errors.NewError("cannot decode registry JSON output").WithCause(err).WithDetails(registryJSON)
	}
	objList := obj.(*kapi.List)

	if errs := runtime.DecodeList(objList.Items, kapi.Codecs.UniversalDecoder()); len(errs) > 0 {
		return errors.NewError("cannot decode registry objects").WithCause(utilerrors.NewAggregate(errs))
	}

	// Update the ClusterIP on the Docker registry service definition
	for _, item := range objList.Items {
		if svc, ok := item.(*kapi.Service); ok {
			svc.Spec.ClusterIP = RegistryServiceIP
		}
	}

	// Create objects
	mapper := clientcmd.ResourceMapper(f)
	bulk := &configcmd.Bulk{
		Mapper: mapper,
		Op:     configcmd.Create,
	}
	if errs := bulk.Run(objList, DefaultNamespace); len(errs) > 0 {
		err = utilerrors.NewAggregate(errs)
		return errors.NewError("cannot create registry objects").WithCause(err)
	}
	return nil
}

// InstallRouter installs a default router on the OpenShift server
func (h *Helper) InstallRouter(kubeClient kclientset.Interface, f *clientcmd.Factory, configDir, images, hostIP string, portForwarding bool, out, errout io.Writer) error {
	_, err := kubeClient.Core().Services(DefaultNamespace).Get(SvcRouter, metav1.GetOptions{})
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
	privilegedSCC, err := legacyclient.NewFromClient(kubeClient.Core().RESTClient()).Get("privileged", metav1.GetOptions{})
	if err != nil {
		return errors.NewError("cannot retrieve privileged SCC").WithCause(err).WithDetails(h.OriginLog())
	}
	privilegedSCC.Users = append(privilegedSCC.Users, serviceaccount.MakeUsername("default", "router"))
	_, err = legacyclient.NewFromClient(kubeClient.Core().RESTClient()).Update(privilegedSCC)
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

	err = h.hostHelper.UploadFileToContainer(filepath.Join(masterDir, "router.pem"), routerCertPath)
	if err != nil {
		return errors.NewError("cannot upload router cert to origin container").WithCause(err)
	}

	_, stdErr, err := h.execHelper.Command("oc", "adm", "router",
		"--host-ports=true",
		fmt.Sprintf("--host-network=%v", !portForwarding),
		fmt.Sprintf("--images=%s", images),
		fmt.Sprintf("--default-cert=%s", routerCertPath)).Output()

	if err != nil {
		// In origin v1.3.1, the 'oc adm router' command exits with an error
		// about an existing router service account. However, the router is
		// created successfully.
		if strings.Contains(stdErr, "error: serviceaccounts \"router\" already exists") {
			glog.V(2).Infof("ignoring error about existing router service account")
		} else {
			return errors.NewError("error creating router").WithCause(err)
		}
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
		SCCInterface: legacyclient.NewFromClient(kubeClient.Core().RESTClient()),
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
