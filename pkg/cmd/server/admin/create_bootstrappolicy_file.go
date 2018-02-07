package admin

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/apis/rbac"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
)

const (
	DefaultPolicyFile                    = "openshift.local.config/master/policy.json"
	CreateBootstrapPolicyFileCommand     = "create-bootstrap-policy-file"
	CreateBootstrapPolicyFileFullCommand = "oc adm " + CreateBootstrapPolicyFileCommand
)

type CreateBootstrapPolicyFileOptions struct {
	File string

	OpenShiftSharedResourcesNamespace string
}

func NewCommandCreateBootstrapPolicyFile(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateBootstrapPolicyFileOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create the default bootstrap policy",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.CreateBootstrapPolicyFile(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.File, "filename", DefaultPolicyFile, "The policy template file that will be written with roles and bindings.")
	flags.StringVar(&options.OpenShiftSharedResourcesNamespace, "openshift-namespace", "openshift", "Namespace for shared resources.")
	flags.MarkDeprecated("openshift-namespace", "this field is no longer supported and using it can lead to undefined behavior")

	// autocompletion hints
	cmd.MarkFlagFilename("filename")

	return cmd
}

func (o CreateBootstrapPolicyFileOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.File) == 0 {
		return errors.New("filename must be provided")
	}
	if len(o.OpenShiftSharedResourcesNamespace) == 0 {
		return errors.New("openshift-namespace must be provided")
	}

	return nil
}

func (o CreateBootstrapPolicyFileOptions) CreateBootstrapPolicyFile() error {
	if err := os.MkdirAll(path.Dir(o.File), os.FileMode(0755)); err != nil {
		return err
	}

	policyTemplate := &templateapi.Template{}
	policy := bootstrappolicy.Policy()

	for i := range policy.ClusterRoles {
		originObject := &authorizationapi.ClusterRole{}
		if err := legacyscheme.Scheme.Convert(&policy.ClusterRoles[i], originObject, nil); err != nil {
			return err
		}
		versionedObject, err := legacyscheme.Scheme.ConvertToVersion(originObject, latest.Version)
		if err != nil {
			return err
		}
		policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
	}

	for i := range policy.ClusterRoleBindings {
		originObject := &authorizationapi.ClusterRoleBinding{}
		if err := legacyscheme.Scheme.Convert(&policy.ClusterRoleBindings[i], originObject, nil); err != nil {
			return err
		}
		versionedObject, err := legacyscheme.Scheme.ConvertToVersion(originObject, latest.Version)
		if err != nil {
			return err
		}
		policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
	}

	openshiftRoles := map[string][]rbac.Role{}
	for namespace, roles := range policy.Roles {
		if namespace == bootstrappolicy.DefaultOpenShiftSharedResourcesNamespace {
			r := make([]rbac.Role, len(roles))
			for i := range roles {
				r[i] = roles[i]
				r[i].Namespace = o.OpenShiftSharedResourcesNamespace
			}
			openshiftRoles[o.OpenShiftSharedResourcesNamespace] = r
		} else {
			openshiftRoles[namespace] = roles
		}
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(openshiftRoles).List() {
		roles := openshiftRoles[namespace]
		for i := range roles {
			originObject := &authorizationapi.Role{}
			if err := legacyscheme.Scheme.Convert(&roles[i], originObject, nil); err != nil {
				return err
			}
			versionedObject, err := legacyscheme.Scheme.ConvertToVersion(originObject, latest.Version)
			if err != nil {
				return err
			}
			policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
		}
	}

	openshiftRoleBindings := map[string][]rbac.RoleBinding{}
	for namespace, roleBindings := range policy.RoleBindings {
		if namespace == bootstrappolicy.DefaultOpenShiftSharedResourcesNamespace {
			rb := make([]rbac.RoleBinding, len(roleBindings))
			for i := range roleBindings {
				rb[i] = roleBindings[i]
				rb[i].Namespace = o.OpenShiftSharedResourcesNamespace
			}
			openshiftRoleBindings[o.OpenShiftSharedResourcesNamespace] = rb
		} else {
			openshiftRoleBindings[namespace] = roleBindings
		}
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(openshiftRoleBindings).List() {
		roleBindings := openshiftRoleBindings[namespace]
		for i := range roleBindings {
			originObject := &authorizationapi.RoleBinding{}
			if err := legacyscheme.Scheme.Convert(&roleBindings[i], originObject, nil); err != nil {
				return err
			}
			versionedObject, err := legacyscheme.Scheme.ConvertToVersion(originObject, latest.Version)
			if err != nil {
				return err
			}
			policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
		}
	}

	versionedPolicyTemplate, err := legacyscheme.Scheme.ConvertToVersion(policyTemplate, latest.Version)
	if err != nil {
		return err
	}

	buffer := &bytes.Buffer{}
	(&kprinters.JSONPrinter{}).PrintObj(versionedPolicyTemplate, buffer)

	if err := ioutil.WriteFile(o.File, buffer.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
