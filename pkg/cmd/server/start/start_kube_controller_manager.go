package start

import (
	"fmt"
	"strconv"

	"github.com/golang/glog"

	controllerapp "k8s.io/kubernetes/cmd/kube-controller-manager/app"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

func computeKubeControllerManagerArgs(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile string, dynamicProvisioningEnabled bool, qps float32, burst int) []string {
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
	if _, ok := cmdLineArgs["kube-api-content-type"]; !ok {
		cmdLineArgs["kube-api-content-type"] = []string{"application/vnd.kubernetes.protobuf"}
	}
	if _, ok := cmdLineArgs["kube-api-qps"]; !ok {
		cmdLineArgs["kube-api-qps"] = []string{fmt.Sprintf("%v", qps)}
	}
	if _, ok := cmdLineArgs["kube-api-burst"]; !ok {
		cmdLineArgs["kube-api-burst"] = []string{fmt.Sprintf("%v", burst)}
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

	args := []string{}
	for key, value := range cmdLineArgs {
		for _, token := range value {
			args = append(args, fmt.Sprintf("--%s=%v", key, token))
		}
	}
	return args
}

func runEmbeddedKubeControllerManager(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile string, dynamicProvisioningEnabled bool, qps float32, burst int) {
	cmd := controllerapp.NewControllerManagerCommand()
	args := computeKubeControllerManagerArgs(kubeconfigFile, saPrivateKeyFile, saRootCAFile, podEvictionTimeout, openshiftConfigFile, dynamicProvisioningEnabled, qps, burst)
	if err := cmd.ParseFlags(args); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("`kube-controller-manager %v`", args)
	cmd.Run(nil, nil)
	panic(fmt.Sprintf("`kube-controller-manager %v` exited", args))
}
