package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/rest"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	oldresource "k8s.io/kubernetes/pkg/kubectl/resource"

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
	genericclioptions.IOStreams
	oldresource.FilenameOptions
	PrintFlags *genericclioptions.PrintFlags

	Exporter Exporter

	AsTemplateName string
	Exact          bool
	Raw            bool
	AllNamespaces  bool
	Selector       string
	OutputVersion  string
	Args           []string

	Builder          *oldresource.Builder
	Namespace        string
	RequireNamespace bool
	Typer            runtime.ObjectTyper
	ClientConfig     *rest.Config

	PrintObj printers.ResourcePrinterFunc
}

func NewExportOptions(streams genericclioptions.IOStreams) *ExportOptions {
	return &ExportOptions{
		IOStreams:  streams,
		PrintFlags: genericclioptions.NewPrintFlags("exported").WithDefaultOutput("yaml"),

		Exporter: &DefaultExporter{},
	}
}

func NewCmdExport(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewExportOptions(streams)

	cmd := &cobra.Command{
		Use:        "export RESOURCE/NAME ... [flags]",
		Short:      "Export resources so they can be used elsewhere",
		Long:       exportLong,
		Example:    fmt.Sprintf(exportExample, fullName),
		Deprecated: "use the oc get --export",
		Hidden:     true,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)
	kcmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, "Filename, directory, or URL to file for the resource to export.")

	cmd.Flags().StringVar(&o.AsTemplateName, "as-template", o.AsTemplateName, "Output a Template object with specified name instead of a List or single object.")
	cmd.Flags().BoolVar(&o.Exact, "exact", o.Exact, "If true, preserve fields that may be cluster specific, such as service clusterIPs or generated names")
	cmd.Flags().BoolVar(&o.Raw, "raw", o.Raw, "If true, do not alter the resources in any way after they are loaded.")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on")
	cmd.Flags().BoolVar(&o.AllNamespaces, "all-namespaces", o.AllNamespaces, "If true, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().Bool("all", true, "DEPRECATED: all is ignored, specifying a resource without a name selects all the instances of that resource")
	cmd.Flags().MarkDeprecated("all", "all is ignored because specifying a resource without a name selects all the instances of that resource")
	cmd.Flags().StringVar(&o.OutputVersion, "output-version", o.OutputVersion, "The preferred API versions of the output objects")

	return cmd
}

func (o *ExportOptions) Complete(f *clientcmd.Factory, args []string) error {
	var err error
	o.ClientConfig, err = f.ClientConfig()
	if err != nil {
		return err
	}

	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	o.PrintObj = printer.PrintObj
	o.Args = args

	o.Namespace, o.RequireNamespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Builder = f.NewBuilder()
	_, o.Typer = f.Object()
	return nil
}

func (o *ExportOptions) Validate() error {
	if o.Exact && o.Raw {
		return fmt.Errorf("--exact and --raw may not both be specified")
	}

	return nil
}

func (o *ExportOptions) Run() error {
	var outputVersion schema.GroupVersion
	var err error
	if len(o.OutputVersion) == 0 {
		outputVersion = *o.ClientConfig.GroupVersion
	} else {
		outputVersion, err = schema.ParseGroupVersion(o.OutputVersion)
		if err != nil {
			return err
		}
	}

	b := o.Builder.
		Unstructured().
		NamespaceParam(o.Namespace).DefaultNamespace().AllNamespaces(o.AllNamespaces).
		FilenameParam(o.RequireNamespace, &oldresource.FilenameOptions{Recursive: false, Filenames: o.Filenames}).
		LabelSelectorParam(o.Selector).
		ResourceTypeOrNameArgs(true, o.Args...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		return fmt.Errorf("no resources found - nothing to export")
	}

	if !o.Raw {
		newInfos := []*oldresource.Info{}
		errs := []error{}
		for _, info := range infos {
			converted := false

			// convert unstructured object to runtime.Object
			data, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(), info.Object)
			if err != nil {
				errs = append(errs, err)
				continue
			}
			decoded, err := runtime.Decode(legacyscheme.Codecs.UniversalDecoder(), data)
			if err == nil {
				// ignore error, if any, in order to allow resources
				// not known by the client to still be exported
				info.Object = decoded
				converted = true
			}

			if err := o.Exporter.Export(info.Object, o.Exact); err != nil {
				if err == ErrExportOmit {
					continue
				}
				errs = append(errs, err)
			}

			// if an unstructured resource was successfully converted by the universal decoder,
			// re-convert that object once again into its external version.
			// If object cannot be converted to an external version, ignore error and proceed with
			// internal version.
			if converted {
				if data, err = runtime.Encode(legacyscheme.Codecs.LegacyCodec(outputVersion), info.Object); err == nil {
					external, err := runtime.Decode(legacyscheme.Codecs.UniversalDeserializer(), data)
					if err != nil {
						errs = append(errs, fmt.Errorf("error: failed to convert resource to external version: %v", err))
						continue
					}
					info.Object = external
				}
			}

			newInfos = append(newInfos, info)
		}
		if len(errs) > 0 {
			return utilerrors.NewAggregate(errs)
		}
		infos = newInfos
	}

	var result runtime.Object
	if len(o.AsTemplateName) > 0 {
		objects, err := clientcmd.AsVersionedObjects(infos, outputVersion, legacyscheme.Codecs.LegacyCodec(outputVersion))
		if err != nil {
			return err
		}
		template := &templateapi.Template{
			Objects: objects,
		}
		template.Name = o.AsTemplateName
		result, err = legacyscheme.Scheme.ConvertToVersion(template, outputVersion)
		if err != nil {
			return err
		}
	} else {
		object, err := clientcmd.AsVersionedObject(infos, !one, outputVersion, legacyscheme.Codecs.LegacyCodec(outputVersion))
		if err != nil {
			return err
		}
		result = object
	}

	fmt.Printf("")
	return o.PrintObj(result, o.Out)
}
