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
	"github.com/openshift/origin/pkg/cmd/server/origin/node"
)

func main() {
	logs.InitLogs()
	defer logs.FlushLogs()

	rand.Seed(time.Now().UTC().UnixNano())

	var configFile string

	cmd := &cobra.Command{
		Use: "openshift-node-config",
		Long: heredoc.Doc(`
			Generate Kubelet configuration from node-config.yaml

			This command converts an existing OpenShift node configuration into the appropriate
			Kubelet command-line flags.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			configapi.AddToScheme(configapi.Scheme)
			configapiv1.AddToScheme(configapi.Scheme)

			if len(configFile) == 0 {
				return fmt.Errorf("you must specify a --config file to read")
			}
			nodeConfig, err := configapilatest.ReadAndResolveNodeConfig(configFile)
			if err != nil {
				return fmt.Errorf("unable to read node config: %v", err)
			}
			if glog.V(2) {
				out, _ := yaml.Marshal(nodeConfig)
				glog.V(2).Infof("Node config:\n%s", out)
			}
			return node.WriteKubeletFlags(*nodeConfig)
		},
		SilenceUsage: true,
	}
	cmd.Flags().StringVar(&configFile, "config", "", "The config file to convert to Kubelet arguments.")
	flagtypes.GLog(cmd.PersistentFlags())

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
