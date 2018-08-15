package admin

import (
	"bytes"
	"errors"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
)

const (
	DefaultPolicyFile                = "openshift.local.config/master/policy.json"
	CreateBootstrapPolicyFileCommand = "create-bootstrap-policy-file"
)

type CreateBootstrapPolicyFileOptions struct {
	File string

	genericclioptions.IOStreams
}

func NewCreateBootstrapPolicyFileOptions(streams genericclioptions.IOStreams) *CreateBootstrapPolicyFileOptions {
	return &CreateBootstrapPolicyFileOptions{
		File:      DefaultPolicyFile,
		IOStreams: streams,
	}
}

func NewCommandCreateBootstrapPolicyFile(commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateBootstrapPolicyFileOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create the default bootstrap policy",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().StringVar(&o.File, "filename", o.File, "The policy template file that will be written with roles and bindings.")

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

func (o CreateBootstrapPolicyFileOptions) Run() error {
	if err := os.MkdirAll(path.Dir(o.File), os.FileMode(0755)); err != nil {
		return err
	}

	policyList := &corev1.List{}
	policyList.SetGroupVersionKind(corev1.SchemeGroupVersion.WithKind("List"))
	policy := bootstrappolicy.Policy()
	rbacEncoder := scheme.Codecs.LegacyCodec(rbacv1.SchemeGroupVersion)

	for i := range policy.ClusterRoles {
		versionedObject, err := runtime.Encode(rbacEncoder, &policy.ClusterRoles[i])
		if err != nil {
			return err
		}
		policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
	}

	for i := range policy.ClusterRoleBindings {
		versionedObject, err := runtime.Encode(rbacEncoder, &policy.ClusterRoleBindings[i])
		if err != nil {
			return err
		}
		policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.Roles).List() {
		roles := policy.Roles[namespace]
		for i := range roles {
			versionedObject, err := runtime.Encode(rbacEncoder, &roles[i])
			if err != nil {
				return err
			}
			policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
		}
	}

	// iterate in a defined order
	for _, namespace := range sets.StringKeySet(policy.RoleBindings).List() {
		roleBindings := policy.RoleBindings[namespace]
		for i := range roleBindings {
			versionedObject, err := runtime.Encode(rbacEncoder, &roleBindings[i])
			if err != nil {
				return err
			}
			policyList.Items = append(policyList.Items, runtime.RawExtension{Raw: versionedObject})
		}
	}

	buffer := &bytes.Buffer{}
	if err := (&printers.JSONPrinter{}).PrintObj(policyList, buffer); err != nil {
		return err
	}

	if err := ioutil.WriteFile(o.File, buffer.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
