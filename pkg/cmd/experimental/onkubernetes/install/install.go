package install

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kubeclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	klatest "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api/latest"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/wait"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// InstallRecommendedCommandName is the recommended command name
const InstallRecommendedCommandName = "install"

const (
	installLong = `
Installs OpenShift on a Kubernetes cluster.

It installs the following:
 * etcd discovery
 * etcd 
 * openshift master`
)

const (
	openshiftName     = "openshift"
	etcdDiscoveryName = "etcd-discovery"
	etcdName          = "etcd"
	// TODO: pass as flag, generate it, hold it in a secret?
	etcdClusterToken   = "hw6yi4w4x4mlih44acymeq1yh53uimem463rm0kd"
	etcdDiscoveryToken = "etcd-cluster-kxzol"
)

// NewCmdInstall installs OpenShift on a Kubernetes cluster.
func NewCmdInstall(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewDefaultOptions(out)
	cmd := &cobra.Command{
		Use:   name,
		Short: "Install OpenShift on a Kubernetes cluster.",
		Long:  installLong,
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, c, args, out))
			kcmdutil.CheckErr(options.Validate(args))
			if err := options.Install(); err != nil {
				if kerrors.IsInvalid(err) {
					if details := err.(*kerrors.StatusError).ErrStatus.Details; details != nil {
						fmt.Fprintf(c.Out(), "error: Invalid %s %s\n", details.Kind, details.Name)
						for _, cause := range details.Causes {
							fmt.Fprintf(c.Out(), "  %s: %s\n", cause.Field, cause.Message)
						}
						os.Exit(255)
					}
				}
				glog.Fatalf("Installation of OpenShift could not complete: %v", err)
			}
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&options.ServiceAccountPublicKeyFilePath, "service-account-public-key", "", "", "")
	start.BindKubeConnectionArgs(options.KubeConnectionArgs, flags, "")
	return cmd
}

// Options describes how OpenShift is installed on Kubernetes.
type Options struct {
	// Location to output results from command
	Output io.Writer
	// How to connect to the kubernetes cluster
	KubeConnectionArgs *start.KubeConnectionArgs
	// Namespace to create and provision OpenShift within.
	Namespace string
	// The public key file to use for service accounts.
	ServiceAccountPublicKeyFilePath string
}

// NewDefaultOptions creates an options object with default values.
func NewDefaultOptions(out io.Writer) *Options {
	return &Options{
		Output:             out,
		KubeConnectionArgs: start.NewDefaultKubeConnectionArgs(),
	}
}

// Validate validates install options.
func (o *Options) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported for install")
	}
	if len(o.Namespace) == 0 {
		return fmt.Errorf("namespace must be known")
	}
	if len(o.ServiceAccountPublicKeyFilePath) == 0 {
		return fmt.Errorf("service account public key file path must be specified")
	}
	return nil
}

// Complete finishes configuration of install options.
func (o *Options) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string, out io.Writer) error {
	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = namespace
	return nil
}

