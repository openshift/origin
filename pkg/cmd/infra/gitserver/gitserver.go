package gitserver

import (
	"flag"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/gitserver"
	"github.com/openshift/origin/pkg/gitserver/autobuild"
)

const LogLevelEnv = "LOGLEVEL"

var (
	longCommandDesc = templates.LongDesc(`
		Start a Git server

		This command launches a Git HTTP/HTTPS server that supports push and pull, mirroring,
		and automatic creation of applications on push.

		%[1]s`)

	repositoryBuildConfigsDesc = templates.LongDesc(`
		Retrieve build configs for a gitserver repository

		This command lists build configurations in the current namespace that correspond to a given git repository.`)
)

// CommandFor returns gitrepo-buildconfigs command or gitserver command
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	out := os.Stdout

	setLogLevel()

	switch basename {
	case "gitrepo-buildconfigs":
		cmd = NewCommandRepositoryBuildConfigs(basename, out)
	default:
		cmd = NewCommandGitServer("gitserver")
	}
	return cmd
}

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
	config, err := gitserver.NewEnvironmentConfig()
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

func NewCommandRepositoryBuildConfigs(name string, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s REPOSITORY_NAME", name),
		Short: "Retrieve build configs for a gitserver repository",
		Long:  repositoryBuildConfigsDesc,
		Run: func(c *cobra.Command, args []string) {
			if len(args) != 1 {
				err := cmdutil.UsageErrorf(c, "This command takes a single argument - the name of the repository")
				cmdutil.CheckErr(err)
			}
			repoName := args[0]
			client, err := gitserver.GetClient()
			cmdutil.CheckErr(err)
			err = gitserver.GetRepositoryBuildConfigs(client, repoName, out)
			cmdutil.CheckErr(err)
		},
	}
	return cmd
}

func setLogLevel() {
	logLevel := os.Getenv(LogLevelEnv)
	if len(logLevel) > 0 {
		if flag.CommandLine.Lookup("v") != nil {
			flag.CommandLine.Set("v", logLevel)
		}
	}
}
