package set

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	ometa "github.com/openshift/origin/pkg/api/meta"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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
	Out io.Writer
	Err io.Writer

	Filenames []string
	Selector  string
	All       bool

	Builder *resource.Builder
	Infos   []*resource.Info

	Encoder runtime.Encoder

	ShortOutput   bool
	Mapper        meta.RESTMapper
	OutputVersion schema.GroupVersion

	PrintTable  bool
	PrintObject func(runtime.Object) error

	List  bool
	Local bool

	Enabled bool
}

// NewCmdImageLookup implements the set image-lookup command
func NewCmdImageLookup(fullName, parentName string, f *clientcmd.Factory, out, errOut io.Writer) *cobra.Command {
	options := &ImageLookupOptions{
		Out:     out,
		Err:     errOut,
		Enabled: true,
	}
	cmd := &cobra.Command{
		Use:     "image-lookup STREAMNAME [...]",
		Short:   "Change how images are resolved when deploying applications",
		Long:    fmt.Sprintf(imageLookupLong, fullName, parentName),
		Example: fmt.Sprintf(imageLookupExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			kcmdutil.CheckErr(options.Run())
		},
	}

	kcmdutil.AddPrinterFlags(cmd)
	cmd.Flags().StringVarP(&options.Selector, "selector", "l", options.Selector, "Selector (label query) to filter on.")
	cmd.Flags().BoolVar(&options.All, "all", options.All, "If true, select all resources in the namespace of the specified resource types.")
	cmd.Flags().StringSliceVarP(&options.Filenames, "filename", "f", options.Filenames, "Filename, directory, or URL to file to use to edit the resource.")

	cmd.Flags().BoolVar(&options.List, "list", false, "Display the current states of the requested resources.")
	cmd.Flags().BoolVar(&options.Enabled, "enabled", options.Enabled, "Mark the image stream as resolving tagged images in this namespace.")

	cmd.Flags().BoolVar(&options.Local, "local", false, "If true, operations will be performed locally.")
	kcmdutil.AddDryRunFlag(cmd)
	cmd.MarkFlagFilename("filename", "yaml", "yml", "json")

	return cmd
}

// Complete takes command line information to fill out ImageLookupOptions or returns an error.
func (o *ImageLookupOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	outputVersionString := kcmdutil.GetFlagString(cmd, "output-version")
	if len(outputVersionString) == 0 {
		o.OutputVersion = *clientConfig.GroupVersion
	} else {
		o.OutputVersion, err = schema.ParseGroupVersion(outputVersionString)
		if err != nil {
			return err
		}
	}

	o.PrintTable = (len(args) == 0 && !o.All) || o.List

	mapper, _ := f.Object()
	o.Builder = f.NewBuilder().
		Internal().
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		Flatten()

	switch {
	case o.Local && len(args) > 0:
		return kcmdutil.UsageErrorf(cmd, "Pass files with -f when using --local")
	case o.Local:
		// perform no lookups on the server
		// TODO: discovery still requires a running server, doesn't fall back correctly
	case len(args) == 0 && len(o.Filenames) == 0:
		o.Builder = o.Builder.
			LabelSelectorParam(o.Selector).
			SelectAllParam(true).
			ResourceTypes("imagestreams")
	case o.List:
		o.Builder = o.Builder.
			LabelSelectorParam(o.Selector).
			SelectAllParam(o.All).
			ResourceTypeOrNameArgs(true, args...)
	default:
		o.Builder = o.Builder.
			LabelSelectorParam(o.Selector).
			SelectAllParam(o.All).
			ResourceNames("imagestreams", args...)
	}

	output := kcmdutil.GetFlagString(cmd, "output")
	if len(output) != 0 || o.Local || kcmdutil.GetDryRunFlag(cmd) {
		o.PrintObject = func(obj runtime.Object) error { return f.PrintObject(cmd, o.Local, mapper, obj, o.Out) }
	}

	o.Encoder = f.JSONEncoder()
	o.ShortOutput = kcmdutil.GetFlagString(cmd, "output") == "name"
	o.Mapper = mapper

	return nil
}

