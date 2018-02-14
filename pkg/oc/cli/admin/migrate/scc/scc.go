package scc

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	securityv1 "github.com/openshift/api/security/v1"
	"github.com/openshift/origin/pkg/oc/cli/admin/migrate"
	policy "k8s.io/api/policy/v1beta1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	policyv1beta1client "k8s.io/client-go/kubernetes/typed/policy/v1beta1"
	rbacv1client "k8s.io/client-go/kubernetes/typed/rbac/v1"
	rbacv1helper "k8s.io/kubernetes/pkg/apis/rbac/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/resource"
)

// clusterRoleAndBindingNamePrefix is a prefix that each auto-generated ClusterRole and ClusterRoleBinding have.
// It simplifies their identification.
const clusterRoleAndBindingNamePrefix = "psp:"

var (
	internalMigrateSCCShort = "Converts SCCs to similar PSPs"
	internalMigrateSCCLong  = templates.LongDesc(`
		Converts SecurityContextConstraints to the equivalent PodSecurityPolicy objects.
		For each SCC it also creates a ClusterRole with the similar name and ClusterRoleBinding
		for every user or group that SCC has. To easy identify such auto-generated roles and bindings,
		their names are prefixed by 'psp:' string.

		NOTE: it does not modify anything by default.`)

	internalMigrateSCCExample = templates.Examples(`
		# Perform a dry-run of migrating all objects
		%[1]s

		# To actually perform the migration, the confirm flag must be appended
		%[1]s --confirm`)
)

type MigrateSCCOptions struct {
	kubeRbacClient   *rbacv1client.RbacV1Client
	kubePolicyClient *policyv1beta1client.PolicyV1beta1Client

	migrate.ResourceOptions
}

func NewMigrateSCCOptions(streams genericclioptions.IOStreams) *MigrateSCCOptions {
	return &MigrateSCCOptions{
		ResourceOptions: *migrate.NewResourceOptions(streams).WithIncludes([]string{"securitycontextconstraints.security.openshift.io"}).WithAllNamespaces(),
	}
}

func NewCmdMigrateSCC(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMigrateSCCOptions(streams)

	cmd := &cobra.Command{
		Use:     name,
		Short:   internalMigrateSCCShort,
		Long:    internalMigrateSCCLong,
		Example: fmt.Sprintf(internalMigrateSCCExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(name, f, cmd, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	o.ResourceOptions.Bind(cmd)

	// these flags aren't applied for SCC migrations
	cmd.Flags().MarkHidden("to-key")
	cmd.Flags().MarkHidden("from-key")
	cmd.Flags().MarkHidden("include")
	cmd.Flags().MarkHidden("filename")
	cmd.Flags().MarkHidden("show-all")
	cmd.Flags().MarkHidden("no-headers")
	cmd.Flags().MarkHidden("show-labels")
	cmd.Flags().MarkHidden("all-namespaces")

	return cmd
}

func (o *MigrateSCCOptions) Complete(name string, f kcmdutil.Factory, c *cobra.Command, args []string) error {
	o.ResourceOptions.SaveFn = o.save
	if err := o.ResourceOptions.Complete(f, c); err != nil {
		return err
	}
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.kubeRbacClient, err = rbacv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	o.kubePolicyClient, err = policyv1beta1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return nil
}

func (o *MigrateSCCOptions) Validate() error {
	if len(o.ResourceOptions.Include) != 1 || o.ResourceOptions.Include[0] != "securitycontextconstraints.security.openshift.io" {
		return fmt.Errorf("the only supported resources are securitycontextconstraints")
	}
	return o.ResourceOptions.Validate()
}

func (o *MigrateSCCOptions) Run() error {
	return o.ResourceOptions.Visitor().Visit(func(info *resource.Info) (migrate.Reporter, error) {
		if _, ok := info.Object.(*securityv1.SecurityContextConstraints); !ok {
			return nil, fmt.Errorf("unrecognized object %#v", info.Object)
		}

		// all SCCs have to be migrated
		return migrate.ReporterBool(true), nil
	})
}

// save invokes the API to alter an object. The reporter passed to this method is the same returned by
// the migration visitor method. It should return an error  if the input type cannot be saved
// It returns migrate.ErrRecalculate if migration should be re-run on the provided object.
func (o *MigrateSCCOptions) save(info *resource.Info, reporter migrate.Reporter) error {
	scc, ok := info.Object.(*securityv1.SecurityContextConstraints)
	if !ok {
		return fmt.Errorf("unrecognized object %#v", info.Object)
	}

	// TODO: do we need always return migrate.DefaultRetriable(info, err) ?
	psp, err := convertSccToPsp(scc)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "ERROR: could not create PSP for SCC %q: %v\n", scc.Name, err)
		return err
	}

	role, err := convertSccToClusterRole(scc)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "ERROR: could not create cluster role for SCC %q: %v\n", scc.Name, err)
		return err
	}

	binding, err := convertSccToClusterRoleBinding(scc)
	if err != nil {
		fmt.Fprintf(o.ErrOut, "ERROR: could not create cluster role binding for SCC %q: %v\n", scc.Name, err)
		return err
	}

	if _, err = o.kubePolicyClient.PodSecurityPolicies().Create(psp); err != nil {
		fmt.Fprintf(o.ErrOut, "ERROR: could not create PSP %q: %v\n", psp.Name, err)
		return err
	}

	// cluster role and binding is optional:
	// we don't create them if the access to SCC hasn't been granted to someone
	if role != nil && binding != nil {
		_, err = o.kubeRbacClient.ClusterRoles().Create(role)
		if err == nil {
			_, err = o.kubeRbacClient.ClusterRoleBindings().Create(binding)
		}
		if err != nil {
			return err
		}
	}

	return migrate.DefaultRetriable(info, err)
}

