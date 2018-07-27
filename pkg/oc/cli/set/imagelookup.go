package set

import (
	"fmt"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	imagev1 "github.com/openshift/api/image/v1"
	ometa "github.com/openshift/origin/pkg/api/imagereferencemutators"
)

var (
	imageLookupLong = templates.LongDesc(`
		Use an image stream from pods and other objects

		Image streams make it easy to tag images, track changes from other registries, and centralize
		access control to images. Local name lookup allows an image stream to be the source of
		images for pods, deployments, replica sets, and other resources that reference images, without
		having to provide the full registry URL. If local name lookup is enabled for an image stream
		named 'mysql', a pod or other resource that references 'mysql:latest' (or any other tag) will
		pull from the location specified by the image stream tag, not from an upstream registry.

		Once lookup is enabled, simply reference the image stream tag in the image field of the object.
		For example:

				$ %[2]s import-image mysql:latest --confirm
				$ %[1]s image-lookup mysql
				$ %[2]s run mysql --image=mysql

		will import the latest MySQL image from the DockerHub, set that image stream to handle the
		"mysql" name within the project, and then launch a deployment that points to the image we
		imported.

		You may also force image lookup for all of the images on a resource with this command. An
		annotation is placed on the object which forces an image stream tag lookup in the current
		namespace for any image that matches, regardless of whether the image stream has lookup
		enabled.

				$ %[2]s run mysql --image=myregistry:5000/test/mysql:v1
				$ %[2]s tag --source=docker myregistry:5000/test/mysql:v1 mysql:v1
				$ %[1]s image-lookup deploy/mysql

		Which should trigger a deployment pointing to the imported mysql:v1 tag.

		Experimental: This feature is under active development and may change without notice.`)

	imageLookupExample = templates.Examples(`
		# Print all of the image streams and whether they resolve local names
		%[1]s image-lookup

		# Use local name lookup on image stream mysql
		%[1]s image-lookup mysql

		# Force a deployment to use local name lookup
		%[1]s image-lookup deploy/mysql

		# Show the current status of the deployment lookup
		%[1]s image-lookup deploy/mysql --list

		# Disable local name lookup on image stream mysql
		%[1]s image-lookup mysql --enabled=false

		# Set local name lookup on all image streams
		%[1]s image-lookup --all`)
)

const alphaResolveNamesAnnotation = "alpha.image.policy.openshift.io/resolve-names"

type ImageLookupOptions struct {
	PrintFlags *genericclioptions.PrintFlags

	Selector   string
	All        bool
	List       bool
	Local      bool
	Enabled    bool
	PrintTable bool

	Mapper            meta.RESTMapper
	Client            dynamic.Interface
	Printer           printers.ResourcePrinter
	Builder           func() *resource.Builder
	Namespace         string
	ExplicitNamespace bool
	DryRun            bool
	Args              []string

	resource.FilenameOptions
	genericclioptions.IOStreams
}

func NewImageLookupOptions(streams genericclioptions.IOStreams) *ImageLookupOptions {
	return &ImageLookupOptions{
		PrintFlags: genericclioptions.NewPrintFlags("image lookup updated").WithTypeSetter(scheme.Scheme),
		IOStreams:  streams,
		Enabled:    true,
	}
}

