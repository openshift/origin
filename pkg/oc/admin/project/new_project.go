package project

import (
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	errorsutil "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	oapi "github.com/openshift/origin/pkg/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	authorizationtypedclient "github.com/openshift/origin/pkg/authorization/generated/internalclientset/typed/authorization/internalversion"
	authorizationregistryutil "github.com/openshift/origin/pkg/authorization/registry/util"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/oc/admin/policy"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset/typed/project/internalversion"
)

const NewProjectRecommendedName = "new-project"

type NewProjectOptions struct {
	ProjectName  string
	DisplayName  string
	Description  string
	NodeSelector string

	ProjectClient     projectclient.ProjectInterface
	RoleBindingClient authorizationtypedclient.RoleBindingsGetter
	SARClient         authorizationtypedclient.SubjectAccessReviewInterface

	AdminRole string
	AdminUser string

	Output io.Writer
}

var newProjectLong = templates.LongDesc(`
	Create a new project

	Use this command to create a project. You may optionally specify metadata about the project,
	an admin user (and role, if you want to use a non-default admin role), and a node selector
	to restrict which nodes pods in this project can be scheduled to.`)

// NewCmdNewProject implements the OpenShift cli new-project command
func NewCmdNewProject(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewProjectOptions{}

	cmd := &cobra.Command{
		Use:   name + " NAME [--display-name=DISPLAYNAME] [--description=DESCRIPTION]",
		Short: "Create a new project",
		Long:  newProjectLong,
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			// We can't depend on len(options.NodeSelector) > 0 as node-selector="" is valid
			// and we want to populate node selector as project annotation only if explicitly set by user
			useNodeSelector := cmd.Flag("node-selector").Changed

			if err := options.Run(useNodeSelector); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&options.AdminRole, "admin-role", bootstrappolicy.AdminRoleName, "Project admin role name in the cluster policy")
	cmd.Flags().StringVar(&options.AdminUser, "admin", "", "Project admin username")
	cmd.Flags().StringVar(&options.DisplayName, "display-name", "", "Project display name")
	cmd.Flags().StringVar(&options.Description, "description", "", "Project description")
	cmd.Flags().StringVar(&options.NodeSelector, "node-selector", "", "Restrict pods onto nodes matching given label selector. Format: '<key1>=<value1>, <key2>=<value2>...'. Specifying \"\" means any node, not default. If unspecified, cluster default node selector will be used.")

	return cmd
}

func (o *NewProjectOptions) complete(f *clientcmd.Factory, args []string) error {
	if len(args) != 1 {
		return errors.New("you must specify one argument: project name")
	}

	o.ProjectName = args[0]

	projectClient, err := f.OpenshiftInternalProjectClient()
	if err != nil {
		return err
	}
	o.ProjectClient = projectClient.Project()
	authorizationClient, err := f.OpenshiftInternalAuthorizationClient()
	if err != nil {
		return err
	}
	authorizationInterface := authorizationClient.Authorization()
	o.RoleBindingClient = authorizationInterface
	o.SARClient = authorizationInterface.SubjectAccessReviews()

	return nil
}

func (o *NewProjectOptions) Run(useNodeSelector bool) error {
	if _, err := o.ProjectClient.Projects().Get(o.ProjectName, metav1.GetOptions{}); err != nil {
		if !kerrors.IsNotFound(err) {
			return err
		}
	} else {
		return fmt.Errorf("project %v already exists", o.ProjectName)
	}

	project := &projectapi.Project{}
	project.Name = o.ProjectName
	project.Annotations = make(map[string]string)
	project.Annotations[oapi.OpenShiftDescription] = o.Description
	project.Annotations[oapi.OpenShiftDisplayName] = o.DisplayName
	if useNodeSelector {
		project.Annotations[projectapi.ProjectNodeSelector] = o.NodeSelector
	}
	project, err := o.ProjectClient.Projects().Create(project)
	if err != nil {
		return err
	}

	output := o.Output
	if output == nil {
		output = os.Stdout
	}

	fmt.Fprintf(output, "Created project %v\n", o.ProjectName)

	errs := []error{}
	if len(o.AdminUser) != 0 {
		adduser := &policy.RoleModificationOptions{
			RoleName:            o.AdminRole,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(project.Name, o.RoleBindingClient),
			Users:               []string{o.AdminUser},
		}

		if err := adduser.AddRole(); err != nil {
			fmt.Fprintf(output, "%v could not be added to the %v role: %v\n", o.AdminUser, o.AdminRole, err)
			errs = append(errs, err)
		} else {
			if err := wait.PollImmediate(time.Second, time.Minute, func() (bool, error) {
				resp, err := o.SARClient.Create(&authorizationapi.SubjectAccessReview{
					Action: authorizationapi.Action{
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

	for _, rbacBinding := range bootstrappolicy.GetBootstrapServiceAccountProjectRoleBindings(o.ProjectName) {
		binding, err := authorizationregistryutil.RoleBindingFromRBAC(&rbacBinding)
		if err != nil {
			fmt.Fprintf(output, "Could not convert Role Binding %s in the %q namespace: %v\n", rbacBinding.Name, o.ProjectName, err)
			errs = append(errs, err)
			continue
		}
		addRole := &policy.RoleModificationOptions{
			RoleName:            binding.RoleRef.Name,
			RoleNamespace:       binding.RoleRef.Namespace,
			RoleBindingAccessor: policy.NewLocalRoleBindingAccessor(o.ProjectName, o.RoleBindingClient),
			Subjects:            binding.Subjects,
		}
		if err := addRole.AddRole(); err != nil {
			fmt.Fprintf(output, "Could not add service accounts to the %v role: %v\n", binding.RoleRef.Name, err)
			errs = append(errs, err)
		}
	}

	return errorsutil.NewAggregate(errs)
}
