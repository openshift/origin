package openshift

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

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
	"github.com/openshift/origin/pkg/oc/cli/cmd"
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
	root.AddCommand(cmd.NewCmdOptions(out))

	// TODO: add groups
	templates.ActsAsRootCommand(root, []string{"options"})

	return root
}

func newCompletionCommand(name, fullName string) *cobra.Command {
	return cmd.NewCmdCompletion(fullName, os.Stdout)

}
