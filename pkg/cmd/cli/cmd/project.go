package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kubecmdconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	cliconfig "github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/project/api"

	"github.com/spf13/cobra"
)

type ProjectOptions struct {
	Config       clientcmdapi.Config
	Client       *client.Client
	ClientConfig *kclient.Config
	Out          io.Writer
	PathOptions  *kubecmdconfig.PathOptions

	ProjectName string
	ProjectOnly bool
}

const (
	projectLong = `
Switch to another project and make it the default in your configuration

If no project was specified on the command line, display information about the current active
project. Since you can use this command to connect to projects on different servers, you will
occasionally encounter projects of the same name on different servers. When switching to that
project, a new local context will be created that will have a unique name - for instance,
'myapp-2'. If you have previously created a context with a different name than the project
name, this command will accept that context name instead.

For advanced configuration, or to manage the contents of your config file, use the 'config'
command.`

	projectExample = `  // Switch to 'myapp' project
  $ %[1]s myapp

  // Display the project currently in use
  $ %[1]s`
)

// NewCmdProject implements the OpenShift cli rollback command
func NewCmdProject(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &ProjectOptions{}

	cmd := &cobra.Command{
		Use:     "project [NAME]",
		Short:   "Switch to another project",
		Long:    projectLong,
		Example: fmt.Sprintf(projectExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			options.PathOptions = cliconfig.NewPathOptions(cmd)

			if err := options.Complete(f, args, out); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.RunProject(); err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}
	return cmd
}

func (o *ProjectOptions) Complete(f *clientcmd.Factory, args []string, out io.Writer) error {
	var err error

	argsLength := len(args)
	switch {
	case argsLength > 1:
		return errors.New("Only one argument is supported (project name or context name).")
	case argsLength == 1:
		o.ProjectName = args[0]
	}

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
func (o ProjectOptions) Validate() error {
	return nil
}

// RunProject contains all the necessary functionality for the OpenShift cli project command
func (o ProjectOptions) RunProject() error {
	config := o.Config
	clientCfg := o.ClientConfig
	out := o.Out

	// No argument provided, we will just print info
	if len(o.ProjectName) == 0 {
		currentContext := config.Contexts[config.CurrentContext]
		currentProject := currentContext.Namespace

		if len(currentProject) > 0 {
			_, err := o.Client.Projects().Get(currentProject)
			if err != nil {
				if kapierrors.IsNotFound(err) {
					return fmt.Errorf("the project %q specified in your config does not exist.", currentProject)
				}
				if clientcmd.IsForbidden(err) {
					return fmt.Errorf("you do not have rights to view project %q.", currentProject)
				}
				return err
			}

			if config.CurrentContext != currentProject {
				if len(currentProject) > 0 {
					fmt.Fprintf(out, "Using project %q from context named %q on server %q.\n", currentProject, config.CurrentContext, clientCfg.Host)
				} else {
					fmt.Fprintf(out, "Using context named %q on server %q.\n", config.CurrentContext, clientCfg.Host)
				}
			} else {
				fmt.Fprintf(out, "Using project %q on server %q.\n", currentProject, clientCfg.Host)
			}

		} else {
			fmt.Fprintf(out, "No project has been set. Pass a project name to make that the default.\n")
		}
		return nil
	}

	// We have an argument that can be either a context or project
	argument := o.ProjectName

	contextInUse := ""
	namespaceInUse := ""
	contextNameIsGenerated := false

	// Check if argument is an existing context, if so just set it as the context in use.
	// If not a context then we will try to handle it as a project.
	if context, contextExists := config.Contexts[argument]; !o.ProjectOnly && contextExists {
		contextInUse = argument
		namespaceInUse = context.Namespace

		config.CurrentContext = argument

	} else {
		project, err := o.Client.Projects().Get(argument)
		if err != nil {
			if isNotFound, isForbidden := kapierrors.IsNotFound(err), clientcmd.IsForbidden(err); isNotFound || isForbidden {
				var msg string
				if isForbidden {
					msg = fmt.Sprintf("You are not a member of project %q.", argument)
				} else {
					msg = fmt.Sprintf("A project named %q does not exist on %q.", argument, clientCfg.Host)
				}

				projects, err := getProjects(o.Client)
				if err == nil {
					switch len(projects) {
					case 0:
						msg += "\nYou are not a member of any projects. You can request a project to be created with the 'new-project' command."
					case 1:
						msg += fmt.Sprintf("\nYou have one project on this server: %s", api.DisplayNameAndNameForProject(&projects[0]))
					default:
						msg += "\nYour projects are:"
						for _, project := range projects {
							msg += fmt.Sprintf("\n* %s", api.DisplayNameAndNameForProject(&project))
						}
					}
				}

				if hasMultipleServers(config) {
					msg += "\nTo see projects on another server, pass '--server=<server>'."
				}
				return errors.New(msg)
			}
			return err
		}

		kubeconfig, err := cliconfig.CreateConfig(project.Name, o.ClientConfig)
		if err != nil {
			return err
		}

		merged, err := cliconfig.MergeConfig(config, *kubeconfig)
		if err != nil {
			return err
		}
		config = *merged

		namespaceInUse = project.Name
		contextInUse = merged.CurrentContext
		contextNameIsGenerated = true
	}

	if err := kubecmdconfig.ModifyConfig(o.PathOptions, config); err != nil {
		return err
	}

	if contextInUse != namespaceInUse && !contextNameIsGenerated {
		if len(namespaceInUse) > 0 {
			fmt.Fprintf(out, "Now using project %q from context named %q on server %q.\n", namespaceInUse, contextInUse, clientCfg.Host)
		} else {
			fmt.Fprintf(out, "Now using context named %q on server %q.\n", contextInUse, clientCfg.Host)
		}
	} else {
		fmt.Fprintf(out, "Now using project %q on server %q.\n", namespaceInUse, clientCfg.Host)
	}
	return nil
}

func getProjects(oClient *client.Client) ([]api.Project, error) {
	projects, err := oClient.Projects().List(labels.Everything(), fields.Everything())
	if err != nil {
		return nil, err
	}
	return projects.Items, nil
}

func clusterAndAuthEquality(clientCfg *kclient.Config, cluster clientcmdapi.Cluster, authInfo clientcmdapi.AuthInfo) bool {
	return cluster.Server == clientCfg.Host &&
		cluster.InsecureSkipTLSVerify == clientCfg.Insecure &&
		cluster.CertificateAuthority == clientCfg.CAFile &&
		bytes.Equal(cluster.CertificateAuthorityData, clientCfg.CAData) &&
		authInfo.Token == clientCfg.BearerToken &&
		authInfo.ClientCertificate == clientCfg.TLSClientConfig.CertFile &&
		bytes.Equal(authInfo.ClientCertificateData, clientCfg.TLSClientConfig.CertData) &&
		authInfo.ClientKey == clientCfg.TLSClientConfig.KeyFile &&
		bytes.Equal(authInfo.ClientKeyData, clientCfg.TLSClientConfig.KeyData)
}

// TODO these kind of funcs could be moved to some kind of clientcmd util
func hasMultipleServers(config clientcmdapi.Config) bool {
	server := ""
	for _, cluster := range config.Clusters {
		if len(server) == 0 {
			server = cluster.Server
		}
		if server != cluster.Server {
			return true
		}
	}
	return false
}