func convertSccToPsp(scc *securityv1.SecurityContextConstraints) (*policy.PodSecurityPolicy, error) {
	annotations := make(map[string]string)
	extractSeccompProfiles(scc, annotations)
	extractSysctls(scc, annotations)

	selinux, err := extractSELinux(scc)
	if err != nil {
		return nil, err
	}

	runAsUser, err := extractRunAsUser(scc)
	if err != nil {
		return nil, err
	}

	supplementalGroups, err := extractSupplementalGroups(scc)
	if err != nil {
		return nil, err
	}

	fsGroup, err := extractFSGroup(scc)
	if err != nil {
		return nil, err
	}

	// TODO: migrate DefaultAllowPrivilegeEscalation and AllowPrivilegeEscalation when they will be implemented for SCC
	// Note that SCCs have "kubernetes.io/description" annotation but looks like no one uses it,
	// so we don't copy it over
	return &policy.PodSecurityPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:        scc.Name,
			Annotations: annotations,
		},
		Spec: policy.PodSecurityPolicySpec{
			Privileged:               extractPrivileged(scc),
			DefaultAddCapabilities:   extractDefaultAddCapabilities(scc),
			RequiredDropCapabilities: extractRequiredDropCapabilities(scc),
			AllowedCapabilities:      extractAllowedCapabilities(scc),
			Volumes:                  extractVolumes(scc),
			HostNetwork:              extractHostNetwork(scc),
			HostPorts:                extractHostPorts(scc),
			HostPID:                  extractHostPID(scc),
			HostIPC:                  extractHostIPC(scc),
			SELinux:                  selinux,
			RunAsUser:                runAsUser,
			SupplementalGroups:       supplementalGroups,
			FSGroup:                  fsGroup,
			ReadOnlyRootFilesystem:   extractReadOnlyRootFilesystem(scc),
			AllowedFlexVolumes:       extractAllowedFlexVolumes(scc),
			// AllowedHostPaths exists only in PSP.
			// Leave it empty, so Kubernetes will use its default value that means "allow all".
		},
	}, nil
}

func convertSccToClusterRole(scc *securityv1.SecurityContextConstraints) (*rbacv1.ClusterRole, error) {
	if len(scc.Users) == 0 && len(scc.Groups) == 0 {
		return nil, nil
	}

	rule, err := rbacv1helper.NewRule("use").
		Groups("policy").
		Resources("podsecuritypolicies").
		Names(scc.Name).
		Rule()
	if err != nil {
		return nil, err
	}

	return &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterRoleAndBindingNamePrefix + scc.Name,
		},
		Rules: []rbacv1.PolicyRule{rule},
	}, nil
}

func convertSccToClusterRoleBinding(scc *securityv1.SecurityContextConstraints) (*rbacv1.ClusterRoleBinding, error) {
	if len(scc.Users) == 0 && len(scc.Groups) == 0 {
		return nil, nil
	}

	bindingBuilder := rbacv1helper.NewClusterBinding(clusterRoleAndBindingNamePrefix + scc.Name).Groups(scc.Groups...)

	for _, user := range scc.Users {
		if !strings.HasPrefix(user, "system:serviceaccount:") {
			bindingBuilder.Users(user)
			continue
		}

		parts := strings.Split(user, ":")
		if len(parts) != 4 {
			msg := "Users contains user %q that looks like a Service Account but doesn't conform to the naming convention: expected to be in form \"system:serviceaccount:<namespace>:<name>\""
			return nil, fmt.Errorf(msg, user)
		}

		namespace := parts[2]
		sa := parts[3]
		bindingBuilder.SAs(namespace, sa)
	}

	binding, err := bindingBuilder.Binding()
	if err != nil {
		return nil, err
	}

	return &binding, nil
}
