package start

import (
	"strconv"

	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	controlleroptions "k8s.io/kubernetes/cmd/kube-controller-manager/app/options"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func kubeControllerManagerAddFlags(cmserver *controlleroptions.CMServer) func(flags *pflag.FlagSet) {
	return func(flags *pflag.FlagSet) {
		cmserver.AddFlags(flags, controllerapp.KnownControllers(), controllerapp.ControllersDisabledByDefault.List())
	}
}

func newKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile string, dynamicProvisioningEnabled bool) (*controlleroptions.CMServer, error) {
	cmdLineArgs := map[string][]string{}
	if _, ok := cmdLineArgs["controllers"]; !ok {
		cmdLineArgs["controllers"] = []string{
			"*", // start everything but the exceptions}
			// not used in openshift
			"-ttl",
			"-bootstrapsigner",
			"-tokencleaner",
			// we have to configure this separately until it is generic
			"-horizontalpodautoscaling",
			// we carry patches on this. For now....
			"-serviceaccount-token",
		}
	}
	if _, ok := cmdLineArgs["service-account-private-key-file"]; !ok {
		cmdLineArgs["service-account-private-key-file"] = []string{saPrivateKeyFile}
	}
	if _, ok := cmdLineArgs["root-ca-file"]; !ok {
		cmdLineArgs["root-ca-file"] = []string{saRootCAFile}
	}
	if _, ok := cmdLineArgs["kubeconfig"]; !ok {
		cmdLineArgs["kubeconfig"] = []string{kubeconfigFile}
	}
	if _, ok := cmdLineArgs["pod-eviction-timeout"]; !ok {
		cmdLineArgs["pod-eviction-timeout"] = []string{podEvictionTimeout}
	}
	if _, ok := cmdLineArgs["enable-dynamic-provisioning"]; !ok {
		cmdLineArgs["enable-dynamic-provisioning"] = []string{strconv.FormatBool(dynamicProvisioningEnabled)}
	}

	// disable serving http since we didn't used to expose it
	if _, ok := cmdLineArgs["port"]; !ok {
		cmdLineArgs["port"] = []string{"-1"}
	}

	// these force "default" values to match what we want
	if _, ok := cmdLineArgs["use-service-account-credentials"]; !ok {
		cmdLineArgs["use-service-account-credentials"] = []string{"true"}
	}
	if _, ok := cmdLineArgs["cluster-signing-cert-file"]; !ok {
		cmdLineArgs["cluster-signing-cert-file"] = []string{""}
	}
	if _, ok := cmdLineArgs["cluster-signing-key-file"]; !ok {
		cmdLineArgs["cluster-signing-key-file"] = []string{""}
	}
	if _, ok := cmdLineArgs["experimental-cluster-signing-duration"]; !ok {
		cmdLineArgs["experimental-cluster-signing-duration"] = []string{"720h"}
	}
	if _, ok := cmdLineArgs["leader-elect-retry-period"]; !ok {
		cmdLineArgs["leader-elect-retry-period"] = []string{"3s"}
	}
	if _, ok := cmdLineArgs["leader-elect-resource-lock"]; !ok {
		cmdLineArgs["leader-elect-resource-lock"] = []string{"configmaps"}
	}
	cmdLineArgs["openshift-config"] = []string{openshiftConfigFile}

	// resolve arguments
	controllerManager := controlleroptions.NewCMServer()
	if err := cmdflags.Resolve(cmdLineArgs, kubeControllerManagerAddFlags(controllerManager)); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}

	return controllerManager, nil
}

func runEmbeddedKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile string, dynamicProvisioningEnabled bool) {
	// TODO we need a real identity for this.  Right now it's just using the loopback connection like it used to.
	controllerManager, err := newKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile, dynamicProvisioningEnabled)
	if err != nil {
		glog.Fatal(err)
	}
	if err := controllerapp.Run(controllerManager); err != nil {
		glog.Fatal(err)
	}
}
