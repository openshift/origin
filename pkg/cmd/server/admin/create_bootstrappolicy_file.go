package admin

import (
	"bytes"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kprinters "k8s.io/kubernetes/pkg/printers"

	"github.com/openshift/origin/pkg/api/latest"
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

	return nil
}

func (o CreateBootstrapPolicyFileOptions) CreateBootstrapPolicyFile() error {
	if err := os.MkdirAll(path.Dir(o.File), os.FileMode(0755)); err != nil {
		return err
	}

	policyTemplate := &templateapi.Template{}
	policy := bootstrappolicy.Policy()

	for i := range policy.ClusterRoles {
		versionedObject, err := legacyscheme.Scheme.ConvertToVersion(&policy.ClusterRoles[i], rbacv1.SchemeGroupVersion)
		if err != nil {
			return err
		}
		policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
	}

	for i := range policy.ClusterRoleBindings {
		versionedObject, err := legacyscheme.Scheme.ConvertToVersion(&policy.ClusterRoleBindings[i], rbacv1.SchemeGroupVersion)
		if err != nil {
			return err
		}
		policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.Roles).List() {
		roles := policy.Roles[namespace]
		for i := range roles {
			versionedObject, err := legacyscheme.Scheme.ConvertToVersion(&roles[i], rbacv1.SchemeGroupVersion)
			if err != nil {
				return err
			}
			policyTemplate.Objects = append(policyTemplate.Objects, versionedObject)
		}
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.RoleBindings).List() {
		roleBindings := policy.RoleBindings[namespace]
		for i := range roleBindings {
			versionedObject, err := legacyscheme.Scheme.ConvertToVersion(&roleBindings[i], rbacv1.SchemeGroupVersion)
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
