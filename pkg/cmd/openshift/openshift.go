package openshift

import (
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/admin"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	"github.com/openshift/origin/pkg/cmd/experimental/bundlesecret"
	exipfailover "github.com/openshift/origin/pkg/cmd/experimental/ipfailover"
	exregistry "github.com/openshift/origin/pkg/cmd/experimental/registry"
	exrouter "github.com/openshift/origin/pkg/cmd/experimental/router"
	"github.com/openshift/origin/pkg/cmd/experimental/tokens"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	"github.com/openshift/origin/pkg/cmd/infra/gitserver"
	"github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/server/start/kubernetes"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/version"
)

const openshift_long = `OpenShift Application Platform.

OpenShift helps you build, deploy, and manage your applications. To start an all-in-one server, run:

  $ openshift start &

OpenShift is built around Docker and the Kubernetes cluster container manager.  You must have
Docker installed on this machine to start your server.

Note: This is a beta release of OpenShift and may change significantly.  See
    https://github.com/openshift/origin for the latest information on OpenShift.`

// CommandFor returns the appropriate command for this base name,
// or the global OpenShift command
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}

	switch basename {
	case "openshift-router":
		cmd = router.NewCommandTemplateRouter(basename)
	case "openshift-deploy":
		cmd = deployer.NewCommandDeployer(basename)
	case "openshift-sti-build":
		cmd = builder.NewCommandSTIBuilder(basename)
	case "openshift-docker-build":
		cmd = builder.NewCommandDockerBuilder(basename)
	case "openshift-gitserver":
		cmd = gitserver.NewCommandGitServer(basename)
	case "osc", "os":
		cmd = cli.NewCommandCLI(basename, basename)
	case "osadm", "oadm":
		cmd = admin.NewCommandAdmin(basename, basename, os.Stdout)
	case "kubectl":
		cmd = cli.NewCmdKubectl(basename, os.Stdout)
	case "kube-apiserver":
		cmd = kubernetes.NewAPIServerCommand(basename, basename, os.Stdout)
	case "kube-controller-manager":
		cmd = kubernetes.NewControllersCommand(basename, basename, os.Stdout)
	case "kubelet":
		cmd = kubernetes.NewKubeletCommand(basename, basename, os.Stdout)
	case "kube-proxy":
		cmd = kubernetes.NewProxyCommand(basename, basename, os.Stdout)
	case "kube-scheduler":
		cmd = kubernetes.NewSchedulerCommand(basename, basename, os.Stdout)
	case "kubernetes":
		cmd = kubernetes.NewCommand(basename, basename, os.Stdout)
	case "origin":
		cmd = NewCommandOpenShift("origin")
	default:
		cmd = NewCommandOpenShift("openshift")
	}

	templates.UseMainTemplates(cmd)
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

// NewCommandOpenShift creates the standard OpenShift command
func NewCommandOpenShift(name string) *cobra.Command {
	root := &cobra.Command{
		Use:   name,
		Short: "OpenShift helps you build, deploy, and manage your cloud applications",
		Long:  openshift_long,
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	startAllInOne, _ := start.NewCommandStartAllInOne(name, os.Stdout)
	root.AddCommand(startAllInOne)
	root.AddCommand(admin.NewCommandAdmin("admin", name+" admin", os.Stdout))
	root.AddCommand(cli.NewCommandCLI("cli", name+" cli"))
	root.AddCommand(cli.NewCmdKubectl("kube", os.Stdout))
	root.AddCommand(newExperimentalCommand("ex", name+" ex"))
	root.AddCommand(version.NewVersionCommand(name))

	// infra commands are those that are bundled with the binary but not displayed to end users
	// directly
	infra := &cobra.Command{
		Use: "infra", // Because this command exposes no description, it will not be shown in help
	}

	infra.AddCommand(
		router.NewCommandTemplateRouter("router"),
		deployer.NewCommandDeployer("deploy"),
		builder.NewCommandSTIBuilder("sti-build"),
		builder.NewCommandDockerBuilder("docker-build"),
		gitserver.NewCommandGitServer("git-server"),
	)
	root.AddCommand(infra)

	return root
}

func newExperimentalCommand(name, fullName string) *cobra.Command {
	experimental := &cobra.Command{
		Use:   name,
		Short: "Experimental commands under active development",
		Long:  "The commands grouped here are under development and may change without notice.",
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	f := clientcmd.New(experimental.PersistentFlags())
	out := os.Stdout

	experimental.AddCommand(tokens.NewCmdTokens(tokens.TokenRecommendedCommandName, fullName+" "+tokens.TokenRecommendedCommandName, f, out))
	experimental.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", os.Stdout))
	experimental.AddCommand(exrouter.NewCmdRouter(f, fullName, "router", os.Stdout))
	experimental.AddCommand(exregistry.NewCmdRegistry(f, fullName, "registry", os.Stdout))
	experimental.AddCommand(buildchain.NewCmdBuildChain(f, fullName, "build-chain"))
	experimental.AddCommand(bundlesecret.NewCmdBundleSecret(f, fullName, "bundle-secret", os.Stdout))
	experimental.AddCommand(cmd.NewCmdOptions(f, out))
	return experimental
}
