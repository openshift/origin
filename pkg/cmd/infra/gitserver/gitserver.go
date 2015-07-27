package gitserver

import (
	"fmt"
	"log"
	"net/url"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/gitserver"
	"github.com/openshift/origin/pkg/gitserver/autobuild"
)

const longCommandDesc = `
Start a Git server

This command launches a Git HTTP/HTTPS server that supports push and pull, mirroring,
and automatic creation of applications on push.

%[1]s
`

// NewCommandGitServer launches a Git server
func NewCommandGitServer(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Start a Git server",
		Long:  fmt.Sprintf(longCommandDesc, gitserver.EnvironmentHelp),
		Run: func(c *cobra.Command, args []string) {
			err := RunGitServer()
			cmdutil.CheckErr(err)
		},
	}

	return cmd
}

func RunGitServer() error {
	config, err := gitserver.NewEnviromentConfig()
	if err != nil {
		return err
	}
	link, err := autobuild.NewAutoLinkBuildsFromEnvironment()
	switch {
	case err == autobuild.ErrNotEnabled:
	case err != nil:
		log.Fatal(err)
	default:
		link.LinkFn = func(name string) *url.URL { return gitserver.RepositoryURL(config, name, nil) }
		clones, err := link.Link()
		if err != nil {
			log.Printf("error: %v", err)
			break
		}
		for name, v := range clones {
			config.InitialClones[name] = v
		}
	}
	return gitserver.Start(config)
}
