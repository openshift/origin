package cmd

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/api"
)

type NewProjectOptions struct {
	ProjectName string
	DisplayName string
	Description string

	Client client.Interface
}

const requestProjectLongDesc = `
Create a new project for yourself in OpenShift with you as the project admin.

Assuming your cluster admin has granted you permission, this command will 
create a new project for you and assign you as the project admin.  You must 
be logged in, so you might have to run %[2]s first.

Examples:

	$ Create a new project with minimal information
	$ %[1]s web-team-dev

	# Create a new project with a description
	$ %[1]s web-team-dev --display-name="Web Team Development" --description="Development project for the web team."

After your project is created you can switch to it using %[3]s <project name>.
`

func NewCmdRequestProject(name, fullName, oscLoginName, oscProjectName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <project-name> [--display-name=<your display name> --description=<your description]", name),
		Short: "request a new project",
		Long:  fmt.Sprintf(requestProjectLongDesc, fullName, oscLoginName, oscProjectName),
		Run: func(cmd *cobra.Command, args []string) {
			if !options.complete(cmd) {
				return
			}

			var err error
			if options.Client, _, err = f.Clients(); err != nil {
				glog.Fatalf("Error getting client: %v", err)
			}
			if err := options.Run(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	cmd.SetOutput(out)

	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "project description")

	return cmd
}

func (o *NewProjectOptions) complete(cmd *cobra.Command) bool {
	args := cmd.Flags().Args()
	if len(args) != 1 {
		cmd.Help()
		return false
	}

	o.ProjectName = args[0]

	return true
}

func (o *NewProjectOptions) Run() error {
	projectRequest := &projectapi.ProjectRequest{}
	projectRequest.Name = o.ProjectName
	projectRequest.DisplayName = o.DisplayName
	projectRequest.Annotations = make(map[string]string)
	projectRequest.Annotations["description"] = o.Description
	if _, err := o.Client.ProjectRequests().Create(projectRequest); err != nil {
		return err
	}

	return nil
}
