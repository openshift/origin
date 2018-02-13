package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

var (
	exportLong = templates.LongDesc(`
		Export resources so they can be used elsewhere

		The export command makes it easy to take existing objects and convert them to configuration files
		for backups or for creating elsewhere in the cluster. Fields that cannot be specified on create
		will be set to empty, and any field which is assigned on creation (like a service's clusterIP, or
		a deployment config's latestVersion). The status part of objects is also cleared.

		Some fields like clusterIP may be useful when exporting an application from one cluster to apply
		to another - assuming another service on the destination cluster does not already use that IP.
		The --exact flag will instruct export to not clear fields that might be useful. You may also use
		--raw to get the exact values for an object - useful for converting a file on disk between API
		versions.

		Another use case for export is to create reusable templates for applications. Pass --as-template
		to generate the API structure for a template to which you can add parameters and object labels.`)

	exportExample = templates.Examples(`
		# export the services and deployment configurations labeled name=test
	  %[1]s export svc,dc -l name=test

	  # export all services to a template
	  %[1]s export service --as-template=test

	  # export to JSON
	  %[1]s export service -o json`)
)

type ExportOptions struct {
	asTemplate string
	selector   string
	output     string
	template   string

	allNamespaces bool
	exact         bool
	raw           bool
	forceList     bool

	filenames []string

	infos []*resource.Info

	encoder runtime.Encoder
	decoder runtime.Decoder

	printObj func(runtime.Object) error

	outputVersion schema.GroupVersion
	exporter      *DefaultExporter

	in  io.Reader
	out io.Writer
}

