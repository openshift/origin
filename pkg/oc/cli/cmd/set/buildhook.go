package set

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	oldresource "k8s.io/kubernetes/pkg/kubectl/resource"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/oauth/generated/clientset/scheme"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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
	PrintFlags *genericclioptions.PrintFlags
	oldresource.FilenameOptions
	genericclioptions.IOStreams

	Selector   string
	All        bool
	Output     string
	Local      bool
	Script     string
	Entrypoint bool
	Remove     bool
	PostCommit bool

	Mapper            meta.RESTMapper
	PrintObj          printers.ResourcePrinterFunc
	Builder           func() *oldresource.Builder
	Encoder           runtime.Encoder
	Namespace         string
	ExplicitNamespace bool
	Command           []string
	Resources         []string
	DryRun            bool
}

func NewBuildHookOptions(streams genericclioptions.IOStreams) *BuildHookOptions {
	return &BuildHookOptions{
		PrintFlags: genericclioptions.NewPrintFlags("hooks updated").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdBuildHook implements the set build-hook command
func NewCmdBuildHook(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewBuildHookOptions(streams)
	cmd := &cobra.Command{
		Use:     "build-hook BUILDCONFIG --post-commit [--command] [--script] -- CMD",
		Short:   "Update a build hook on a build config",
		Long:    buildHookLong,
		Example: fmt.Sprintf(buildHookExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter build configs")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all build configs in the namespace")
	cmd.Flags().BoolVar(&o.PostCommit, "post-commit", o.PostCommit, "If true, set the post-commit build hook on a build config")
	cmd.Flags().BoolVar(&o.Entrypoint, "command", o.Entrypoint, "If true, set the entrypoint of the hook container to the given command")
	cmd.Flags().StringVar(&o.Script, "script", o.Script, "Specify a script to run for the build-hook")
	cmd.Flags().BoolVar(&o.Remove, "remove", o.Remove, "If true, remove the build hook.")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set image will NOT contact api-server but run locally.")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *BuildHookOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	o.Resources = args
	if i := cmd.ArgsLenAtDash(); i != -1 {
		o.Resources = args[:i]
		o.Command = args[i:]
	}
	if len(o.Filenames) == 0 && len(args) < 1 {
		return kcmdutil.UsageErrorf(cmd, "one or more build configs must be specified as <name> or <resource>/<name>")
	}

	var err error
	o.Namespace, o.ExplicitNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.Mapper, _ = f.Object()
	o.Encoder = kcmdutil.InternalVersionJSONEncoder()
	o.Builder = f.NewBuilder

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = printer.PrintObj

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
	b := o.Builder().
		Internal().
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()

	if !o.Local {
		b = b.
			LabelSelectorParam(o.Selector).
			ResourceNames("buildconfigs", o.Resources...)
		if o.All {
			b = b.ResourceTypes("buildconfigs").SelectAllParam(o.All)
		}
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	// FIXME-REBASE
	patches := CalculatePatches(infos, o.Encoder /*scheme.DefaultJSONEncoder()*/, func(info *oldresource.Info) (bool, error) {
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

	allErrs := []error{}
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			continue
		}

		if o.Local || o.DryRun {
			if err := o.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := oldresource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch build hook: %v\n", err))
			continue
		}

		if err := o.PrintObj(actual, o.Out); err != nil {
			// FIXME-REBASE
			// allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
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
