package extensionadmission

import (
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubectl/pkg/util/templates"
	"sigs.k8s.io/yaml"

	testextensionv1 "github.com/openshift/origin/pkg/apis/testextension/v1"
	"github.com/openshift/origin/pkg/cmd"
	exutil "github.com/openshift/origin/test/extended/util"
)

//go:embed testextensionadmission-crd.yaml
var crdYAML []byte

func NewExtensionAdmissionCommand(ioStreams genericclioptions.IOStreams) *cobra.Command {
	o := &extensionAdmissionOptions{
		ioStreams: ioStreams,
	}

	command := &cobra.Command{
		Use:   "extension-admission",
		Short: "Manage TestExtensionAdmission resources",
		Long: templates.LongDesc(`
		Manage TestExtensionAdmission resources for controlling which ImageStreams
		are permitted to provide extension test binaries.

		TestExtensionAdmission acts as an admission controller to determine which
		ImageStreams are permitted to provide test binaries outside the main
		OpenShift release payload.

		To list or delete TestExtensionAdmission resources, use standard kubectl/oc commands:
		  oc get testextensionadmissions
		  oc delete testextensionadmission <name>
		`),
		PersistentPreRun: cmd.NoPrintVersion,
	}

	command.AddCommand(
		newInstallCRDCommand(o),
		newCreateCommand(o),
	)

	return command
}

type extensionAdmissionOptions struct {
	ioStreams genericclioptions.IOStreams
}

func newInstallCRDCommand(o *extensionAdmissionOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "install-crd",
		Short: "Install the TestExtensionAdmission CRD",
		Long: templates.LongDesc(`
		Install the TestExtensionAdmission CustomResourceDefinition to the cluster.

		This CRD must be installed before creating TestExtensionAdmission instances.
		`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return o.installCRD()
		},
	}

	return command
}

func newCreateCommand(o *extensionAdmissionOptions) *cobra.Command {
	createOpts := &createOptions{
		extensionAdmissionOptions: o,
	}

	command := &cobra.Command{
		Use:   "create NAME --permit=PATTERN [--permit=PATTERN...]",
		Short: "Create a TestExtensionAdmission resource",
		Long: templates.LongDesc(`
		Create a TestExtensionAdmission resource with the specified permit patterns.

		Permit patterns are in the format "namespace/imagestream" and support wildcards:
		  - "openshift/*" - All ImageStreams in the openshift namespace
		  - "test-extensions/*" - All ImageStreams in test-extensions namespace
		  - "my-ns/my-stream" - Specific ImageStream
		  - "*/*" - All ImageStreams in all namespaces (use with caution)

		Example:
		  openshift-tests extension-admission create my-admission \
		    --permit=openshift/* \
		    --permit=test-extensions/*
		`),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			createOpts.name = args[0]
			return createOpts.create()
		},
	}

	command.Flags().StringSliceVar(&createOpts.permits, "permit", nil, "Permit pattern(s) (can be specified multiple times)")
	command.MarkFlagRequired("permit")

	return command
}

type createOptions struct {
	*extensionAdmissionOptions
	name    string
	permits []string
}

func (o *createOptions) create() error {
	if len(o.permits) == 0 {
		return fmt.Errorf("at least one --permit pattern is required")
	}

	// Create the TestExtensionAdmission object
	admission := &testextensionv1.TestExtensionAdmission{
		TypeMeta: metav1.TypeMeta{
			APIVersion: testextensionv1.SchemeGroupVersion.String(),
			Kind:       "TestExtensionAdmission",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.name,
		},
		Spec: testextensionv1.TestExtensionAdmissionSpec{
			Permit: o.permits,
		},
	}

	// Convert to YAML
	yamlBytes, err := yaml.Marshal(admission)
	if err != nil {
		return fmt.Errorf("failed to marshal TestExtensionAdmission to YAML: %w", err)
	}

	// Apply using kubectl/oc
	artifactName := fmt.Sprintf("testextensionadmission-%s.yaml", o.name)
	if err := o.applyYAML(yamlBytes, artifactName); err != nil {
		return fmt.Errorf("failed to apply TestExtensionAdmission: %w", err)
	}

	fmt.Fprintf(o.ioStreams.Out, "TestExtensionAdmission %q created successfully\n", o.name)
	return nil
}

func (o *extensionAdmissionOptions) installCRD() error {
	if err := o.applyYAML(crdYAML, "testextensionadmission-crd.yaml"); err != nil {
		return fmt.Errorf("failed to install CRD: %w", err)
	}

	fmt.Fprintln(o.ioStreams.Out, "TestExtensionAdmission CRD installed successfully")
	return nil
}

// saveToArtifactDir saves the YAML content to ARTIFACT_DIR if the environment variable is set.
// This helps with debugging by preserving the applied manifests.
func saveToArtifactDir(yamlBytes []byte, basename string) {
	artifactDir := os.Getenv("ARTIFACT_DIR")
	if artifactDir == "" {
		return
	}

	// Create a timestamped filename to avoid collisions
	timestamp := time.Now().UTC().Format("20060102-150405")
	filename := fmt.Sprintf("%s-%s", timestamp, basename)
	artifactPath := filepath.Join(artifactDir, filename)

	if err := os.WriteFile(artifactPath, yamlBytes, 0644); err != nil {
		// Don't fail the operation, just log the error
		fmt.Fprintf(os.Stderr, "Warning: Failed to save artifact to %s: %v\n", artifactPath, err)
		return
	}

	fmt.Fprintf(os.Stderr, "Saved artifact to %s\n", artifactPath)
}

func (o *extensionAdmissionOptions) applyYAML(yamlBytes []byte, artifactName string) error {
	// Write YAML to a temporary file
	tmpFile, err := os.CreateTemp("", "testextensionadmission-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(yamlBytes); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write YAML to temp file: %w", err)
	}
	tmpFile.Close()

	// Use oc apply via exec.Command
	ocPath := "oc"
	kubeconfig := exutil.KubeConfigPath()

	var cmd *exec.Cmd
	if kubeconfig != "" {
		cmd = exec.Command(ocPath, "--kubeconfig="+kubeconfig, "apply", "-f", tmpFile.Name())
	} else {
		cmd = exec.Command(ocPath, "apply", "-f", tmpFile.Name())
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("oc apply failed: %w\nOutput: %s", err, string(output))
	}

	fmt.Fprintf(o.ioStreams.Out, "%s", string(output))
	saveToArtifactDir(yamlBytes, artifactName)
	return nil
}
