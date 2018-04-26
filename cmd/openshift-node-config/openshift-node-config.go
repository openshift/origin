package main

import (
	"fmt"
	"math/rand"
	"os"
	"time"

	"github.com/MakeNowJust/heredoc"
	"github.com/spf13/cobra"

	"k8s.io/apiserver/pkg/util/logs"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
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
			nodeConfig, err := configapilatest.ReadAndResolveNodeConfig(configFile)
			if err != nil {
				return err
			}
			return node.WriteKubeletFlags(*nodeConfig)
		},
	}
	cmd.Flags().StringVar(&configFile, "config", "", "The config file to convert to Kubelet arguments.")

	if err := cmd.RunE(cmd, os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err)
		os.Exit(1)
	}
}