func NewCmdExport(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	opts := &ExportOptions{
		exporter: &DefaultExporter{},

		in:  in,
		out: out,
	}

	cmd := &cobra.Command{
		Use:     "export RESOURCE/NAME ... [options]",
		Short:   "Export resources so they can be used elsewhere",
		Long:    exportLong,
		Example: fmt.Sprintf(exportExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, args, c); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := opts.Validate(c); err != nil {
				kcmdutil.CheckErr(err)
			}
			if err := opts.RunExport(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.Flags().String("as-template", "", "Output a Template object with specified name instead of a List or single object.")
	cmd.Flags().Bool("exact", false, "If true, preserve fields that may be cluster specific, such as service clusterIPs or generated names")
	cmd.Flags().Bool("raw", false, "If true, do not alter the resources in any way after they are loaded.")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all-namespaces", false, "If true, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringSliceVarP(&opts.filenames, "filename", "f", opts.filenames, "Filename, directory, or URL to file for the resource to export.")
	cmd.MarkFlagFilename("filename")
	cmd.Flags().Bool("all", true, "DEPRECATED: all is ignored, specifying a resource without a name selects all the instances of that resource")
	cmd.Flags().MarkDeprecated("all", "all is ignored because specifying a resource without a name selects all the instances of that resource")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *ExportOptions) Complete(f kcmdutil.Factory, args []string, cmd *cobra.Command) error {
	o.selector = kcmdutil.GetFlagString(cmd, "selector")
	o.allNamespaces = kcmdutil.GetFlagBool(cmd, "all-namespaces")
	o.exact = kcmdutil.GetFlagBool(cmd, "exact")
	o.asTemplate = kcmdutil.GetFlagString(cmd, "as-template")
	o.raw = kcmdutil.GetFlagBool(cmd, "raw")

	outputVersion := kcmdutil.GetFlagString(cmd, "output-version")
	if len(outputVersion) > 0 {
		version, err := schema.ParseGroupVersion(outputVersion)
		if err != nil {
			return err
		}

		o.outputVersion = version
	}

	o.output = kcmdutil.GetFlagString(cmd, "output")
	o.template = kcmdutil.GetFlagString(cmd, "template")

	// default --output to "yaml"
	if len(o.output) == 0 && len(o.template) > 0 {
		o.output = "template"
	}
	if len(o.output) == 0 {
		o.output = "yaml"
	}

	o.printObj = func(obj runtime.Object) error {
		printOpts := kcmdutil.ExtractCmdPrintOptions(cmd, false)
		printOpts.OutputFormatType = o.output
		printOpts.OutputFormatArgument = o.template
		printOpts.AllowMissingKeys = kcmdutil.GetFlagBool(cmd, "allow-missing-template-keys")

		p, err := f.PrinterForOptions(printOpts)
		if err != nil {
			return err
		}

		return p.PrintObj(obj, o.out)
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.encoder = f.JSONEncoder()
	o.decoder = f.Decoder(true)

	b := f.NewBuilder().
		Unstructured().
		NamespaceParam(cmdNamespace).DefaultNamespace().AllNamespaces(o.allNamespaces).
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: o.filenames}).
		LabelSelectorParam(o.selector).
		ResourceTypeOrNameArgs(true, args...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}

	o.forceList = !one

	if len(infos) == 0 {
		return fmt.Errorf("no resources found - nothing to export")
	}

	o.infos = infos
	return nil
}

func (o *ExportOptions) Validate(cmd *cobra.Command) error {
	if o.exact && o.raw {
		return kcmdutil.UsageErrorf(cmd, "--exact and --raw may not both be specified")
	}

	return nil
}

func (o *ExportOptions) RunExport() error {
	if !o.raw {
		newInfos := []*resource.Info{}
		errs := []error{}
		for _, info := range o.infos {
			converted := false

			// capture original gvk in order to reconvert object later
			originalGVK := info.Object.GetObjectKind().GroupVersionKind()

			// convert unstructured object to runtime.Object
			data, err := runtime.Encode(o.encoder, info.Object)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			decoded, err := runtime.Decode(o.decoder, data)
			if err == nil {
				// ignore error, if any, in order to allow resources
				// not known by the client to still be exported
				info.Object = decoded
				converted = true
			}

			if err := o.exporter.Export(info.Object, o.exact); err != nil {
				if err == ErrExportOmit {
					continue
				}
				errs = append(errs, err)
			}

			// if an unstructured resource was successfully converted by the universal decoder,
			// re-convert that object once again into its external version.
			// If object cannot be converted to an external version, ignore error and proceed with
			// internal version.

			// re-convert objects back to original groupVersion and decode back to unstructured
			// only if an --output-version has not been provided.
			if converted && o.outputVersion.Empty() {
				convertedObj, err := info.Mapping.ConvertToVersion(info.Object, originalGVK.GroupVersion())
				if err != nil {
					errs = append(errs, fmt.Errorf("error: failed to convert resource to external version: %v", err))
					continue
				}

				// convert back to unstructured
				unstructBytes, err := runtime.Encode(unstructured.UnstructuredJSONScheme, convertedObj)
				if err != nil {
					errs = append(errs, fmt.Errorf("error: failed to encode the object: %v", err))
					continue
				}

				unstruct, err := runtime.Decode(unstructured.UnstructuredJSONScheme, unstructBytes)
				if err != nil {
					errs = append(errs, fmt.Errorf("error: failed to convert resource to *unstructured.Unstructured: %v", err))
					continue
				}

				info.Object = unstruct
			}

			newInfos = append(newInfos, info)
		}
		if len(errs) > 0 {
			return utilerrors.NewAggregate(errs)
		}
		o.infos = newInfos
	}

	if o.outputVersion.Empty() {
		result := o.infos[0].Object

		template := &templateapi.Template{}
		list := &unstructured.UnstructuredList{
			Object: map[string]interface{}{
				"kind":       "List",
				"apiVersion": "v1",
				"metadata":   map[string]interface{}{},
			},
		}

		for _, info := range o.infos {
			// we are fetching unstructured objects from the resource builder to begin with,
			// so no need to worry about casting here
			list.Items = append(list.Items, *info.Object.(*unstructured.Unstructured))
			template.Objects = append(template.Objects, info.Object)
		}

		if len(o.asTemplate) > 0 {
			template.Name = o.asTemplate
			result = template
		} else if len(o.infos) > 1 {
			result = list
		}

		return o.printObj(result)
	}

	// handle --output-version provided
	var result runtime.Object
	if len(o.asTemplate) > 0 {
		objects, err := clientcmd.AsVersionedObjects(o.infos, o.outputVersion, legacyscheme.Codecs.LegacyCodec(o.outputVersion))
		if err != nil {
			return err
		}
		template := &templateapi.Template{
			Objects: objects,
		}
		template.Name = o.asTemplate
		result, err = legacyscheme.Scheme.ConvertToVersion(template, o.outputVersion)
		if err != nil {
			return err
		}
	} else {
		object, err := clientcmd.AsVersionedObject(o.infos, o.forceList, o.outputVersion, legacyscheme.Codecs.LegacyCodec(o.outputVersion))
		if err != nil {
			return err
		}
		result = object
	}

	return o.printObj(result)
}
