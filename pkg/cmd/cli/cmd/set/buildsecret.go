package set

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/errors"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

var (
	buildSecretLong = templates.LongDesc(`
		Set or remove a build secret on a build config

		A build config can reference a secret to push or pull images from private registries or
		to access private source repositories.

		Specify the type of secret being set by using the --push, --pull, or --source flags.
		A secret reference can be removed by using --remove flag.

		A label selector may be specified with the --selector flag to select the build configs
		on which to set or remove secrets. Alternatively, all build configs in the namespace can
		be selected with the --all flag.`)

	buildSecretExample = templates.Examples(`  
		# Clear push secret on a build config
		%[1]s build-secret --push --remove bc/mybuild

		# Set the pull secret on a build config
		%[1]s build-secret --pull bc/mybuild mysecret

		# Set the push and pull secret on a build config
		%[1]s build-secret --push --pull bc/mybuild mysecret

		# Set the source secret on a set of build configs matching a selector
		%[1]s build-secret --source -l app=myapp gitsecret`)
)

type BuildSecretOptions struct {
	Out io.Writer
	Err io.Writer

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder       runtime.Encoder
	OutputVersion unversioned.GroupVersion

	Filenames []string
	Selector  string
	All       bool

	ShortOutput bool
	Local       bool
	Mapper      meta.RESTMapper

	PrintObject func([]*resource.Info) error

	Secret string
	Push   bool
	Pull   bool
	Source bool
	Remove bool
}

// NewCmdBuildSecret implements the set build-secret command
func NewCmdBuildSecret(fullName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &BuildSecretOptions{
		Out: out,
		Err: errOut,
	}
	cmd := &cobra.Command{
		Use:     "build-secret BUILDCONFIG SECRETNAME",
		Short:   "Update a build secret on a build config",
		Long:    buildSecretLong,
		Example: fmt.Sprintf(buildSecretExample, fullName),
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
	cmd.Flags().BoolVar(&options.All, "all", options.All, "Select all build configs in the namespace")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&options.Push, "push", options.Push, "If true, set the push secret on a build config")
	cmd.Flags().BoolVar(&options.Pull, "pull", options.Pull, "If true, set the pull secret on a build config")
	cmd.Flags().BoolVar(&options.Source, "source", options.Source, "If true, set the source secret on a build config")
	cmd.Flags().BoolVar(&options.Remove, "remove", options.Remove, "If true, remove the build secret.")

	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, set build-secret will NOT contact api-server but run locally.")

	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	return cmd
}

var supportedBuildTypes = []string{"buildconfigs"}

func (o *BuildSecretOptions) secretFromArg(f *clientcmd.Factory, mapper meta.RESTMapper, typer runtime.ObjectTyper, namespace, arg string) (string, error) {
	builder := resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		NamespaceParam(namespace).DefaultNamespace().
		RequireObject(false).
		ContinueOnError().
		ResourceNames("secrets", arg).
		Flatten()

	var secretName string
	err := builder.Do().Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		if info.Mapping.Resource != "secrets" {
			return fmt.Errorf("please specify a secret")
		}
		secretName = info.Name
		return nil
	})
	if err != nil {
		return "", err
	}
	return secretName, nil
}

