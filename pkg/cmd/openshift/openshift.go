package openshift

import (
	"flag"
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kcmd "k8s.io/kubernetes/pkg/kubectl/cmd"
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
	cmdversion "github.com/openshift/origin/pkg/cmd/version"
	osversion "github.com/openshift/origin/pkg/version/openshift"
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

	startAllInOne, _ := start.NewCommandStartAllInOne(name, out, errout)
	root.AddCommand(startAllInOne)
	root.AddCommand(newCompletionCommand("completion", name+" completion"))
	root.AddCommand(cmdversion.NewCmdVersion(name, osversion.Get(), os.Stdout))
	root.AddCommand(newCmdOptions())

	// TODO: add groups
	templates.ActsAsRootCommand(root, []string{"options"})

	return root
}

func newCompletionCommand(name, fullName string) *cobra.Command {
	return NewCmdCompletion(fullName, os.Stdout)

}

// newCmdOptions implements the OpenShift cli options command
func newCmdOptions() *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	ktemplates.UseOptionsTemplates(cmd)

	return cmd
}

// from here down probably deserves some common usage
var (
	completionLong = ktemplates.LongDesc(`
		This command prints shell code which must be evaluated to provide interactive
		completion of %s commands.`)

	completionExample = ktemplates.Examples(`
		# Generate the %s completion code for bash
	  %s completion bash > bash_completion.sh
	  source bash_completion.sh

	  # The above example depends on the bash-completion framework.
	  # It must be sourced before sourcing the openshift cli completion,
		# i.e. on the Mac:

	  brew install bash-completion
	  source $(brew --prefix)/etc/bash_completion
	  %s completion bash > bash_completion.sh
	  source bash_completion.sh

	  # In zsh*, the following will load openshift cli zsh completion:
	  source <(%s completion zsh)

	  * zsh completions are only supported in versions of zsh >= 5.2`)
)

func NewCmdCompletion(fullName string, out io.Writer) *cobra.Command {
	cmdHelpName := fullName

	if strings.HasSuffix(fullName, "completion") {
		cmdHelpName = "openshift"
	}

	cmd := kcmd.NewCmdCompletion(out, "\n")
	cmd.Long = fmt.Sprintf(completionLong, cmdHelpName)
	cmd.Example = fmt.Sprintf(completionExample, cmdHelpName, cmdHelpName, cmdHelpName, cmdHelpName)
	// mark all statically included flags as hidden to prevent them appearing in completions
	cmd.PreRun = func(c *cobra.Command, _ []string) {
		pflag.CommandLine.VisitAll(func(flag *pflag.Flag) {
			flag.Hidden = true
		})
		hideGlobalFlags(c.Root(), flag.CommandLine)
	}
	return cmd
}

// hideGlobalFlags marks any flag that is in the global flag set as
// hidden to prevent completion from varying by platform due to conditional
// includes. This means that some completions will not be possible unless
// they are registered in cobra instead of being added to flag.CommandLine.
func hideGlobalFlags(c *cobra.Command, fs *flag.FlagSet) {
	fs.VisitAll(func(flag *flag.Flag) {
		if f := c.PersistentFlags().Lookup(flag.Name); f != nil {
			f.Hidden = true
		}
		if f := c.LocalFlags().Lookup(flag.Name); f != nil {
			f.Hidden = true
		}
	})
	for _, child := range c.Commands() {
		hideGlobalFlags(child, fs)
	}
}
