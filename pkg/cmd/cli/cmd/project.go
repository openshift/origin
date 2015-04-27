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
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

// NewCmdProject implements the OpenShift cli rollback command
func NewCmdProject(f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &ProjectOptions{}

	cmd := &cobra.Command{
		Use:   "project <project-name>",
		Short: "switch to another project",
		Long:  `Switch to another project and make it the default in your configuration.`,
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
		return errors.New("Only one argument is supported (project name).")
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

	// Check if argument is an existing context, if so just set it as the context in use.
	// If not a context then we will try to handle it as a project.
	if context, ok := config.Contexts[argument]; !o.ProjectOnly && (ok && len(context.Namespace) > 0) {
		contextInUse = argument
		namespaceInUse = context.Namespace

		config.CurrentContext = argument

	} else {
		project, err := o.Client.Projects().Get(argument)
		if err != nil {
			if isNotFound, isForbidden := kapierrors.IsNotFound(err), clientcmd.IsForbidden(err); isNotFound || isForbidden {
				msg := ""

				if isNotFound {
					msg = fmt.Sprintf("A project named %q does not exist on server %q.", argument, clientCfg.Host)
				} else {
					msg = fmt.Sprintf("You do not have rights to view project %q on server %q.", argument, clientCfg.Host)
				}

				projects, err := getProjects(o.Client)
				if err == nil {
					msg += "\nYour projects are:"
					for _, project := range projects {
						msg += "\n" + project.Name
					}
				}

				if hasMultipleServers(config) {
					msg += "\nTo see projects on another server, pass '--server=<server>'."
				}
				fmt.Fprintln(out, msg)
				return errExit
			}
			return err
		}

		// If a context exists, just set it as the current one.
		exists := false
		for k, ctx := range config.Contexts {
			namespace := ctx.Namespace
			cluster := config.Clusters[ctx.Cluster]
			authInfo := config.AuthInfos[ctx.AuthInfo]

			if len(namespace) > 0 && namespace == project.Name && clusterAndAuthEquality(clientCfg, cluster, authInfo) {
				exists = true
				config.CurrentContext = k

				contextInUse = k
				namespaceInUse = namespace

				break
			}
		}

		// Otherwise create a new context, reusing the cluster and auth info
		if !exists {
			currentCtx := config.CurrentContext

			newCtx := clientcmdapi.NewContext()
			newCtx.Namespace = project.Name

			newCtx.AuthInfo = config.Contexts[currentCtx].AuthInfo
			newCtx.Cluster = config.Contexts[currentCtx].Cluster

			existingContexIdentifiers := &util.StringSet{}
			for key := range config.Contexts {
				existingContexIdentifiers.Insert(key)
			}

			newCtxName := cliconfig.GenerateContextIdentifier(newCtx.Namespace, newCtx.Cluster, "", existingContexIdentifiers)

			config.Contexts[newCtxName] = *newCtx
			config.CurrentContext = newCtxName

			contextInUse = newCtxName
			namespaceInUse = project.Name
		}
	}

	if err := kubecmdconfig.ModifyConfig(o.PathOptions, config); err != nil {
		return err
	}

	if contextInUse != namespaceInUse {
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
