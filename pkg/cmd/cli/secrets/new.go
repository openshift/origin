package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

const NewSecretRecommendedCommandName = "new"

type CreateSecretOptions struct {
	// Name of the resulting secret
	Name string

	// SecretTypeName is the type to use when creating the secret.  It is checked against known types.
	SecretTypeName string

	// Files/Directories to read from.
	// Directory sources are listed and any direct file children included (but subfolders are not traversed)
	Sources util.StringList

	SecretsInterface kclient.SecretsInterface

	// Writer to write warnings to
	Stderr io.Writer

	Out io.Writer

	// Controls whether to output warnings
	Quiet bool
}

func NewCmdCreateSecret(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := NewCreateSecretOptions()
	options.Out = out

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s NAME SOURCE [SOURCE ...]", name),
		Short: "Create a new secret based on a file or files within a directory",
		Long: fmt.Sprintf(`Create a new secret based on a file or files within a directory.

  $ %s <secret-name> <source> [<source>...]
		`, fullName),
		Run: func(c *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Complete(args, f))

			cmdutil.CheckErr(options.Validate())

			if len(cmdutil.GetFlagString(c, "output")) != 0 {
				secret, err := options.BundleSecret()
				cmdutil.CheckErr(err)

				cmdutil.CheckErr(f.Factory.PrintObject(c, secret, out))
				return
			}
			_, err := options.CreateSecret()
			cmdutil.CheckErr(err)
		},
	}

	cmd.Flags().BoolVarP(&options.Quiet, "quiet", "q", options.Quiet, "Suppress warnings")
	cmd.Flags().VarP(&options.Sources, "source", "f", "List of filenames or directories to populate the data elements in a secret")
	cmd.Flags().StringVar(&options.SecretTypeName, "type", "", "The type of secret")
	cmdutil.AddPrinterFlags(cmd)

	return cmd
}

func NewCreateSecretOptions() *CreateSecretOptions {
	return &CreateSecretOptions{
		Stderr:  os.Stderr,
		Sources: util.StringList{},
	}
}

func (o *CreateSecretOptions) Complete(args []string, f *clientcmd.Factory) error {
	// Fill name from args[0]
	if len(args) > 0 {
		o.Name = args[0]
	}

	// Add sources from args[1:...] in addition to -f
	if len(args) > 1 {
		o.Sources = append(o.Sources, args[1:]...)
	}

	if f != nil {
		_, kubeClient, err := f.Clients()
		if err != nil {
			return err
		}
		namespace, err := f.Factory.DefaultNamespace()
		if err != nil {
			return err
		}
		o.SecretsInterface = kubeClient.Secrets(namespace)
	}

	return nil
}

func (o *CreateSecretOptions) Validate() error {
	if len(o.Name) == 0 {
		return errors.New("Secret name is required")
	}
	if len(o.Sources) == 0 {
		return errors.New("At least one source file or directory must be specified")
	}

nameCheck:
	switch o.SecretTypeName {
	case string(kapi.SecretTypeOpaque), "":
		// this is ok
	default:
		// TODO this probably isn't a good idea.  It limits the power of this command.  Maybe allow unknown names with a force?
		for _, secretType := range KnownSecretTypes {
			if o.SecretTypeName == string(secretType.Type) {
				break nameCheck
			}
		}
		return fmt.Errorf("unknown secret type: %v", o.SecretTypeName)
	}

	return nil
}

func (o *CreateSecretOptions) CreateSecret() (*kapi.Secret, error) {
	secret, err := o.BundleSecret()
	if err != nil {
		return nil, err
	}

	persistedSecret, err := o.SecretsInterface.Create(secret)
	if err == nil {
		fmt.Fprintf(o.Out, "secret/%s\n", persistedSecret.Name)
	}

	return persistedSecret, err
}

func (o *CreateSecretOptions) BundleSecret() (*kapi.Secret, error) {
	secretData := make(map[string][]byte)

	for _, source := range o.Sources {
		info, err := os.Stat(source)
		if err != nil {
			switch err := err.(type) {
			case *os.PathError:
				return nil, fmt.Errorf("Error reading %s: %v", source, err.Err)
			default:
				return nil, fmt.Errorf("Error reading %s: %v", source, err)
			}
		}

		if info.IsDir() {
			fileList, err := ioutil.ReadDir(source)
			if err != nil {
				return nil, fmt.Errorf("Error listing files in %s: %v", source, err)
			}

			for _, item := range fileList {
				itemPath := path.Join(source, item.Name())
				if !item.Mode().IsRegular() {
					if o.Stderr != nil && o.Quiet != true {
						fmt.Fprintf(o.Stderr, "Skipping resource %s\n", itemPath)
					}
				} else {
					if err := readFile(itemPath, secretData); err != nil {
						return nil, err
					}
				}
			}
		} else if err := readFile(source, secretData); err != nil {
			return nil, err
		}
	}

	if len(secretData) == 0 {
		return nil, errors.New("No files selected")
	}

	// if the secret type isn't specified, attempt to auto-detect likely hit
	secretType := kapi.SecretType(o.SecretTypeName)
	if len(o.SecretTypeName) == 0 {
		secretType = kapi.SecretTypeOpaque

		for _, knownSecretType := range KnownSecretTypes {
			if knownSecretType.Matches(secretData) {
				secretType = knownSecretType.Type
			}
		}
	}

	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{Name: o.Name},
		Type:       secretType,
		Data:       secretData,
	}

	return secret, nil
}

func readFile(filePath string, dataMap map[string][]byte) error {
	fileName := path.Base(filePath)
	if !kvalidation.IsSecretKey(fileName) {
		return fmt.Errorf("%s cannot be used as a key in a secret", filePath)
	}
	if _, exists := dataMap[fileName]; exists {
		return fmt.Errorf("Multiple files with the same name (%s) cannot be included in a secret", fileName)
	}

	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return err
	}
	dataMap[fileName] = data
	return nil
}