// Validate verifies the provided options are valid or returns an error.
func (o *ImageLookupOptions) Validate() error {
	return nil
}

// Run executes the ImageLookupOptions or returns an error.
func (o *ImageLookupOptions) Run() error {
	infos := o.Infos
	singleItemImplied := len(o.Infos) <= 1
	if o.Builder != nil {
		loaded, err := o.Builder.Do().IntoSingleItemImplied(&singleItemImplied).Infos()
		if err != nil {
			return err
		}
		infos = loaded
	}

	if o.PrintTable && o.PrintObject == nil {
		return o.printImageLookup(infos)
	}

	patches := CalculatePatches(infos, o.Encoder, func(info *resource.Info) (bool, error) {
		switch t := info.Object.(type) {
		case *imageapi.ImageStream:
			t.Spec.LookupPolicy.Local = o.Enabled
			return true, nil
		default:
			accessor, ok := ometa.GetAnnotationAccessor(info.Object)
			if !ok {
				return true, fmt.Errorf("the resource %s/%s does not support altering image lookup", info.Mapping.Resource, info.Name)
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
	if singleItemImplied && len(patches) == 0 {
		return fmt.Errorf("%s/%s no changes", infos[0].Mapping.Resource, infos[0].Name)
	}
	if o.PrintObject != nil {
		object, err := clientcmd.AsVersionedObject(infos, !singleItemImplied, o.OutputVersion, legacyscheme.Codecs.LegacyCodec(o.OutputVersion))
		if err != nil {
			return err
		}
		return o.PrintObject(object)
	}

	failed := false
	for _, patch := range patches {
		info := patch.Info
		if patch.Err != nil {
			failed = true
			fmt.Fprintf(o.Err, "error: %s/%s %v\n", info.Mapping.Resource, info.Name, patch.Err)
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			fmt.Fprintf(o.Err, "info: %s %q was not changed\n", info.Mapping.Resource, info.Name)
			continue
		}

		glog.V(4).Infof("Calculated patch %s", patch.Patch)

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.StrategicMergePatchType, patch.Patch)
		if err != nil {
			handlePodUpdateError(o.Err, err, "altered")
			failed = true
			continue
		}

		info.Refresh(obj, true)
		kcmdutil.PrintSuccess(o.Mapper, o.ShortOutput, o.Out, info.Mapping.Resource, info.Name, false, "updated")
	}
	if failed {
		return kcmdutil.ErrExit
	}
	return nil
}

// printImageLookup displays a tabular output of the imageLookup for each object.
func (o *ImageLookupOptions) printImageLookup(infos []*resource.Info) error {
	w := tabwriter.NewWriter(o.Out, 0, 2, 2, ' ', 0)
	defer w.Flush()
	fmt.Fprintf(w, "NAME\tLOCAL\n")
	for _, info := range infos {
		switch t := info.Object.(type) {
		case *imageapi.ImageStream:
			fmt.Fprintf(w, "%s\t%t\n", info.Name, t.Spec.LookupPolicy.Local)
		default:
			accessor, ok := ometa.GetAnnotationAccessor(info.Object)
			if !ok {
				// has no annotations
				fmt.Fprintf(w, "%s/%s\tUNKNOWN\n", info.Mapping.Resource, info.Name)
				break
			}
			var enabled bool
			if a, ok := accessor.TemplateAnnotations(); ok {
				enabled = a[alphaResolveNamesAnnotation] == "*"
			}
			if !enabled {
				enabled = accessor.Annotations()[alphaResolveNamesAnnotation] == "*"
			}
			fmt.Fprintf(w, "%s/%s\t%t\n", info.Mapping.Resource, info.Name, enabled)
		}
	}
	return nil
}
