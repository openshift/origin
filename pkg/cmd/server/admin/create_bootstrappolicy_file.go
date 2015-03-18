package admin

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/api/latest"
	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	"github.com/openshift/origin/pkg/template/api"
)

const (
	DefaultPolicyFile                    = "openshift.local.policy/policy.json"
	CreateBootstrapPolicyFileCommand     = "create-bootstrap-policy-file"
	CreateBootstrapPolicyFileFullCommand = "openshift admin " + CreateBootstrapPolicyFileCommand
)

type CreateBootstrapPolicyFileOptions struct {
	File string

	MasterAuthorizationNamespace      string
	OpenShiftSharedResourcesNamespace string
}

func NewCommandCreateBootstrapPolicyFile() *cobra.Command {
	options := &CreateBootstrapPolicyFileOptions{}

	cmd := &cobra.Command{
		Use:   CreateBootstrapPolicyFileCommand,
		Short: "Create bootstrap policy for OpenShift.",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if err := options.CreateBootstrapPolicyFile(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.File, "filename", DefaultPolicyFile, "The policy template file that will be written with roles and bindings.")

	flags.StringVar(&options.MasterAuthorizationNamespace, "master-namespace", bootstrappolicy.DefaultMasterAuthorizationNamespace, "Global authorization namespace.")
	flags.StringVar(&options.OpenShiftSharedResourcesNamespace, "openshift-namespace", "openshift", "Namespace for shared openshift resources.")

	return cmd
}

func (o CreateBootstrapPolicyFileOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.File) == 0 {
		return errors.New("filename must be provided")
	}
	if len(o.MasterAuthorizationNamespace) == 0 {
		return errors.New("master-namespace must be provided")
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

	policyTemplate := &api.Template{}

	roles := bootstrappolicy.GetBootstrapRoles(o.MasterAuthorizationNamespace, o.OpenShiftSharedResourcesNamespace)
	for i := range roles {
		policyTemplate.Objects = append(policyTemplate.Objects, &roles[i])
	}

	roleBindings := bootstrappolicy.GetBootstrapRoleBindings(o.MasterAuthorizationNamespace, o.OpenShiftSharedResourcesNamespace)
	for i := range roleBindings {
		policyTemplate.Objects = append(policyTemplate.Objects, &roleBindings[i])
	}

	versionedPolicyTemplate, err := kapi.Scheme.ConvertToVersion(policyTemplate, latest.Version)
	if err != nil {
		return err
	}

	buffer := &bytes.Buffer{}
	(&kubectl.JSONPrinter{}).PrintObj(versionedPolicyTemplate, buffer)

	if err := ioutil.WriteFile(o.File, buffer.Bytes(), 0644); err != nil {
		return err
	}

	return nil
}
