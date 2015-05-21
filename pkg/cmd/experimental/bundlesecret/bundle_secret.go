package bundlesecret

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/spf13/cobra"
)

type CreateSecretOptions struct {
	// Name of the resulting secret
	Name string
	// Files/Directories to read from.
	// Directory sources are listed and any direct file children included (but subfolders are not traversed)
	Sources util.StringList
	// Writer to write warnings to
	Stderr io.Writer
	// Controls whether to output warnings
	Quiet bool
}

func NewCmdBundleSecret(f *clientcmd.Factory, parentName, name string, out io.Writer) *cobra.Command {

	options := NewDefaultOptions()

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s NAME SOURCE [SOURCE ...]", name),
		Short: "Bundle files (or files within directories) into a Kubernetes secret",
		Long: fmt.Sprintf(`Bundle files (or files within directories) into a Kubernetes secret.

  $ %s %s <secret-name> <source> [<source>...]
		`, parentName, name),
		Run: func(c *cobra.Command, args []string) {
			options.Complete(args)

			err := options.Validate()
			if err != nil {
				fmt.Fprintf(c.Out(), "Error: %v\n\n", err.Error())
				c.Help()
				return
			}

			secret, err := options.CreateSecret()
			if err != nil {
				cmdutil.CheckErr(err)
			}

			err = f.Factory.PrintObject(c, secret, out)
			if err != nil {
				cmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().BoolVarP(&options.Quiet, "quiet", "q", options.Quiet, "Suppress warnings")
	cmd.Flags().VarP(&options.Sources, "source", "f", "List of filenames or directories to use as sources of Kubernetes Secret.Data")
	cmdutil.AddPrinterFlags(cmd)

	// Default to JSON
	if flag := cmd.Flags().Lookup("output"); flag != nil {
		flag.Value.Set("json")
	}

	return cmd
}

func NewDefaultOptions() *CreateSecretOptions {
	return &CreateSecretOptions{
		Stderr:  os.Stderr,
		Sources: util.StringList{},
	}
}

func (o *CreateSecretOptions) Complete(args []string) {
	// Fill name from args[0]
	if len(args) > 0 {
		o.Name = args[0]
	}

	// Add sources from args[1:...] in addition to -f
	if len(args) > 1 {
		o.Sources = append(o.Sources, args[1:]...)
	}
}

func (o *CreateSecretOptions) Validate() error {
	if len(o.Name) == 0 {
		return errors.New("Secret name is required")
	}
	if len(o.Sources) == 0 {
		return errors.New("At least one source file or directory must be specified")
	}
	return nil
}

func (o *CreateSecretOptions) CreateSecret() (*kapi.Secret, error) {
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

	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{Name: o.Name},
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
