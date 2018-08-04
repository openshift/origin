package project

import (
	"errors"
	"fmt"
	"net/url"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	projectv1 "github.com/openshift/api/project/v1"
	projectv1client "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	oapi "github.com/openshift/origin/pkg/api"
	clientcfg "github.com/openshift/origin/pkg/client/config"
	cliconfig "github.com/openshift/origin/pkg/oc/lib/kubeconfig"
)

type ProjectOptions struct {
	Config      clientcmdapi.Config
	RESTConfig  *rest.Config
	ClientFn    func() (projectv1client.ProjectV1Interface, corev1client.CoreV1Interface, error)
	PathOptions *kclientcmd.PathOptions

	ProjectName  string
	ProjectOnly  bool
	DisplayShort bool

	// SkipAccessValidation means that if a specific name is requested, don't bother checking for access to the project
	SkipAccessValidation bool

	genericclioptions.IOStreams
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
		%[1]s project myapp

		# Display the project currently in use
		%[1]s project`)
)

func NewProjectOptions(streams genericclioptions.IOStreams) *ProjectOptions {
	return &ProjectOptions{
		IOStreams: streams,
	}
}

// NewCmdProject implements the OpenShift cli rollback command
func NewCmdProject(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewProjectOptions(streams)
	cmd := &cobra.Command{
		Use:     "project [NAME]",
		Short:   "Switch to another project",
		Long:    projectLong,
		Example: fmt.Sprintf(projectExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			o.PathOptions = cliconfig.NewPathOptions(cmd)
			kcmdutil.CheckErr(o.Complete(f, args))
			kcmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().BoolVarP(&o.DisplayShort, "short", "q", o.DisplayShort, "If true, display only the project name")

	return cmd
}

func (o *ProjectOptions) Complete(f genericclioptions.RESTClientGetter, args []string) error {
	switch argsLength := len(args); {
	case argsLength > 1:
		return errors.New("Only one argument is supported (project name or context name).")
	case argsLength == 1:
		o.ProjectName = args[0]
	}

	var err error
	o.Config, err = f.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	o.RESTConfig, err = f.ToRESTConfig()
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
		// server or an unfetchable RESTConfig.
		o.Config.CurrentContext = o.ProjectName
		if err := kclientcmd.ModifyConfig(o.PathOptions, o.Config, true); err != nil {
			return err
		}

		// since we failed to retrieve RESTConfig for the current server,
		// fetch local OpenShift client config
		o.RESTConfig, err = f.ToRESTConfig()
		if err != nil {
			return err
		}

	}

	o.ClientFn = func() (projectv1client.ProjectV1Interface, corev1client.CoreV1Interface, error) {
		kc, err := corev1client.NewForConfig(o.RESTConfig)
		if err != nil {
			return nil, nil, err
		}
		projectClient, err := projectv1client.NewForConfig(o.RESTConfig)
		if err != nil {
			return nil, nil, err
		}
		return projectClient, kc, nil
	}

	return nil
}

// RunProject contains all the necessary functionality for the OpenShift cli project command
func (o ProjectOptions) Run() error {
	config := o.Config
	clientCfg := o.RESTConfig

	var currentProject string
	currentContext := config.Contexts[config.CurrentContext]
	if currentContext != nil {
		currentProject = currentContext.Namespace
	}

	// No argument provided, we will just print info
	if len(o.ProjectName) == 0 {
		if len(currentProject) > 0 {
			client, kubeclient, err := o.ClientFn()
			if err != nil {
				return err
			}

			switch err := ConfirmProjectAccess(currentProject, client, kubeclient); {
			case kapierrors.IsForbidden(err):
				return fmt.Errorf("you do not have rights to view project %q.", currentProject)
			case kapierrors.IsNotFound(err):
				return fmt.Errorf("the project %q specified in your config does not exist.", currentProject)
			case err != nil:
				return err
			}

			if o.DisplayShort {
				fmt.Fprintln(o.Out, currentProject)
				return nil
			}

			defaultContextName := clientcfg.GetContextNickname(currentContext.Namespace, currentContext.Cluster, currentContext.AuthInfo)
			// if they specified a project name and got a generated context, then only show the information they care about.  They won't recognize
			// a context name they didn't choose
			if config.CurrentContext == defaultContextName {
				fmt.Fprintf(o.Out, "Using project %q on server %q.\n", currentProject, clientCfg.Host)
			} else {
				fmt.Fprintf(o.Out, "Using project %q from context named %q on server %q.\n", currentProject, config.CurrentContext, clientCfg.Host)
			}

		} else {
			if o.DisplayShort {
				return fmt.Errorf("no project has been set")
			}
			fmt.Fprintf(o.Out, "No project has been set. Pass a project name to make that the default.\n")
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

			if err := ConfirmProjectAccess(argument, client, kubeclient); err != nil {
				if isNotFound, isForbidden := kapierrors.IsNotFound(err), kapierrors.IsForbidden(err); isNotFound || isForbidden {
					var msg string
					if isForbidden {
						msg = fmt.Sprintf("You are not a member of project %q.", argument)
					} else {
						msg = fmt.Sprintf("A project named %q does not exist on %q.", argument, clientCfg.Host)
					}

					projects, err := GetProjects(client, kubeclient)
					if err == nil {
						switch len(projects) {
						case 0:
							msg += "\nYou are not a member of any projects. You can request a project to be created with the 'new-project' command."
						case 1:
							msg += fmt.Sprintf("\nYou have one project on this server: %s", DisplayNameForProject(&projects[0]))
						default:
							msg += "\nYour projects are:"
							for _, project := range projects {
								msg += fmt.Sprintf("\n* %s", DisplayNameForProject(&project))
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

		kubeconfig, err := cliconfig.CreateConfig(projectName, o.RESTConfig)
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
		fmt.Fprintln(o.Out, namespaceInUse)
		return nil
	}

	// calculate what name we'd generate for the context.  If the context has the same name, don't drop it into the output, because the user won't
	// recognize the name since they didn't choose it.
	defaultContextName := clientcfg.GetContextNickname(namespaceInUse, config.Contexts[contextInUse].Cluster, config.Contexts[contextInUse].AuthInfo)

	switch {
	// if there is no namespace, then the only information we can provide is the context and server
	case (len(namespaceInUse) == 0):
		fmt.Fprintf(o.Out, "Now using context named %q on server %q.\n", contextInUse, clientCfg.Host)

	// inform them that they are already in the project they are trying to switch to
	case currentProject == namespaceInUse:
		fmt.Fprintf(o.Out, "Already on project %q on server %q.\n", currentProject, clientCfg.Host)

	// if they specified a project name and got a generated context, then only show the information they care about.  They won't recognize
	// a context name they didn't choose
	case (argument == namespaceInUse) && (contextInUse == defaultContextName):
		fmt.Fprintf(o.Out, "Now using project %q on server %q.\n", namespaceInUse, clientCfg.Host)

	// in all other cases, display all information
	default:
		fmt.Fprintf(o.Out, "Now using project %q from context named %q on server %q.\n", namespaceInUse, contextInUse, clientCfg.Host)

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

func ConfirmProjectAccess(currentProject string, projectClient projectv1client.ProjectV1Interface, kClient corev1client.CoreV1Interface) error {
	_, projectErr := projectClient.Projects().Get(currentProject, metav1.GetOptions{})
	if !kapierrors.IsNotFound(projectErr) && !kapierrors.IsForbidden(projectErr) {
		return projectErr
	}

	// at this point we know the error is a not found or forbidden, but we'll test namespaces just in case we're running on kube
	if _, err := kClient.Namespaces().Get(currentProject, metav1.GetOptions{}); err == nil {
		return nil
	}

	// otherwise return the openshift error default
	return projectErr
}

func GetProjects(projectClient projectv1client.ProjectV1Interface, kClient corev1client.CoreV1Interface) ([]projectv1.Project, error) {
	projects, err := projectClient.Projects().List(metav1.ListOptions{})
	if err == nil {
		return projects.Items, nil
	}
	// if this is kube with authorization enabled, this endpoint will be forbidden.  OpenShift allows this for everyone.
	if err != nil && !(kapierrors.IsNotFound(err) || kapierrors.IsForbidden(err)) {
		return nil, err
	}

	namespaces, err := kClient.Namespaces().List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	projects = convertNamespaceList(namespaces)
	return projects.Items, nil
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

func convertNamespaceList(namespaceList *corev1.NamespaceList) *projectv1.ProjectList {
	projects := &projectv1.ProjectList{}
	for _, ns := range namespaceList.Items {
		projects.Items = append(projects.Items, projectv1.Project{
			ObjectMeta: ns.ObjectMeta,
			Spec: projectv1.ProjectSpec{
				Finalizers: ns.Spec.Finalizers,
			},
			Status: projectv1.ProjectStatus{
				Phase: ns.Status.Phase,
			},
		})
	}
	return projects
}

func DisplayNameForProject(project *projectv1.Project) string {
	displayName := project.Annotations[oapi.OpenShiftDisplayName]
	if len(displayName) == 0 {
		displayName = project.Annotations["displayName"]
	}
	if len(displayName) > 0 && displayName != project.Name {
		return fmt.Sprintf("%s (%s)", displayName, project.Name)
	}
	return project.Name
}
