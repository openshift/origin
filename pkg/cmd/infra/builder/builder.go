package builder

import (
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/build/builder/cmd"
	"github.com/openshift/origin/pkg/version"
)

const (
	stiBuilderLong = `
Perform a Source-to-Image build

This command executes a Source-to-Image build using arguments passed via the environment.
It expects to be run inside of a container.`

	dockerBuilderLong = `
Perform a Docker build

This command executes a Docker build using arguments passed via the environment.
It expects to be run inside of a container.`
)

// NewCommandSTIBuilder provides a CLI handler for STI build type
func NewCommandSTIBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Source-to-Images build",
		Long:  stiBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			go func() {
				for {
					sigs := make(chan os.Signal, 1)
					signal.Notify(sigs, syscall.SIGQUIT)
					buf := make([]byte, 1<<20)
					for {
						<-sigs
						runtime.Stack(buf, true)
						glog.Infof("=== received SIGQUIT ===\n*** goroutine dump...\n%s\n*** end\n", buf)
					}
				}
			}()
			cmd.RunSTIBuild()
		},
	}

	cmd.AddCommand(version.NewVersionCommand(name, false))
	return cmd
}

// NewCommandDockerBuilder provides a CLI handler for Docker build type
func NewCommandDockerBuilder(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Run a Docker build",
		Long:  dockerBuilderLong,
		Run: func(c *cobra.Command, args []string) {
			cmd.RunDockerBuild()
		},
	}
	cmd.AddCommand(version.NewVersionCommand(name, false))
	return cmd
}
