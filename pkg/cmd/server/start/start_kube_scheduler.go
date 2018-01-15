package start

import (
	"github.com/golang/glog"
	"github.com/spf13/pflag"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	schedulerapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"

	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func newScheduler(kubeconfigFile, schedulerConfigFile string, schedulerArgs map[string][]string) (*schedulerapp.Options, error) {
	cmdLineArgs := map[string][]string{}
	// deep-copy the input args to avoid mutation conflict.
	for k, v := range schedulerArgs {
		cmdLineArgs[k] = append([]string{}, v...)
	}
	if len(cmdLineArgs["kubeconfig"]) == 0 {
		cmdLineArgs["kubeconfig"] = []string{kubeconfigFile}
	}
	if len(cmdLineArgs["policy-config-file"]) == 0 {
		cmdLineArgs["policy-config-file"] = []string{schedulerConfigFile}
	}
	if _, ok := cmdLineArgs["leader-elect"]; !ok {
		cmdLineArgs["leader-elect"] = []string{"true"}
	}
	if len(cmdLineArgs["leader-elect-resource-lock"]) == 0 {
		cmdLineArgs["leader-elect-resource-lock"] = []string{"configmaps"}
	}

	// disable serving http since we didn't used to expose it
	if len(cmdLineArgs["port"]) == 0 {
		cmdLineArgs["port"] = []string{"-1"}
	}

	// resolve arguments
	schedulerOptions, err := schedulerapp.NewOptions()
	if err != nil {
		return nil, err
	}
	if err := schedulerOptions.ReallyApplyDefaults(); err != nil {
		return nil, err
	}
	if err := cmdflags.Resolve(cmdLineArgs, func(fs *pflag.FlagSet) {
		schedulerapp.AddFlags(schedulerOptions, fs)
	}); len(err) > 0 {
		return nil, kerrors.NewAggregate(err)
	}
	if err := schedulerOptions.Complete(); err != nil {
		return nil, err
	}

	return schedulerOptions, nil
}

func runEmbeddedScheduler(kubeconfigFile, schedulerConfigFile string, cmdLineArgs map[string][]string) {
	// TODO we need a real identity for this.  Right now it's just using the loopback connection like it used to.
	schedulerOptions, err := newScheduler(kubeconfigFile, schedulerConfigFile, cmdLineArgs)
	if err != nil {
		glog.Fatal(err)
	}
	// this does a second leader election, but doing the second leader election will allow us to move out process in
	// 3.8 if we so choose.
	if err := schedulerOptions.Run(); err != nil {
		glog.Fatal(err)
	}
}
