package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const (
	exportLong = `Export resources so they can be used elsewhere

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
to generate the API structure for a template to which you can add parameters and object labels.`

	exportExample = `  // export the services and deployment configurations labeled name=test
  %[1]s export svc,dc -l name=test

  // export all services to a template
  %[1]s export service --all --as-template=test

  // export to JSON
  %[1]s export service --all -o json

  // convert a file on disk to the latest API version (in YAML, the default)
  %[1]s export -f a_v1beta3_service.json --output-version=v1 --exact`
)

func NewCmdExport(fullName string, f *clientcmd.Factory, in io.Reader, out io.Writer) *cobra.Command {
	exporter := &defaultExporter{}
	var filenames util.StringList
	cmd := &cobra.Command{
		Use:     "export RESOURCE/NAME ... [options]",
		Short:   "Export resources so they can be used elsewhere",
		Long:    exportLong,
		Example: fmt.Sprintf(exportExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunExport(f, exporter, in, out, cmd, args, filenames)
			if err == errExit {
				os.Exit(1)
			}
			cmdutil.CheckErr(err)
		},
	}
	cmd.Flags().String("as-template", "", "Output a Template object with specified name instead of a List or single object.")
	cmd.Flags().Bool("exact", false, "Preserve fields that may be cluster specific, such as service portalIPs or generated names")
	cmd.Flags().Bool("raw", false, "If true, do not alter the resources in any way after they are loaded.")
	cmd.Flags().StringP("selector", "l", "", "Selector (label query) to filter on")
	cmd.Flags().Bool("all", false, "Select all resources in the namespace of the specified resource types")
	cmd.Flags().VarP(&filenames, "filename", "f", "Filename, directory, or URL to file to use to edit the resource.")
	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func RunExport(f *clientcmd.Factory, exporter Exporter, in io.Reader, out io.Writer, cmd *cobra.Command, args []string, filenames util.StringList) error {
	selector := cmdutil.GetFlagString(cmd, "selector")
	all := cmdutil.GetFlagBool(cmd, "all")
	exact := cmdutil.GetFlagBool(cmd, "exact")
	asTemplate := cmdutil.GetFlagString(cmd, "as-template")
	raw := cmdutil.GetFlagBool(cmd, "raw")
	if exact && raw {
		return cmdutil.UsageError(cmd, "--exact and --raw may not both be specified")
	}

	clientConfig, err := f.ClientConfig()
	if err != nil {
		return err
	}
	outputVersion := cmdutil.OutputVersion(cmd, clientConfig.Version)

	cmdNamespace, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	mapper, typer := f.Object()
	b := resource.NewBuilder(mapper, typer, f.ClientMapperForCommand()).
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(filenames...).
		SelectorParam(selector).
		ResourceTypeOrNameArgs(all, args...).
		Flatten()

	one := false
	infos, err := b.Do().IntoSingular(&one).Infos()
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
			if err := exporter.Export(info.Object, exact); err != nil {
				if err == ErrExportOmit {
					continue
				}
				errs = append(errs, err)
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
		objects, err := resource.AsVersionedObjects(infos, outputVersion)
		if err != nil {
			return err
		}
		template := &templateapi.Template{
			Objects: objects,
		}
		template.Name = asTemplate
		result, err = kapi.Scheme.ConvertToVersion(template, outputVersion)
		if err != nil {
			return err
		}
	} else {
		object, err := resource.AsVersionedObject(infos, !one, outputVersion)
		if err != nil {
			return err
		}
		result = object
	}

	// use YAML as the default format
	outputFormat := cmdutil.GetFlagString(cmd, "output")
	templateFile := cmdutil.GetFlagString(cmd, "template")
	if len(outputFormat) == 0 && len(templateFile) != 0 {
		outputFormat = "template"
	}
	if len(outputFormat) == 0 {
		outputFormat = "yaml"
	}
	p, _, err := kubectl.GetPrinter(outputFormat, templateFile)
	if err != nil {
		return err
	}
	return p.PrintObj(result, out)
}
