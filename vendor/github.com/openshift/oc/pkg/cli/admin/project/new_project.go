package project

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/api/annotations"
	authorizationv1 "github.com/openshift/api/authorization/v1"
	projectv1 "github.com/openshift/api/project/v1"
	authorizationv1typedclient "github.com/openshift/client-go/authorization/clientset/versioned/typed/authorization/v1"
	projectv1typedclient "github.com/openshift/client-go/project/clientset/versioned/typed/project/v1"
	"github.com/openshift/oc/pkg/cli/admin/policy"
)

const NewProjectRecommendedName = "new-project"

type NewProjectOptions struct {
	ProjectName  string
	DisplayName  string
	Description  string
	NodeSelector string

	UseNodeSelector bool
	ProjectClient   projectv1typedclient.ProjectV1Interface
	RbacClient      rbacv1client.RbacV1Interface
	SARClient       authorizationv1typedclient.SubjectAccessReviewInterface

	AdminRole string
	AdminUser string

	genericclioptions.IOStreams
}

var newProjectLong = templates.LongDesc(`
	Create a new project

	Use this command to create a project. You may optionally specify metadata about the project,
	an admin user (and role, if you want to use a non-default admin role), and a node selector
	to restrict which nodes pods in this project can be scheduled to.`)

func NewNewProjectOptions(streams genericclioptions.IOStreams) *NewProjectOptions {
	return &NewProjectOptions{
		AdminRole: "admin",
		IOStreams: streams,
	}
}

// NewCmdNewProject implements the OpenShift cli new-project command
func NewCmdNewProject(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewNewProjectOptions(streams)
	cmd := &cobra.Command{
		Use:   name + " NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]",
		Short: "Create a new project",
		Long:  newProjectLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, cmd, args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.AdminRole, "admin-role", o.AdminRole, "Project admin role name in the cluster policy")
	cmd.Flags().StringVar(&o.AdminUser, "admin", o.AdminUser, "Project admin username")
	cmd.Flags().StringVar(&o.DisplayName, "display-name", o.DisplayName, "Project display name")
	cmd.Flags().StringVar(&o.Description, "description", o.Description, "Project description")
	cmd.Flags().StringVar(&o.NodeSelector, "node-selector", o.NodeSelector, "Restrict pods onto nodes matching given label selector. Format: '<key1>=<value1>, <key2>=<value2>...'. Specifying \"\" means any node, not default. If unspecified, cluster default node selector will be used.")

	return cmd
}

func (o *NewProjectOptions) Complete(f kcmdutil.Factory, cmd *cobra.Command, args []string) error {
	if len(args) != 1 {
		return errors.New("you must specify one argument: project name")
	}

	// We can't depend on len(options.NodeSelector) > 0 as node-selector="" is valid
	// and we want to populate node selector as project annotation only if explicitly set by user
	o.UseNodeSelector = cmd.Flag("node-selector").Changed

	o.ProjectName = args[0]

	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.ProjectClient, err = projectv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.RbacClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	authorizationClient, err := authorizationv1typedclient.NewForConfig(clientConfig)
	if err != nil {
		return err
	}
	o.SARClient = authorizationClient.SubjectAccessReviews()

	return nil
}

func (o *NewProjectOptions) Run() error {
	if _, err := o.ProjectClient.Projects().Get(o.ProjectName, metav1.GetOptions{}); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		return fmt.Errorf("project %v already exists", o.ProjectName)
	}

	project := &projectv1.Project{}
	project.Name = o.ProjectName
	project.Annotations = make(map[string]string)
	project.Annotations[annotations.OpenShiftDescription] = o.Description
	project.Annotations[annotations.OpenShiftDisplayName] = o.DisplayName
	if o.UseNodeSelector {
		project.Annotations[projectv1.ProjectNodeSelector] = o.NodeSelector
	}
	project, err := o.ProjectClient.Projects().Create(project)
	if err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Created project %v\n", o.ProjectName)

	errs := []error{}
	if len(o.AdminUser) != 0 {
		adduser := policy.NewRoleModificationOptions(o.IOStreams)
		adduser.RoleName = o.AdminRole
		adduser.RoleKind = "ClusterRole"
		adduser.RoleBindingNamespace = project.Name
		adduser.RbacClient = o.RbacClient
		adduser.Users = []string{o.AdminUser}
		adduser.ToPrinter = noopPrinter

		if err := adduser.AddRole(); err != nil {
			fmt.Fprintf(o.Out, "%v could not be added to the %v role: %v\n", o.AdminUser, o.AdminRole, err)
			errs = append(errs, err)
		} else {
			if err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
				resp, err := o.SARClient.Create(&authorizationv1.SubjectAccessReview{
					Action: authorizationv1.Action{
						Namespace: o.ProjectName,
						Verb:      "get",
						Resource:  "projects",
					},
					User: o.AdminUser,
				})
				if err != nil {
					return false, err
				}
				if !resp.Allowed {
					return false, nil
				}
				return true, nil
			}); err != nil {
				fmt.Printf("%s is not able to get project %s with the %s role: %v\n", o.AdminUser, o.ProjectName, o.AdminRole, err)
				errs = append(errs, err)
			}
		}
	}

	return errorsutil.NewAggregate(errs)
}

func noopPrinter(operation string) (printers.ResourcePrinter, error) {
	return printers.NewDiscardingPrinter(), nil
}
