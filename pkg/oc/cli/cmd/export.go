package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	kprinters "k8s.io/kubernetes/pkg/printers"

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

func NewCmdExport(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	exporter := &DefaultExporter{}
	var filenames []string
	cmd := &cobra.Command{
		Use:     "export RESOURCE/NAME ... [flags]",
		Short:   "Export resources so they can be used elsewhere",
		Long:    exportLong,
		Example: fmt.Sprintf(exportExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunExport(f, exporter, in, out, cmd, args, filenames)
			if err == kcmdutil.ErrExit {
				os.Exit(1)
			}
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Deprecated = "Use the `get --export` command instead."

	cmd.Flags().String("as-template", "", "Output a Template object with specified name instead of a List or single object.")
	cmd.Flags().Bool("exact", false, "If true, preserve fields that may be cluster specific, such as service clusterIPs or generated names")
	cmd.Flags().Bool("raw", false, "If true, do not alter the resources in any way after they are loaded.")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all-namespaces", false, "If true, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().StringSliceVarP(&filenames, "filename", "f", filenames, "Filename, directory, or URL to file for the resource to export.")
	cmd.MarkFlagFilename("filename")
	cmd.Flags().Bool("all", true, "DEPRECATED: all is ignored, specifying a resource without a name selects all the instances of that resource")
	cmd.Flags().MarkDeprecated("all", "all is ignored because specifying a resource without a name selects all the instances of that resource")
	kcmdutil.AddPrinterFlags(cmd)
	return cmd
}

func RunExport(f *clientcmd.Factory, exporter Exporter, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, filenames []string) error {
	selector := kcmdutil.GetFlagString(cmd, "selector")
	allNamespaces := kcmdutil.GetFlagBool(cmd, "all-namespaces")
	exact := kcmdutil.GetFlagBool(cmd, "exact")
	asTemplate := kcmdutil.GetFlagString(cmd, "as-template")
	raw := kcmdutil.GetFlagBool(cmd, "raw")
	if exact && raw {
		return kcmdutil.UsageErrorf(cmd, "--exact and --raw may not both be specified")
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}

	var outputVersion schema.GroupVersion
	outputVersionString := kcmdutil.GetFlagString(cmd, "output-version")
	if len(outputVersionString) == 0 {
		outputVersion = *clientConfig.GroupVersion
	} else {
		outputVersion, err = schema.ParseGroupVersion(outputVersionString)
		if err != nil {
			return err
		}
	}

	cmdNamespace, explicit, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	b := f.NewBuilder().
		Unstructured().
		NamespaceParam(cmdNamespace).DefaultNamespace().AllNamespaces(allNamespaces).
		FilenameParam(explicit, &resource.FilenameOptions{Recursive: false, Filenames: filenames}).
		LabelSelectorParam(selector).
		ResourceTypeOrNameArgs(true, args...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingleItemImplied(&one).Infos()
	if err != nil {
		return err
	}

	if len(infos) == 0 {
		return fmt.Errorf("no resources found - nothing to export")
	}

	if !raw {
		newInfos := []*resource.Info{}
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

			if err := exporter.Export(info.Object, exact); err != nil {
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
	if len(asTemplate) > 0 {
		objects, err := clientcmd.AsVersionedObjects(infos, outputVersion, legacyscheme.Codecs.LegacyCodec(outputVersion))
		if err != nil {
			return err
		}
		template := &templateapi.Template{
			Objects: objects,
		}
		template.Name = asTemplate
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

	// use YAML as the default format
	outputFormat := kcmdutil.GetFlagString(cmd, "output")
	templateFile := kcmdutil.GetFlagString(cmd, "template")
	if len(outputFormat) == 0 && len(templateFile) != 0 {
		outputFormat = "template"
	}
	if len(outputFormat) == 0 {
		outputFormat = "yaml"
	}
	decoders := []runtime.Decoder{legacyscheme.Codecs.UniversalDeserializer(), unstructured.UnstructuredJSONScheme}
	printOpts := kcmdutil.ExtractCmdPrintOptions(cmd, false)
	printOpts.OutputFormatType = outputFormat
	printOpts.OutputFormatArgument = templateFile
	printOpts.AllowMissingKeys = kcmdutil.GetFlagBool(cmd, "allow-missing-template-keys")

	_, typer := f.Object()
	p, err := kprinters.GetStandardPrinter(
		typer, legacyscheme.Codecs.LegacyCodec(outputVersion), decoders, *printOpts)

	if err != nil {
		return err
	}
	return p.PrintObj(result, out)
}
