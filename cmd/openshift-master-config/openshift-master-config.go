package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apiserver/pkg/util/logs"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	configapiv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"regexp"
	"strconv"
	"strings"
)

// safeArgRegexp matches only characters that are known safe. DO NOT add to this list
// without fully considering whether that new character can be used to break shell escaping
// rules.
var safeArgRegexp = regexp.MustCompile(`^[\da-zA-Z\-=_\.,/\:]+$`)

// shellEscapeArg quotes an argument if it contains characters that my cause a shell
// interpreter to split the single argument into multiple.
func shellEscapeArg(s string) string {
	if safeArgRegexp.MatchString(s) {
		return s
	}
	return strconv.Quote(s)
}

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	rand.Seed(time.Now().UTC().UnixNano())

	var configFile string
	// As of now, they are mutually exclusive as I don't see any reason for having all of them.
	var getschedulerArgs, controllerArgs, apiServerArgs bool

	cmd := &cobra.Command{
		Use: "openshift-master-config",
		Long: heredoc.Doc(`
			Generate master configuration from master-config.yaml

			This command converts an existing OpenShift master configuration into the appropriate component specific
			command-line flags.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			configapi.AddToScheme(configapi.Scheme)
			configapiv1.AddToScheme(configapi.Scheme)

			if len(configFile) == 0 {
				return fmt.Errorf("you must specify a --config file to read")
			}
			masterConfig, err := configapilatest.ReadAndResolveMasterConfig(configFile)
			if err != nil {
				return fmt.Errorf("unable to read master config: %v", err)
			}

			if glog.V(2) {
				out, _ := yaml.Marshal(masterConfig)
				glog.V(2).Infof("Master config:\n%s", out)
			}
			if getschedulerArgs {
				return writeSchedulerArgs(*masterConfig)
			}
			// Return controllers and api-server arguments.
			return fmt.Errorf("Expected one of scheduler, api-server or controllers flags to be set.")
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&configFile, "config", "", "The config file to convert to master arguments.")
	cmd.Flags().BoolVar(&getschedulerArgs, "scheduler", false, "Gets the scheduler arguments")
	cmd.Flags().BoolVar(&controllerArgs, "controllers", false, "Gets the controller arguments")
	cmd.Flags().BoolVar(&apiServerArgs, "api-server", false, "Gets the api-server arguments")
	flagtypes.GLog(cmd.PersistentFlags())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}

// writeSchedulerArgs returns scheduler arguments to be written.
func writeSchedulerArgs(masterConfig configapi.MasterConfig) error {
	privilegedLoopbackConfig, err := configapi.GetClientConfig(masterConfig.MasterClients.OpenShiftLoopbackKubeConfig, masterConfig.MasterClients.OpenShiftLoopbackClientConnectionOverrides)
	if err != nil {
		return err
	}
	// Build scheduler args.
	schedulerArgs := start.ComputeSchedulerArgs(masterConfig.MasterClients.OpenShiftLoopbackKubeConfig,
		masterConfig.KubernetesMasterConfig.SchedulerConfigFile, privilegedLoopbackConfig.QPS,
		privilegedLoopbackConfig.Burst,
		masterConfig.KubernetesMasterConfig.SchedulerArguments)
	var outputArgs []string
	for _, s := range schedulerArgs {
		outputArgs = append(outputArgs, shellEscapeArg(s))
	}
	fmt.Println(strings.Join(outputArgs, " "))
	return nil
}
