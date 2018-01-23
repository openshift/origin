package start

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	schedulerapp "k8s.io/kubernetes/plugin/cmd/kube-scheduler/app"
	_ "k8s.io/kubernetes/plugin/pkg/scheduler/algorithmprovider"
)

func computeSchedulerArgs(kubeconfigFile, schedulerConfigFile string, schedulerArgs map[string][]string) []string {
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

	args := []string{}
	for key, value := range cmdLineArgs {
		for _, token := range value {
			args = append(args, fmt.Sprintf("--%s=%v", key, token))
		}
	}
	return args
}

func runEmbeddedScheduler(kubeconfigFile, schedulerConfigFile string, cmdLineArgs map[string][]string) {
	cmd := schedulerapp.NewSchedulerCommand()
	args := computeSchedulerArgs(kubeconfigFile, schedulerConfigFile, cmdLineArgs)
	if err := cmd.ParseFlags(args); err != nil {
		glog.Fatal(err)
	}
	glog.Infof("`kube-scheduler %v`", args)
	cmd.Run(nil, nil)
	glog.Fatalf("`kube-scheduler %v` exited", args)
	time.Sleep(10 * time.Second)
}
