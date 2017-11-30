package openshift

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/infra/builder"
	"github.com/openshift/origin/pkg/cmd/infra/deployer"
	irouter "github.com/openshift/origin/pkg/cmd/infra/router"
	"github.com/openshift/origin/pkg/cmd/recycle"
	"github.com/openshift/origin/pkg/cmd/server/start"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/oc/cli/cmd"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	openshiftLong = ktemplates.LongDesc(`
		%[2]s

		The %[3]s helps you build, deploy, and manage your applications on top of
		Docker containers. To start an all-in-one server with the default configuration, run:

		    $ %[1]s start &`)
)

// CommandFor returns the appropriate command for this base name,
// or the global OpenShift command
func CommandFor(basename string) *cobra.Command {
	var cmd *cobra.Command

	out := os.Stdout

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
	case "openshift-git-clone":
		cmd = builder.NewCommandGitClone(basename)
	case "openshift-manage-dockerfile":
		cmd = builder.NewCommandManageDockerfile(basename)
	case "openshift-extract-image-content":
		cmd = builder.NewCommandExtractImageContent(basename)
	case "origin":
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
	out, errout := os.Stdout, os.Stderr

	root := &cobra.Command{
		Use:   name,
		Short: "Build, deploy, and manage your cloud applications",
		Long:  fmt.Sprintf(openshiftLong, name, cmdutil.GetPlatformName(name), cmdutil.GetDistributionName(name)),
		Run:   kcmdutil.DefaultSubCommandRun(out),
	}

	f := clientcmd.New(pflag.NewFlagSet("", pflag.ContinueOnError))

	startAllInOne, _ := start.NewCommandStartAllInOne(name, out, errout)
	root.AddCommand(startAllInOne)
	root.AddCommand(newCompletionCommand("completion", name+" completion"))
	root.AddCommand(cmd.NewCmdVersion(name, f, out, cmd.VersionOptions{PrintEtcdVersion: true, IsServer: true}))
	root.AddCommand(cmd.NewCmdOptions(out))

	// TODO: add groups
	templates.ActsAsRootCommand(root, []string{"options"})

	return root
}

var (
	completion_long = ktemplates.LongDesc(`
		Output shell completion code for the given shell (bash or zsh).

		This command prints shell code which must be evaluation to provide interactive
		completion of kubectl commands.`)

	completion_example = ktemplates.Examples(`
		$ source <(kubectl completion bash)

		will load the kubectl completion code for bash. Note that this depends on the bash-completion
		framework. It must be sourced before sourcing the kubectl completion, i.e. on the Mac:

		$ brew install bash-completion
		$ source $(brew --prefix)/etc/bash_completion
		$ source <(kubectl completion bash)

		If you use zsh, the following will load kubectl zsh completion:

		$ source <(kubectl completion zsh)`)
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
