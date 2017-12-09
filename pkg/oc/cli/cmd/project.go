package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/url"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kclientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	clientcfg "github.com/openshift/origin/pkg/client/config"
	cliconfig "github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectapihelpers "github.com/openshift/origin/pkg/project/apis/project/helpers"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
	projectutil "github.com/openshift/origin/pkg/project/util"

	"github.com/spf13/cobra"
)

type ProjectOptions struct {
	Config       clientcmdapi.Config
	ClientConfig *restclient.Config
	ClientFn     func() (projectclient.ProjectInterface, kclientset.Interface, error)
	Out          io.Writer
	PathOptions  *kclientcmd.PathOptions

	ProjectName  string
	ProjectOnly  bool
	DisplayShort bool

	// SkipAccessValidation means that if a specific name is requested, don't bother checking for access to the project
	SkipAccessValidation bool
}

var (
	projectLong = templates.LongDesc(`
		Switch to another project and make it the default in your configuration

		If no project was specified on the command line, display information about the current active
		project. Since you can use this command to connect to projects on different servers, you will
		occasionally encounter projects of the same name on different servers. When switching to that
		project, a new local context will be created that will have a unique name - for instance,
		'myapp-2'. If you have previously created a context with a different name than the project
		name, this command will accept that context name instead.

		For advanced configuration, or to manage the contents of your config file, use the 'config'
		command.`)

	projectExample = templates.Examples(`
		# Switch to 'myapp' project
	  %[1]s myapp

	  # Display the project currently in use
	  %[1]s`)
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
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.RunProject(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}
	cmd.Flags().BoolVarP(&options.DisplayShort, "short", "q", false, "If true, display only the project name")
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

	o.Config, err = f.OpenShiftClientConfig().RawConfig()
	if err != nil {
		return err
	}

	o.ClientConfig, err = f.ClientConfig()
	if err != nil {
		contextNameExists := false
		if _, exists := o.GetContextFromName(o.ProjectName); exists {
			contextNameExists = exists
		}

		if _, isURLError := err.(*url.Error); !(isURLError || kapierrors.IsInternalError(err)) || !contextNameExists {
			return err
		}

		// if the argument for o.ProjectName passed by the user is a context name,
		// prevent local context-switching from failing due to an unreachable
		// server or an unfetchable ClientConfig.
		o.Config.CurrentContext = o.ProjectName
		if err := kclientcmd.ModifyConfig(o.PathOptions, o.Config, true); err != nil {
			return err
		}

		// since we failed to retrieve ClientConfig for the current server,
		// fetch local OpenShift client config
		o.ClientConfig, err = f.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return err
		}

	}

	o.ClientFn = func() (projectclient.ProjectInterface, kclientset.Interface, error) {
		kc, err := f.ClientSet()
		if err != nil {
			return nil, nil, err
		}
		projectClient, err := f.OpenshiftInternalProjectClient()
		if err != nil {
			return nil, nil, err
		}
		return projectClient.Project(), kc, nil
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

	var currentProject string
	currentContext := config.Contexts[config.CurrentContext]
	if currentContext != nil {
		currentProject = currentContext.Namespace
	}

	// No argument provided, we will just print info
	if len(o.ProjectName) == 0 {
		if len(currentProject) > 0 {
			if o.DisplayShort {
				fmt.Fprintln(out, currentProject)
				return nil
			}

			client, kubeclient, err := o.ClientFn()
			if err != nil {
				return err
			}

			switch err := confirmProjectAccess(currentProject, client, kubeclient); {
			case clientcmd.IsForbidden(err):
				return fmt.Errorf("you do not have rights to view project %q.", currentProject)
			case kapierrors.IsNotFound(err):
				return fmt.Errorf("the project %q specified in your config does not exist.", currentProject)
			case err != nil:
				return err
			}

			defaultContextName := clientcfg.GetContextNickname(currentContext.Namespace, currentContext.Cluster, currentContext.AuthInfo)

			// if they specified a project name and got a generated context, then only show the information they care about.  They won't recognize
			// a context name they didn't choose
			if config.CurrentContext == defaultContextName {
				fmt.Fprintf(out, "Using project %q on server %q.\n", currentProject, clientCfg.Host)

			} else {
				fmt.Fprintf(out, "Using project %q from context named %q on server %q.\n", currentProject, config.CurrentContext, clientCfg.Host)
			}

		} else {
			if o.DisplayShort {
				return fmt.Errorf("no project has been set")
			}
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
	if context, contextExists := o.GetContextFromName(argument); contextExists {
		contextInUse = argument
		namespaceInUse = context.Namespace

		config.CurrentContext = argument
	} else {
		if !o.SkipAccessValidation {
			client, kubeclient, err := o.ClientFn()
			if err != nil {
				return err
			}

			if err := confirmProjectAccess(argument, client, kubeclient); err != nil {
				if isNotFound, isForbidden := kapierrors.IsNotFound(err), clientcmd.IsForbidden(err); isNotFound || isForbidden {
					var msg string
					if isForbidden {
						msg = fmt.Sprintf("You are not a member of project %q.", argument)
					} else {
						msg = fmt.Sprintf("A project named %q does not exist on %q.", argument, clientCfg.Host)
					}

					projects, err := getProjects(client, kubeclient)
					if err == nil {
						switch len(projects) {
						case 0:
							msg += "\nYou are not a member of any projects. You can request a project to be created with the 'new-project' command."
						case 1:
							msg += fmt.Sprintf("\nYou have one project on this server: %s", projectapihelpers.DisplayNameAndNameForProject(&projects[0]))
						default:
							msg += "\nYour projects are:"
							for _, project := range projects {
								msg += fmt.Sprintf("\n* %s", projectapihelpers.DisplayNameAndNameForProject(&project))
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
		}
		projectName := argument

		kubeconfig, err := cliconfig.CreateConfig(projectName, o.ClientConfig)
		if err != nil {
			return err
		}

		merged, err := cliconfig.MergeConfig(config, *kubeconfig)
		if err != nil {
			return err
		}
		config = *merged

		namespaceInUse = projectName
		contextInUse = merged.CurrentContext
	}

	if err := kclientcmd.ModifyConfig(o.PathOptions, config, true); err != nil {
		return err
	}

	if o.DisplayShort {
		fmt.Fprintln(out, namespaceInUse)
		return nil
	}

	// calculate what name we'd generate for the context.  If the context has the same name, don't drop it into the output, because the user won't
	// recognize the name since they didn't choose it.
	defaultContextName := clientcfg.GetContextNickname(namespaceInUse, config.Contexts[contextInUse].Cluster, config.Contexts[contextInUse].AuthInfo)

	switch {
	// if there is no namespace, then the only information we can provide is the context and server
	case (len(namespaceInUse) == 0):
		fmt.Fprintf(out, "Now using context named %q on server %q.\n", contextInUse, clientCfg.Host)

	// inform them that they are already in the project they are trying to switch to
	case currentProject == namespaceInUse:
		fmt.Fprintf(out, "Already on project %q on server %q.\n", currentProject, clientCfg.Host)

	// if they specified a project name and got a generated context, then only show the information they care about.  They won't recognize
	// a context name they didn't choose
	case (argument == namespaceInUse) && (contextInUse == defaultContextName):
		fmt.Fprintf(out, "Now using project %q on server %q.\n", namespaceInUse, clientCfg.Host)

	// in all other cases, display all information
	default:
		fmt.Fprintf(out, "Now using project %q from context named %q on server %q.\n", namespaceInUse, contextInUse, clientCfg.Host)

	}

	return nil
}

// returns a context by the given contextName and a boolean true if the context exists
func (o *ProjectOptions) GetContextFromName(contextName string) (*clientcmdapi.Context, bool) {
	if context, contextExists := o.Config.Contexts[contextName]; !o.ProjectOnly && contextExists {
		return context, true
	}

	return nil, false
}

func confirmProjectAccess(currentProject string, projectClient projectclient.ProjectInterface, kClient kclientset.Interface) error {
	_, projectErr := projectClient.Projects().Get(currentProject, metav1.GetOptions{})
	if !kapierrors.IsNotFound(projectErr) && !kapierrors.IsForbidden(projectErr) {
		return projectErr
	}

	// at this point we know the error is a not found or forbidden, but we'll test namespaces just in case we're running on kube
	if _, err := kClient.Core().Namespaces().Get(currentProject, metav1.GetOptions{}); err == nil {
		return nil
	}

	// otherwise return the openshift error default
	return projectErr
}

func getProjects(projectClient projectclient.ProjectInterface, kClient kclientset.Interface) ([]projectapi.Project, error) {
	projects, err := projectClient.Projects().List(metav1.ListOptions{})
	if err == nil {
		return projects.Items, nil
	}
	// if this is kube with authorization enabled, this endpoint will be forbidden.  OpenShift allows this for everyone.
	if err != nil && !(kapierrors.IsNotFound(err) || kapierrors.IsForbidden(err)) {
		return nil, err
	}

	namespaces, err := kClient.Core().Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	projects = projectutil.ConvertNamespaceList(namespaces)
	return projects.Items, nil
}

func clusterAndAuthEquality(clientCfg *restclient.Config, cluster clientcmdapi.Cluster, authInfo clientcmdapi.AuthInfo) bool {
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