func (o *BuildSecretOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	var secretArg string
	if !o.Remove {
		if len(args) < 1 {
			return kcmdutil.UsageError(cmd, "a secret name must be specified")
		}
		secretArg = args[len(args)-1]
		args = args[:len(args)-1]
	}
	resources := args
	if len(resources) == 0 && len(o.Selector) == 0 && len(o.Filenames) == 0 && !o.All {
		return kcmdutil.UsageError(cmd, "one or more build configs must be specified as <name> or <resource>/<name>")
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	o.OutputVersion, err = kcmdutil.OutputVersion(cmd, clientConfig.GroupVersion)
	if err != nil {
		return err
	}

	mapper, typer := f.Object(false)
	if len(secretArg) > 0 {
		o.Secret, err = o.secretFromArg(f, mapper, typer, cmdNamespace, secretArg)
		if err != nil {
			return err
		}
	}
	o.Builder = resource.NewBuilder(mapper, typer, resource.ClientMapperFunc(f.ClientForMapping), kapi.Codecs.UniversalDecoder()).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, false, o.Filenames...).
		Flatten()

	if !o.Local {
		o.Builder = o.Builder.
			ResourceNames("buildconfigs", resources...).
			SelectorParam(o.Selector).
			Latest()

		if o.All {
			o.Builder.ResourceTypes(supportedBuildTypes...).SelectAllParam(o.All)
		}
	}

	output := kcmdutil.GetFlagString(cmd, "output")
	if len(output) > 0 || o.Local {
		o.PrintObject = func(infos []*resource.Info) error {
			return f.PrintResourceInfos(cmd, infos, o.Out)
		}
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	return nil
}

func (o *BuildSecretOptions) Validate() error {
	if !o.Pull && !o.Push && !o.Source {
		return fmt.Errorf("specify the type of secret to set (--push, --pull, or --source)")
	}
	if !o.Remove && len(o.Secret) == 0 {
		return fmt.Errorf("specify a secret to set")
	}
	if o.Remove && len(o.Secret) > 0 {
		return fmt.Errorf("a secret cannot be specified when using the --remove flag")
	}
	return nil
}

func (o *BuildSecretOptions) Run() error {
	infos := o.Infos
	singular := len(o.Infos) <= 1
	if o.Builder != nil {
		loaded, err := o.Builder.Do().IntoSingular(&singular).Infos()
		if err != nil {
			return err
		}
		infos = loaded
	}

	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
		return o.setBuildSecret(info.Object)
	})

	if singular && len(patches) == 0 {
		return fmt.Errorf("cannot set a build secret on %s/%s", infos[0].Mapping.Resource, infos[0].Name)
	}

	if o.PrintObject != nil {
		return o.PrintObject(infos)
	}

	errs := []error{}
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			errs = append(errs, fmt.Errorf("%s/%s %v", info.Mapping.Resource, info.Name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(o.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, kapi.StrategicMergePatchType, patch.Patch)
		if err != nil {
			errs = append(errs, fmt.Errorf("%s/%s %v", info.Mapping.Resource, info.Name, err))
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if len(errs) > 0 {
		return errors.NewAggregate(errs)
	}
	return nil
}

// setBuildSecret will set a secret on an object. For now the only supported
// object type is BuildConfig.
func (o *BuildSecretOptions) setBuildSecret(obj runtime.Object) (bool, error) {
	switch buildObj := obj.(type) {
	case *buildapi.BuildConfig:
		o.updateBuildConfig(buildObj)
		return true, nil
	default:
		return false, nil
	}
}

func (o *BuildSecretOptions) updateBuildConfig(bc *buildapi.BuildConfig) {
	if o.Push {
		if o.Remove {
			bc.Spec.Output.PushSecret = nil
		} else {
			bc.Spec.Output.PushSecret = &kapi.LocalObjectReference{
				Name: o.Secret,
			}
		}
	}

	if o.Pull {
		switch {
		case bc.Spec.Strategy.DockerStrategy != nil:
			if o.Remove {
				bc.Spec.Strategy.DockerStrategy.PullSecret = nil
			} else {
				bc.Spec.Strategy.DockerStrategy.PullSecret = &kapi.LocalObjectReference{
					Name: o.Secret,
				}
			}
		case bc.Spec.Strategy.SourceStrategy != nil:
			if o.Remove {
				bc.Spec.Strategy.SourceStrategy.PullSecret = nil
			} else {
				bc.Spec.Strategy.SourceStrategy.PullSecret = &kapi.LocalObjectReference{
					Name: o.Secret,
				}
			}
		case bc.Spec.Strategy.CustomStrategy != nil:
			if o.Remove {
				bc.Spec.Strategy.CustomStrategy.PullSecret = nil
			} else {
				bc.Spec.Strategy.CustomStrategy.PullSecret = &kapi.LocalObjectReference{
					Name: o.Secret,
				}
			}
		}
	}

	if o.Source {
		if o.Remove {
			bc.Spec.Source.SourceSecret = nil
		} else {
			bc.Spec.Source.SourceSecret = &kapi.LocalObjectReference{
				Name: o.Secret,
			}
		}
	}
}
