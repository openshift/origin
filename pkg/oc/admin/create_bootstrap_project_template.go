package admin

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/project/registry/projectrequest/delegated"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

const CreateBootstrapProjectTemplateCommand = "create-bootstrap-project-template"

type CreateBootstrapProjectTemplateOptions struct {
	Name string
}

func NewCommandCreateBootstrapProjectTemplate(f *clientcmd.Factory, commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateBootstrapProjectTemplateOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a bootstrap project template",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				cmdutil.CheckErr(cmdutil.UsageErrorf(cmd, err.Error()))
			}

			template, err := options.CreateBootstrapProjectTemplate()
			if err != nil {
				cmdutil.CheckErr(err)
			}

			mapper, _ := f.Object()
			err = f.PrintObject(cmd, true, mapper, template, out)
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.Name, "name", delegated.DefaultTemplateName, "The name of the template to output.")
	cmdutil.AddPrinterFlags(cmd)

	// Default to JSON
	if flag := cmd.Flags().Lookup("output"); flag != nil {
		flag.Value.Set("json")
	}

	return cmd
}

func (o CreateBootstrapProjectTemplateOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Name) == 0 {
		return errors.New("--name must be provided")
	}

	return nil
}

func (o CreateBootstrapProjectTemplateOptions) CreateBootstrapProjectTemplate() (*templateapi.Template, error) {
	template := delegated.DefaultTemplate()
	template.Name = o.Name
	return template, nil
}
