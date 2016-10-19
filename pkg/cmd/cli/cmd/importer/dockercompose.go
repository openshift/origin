package importer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	"k8s.io/kubernetes/pkg/kubectl"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	configcmd "github.com/openshift/origin/pkg/config/cmd"
	"github.com/openshift/origin/pkg/generate/app"
	appcmd "github.com/openshift/origin/pkg/generate/app/cmd"
	"github.com/openshift/origin/pkg/generate/dockercompose"
)

const DockerComposeV1GeneratorName = "docker-compose/v1"

var (
	dockerComposeLong = templates.LongDesc(`
		Import a Docker Compose file as OpenShift objects

		Docker Compose files offer a container centric build and deploy pattern for simple applications.
		This command will transform a provided docker-compose.yml application into its OpenShift equivalent.
		During transformation fields in the compose syntax that are not relevant when running on top of
		a containerized platform will be ignored and a warning printed.

		The command will create objects unless you pass the -o yaml or --as-template flags to generate a
		configuration file for later use.

		Experimental: This command is under active development and may change without notice.`)

	dockerComposeExample = templates.Examples(`
		# Import a docker-compose.yml file into OpenShift
	  %[1]s docker-compose -f ./docker-compose.yml

		# Turn a docker-compose.yml file into a template
	  %[1]s docker-compose -f ./docker-compose.yml -o yaml --as-template`)
)

type DockerComposeOptions struct {
	Action configcmd.BulkAction

	In        io.Reader
	Filenames []string

	Generator  string
	AsTemplate string

	PrintObject    func(runtime.Object) error
	OutputVersions []unversioned.GroupVersion

	Namespace string
	Client    client.TemplateConfigsNamespacer
}

// NewCmdDockerCompose imports a docker-compose file as a template.
func NewCmdDockerCompose(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	options := &DockerComposeOptions{
		Action: configcmd.BulkAction{
			Out:    out,
			ErrOut: errout,
		},
		In:        in,
		Generator: DockerComposeV1GeneratorName,
	}
	cmd := &cobra.Command{
		Use:     "docker-compose -f COMPOSEFILE",
		Short:   "Import a docker-compose.yml project into OpenShift (experimental)",
		Long:    dockerComposeLong,
		Example: fmt.Sprintf(dockerComposeExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f, cmd, args))
			kcmdutil.CheckErr(options.Validate())
			if err := options.Run(); err != nil {
				// TODO: move met to kcmdutil
				if err == cmdutil.ErrExit {
					os.Exit(1)
				}
				kcmdutil.CheckErr(err)
			}
		},
	}
	usage := "Filename, directory, or URL to docker-compose.yml file to use"
	kubectl.AddJsonFilenameFlag(cmd, &options.Filenames, usage)
	cmd.MarkFlagRequired("filename")

	cmd.Flags().String("generator", options.Generator, "The name of the API generator to use.")
	cmd.Flags().StringVar(&options.AsTemplate, "as-template", "", "If set, generate a template with the provided name")

	options.Action.BindForOutput(cmd.Flags())
	cmd.Flags().String("output-version", "", "The preferred API versions of the output objects")

	return cmd
}

func (o *DockerComposeOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, args []string) error {
	version, _ := cmd.Flags().GetString("output-version")
	for _, v := range strings.Split(version, ",") {
		gv, err := unversioned.ParseGroupVersion(v)
		if err != nil {
			return fmt.Errorf("provided output-version %q is not valid: %v", v, err)
		}
		o.OutputVersions = append(o.OutputVersions, gv)
	}
	o.OutputVersions = append(o.OutputVersions, registered.EnabledVersions()...)

	o.Action.Bulk.Mapper = clientcmd.ResourceMapper(f)
	o.Action.Bulk.Op = configcmd.Create
	mapper, _ := f.Object(false)
	o.PrintObject = cmdutil.VersionedPrintObject(f.PrintObject, cmd, mapper, o.Action.Out)

	o.Generator, _ = cmd.Flags().GetString("generator")

	ns, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}
	o.Namespace = ns

	o.Client, _, err = f.Clients()
	return err
}

func (o *DockerComposeOptions) Validate() error {
	if len(o.Filenames) == 0 {
		return fmt.Errorf("you must provide the paths to one or more docker-compose.yml files")
	}
	switch o.Generator {
	case DockerComposeV1GeneratorName:
	default:
		return fmt.Errorf("the generator %q is not supported, use: %s", o.Generator, DockerComposeV1GeneratorName)
	}
	return nil
}

func (o *DockerComposeOptions) Run() error {
	template, err := dockercompose.Generate(o.Filenames...)
	if err != nil {
		return err
	}

	template.ObjectLabels = map[string]string{
		"compose": template.Name,
	}

	// all the types generated into the template should be known
	if errs := app.AsVersionedObjects(template.Objects, kapi.Scheme, kapi.Scheme, o.OutputVersions...); len(errs) > 0 {
		for _, err := range errs {
			fmt.Fprintf(o.Action.ErrOut, "error: %v\n", err)
		}
	}

	if o.Action.ShouldPrint() || (o.Action.Output == "name" && len(o.AsTemplate) > 0) {
		var out runtime.Object
		if len(o.AsTemplate) > 0 {
			template.Name = o.AsTemplate
			out = template
		} else {
			out = &kapi.List{Items: template.Objects}
		}
		return o.PrintObject(out)
	}

	result, err := appcmd.TransformTemplate(template, o.Client, o.Namespace, nil)
	if err != nil {
		return err
	}

	if o.Action.Verbose() {
		appcmd.DescribeGeneratedTemplate(o.Action.Out, "", result, o.Namespace)
	}

	if errs := o.Action.WithMessage("Importing compose file", "created").Run(&kapi.List{Items: result.Objects}, o.Namespace); len(errs) > 0 {
		return cmdutil.ErrExit
	}
	return nil
}
