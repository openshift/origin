package cmd

import (
	"fmt"
	"io"
	"sort"

	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	oapi "github.com/openshift/origin/pkg/api"
	clientcfg "github.com/openshift/origin/pkg/client/config"
	cliconfig "github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectapihelpers "github.com/openshift/origin/pkg/project/apis/project/helpers"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"

	"github.com/spf13/cobra"
)

type ProjectsOptions struct {
	Config       clientcmdapi.Config
	ClientConfig *restclient.Config
	Client       projectclient.ProjectInterface
	KubeClient   kclientset.Interface
	Out          io.Writer
	PathOptions  *kclientcmd.PathOptions

	// internal strings
	CommandName string

	DisplayShort bool
}

// SortByProjectName is sort
type SortByProjectName []projectapi.Project

func (p SortByProjectName) Len() int {
	return len(p)
}
func (p SortByProjectName) Swap(i, j int) {
	p[i], p[j] = p[j], p[i]
}
func (p SortByProjectName) Less(i, j int) bool {
	return p[i].Name < p[j].Name
}

var (
	projectsLong = templates.LongDesc(`
		Display information about the current active project and existing projects on the server.

		For advanced configuration, or to manage the contents of your config file, use the 'config'
		command.`)
)

// NewCmdProjects implements the OpenShift cli rollback command
func NewCmdProjects(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &ProjectsOptions{}

	cmd := &cobra.Command{
		Use:   "projects",
		Short: "Display existing projects",
		Long:  projectsLong,
		Run: func(cmd *cobra.Command, args []string) {
			options.PathOptions = cliconfig.NewPathOptions(cmd)

			if err := options.Complete(f, args, fullName, out); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RunProjects(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVarP(&options.DisplayShort, "short", "q", false, "If true, display only the project names")
	return cmd
}

func (o *ProjectsOptions) Complete(f *clientcmd.Factory, args []string, commandName string, out io.Writer) error {
	if len(args) > 0 {
		return fmt.Errorf("no arguments should be passed")
	}

	o.CommandName = commandName

	var err error
	o.Config, err = f.OpenShiftClientConfig().RawConfig()
	if err != nil {
		return err
	}

	o.ClientConfig, err = f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return err
	}

	o.KubeClient, err = f.ClientSet()
	if err != nil {
		return err
	}
	projectClient, err := f.OpenshiftInternalProjectClient()
	if err != nil {
		return err
	}
	o.Client = projectClient.Project()

	o.Out = out

	return nil
}

// RunProjects lists all projects a user belongs to
func (o ProjectsOptions) RunProjects() error {
	config := o.Config
	clientCfg := o.ClientConfig
	out := o.Out

	var currentProject string
	currentContext := config.Contexts[config.CurrentContext]
	if currentContext != nil {
		currentProject = currentContext.Namespace
	}

	var currentProjectExists bool
	var currentProjectErr error

	client := o.Client

	if len(currentProject) > 0 {
		if currentProjectErr := confirmProjectAccess(currentProject, o.Client, o.KubeClient); currentProjectErr == nil {
			currentProjectExists = true
		}
	}

	var defaultContextName string
	if currentContext != nil {
		defaultContextName = clientcfg.GetContextNickname(currentContext.Namespace, currentContext.Cluster, currentContext.AuthInfo)
	}

	var msg string
	projects, err := getProjects(client, o.KubeClient)
	if err == nil {
		switch len(projects) {
		case 0:
			if !o.DisplayShort {
				msg += "You are not a member of any projects. You can request a project to be created with the 'new-project' command."
			}
		case 1:
			if o.DisplayShort {
				msg += fmt.Sprintf("%s", projects[0].Name)
			} else {
				msg += fmt.Sprintf("You have one project on this server: %q.", projectapihelpers.DisplayNameAndNameForProject(&projects[0]))
			}
		default:
			asterisk := ""
			count := 0
			if !o.DisplayShort {
				msg += fmt.Sprintf("You have access to the following projects and can switch between them with '%s project <projectname>':\n", o.CommandName)
			}

			sort.Sort(SortByProjectName(projects))
			for _, project := range projects {
				count = count + 1
				displayName := project.Annotations[oapi.OpenShiftDisplayName]
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
					msg += fmt.Sprintf("\n"+asterisk+"%s - %s", project.Name, displayName)
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
					fmt.Printf("You do not have rights to view project %q. Please switch to an existing one.\n", currentProject)
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
