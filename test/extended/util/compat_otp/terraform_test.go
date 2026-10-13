package compat_otp

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tfjson "github.com/hashicorp/terraform-json"
)

func TestNewTerraform(t *testing.T) {

	tfDir, err := ioutil.TempDir("", "tfTemp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tfDir)

	var tf *TerraformExec
	tf, err = NewTerraform(tfDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Using terraform binary from: %v", tf.tfbin.ExecPath())

	if _, err := exec.LookPath("terraform"); err != nil {
		// If the system where the test has run doesn't have terraform
		// installed in $PATH then ensure it has been downloaded
		files, _ := filepath.Glob("/tmp/terraform_*/terraform")
		if _, err := os.Stat(files[0]); os.IsNotExist(err) {
			t.Fatalf("Terraform binary was not downloaded")
		}

		zipFiles, _ := filepath.Glob("/tmp/terraform*zip*")
		if len(zipFiles) == 0 {
			t.Fatalf("Terraform zip file was not downloaded")
		}
	}

}

func TestRunAndDestroyTerraform(t *testing.T) {

	tfDir, err := ioutil.TempDir("", "tfTemp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tfDir)
	tfFileContent := "terraform {\n" +
		"  required_version = \">= 1.0.0\"\n" +
		"}\n" +
		"\n" +
		"resource \"local_file\" \"temp\" {\n" +
		"  filename =  \"" + tfDir + "/temp\"\n" +
		"  content = \"This is only a test\"\n" +
		"}"

	tfFile, err := os.Create(filepath.Join(tfDir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}

	defer tfFile.Close()

	_, err = tfFile.WriteString(tfFileContent)
	if err != nil {
		t.Fatal(err)
	}
	tfFile.Close()

	var tf *TerraformExec
	tf, err = NewTerraform(tfDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Using terraform binary from: %v", tf.tfbin.ExecPath())

	err = tf.TerraformInit()
	if err != nil {
		t.Fatal(err)
	}

	// check that init was run and the provider directory and lock files were created
	for _, file := range []string{"/.terraform", "/.terraform.lock.hcl"} {
		if _, err := os.Stat(filepath.Join(tfDir, file)); os.IsNotExist(err) {
			t.Fatalf("Terraform file %v is not present after terraform init", filepath.Join(tfDir, file))
		}
	}

	err = tf.TerraformApply()
	if err != nil {
		t.Fatal(err)
	}

	// check that apply was run and the state file and resource /temp files were created
	for _, file := range []string{"/terraform.tfstate", "/temp"} {
		if _, err := os.Stat(filepath.Join(tfDir, file)); os.IsNotExist(err) {
			t.Fatalf("Terraform file %v is not present after terraform apply", filepath.Join(tfDir, file))
		}
	}

	err = tf.TerraformDestroy()
	if err != nil {
		t.Fatal(err)
	}

	// check that destroy was run and the created local_file /temp was removed
	if _, err := os.Stat(filepath.Join(tfDir, "/temp")); os.IsExist(err) {
		t.Fatalf("Terraform local_file /temp was not cleaned up")
	}

}

func TestOutputAndShowTerraform(t *testing.T) {

	tfDir, err := ioutil.TempDir("", "tfTemp")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tfDir)
	tfFileContent := "terraform {\n" +
		"  required_version = \">= 1.0.0\"\n" +
		"}\n" +
		"\n" +
		"variable \"text\" {\n" +
		"  type = string\n" +
		"  default = \"This is only a test\"\n" +
		"}\n" +
		"\n" +
		"resource \"local_file\" \"temp\" {\n" +
		"  filename =  \"" + tfDir + "/temp\"\n" +
		"  content = \"${var.text}\"\n" +
		"}\n" +
		"output \"text_output\" {\n" +
		"  value = \"${local_file.temp.content}\"\n" +
		"}"

	tfFile, err := os.Create(filepath.Join(tfDir, "main.tf"))
	if err != nil {
		t.Fatal(err)
	}

	defer tfFile.Close()

	_, err = tfFile.WriteString(tfFileContent)
	if err != nil {
		t.Fatal(err)
	}
	tfFile.Close()

	var tf *TerraformExec
	tf, err = NewTerraform(tfDir)
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Using terraform binary from: %v", tf.tfbin.ExecPath())

	err = tf.TerraformInitWithUpgrade()
	if err != nil {
		t.Fatal(err)
	}

	inputText := "Apply with input parameter"

	tfArgs := []string{
		"text=" + inputText,
	}
	err = tf.TerraformApply(tfArgs...)
	if err != nil {
		t.Fatal(err)
	}

	// check that apply was run and the state file and resource /temp files were created
	for _, file := range []string{"/temp"} {
		if _, err := os.Stat(filepath.Join(tfDir, file)); os.IsNotExist(err) {
			t.Fatalf("Terraform file %v is not present after terraform apply", filepath.Join(tfDir, file))
		}
	}

	var tfState *tfjson.State
	tfState, err = tf.TerraformShow()
	if err != nil {
		t.Fatal(err)
	}

	if tfState.Values.Outputs["text_output"].Value != inputText {
		t.Fatal(fmt.Printf("The Terraform state value for text_output is incorrect. Found: %v, Expected: %v", tfState.Values.Outputs["text_output"].Value, inputText))
	}

	var tfOutput map[string]string
	tfOutput, err = tf.TerraformOutput()
	if err != nil {
		t.Fatal(err)
	}

	if strings.Trim(tfOutput["text_output"], "\"") != inputText {
		t.Fatal(fmt.Printf("The Terraform output value for text_output is incorrect. Found: %v, Expected: %v", tfOutput["text_output"], inputText))
	}

	err = tf.TerraformDestroy(tfArgs...)
	if err != nil {
		t.Fatal(err)
	}

	// check that destroy was run and the created local_file /temp was removed
	if _, err := os.Stat(filepath.Join(tfDir, "/temp")); os.IsExist(err) {
		t.Fatalf("Terraform local_file /temp was not cleaned up")
	}

}
