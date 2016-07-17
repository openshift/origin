package openshift

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/openshift/origin/pkg/cmd/admin"
	diagnostics "github.com/openshift/origin/pkg/cmd/admin/diagnostics"
	sync "github.com/openshift/origin/pkg/cmd/admin/groups/sync/cli"
	"github.com/openshift/origin/pkg/cmd/admin/validate"
	"github.com/openshift/origin/pkg/cmd/cli"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/experimental/buildchain"
	configcmd "github.com/openshift/origin/pkg/cmd/experimental/config"
	exipfailover "github.com/openshift/origin/pkg/cmd/experimental/ipfailover"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	irouter "github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/recycle"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/server/start/kubernetes"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	openshiftLong = `
%[2]s

The %[3]s helps you build, deploy, and manage your applications on top of
Docker containers. To start an all-in-one server with the default configuration, run:

  %[1]s start &`
)

// CommandFor returns the appropriate command for this base name,
// or the global OpenShift command
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	in, out, errout := os.Stdin, os.Stdout, os.Stderr

	// Make case-insensitive and strip executable suffix if present
	if runtime.GOOS == "windows" {
		basename = strings.ToLower(basename)
		basename = strings.TrimSuffix(basename, ".exe")
	}

	switch basename {
	case "openshift-router":
		cmd = irouter.NewCommandTemplateRouter(basename)
	case "openshift-f5-router":
		cmd = irouter.NewCommandF5Router(basename)
	case "openshift-deploy":
		cmd = deployer.NewCommandDeployer(basename)
	case "openshift-recycle":
		cmd = recycle.NewCommandRecycle(basename, out)
	case "openshift-sti-build":
		cmd = builder.NewCommandS2IBuilder(basename)
	case "openshift-docker-build":
		cmd = builder.NewCommandDockerBuilder(basename)
	case "oc", "osc":
		cmd = cli.NewCommandCLI(basename, basename, in, out, errout)
	case "oadm", "osadm":
		cmd = admin.NewCommandAdmin(basename, basename, in, out, errout)
	case "kubectl":
		cmd = cli.NewCmdKubectl(basename, out)
	case "kube-apiserver":
		cmd = kubernetes.NewAPIServerCommand(basename, basename, out)
	case "kube-controller-manager":
		cmd = kubernetes.NewControllersCommand(basename, basename, out)
	case "kubelet":
		cmd = kubernetes.NewKubeletCommand(basename, basename, out)
	case "kube-proxy":
		cmd = kubernetes.NewProxyCommand(basename, basename, out)
	case "kube-scheduler":
		cmd = kubernetes.NewSchedulerCommand(basename, basename, out)
	case "kubernetes":
		cmd = kubernetes.NewCommand(basename, basename, out)
	case "origin", "atomic-enterprise":
		cmd = NewCommandOpenShift(basename)
	default:
		cmd = NewCommandOpenShift("openshift")
	}

	if cmd.UsageFunc() == nil {
		templates.ActsAsRootCommand(cmd, []string{"options"})
	}
	flagtypes.GLog(cmd.PersistentFlags())

	return cmd
}

// NewCommandOpenShift creates the standard OpenShift command
func NewCommandOpenShift(name string) *cobra.Command {
	in, out, errout := os.Stdin, os.Stdout, os.Stderr

	root := &cobra.Command{
		Use:   name,
		Short: "Build, deploy, and manage your cloud applications",
		Long:  fmt.Sprintf(openshiftLong, name, cmdutil.GetPlatformName(name), cmdutil.GetDistributionName(name)),
		Run:   cmdutil.DefaultSubCommandRun(out),
	}

	f := clientcmd.New(pflag.NewFlagSet("", pflag.ContinueOnError))

	startAllInOne, _ := start.NewCommandStartAllInOne(name, out)
	root.AddCommand(startAllInOne)
	root.AddCommand(admin.NewCommandAdmin("admin", name+" admin", in, out, errout))
	root.AddCommand(cli.NewCommandCLI("cli", name+" cli", in, out, errout))
	root.AddCommand(cli.NewCmdKubectl("kube", out))
	root.AddCommand(newExperimentalCommand("ex", name+" ex"))
	root.AddCommand(newCompletionCommand("completion", name+" completion"))
	root.AddCommand(cmd.NewCmdVersion(name, f, out, cmd.VersionOptions{PrintEtcdVersion: true, IsServer: true}))

	// infra commands are those that are bundled with the binary but not displayed to end users
	// directly
	infra := &cobra.Command{
		Use: "infra", // Because this command exposes no description, it will not be shown in help
	}

	infra.AddCommand(
		irouter.NewCommandTemplateRouter("router"),
		irouter.NewCommandF5Router("f5-router"),
		deployer.NewCommandDeployer("deploy"),
		recycle.NewCommandRecycle("recycle", out),
		builder.NewCommandS2IBuilder("sti-build"),
		builder.NewCommandDockerBuilder("docker-build"),
		diagnostics.NewCommandPodDiagnostics("diagnostic-pod", out),
		diagnostics.NewCommandNetworkPodDiagnostics("network-diagnostic-pod", out),
	)
	root.AddCommand(infra)

	root.AddCommand(cmd.NewCmdOptions(out))

	// TODO: add groups
	templates.ActsAsRootCommand(root, []string{"options"})

	return root
}

func newExperimentalCommand(name, fullName string) *cobra.Command {
	out := os.Stdout
	errout := os.Stderr

	experimental := &cobra.Command{
		Use:   name,
		Short: "Experimental commands under active development",
		Long:  "The commands grouped here are under development and may change without notice.",
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
		BashCompletionFunction: admin.BashCompletionFunc,
	}

	f := clientcmd.New(experimental.PersistentFlags())

	experimental.AddCommand(validate.NewCommandValidate(validate.ValidateRecommendedName, fullName+" "+validate.ValidateRecommendedName, out))
	experimental.AddCommand(exipfailover.NewCmdIPFailoverConfig(f, fullName, "ipfailover", out, errout))
	experimental.AddCommand(buildchain.NewCmdBuildChain(name, fullName+" "+buildchain.BuildChainRecommendedCommandName, f, out))
	experimental.AddCommand(configcmd.NewCmdConfig(configcmd.ConfigRecommendedName, fullName+" "+configcmd.ConfigRecommendedName, f, out, errout))
	deprecatedDiag := diagnostics.NewCmdDiagnostics(diagnostics.DiagnosticsRecommendedName, fullName+" "+diagnostics.DiagnosticsRecommendedName, out)
	deprecatedDiag.Deprecated = fmt.Sprintf(`use "oadm %[1]s" to run diagnostics instead.`, diagnostics.DiagnosticsRecommendedName)
	experimental.AddCommand(deprecatedDiag)
	experimental.AddCommand(cmd.NewCmdOptions(out))

	// these groups also live under `oadm groups {sync,prune}` and are here only for backwards compatibility
	experimental.AddCommand(sync.NewCmdSync("sync-groups", fullName+" "+"sync-groups", f, out))
	experimental.AddCommand(sync.NewCmdPrune("prune-groups", fullName+" "+"prune-groups", f, out))
	return experimental
}

const (
	completion_long = `Output shell completion code for the given shell (bash or zsh).

This command prints shell code which must be evaluation to provide interactive
completion of kubectl commands.
`
	completion_example = `
$ source <(kubectl completion bash)

will load the kubectl completion code for bash. Note that this depends on the bash-completion
framework. It must be sourced before sourcing the kubectl completion, i.e. on the Mac:

$ brew install bash-completion
$ source $(brew --prefix)/etc/bash_completion
$ source <(kubectl completion bash)

If you use zsh, the following will load kubectl zsh completion:

$ source <(kubectl completion zsh)
`
)

func newCompletionCommand(name, fullName string) *cobra.Command {
	out := os.Stdout

	completion := &cobra.Command{
		Use:     fmt.Sprintf("%s SHELL", name),
		Short:   "Output shell completion code for the given shell (bash or zsh)",
		Long:    completion_long,
		Example: completion_example,
		Run: func(cmd *cobra.Command, args []string) {

		},
	}

	f := clientcmd.New(completion.PersistentFlags())

	return cmd.NewCmdCompletion(fullName, f, out)

}
