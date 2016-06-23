package cmd

import (
	"fmt"
	"io"

	"k8s.io/kubernetes/pkg/client/restclient"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/project/api"

	"github.com/spf13/cobra"
)

type ProjectsOptions struct {
	Config       clientcmdapi.Config
	ClientConfig *restclient.Config
	Client       *client.Client
	Out          io.Writer
	PathOptions  *kclientcmd.PathOptions

	DisplayShort bool
}

const (
	projectsLong = `
Display information about the current active project and existing projects on the server.

For advanced configuration, or to manage the contents of your config file, use the 'config'
command.`

	projectsExample = `  # Display the projects that currently exist
  %[1]s`
)

// NewCmdProjects implements the OpenShift cli rollback command
func NewCmdProjects(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &ProjectsOptions{}

	cmd := &cobra.Command{
		Use:     "projects",
		Short:   "Display existing projects",
		Long:    projectsLong,
		Example: fmt.Sprintf(projectsExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			options.PathOptions = cliconfig.NewPathOptions(cmd)

			if err := options.Complete(f, args, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RunProjects(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVarP(&options.DisplayShort, "short", "q", false, "If true, display only the project names")
	return cmd
}

func (o *ProjectsOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("no arguments should be passed")
	}

	var err error
	o.Config, err = f.OpenShiftClientConfig.RawConfig()
	if err != nil {
		return err
	}

	o.ClientConfig, err = f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	o.Client, _, err = f.Clients()
	if err != nil {
		return err
	}

	o.Out = out

	return nil
}

// RunProjects lists all projects a user belongs to
func (o ProjectsOptions) RunProjects() error {
	config := o.Config
	clientCfg := o.ClientConfig
	out := o.Out

	currentContext := config.Contexts[config.CurrentContext]
	currentProject := currentContext.Namespace

	var currentProjectExists bool = false
	var currentProjectErr error = nil

	client := o.Client

	if len(currentProject) > 0 {
		if _, currentProjectErr := client.Projects().Get(currentProject); currentProjectErr == nil {
			currentProjectExists = true
		}
	}

	defaultContextName := cliconfig.GetContextNickname(currentContext.Namespace, currentContext.Cluster, currentContext.AuthInfo)

	var msg string
	projects, err := getProjects(client)
	if err == nil {
		switch len(projects) {
		case 0:
			msg += "You are not a member of any projects. You can request a project to be created with the 'new-project' command."
		case 1:
			if o.DisplayShort {
				msg += fmt.Sprintf("%s", api.DisplayNameAndNameForProject(&projects[0]))
			} else {
				msg += fmt.Sprintf("You have one project on this server: %q.", api.DisplayNameAndNameForProject(&projects[0]))
			}
		default:
			asterisk := ""
			count := 0
			if !o.DisplayShort {
				msg += "You have access to the following projects and can switch between them with 'oc project <projectname>':\n"
			}
			for _, project := range projects {
				count = count + 1
				displayName := project.Annotations["openshift.io/display-name"]
				linebreak := "\n"
				if len(displayName) == 0 {
					displayName = project.Annotations["displayName"]
				}

				if currentProjectExists && !o.DisplayShort {
					asterisk = "    "
					if currentProject == project.Name {
						asterisk = "  * "
					}
				}
				if len(displayName) > 0 && displayName != project.Name && !o.DisplayShort {
					msg += fmt.Sprintf("\n  "+asterisk+"%s (%s)", displayName, project.Name)
				} else {
					if o.DisplayShort && count == 1 {
						linebreak = ""
					}
					msg += fmt.Sprintf(linebreak+asterisk+"%s", project.Name)
				}
			}
		}
		fmt.Println(msg)

		if len(projects) > 0 && !o.DisplayShort {
			if !currentProjectExists {
				if clientcmd.IsForbidden(currentProjectErr) {
					fmt.Printf("you do not have rights to view project %q. Please switch to an existing one.", currentProject)
				}
				return currentProjectErr
			}

			// if they specified a project name and got a generated context, then only show the information they care about.  They won't recognize
			// a context name they didn't choose
			if config.CurrentContext == defaultContextName {
				fmt.Fprintf(out, "\nUsing project %q on server %q.\n", currentProject, clientCfg.Host)
			} else {
				fmt.Fprintf(out, "\nUsing project %q from context named %q on server %q.\n", currentProject, config.CurrentContext, clientCfg.Host)
			}
		}
		return nil
	}

	return err
}