// NewCmdImageLookup implements the set image-lookup command
func NewCmdImageLookup(fullName, parentName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewImageLookupOptions(streams)
	cmd := &cobra.Command{
		Use:     "image-lookup STREAMNAME [...]",
		Short:   "Change how images are resolved when deploying applications",
		Long:    fmt.Sprintf(imageLookupLong, fullName, parentName),
		Example: fmt.Sprintf(imageLookupExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	usage := "to use to edit the resource"
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on.")
	cmd.Flags().BoolVar(&o.All, "all", o.All, "If true, select all resources in the namespace of the specified resource types.")
	cmd.Flags().BoolVar(&o.List, "list", o.List, "Display the current states of the requested resources.")
	cmd.Flags().BoolVar(&o.Enabled, "enabled", o.Enabled, "Mark the image stream as resolving tagged images in this namespace.")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, operations will be performed locally.")

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddDryRunFlag(cmd)

	return cmd
}

// Complete takes command line information to fill out ImageLookupOptions or returns an error.
func (o *ImageLookupOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	o.Namespace, o.ExplicitNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.PrintTable = (o.PrintFlags.OutputFormat == nil && len(args) == 0 && !o.All) || o.List

	o.Args = args
	o.DryRun = kcmdutil.GetDryRunFlag(cmd)
	o.Mapper, err = f.ToRESTMapper()
	if err != nil {
		return err
	}
	o.Builder = f.NewBuilder

	if o.DryRun {
		o.PrintFlags.Complete("%s (dry run)")
	}
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.Client, err = dynamic.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

func (o *ImageLookupOptions) Validate() error {
	if o.Local && len(o.Args) > 0 {
		return fmt.Errorf("pass files with -f when using --local")
	}

	return nil
}

// Run executes the ImageLookupOptions or returns an error.
func (o *ImageLookupOptions) Run() error {
	b := o.Builder().
		WithScheme(scheme.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.ExplicitNamespace, &o.FilenameOptions).
		Flatten()

	switch {
	case o.Local:
		// perform no lookups on the server
		// TODO: discovery still requires a running server, doesn't fall back correctly
	case len(o.Args) == 0 && len(o.Filenames) == 0:
		b = b.
			LabelSelectorParam(o.Selector).
			SelectAllParam(true).
			ResourceTypes("imagestreams")
	case o.List:
		b = b.
			LabelSelectorParam(o.Selector).
			SelectAllParam(o.All).
			ResourceTypeOrNameArgs(true, o.Args...)
	default:
		b = b.
			LabelSelectorParam(o.Selector).
			SelectAllParam(o.All).
			ResourceNames("imagestreams", o.Args...).
			Latest()
	}

	singleItemImplied := false
	infos, err := b.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
	if err != nil {
		return err
	}

	if o.PrintTable {
		return o.printImageLookup(infos)
	}

	patches := CalculatePatchesExternal(infos, func(info *resource.Info) (bool, error) {
		switch t := info.Object.(type) {
		case *imagev1.ImageStream:
			t.Spec.LookupPolicy.Local = o.Enabled
			return true, nil
		default:
			accessor, ok := ometa.GetAnnotationAccessor(info.Object)
			if !ok {
				return true, fmt.Errorf("the resource %s does not support altering image lookup", getObjectName(info))
			}
			templateAnnotations, ok := accessor.TemplateAnnotations()
			if ok {
				if o.Enabled {
					if templateAnnotations == nil {
						templateAnnotations = make(map[string]string)
					}
					templateAnnotations[alphaResolveNamesAnnotation] = "*"
				} else {
					delete(templateAnnotations, alphaResolveNamesAnnotation)
				}
				accessor.SetTemplateAnnotations(templateAnnotations)
				return true, nil
			}
			annotations := accessor.Annotations()
			if o.Enabled {
				if annotations == nil {
					annotations = make(map[string]string)
				}
				annotations[alphaResolveNamesAnnotation] = "*"
			} else {
				delete(annotations, alphaResolveNamesAnnotation)
			}
			accessor.SetAnnotations(annotations)
			return true, nil
		}
	})

	allErrs := []error{}
	for _, patch := range patches {
		info := patch.Info
		name := getObjectName(info)
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s %v\n", name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			glog.V(1).Infof("info: %s was not changed\n", name)
			continue
		}

		if o.Local || o.DryRun {
			if err := o.Printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		actual, err := o.Client.Resource(info.Mapping.Resource).Namespace(info.Namespace).Patch(info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch image lookup: %v\n", err))
			continue
		}

		if err := o.Printer.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)

}

// printImageLookup displays a tabular output of the imageLookup for each object.
func (o *ImageLookupOptions) printImageLookup(infos []*resource.Info) error {
	w := tabwriter.NewWriter(o.Out, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tLOCAL\n")
	for _, info := range infos {
		switch t := info.Object.(type) {
		case *imagev1.ImageStream:
			fmt.Fprintf(w, "%s\t%t\n", info.Name, t.Spec.LookupPolicy.Local)
		default:
			name := getObjectName(info)
			accessor, ok := ometa.GetAnnotationAccessor(info.Object)
			if !ok {
				// has no annotations
				fmt.Fprintf(w, "%s\tUNKNOWN\n", name)
				break
			}
			var enabled bool
			if a, ok := accessor.TemplateAnnotations(); ok {
				enabled = a[alphaResolveNamesAnnotation] == "*"
			}
			if !enabled {
				enabled = accessor.Annotations()[alphaResolveNamesAnnotation] == "*"
			}
			fmt.Fprintf(w, "%s\t%t\n", name, enabled)
		}
	}
	return nil
}
