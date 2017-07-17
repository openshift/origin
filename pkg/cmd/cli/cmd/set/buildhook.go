package set

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	buildHookLong = templates.LongDesc(`
		Set or remove a build hook on a build config

		Build hooks allow behavior to be injected into the build process.

		A post-commit build hook is executed after a build has committed an image but before the
		image has been pushed to a registry. It can be used to execute tests on the image and verify
		it before it is made available in a registry or for any other logic that is needed to execute
		before the image is pushed to the registry. A new container with the recently built image is
		launched with the build hook command. If the command or script run by the build hook returns a
		non-zero exit code, the resulting image will not be pushed to the registry.

		The command for a build hook may be specified as a shell script (with the --script argument),
		as a new entrypoint command on the image with the --command argument, or as a set of
		arguments to the image's entrypoint (default).`)

	buildHookExample = templates.Examples(`  
		# Clear post-commit hook on a build config
	  %[1]s build-hook bc/mybuild --post-commit --remove

	  # Set the post-commit hook to execute a test suite using a new entrypoint
	  %[1]s build-hook bc/mybuild --post-commit --command -- /bin/bash -c /var/lib/test-image.sh

	  # Set the post-commit hook to execute a shell script
	  %[1]s build-hook bc/mybuild --post-commit --script="/var/lib/test-image.sh param1 param2 && /var/lib/done.sh"

	  # Set the post-commit hook as a set of arguments to the default image entrypoint
	  %[1]s build-hook bc/mybuild --post-commit  -- arg1 arg2`)
)

type BuildHookOptions struct {
	Out io.Writer
	Err io.Writer

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder runtime.Encoder

	Filenames []string
	Selector  string
	All       bool
	Output    string

	Cmd *cobra.Command

	Local       bool
	ShortOutput bool
	Mapper      meta.RESTMapper

	PrintObject func([]*resource.Info) error

	Script     string
	Entrypoint bool
	Remove     bool
	PostCommit bool

	Command []string
}

// NewCmdBuildHook implements the set build-hook command
func NewCmdBuildHook(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &BuildHookOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "build-hook BUILDCONFIG --post-commit [--command] [--script] -- CMD",
		Short:   "Update a build hook on a build config",
		Long:    buildHookLong,
		Example: fmt.Sprintf(buildHookExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move me to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter build configs")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "If true, select all build configs in the namespace")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&options.PostCommit, "post-commit", options.PostCommit, "If true, set the post-commit build hook on a build config")
	cmd.Flags().BoolVar(&options.Entrypoint, "command", options.Entrypoint, "If true, set the entrypoint of the hook container to the given command")
	cmd.Flags().StringVar(&options.Script, "script", options.Script, "Specify a script to run for the build-hook")
	cmd.Flags().BoolVar(&options.Remove, "remove", options.Remove, "If true, remove the build hook.")
	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, set image will NOT contact api-server but run locally.")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *BuildHookOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	resources := args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageError(cmd, "one or more build configs must be specified as <name> or <resource>/<name>")
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Cmd = cmd

	mapper, typer := f.Object()
	o.Builder = resource.NewBuilder(mapper, f.CategoryExpander(), typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		SelectorParam(o.Selector).
		ResourceNames("buildconfigs", resources...).
		Flatten()

	if !o.Local {
		o.Builder = o.Builder.
			SelectorParam(o.Selector).
			ResourceNames("buildconfigs", resources...)
		if o.All {
			o.Builder.ResourceTypes("buildconfigs").SelectAllParam(o.All)
		}
	}

	o.Output = kcmdutil.GetFlagString(cmd, "output")
	o.PrintObject = func(infos []*resource.Info) error {
		return f.PrintResourceInfos(cmd, infos, o.Out)
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	return nil
}

func (o *BuildHookOptions) Validate() error {

	if !o.PostCommit {
		return fmt.Errorf("you must specify a type of hook to set")
	}

	if o.Remove {
		if len(o.Command) > 0 {
			return fmt.Errorf("--remove may not be used with any other option")
		}
		return nil
	}

	if len(o.Script) > 0 && o.Entrypoint {
		return fmt.Errorf("--script and --command cannot be specified together")
	}

	if len(o.Script) > 0 && len(o.Command) > 0 {
		return fmt.Errorf("a command cannot be specified when using the --script argument")
	}

	if len(o.Command) == 0 && len(o.Script) == 0 {
		return fmt.Errorf("you must specify either a script or command for the build hook")
	}
	return nil
}

func (o *BuildHookOptions) Run() error {
	infos := o.Infos
	singleItemImplied := len(o.Infos) <= 1
	if o.Builder != nil {
		loaded, err := o.Builder.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
		if err != nil {
			return err
		}
		infos = loaded
	}

	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
		bc, ok := info.Object.(*buildapi.BuildConfig)
		if !ok {
			return false, nil
		}
		o.updateBuildConfig(bc)
		return true, nil
	})

	if singleItemImplied && len(patches) == 0 {
		return fmt.Errorf("%s/%s is not a build config", infos[0].Mapping.Resource, infos[0].Name)
	}

	if len(o.Output) > 0 || o.Local || kcmdutil.GetDryRunFlag(o.Cmd) {
		return o.PrintObject(infos)
	}

	failed := false
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			fmt.Fprintf(o.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err)
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(o.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			fmt.Fprintf(o.Err, "error: %v\n", err)
			failed = true
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return cmdutil.ErrExit
	}
	return nil
}

func (o *BuildHookOptions) updateBuildConfig(bc *buildapi.BuildConfig) {
	if o.Remove {
		bc.Spec.PostCommit.Args = nil
		bc.Spec.PostCommit.Command = nil
		bc.Spec.PostCommit.Script = ""
		return
	}

	switch {
	case len(o.Script) > 0:
		bc.Spec.PostCommit.Args = nil
		bc.Spec.PostCommit.Command = nil
		bc.Spec.PostCommit.Script = o.Script
	case o.Entrypoint:
		bc.Spec.PostCommit.Command = o.Command[0:1]
		if len(o.Command) > 1 {
			bc.Spec.PostCommit.Args = o.Command[1:]
		}
		bc.Spec.PostCommit.Script = ""
	default:
		bc.Spec.PostCommit.Command = nil
		bc.Spec.PostCommit.Args = o.Command
		bc.Spec.PostCommit.Script = ""
	}
}
