package compat_otp

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/hc-install/checkpoint"
	"github.com/hashicorp/hc-install/product"
	"github.com/hashicorp/terraform-exec/tfexec"

	tfjson "github.com/hashicorp/terraform-json"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

// TerraformExec structure which stores all atributes
// about the Terraform installation and templates location.
type TerraformExec struct {
	tfbin *tfexec.Terraform
}

// NewTerraform function takes care of finding out the terraform binary to use
// or setting up and installing Terraform (if not already installed) in the
// system so that it can be used during the execution of the
// tfexec.Terraform struct.
// Inputs:
//   - workingDir: Directory where the Terraform scripts are located
//
// Returns:
//   - A TerraformExec struct which can be used to invoke other Terraform
//     methods.
func NewTerraform(workingDir string) (*TerraformExec, error) {

	var execPath string
	var err error

	// Look for terraform directory intalled in $PATH
	if execPath, err = exec.LookPath("terraform"); err != nil {

		files, _ := filepath.Glob("/tmp/terraform_*/terraform")
		if len(files) > 0 {
			// If a /tmp/terraform_*/terraform binary exist use it
			execPath = files[0]
			e2e.Logf("using existing terraform binary from %v", execPath)
		} else {
			// If not, install terraform in /tmp
			installer := &checkpoint.LatestVersion{
				Product: product.Terraform,
			}
			execPath, err = installer.Install(context.Background())
			if err != nil {
				e2e.Logf("terraform installation in /tmp %v failed", err)
				return nil, err
			}
			e2e.Logf("terraform installed in %v", execPath)
		}
	}

	tfinit, err := tfexec.NewTerraform(workingDir, execPath)
	if err != nil {
		e2e.Logf("error setting up Terraform in working dir %v", workingDir)
		return nil, err
	}
	return &TerraformExec{
		tfbin: tfinit,
	}, nil
}

// TerraformInit executes terraform init in the workingDir templates
func (tf *TerraformExec) TerraformInit() error {

	err := tf.tfbin.Init(context.Background())
	if err != nil {
		e2e.Logf("error in terraform init: %s", err)
		return err
	}

	return nil
}

// TerraformInitWithUpgrade executes terraform init --upgrade in the workingDir templates
func (tf *TerraformExec) TerraformInitWithUpgrade() error {

	err := tf.tfbin.Init(context.Background(), tfexec.Upgrade(true))
	if err != nil {
		e2e.Logf("error in terraform init: %s", err)
		return err
	}

	return nil
}

// TerraformShow executes the terraform show command
// Returns:
//   - The Terraform state in a tfjson.State struct type
//   - Any error which could occur
func (tf *TerraformExec) TerraformShow() (*tfjson.State, error) {

	state, err := tf.tfbin.Show(context.Background())
	if err != nil {
		e2e.Logf("error in terraform show: %s", err)
		return nil, err
	}
	return state, nil
}

// TerraformApply executes terraform apply in the workingDir templates
// Inputs:
//   - vars: []string including all the vars to be passed during the
//     terraform apply execution. Format: ["host=master.ocp", "num_workers=3"]
func (tf *TerraformExec) TerraformApply(vars ...string) error {

	OptVarList := make([]tfexec.ApplyOption, len(vars))
	// Convert slice of strings into an slice of ApplyOption using Var function
	for i, valVar := range vars {
		OptVarList[i] = tfexec.Var(valVar)
	}

	err := tf.tfbin.Apply(context.Background(), OptVarList...)
	if err != nil {
		e2e.Logf("error in terraform apply: %s", err)
		return err
	}
	return nil
}

// TerraformOutput executes terraform show command and returns a map of the
// output values
// Returns:
//   - Map of key:string and value:string including the output var name
//     and the corresponding value. For more information on output values
//     check: https://www.terraform.io/language/values/outputs
//     Example:
//     { 'instance_ip': '10.0.176.10', 'instance_dns': 'cool.worker.internal.aws.com' }
func (tf *TerraformExec) TerraformOutput() (map[string]string, error) {

	var cmdOutput map[string]tfexec.OutputMeta
	mapReturn := make(map[string]string)

	cmdOutput, err := tf.tfbin.Output(context.Background())
	if err != nil {
		return nil, err
	}

	for key, value := range cmdOutput {
		mapReturn[key] = string(value.Value)
	}

	return mapReturn, nil

}

// TerraformDestroy runs terraform destroy in the workingDir templates directory.
// Inputs:
//   - vars: []string A list of the vars passed to the terraform
//     destroy commmand. In a similar way as in TerraformApply.
//     Format: ["host=master.ocp", "num_workers=3"]
func (tf *TerraformExec) TerraformDestroy(vars ...string) error {

	OptVarList := make([]tfexec.DestroyOption, len(vars))
	// Convert slice of strings into an slice of DestroyOption using Var function
	for i, valVar := range vars {
		OptVarList[i] = tfexec.Var(valVar)
	}

	err := tf.tfbin.Destroy(context.Background(), OptVarList...)
	if err != nil {
		if strings.Contains(err.Error(), "failed to instantiate provider") {
			// Remove .terraform dir and Rerun Terraform Init with --upgrade, then retry Destroy
			os.RemoveAll(tf.tfbin.WorkingDir() + "/.terraform/")
			tf.TerraformInit()
			err = tf.tfbin.Destroy(context.Background(), OptVarList...)
			if err == nil {
				return nil
			}
		}
		e2e.Logf("error in terraform destroy: %s", err)
		return err
	}

	return nil
}