// buildKubernetesClient returns a Kubernetes client.
func (o *Options) buildKubernetesClient() (kubeclient.Interface, error) {
	config, err := o.KubeConnectionArgs.ClientConfig.ClientConfig()
	if err != nil {
		return nil, err
	}
	client, err := kubeclient.New(config)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// createEtcdClusterIfNotFound creates the etcd resources if they do not yet exist.
func (o *Options) createEtcdClusterIfNotFound(kubeClient kubeclient.Interface) error {
	if err := createServiceIfNotFound(o.Output, kubeClient.Services(o.Namespace), newEtcdDiscoveryService(etcdDiscoveryName)); err != nil {
		return err
	}
	if err := createReplicationControllerIfNotFound(o.Output, kubeClient.ReplicationControllers(o.Namespace), newEtcdDiscoveryController(etcdDiscoveryName)); err != nil {
		return err
	}
	if err := createServiceIfNotFound(o.Output, kubeClient.Services(o.Namespace), newEtcdService(etcdName)); err != nil {
		return err
	}
	if err := createReplicationControllerIfNotFound(o.Output, kubeClient.ReplicationControllers(o.Namespace), newEtcdController(etcdName, 1, etcdClusterToken, etcdDiscoveryToken)); err != nil {
		return err
	}
	return nil
}

// writeOpenShiftConfig will generate configuration for openshift using the specified public ip.
// it returns the location of the directory it output the generated config, or an error if it was unable to succeed.
func (o *Options) writeOpenShiftConfig(configDir, openshiftPublicIP string) error {
	// we need to minify, flatten, and merge the provided kubeconfig into a standalone file for ingestion by OpenShift.
	mergeFlag := util.BoolFlag{}
	mergeFlag.Set("true")
	viewOptions := &kcmdconfig.ViewOptions{
		ConfigAccess: kcmdconfig.NewDefaultPathOptions(),
		Merge:        mergeFlag,
		Flatten:      true,
		Minify:       true,
	}
	printer, _, err := kubectl.GetPrinter("yaml", "")
	if err != nil {
		return err
	}
	printer = kubectl.NewVersionedPrinter(printer, kclientcmdapi.Scheme, klatest.ExternalVersion)
	buffer := bytes.NewBuffer([]byte{})
	err = viewOptions.Run(buffer, printer)
	if err != nil {
		return err
	}
	kubeConfigFile := configDir + "/kubeconfig"
	err = ioutil.WriteFile(kubeConfigFile, buffer.Bytes(), 0644)
	if err != nil {
		return err
	}

	// generate configuration for OpenShift
	startMasterOptions := &start.MasterOptions{Output: ioutil.Discard, CreateCertificates: true}
	startMasterOptions.MasterArgs = start.NewDefaultMasterArgs()

	masterAddr := flagtypes.Addr{}
	masterAddr.Set("https://localhost:8443")
	startMasterOptions.MasterArgs.MasterAddr = masterAddr

	etcdAddr := flagtypes.Addr{}
	etcdAddr.Set("http://etcd:2379")
	startMasterOptions.MasterArgs.EtcdAddr = etcdAddr

	masterPublicAddr := flagtypes.Addr{}
	masterPublicAddr.Set("https://" + openshiftPublicIP + ":8443")
	startMasterOptions.MasterArgs.MasterPublicAddr = masterPublicAddr

	kubeConnectionArgs := start.NewDefaultKubeConnectionArgs()
	kubeConnectionArgs.ClientConfigLoadingRules.ExplicitPath = kubeConfigFile
	startMasterOptions.MasterArgs.KubeConnectionArgs = kubeConnectionArgs

	writeConfig := util.NewStringFlag("")
	writeConfig.Set(configDir)
	startMasterOptions.MasterArgs.ConfigDir = &writeConfig
	if err = startMasterOptions.MasterArgs.Validate(); err != nil {
		return err
	}
	if err = startMasterOptions.RunMaster(); err != nil {
		return err
	}

	// copy the service account public key into our config dir
	serviceAccountPublicKeyFile := "serviceaccounts.public.key"
	serviceAccountPublicKeyFilePath := configDir + "/" + serviceAccountPublicKeyFile
	err = copyFile(o.ServiceAccountPublicKeyFilePath, serviceAccountPublicKeyFilePath)
	if err != nil {
		return err
	}

	// fixup the master-config to use our service account public key
	masterConfigFile := configDir + "/master-config.yaml"
	masterConfig, err := latest.ReadMasterConfig(masterConfigFile)
	if err != nil {
		return err
	}

	masterConfig.ServiceAccountConfig.PublicKeyFiles = []string{serviceAccountPublicKeyFile}
	data, err := latest.WriteYAML(masterConfig)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(masterConfigFile, data, 0644)
	if err != nil {
		return err
	}
	return nil
}

// createOpenShiftIfNotFound creates the openshift resources if they do not yet exist.
func (o *Options) createOpenShiftIfNotFound(kubeClient kubeclient.Interface) error {
	if err := createServiceIfNotFound(o.Output, kubeClient.Services(o.Namespace), newOpenShiftService(openshiftName)); err != nil {
		return err
	}

	// wait for the public IP to be assigned by Kubernetes.
	openshiftPublicIP := ""
	poll := 2 * time.Second
	timeout := 2 * time.Minute
	fmt.Fprintf(o.Output, "-- Detecting public IP for service %s ", openshiftName)
	wait.Poll(poll, timeout, func() (bool, error) {
		serviceObj, err := kubeClient.Services(o.Namespace).Get(openshiftName)
		if err != nil {
			return false, err
		}
		for _, ingress := range serviceObj.Status.LoadBalancer.Ingress {
			if len(ingress.IP) > 0 {
				openshiftPublicIP = ingress.IP
				return true, nil
			}
		}
		fmt.Fprintf(o.Output, ".")
		return false, nil
	})
	if len(openshiftPublicIP) == 0 {
		return fmt.Errorf("Unable to find a public IP for service: %v", openshiftName)
	}
	fmt.Fprintf(o.Output, " OK\n")

	// we create a temp directory locally to hold content we generate.
	// the calling function is responsible for cleaning this dir up after bundling the secret
	configDir, err := ioutil.TempDir("", "openshift-config")
	if err != nil {
		return err
	}
	defer os.RemoveAll(configDir)
	err = o.writeOpenShiftConfig(configDir, openshiftPublicIP)
	if err != nil {
		return err
	}
	secretGenerator := kubectl.SecretGeneratorV1{
		Name:        openshiftName,
		FileSources: []string{configDir},
	}
	runtimeObj, err := secretGenerator.StructuredGenerate()
	if err != nil {
		return err
	}
	secretObj := runtimeObj.(*kapi.Secret)
	if err := createSecretIfNotFound(o.Output, kubeClient.Secrets(o.Namespace), secretObj); err != nil {
		return err
	}

	// run openshift
	if err := createReplicationControllerIfNotFound(o.Output, kubeClient.ReplicationControllers(o.Namespace), newOpenShiftController(openshiftName)); err != nil {
		return err
	}

	fmt.Fprintf(o.Output, "-- Server Information ...\n")
	fmt.Fprintf(o.Output, "   OpenShift server starting...\n")
	fmt.Fprintf(o.Output, "   The server is accessible via web console at:\n")
	fmt.Fprintf(o.Output, "   https://%s:8443\n", openshiftPublicIP)

	return nil
}

// Install installs OpenShift on Kubernetes.
func (o *Options) Install() error {
	kubeClient, err := o.buildKubernetesClient()
	if err != nil {
		return err
	}
	namespace := &kapi.Namespace{
		ObjectMeta: kapi.ObjectMeta{
			Name: o.Namespace,
		},
	}
	if err = createNamespaceIfNotFound(o.Output, kubeClient, namespace); err != nil {
		return err
	}
	if err = o.createEtcdClusterIfNotFound(kubeClient); err != nil {
		return err
	}
	if err = o.createOpenShiftIfNotFound(kubeClient); err != nil {
		return err
	}
	return nil
}
