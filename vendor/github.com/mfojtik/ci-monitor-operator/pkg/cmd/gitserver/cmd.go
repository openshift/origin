package gitserver

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/mfojtik/ci-monitor-operator/pkg/gitserver"
)

func NewGitServer() *cobra.Command {
	repositoryPath := "/repository"
	if repositoryPathEnv := os.Getenv("REPOSITORY_PATH"); len(repositoryPathEnv) > 0 {
		repositoryPath = repositoryPathEnv
	}
	cmd := &cobra.Command{
		Run: func(cmd *cobra.Command, args []string) {
			gitserver.Run(repositoryPath, "0.0.0.0:8080")
		},
	}
	cmd.Use = "gitserver"
	cmd.Short = "Start the OpenShift config history HTTP GIT server"

	return cmd
}
