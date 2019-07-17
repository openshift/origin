package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	utillog "github.com/openshift/source-to-image/pkg/util/log"
	"github.com/spf13/cobra"

	"github.com/openshift/source-to-image/pkg/api"
	"github.com/openshift/source-to-image/pkg/cmd/cli/cmd"
	cmdutil "github.com/openshift/source-to-image/pkg/cmd/cli/util"
	"github.com/openshift/source-to-image/pkg/docker"
)

// log is a placeholder until the builders pass an output stream down
// client facing libraries should not be using log
var log = utillog.StderrLog

// NewCmdCLI implements the S2I command line functionality.
func NewCmdCLI() *cobra.Command {
	cfg := &api.Config{}
	s2iCmd := &cobra.Command{
		Use: "s2i",
		Long: "Source-to-image (S2I) is a tool for building repeatable docker images.\n\n" +
			"A command line interface that injects and assembles source code into a docker image.\n" +
			"Complete documentation is available at http://github.com/openshift/source-to-image",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
		},
	}
	cfg.DockerConfig = docker.GetDefaultDockerConfig()
	s2iCmd.PersistentFlags().StringVarP(&(cfg.DockerConfig.Endpoint), "url", "U", cfg.DockerConfig.Endpoint, "Set the url of the docker socket to use")
	s2iCmd.PersistentFlags().StringVar(&(cfg.DockerConfig.CertFile), "cert", cfg.DockerConfig.CertFile, "Set the path of the docker TLS certificate file")
	s2iCmd.PersistentFlags().StringVar(&(cfg.DockerConfig.KeyFile), "key", cfg.DockerConfig.KeyFile, "Set the path of the docker TLS key file")
	s2iCmd.PersistentFlags().StringVar(&(cfg.DockerConfig.CAFile), "ca", cfg.DockerConfig.CAFile, "Set the path of the docker TLS ca file")
	s2iCmd.PersistentFlags().BoolVar(&(cfg.DockerConfig.UseTLS), "tls", cfg.DockerConfig.UseTLS, "Use TLS to connect to docker; implied by --tlsverify")
	s2iCmd.PersistentFlags().BoolVar(&(cfg.DockerConfig.TLSVerify), "tlsverify", cfg.DockerConfig.TLSVerify, "Use TLS to connect to docker and verify the remote")
	s2iCmd.AddCommand(cmd.NewCmdVersion())
	s2iCmd.AddCommand(cmd.NewCmdBuild(cfg))
	s2iCmd.AddCommand(cmd.NewCmdRebuild(cfg))
	s2iCmd.AddCommand(cmd.NewCmdUsage(cfg))
	s2iCmd.AddCommand(cmd.NewCmdCreate())
	cmdutil.SetupLogger(s2iCmd.PersistentFlags())
	basename := filepath.Base(os.Args[0])
	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}
	if basename == "sti" {
		log.Warning("sti binary is deprecated, use s2i instead")
	}

	s2iCmd.AddCommand(cmd.NewCmdCompletion(s2iCmd))

	return s2iCmd
}

// CommandFor returns the appropriate command for this base name,
// or the OpenShift CLI command.
func CommandFor() *cobra.Command {
	return NewCmdCLI()
}
