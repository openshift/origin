package cmd

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/spf13/cobra"

	secretapi "github.com/openshift/origin/pkg/secret/api"
)

func NewCmdCreateSecret(f *Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-secret [name] [--type=binary|text] -f filename[,filename]",
		Short: "Create a new secret with the given name and value",
		Long: `Create a named secret

NOTE: This command is experimental.

The value may come from a command line parameter or from a set of comma-separated files.
If the type is not specified, it is inferred from the secret data provided. A "-" can be specified
to indicate that the secret is to be read from STDIN

Examples:
  $ kubectl create-secret ssh-keys -f ~/id_rsa,~/id_rsa.pub
  <creates a new secret that contains the data in the private and public ssh keys of the current user>

  $ echo $MYPASSWORD  | kubectl create-secret dbpassword -f -
  <creates a new secret using the value passed from STDIN>`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 || len(args[0]) == 0 {
				usageError(cmd, "Must pass a secret name to create it")
			}
			namespace := getOriginNamespace(cmd)
			secretName := args[0]
			filenames := kubecmd.GetFlagString(cmd, "filename")
			if len(filenames) == 0 {
				usageError(cmd, "Must pass at least one filename that contains a secret")
			}
			secretTypeFlag := kubecmd.GetFlagString(cmd, "type")
			if secretTypeFlag != "" && secretTypeFlag != "text" && secretTypeFlag != "binary" {
				usageError(cmd, "Invalid type value. Allowed values are text and binary")
			}
			secretBytes, err := readSecrets(filenames)
			checkErr(err)
			secretData, secretType := convertData(secretTypeFlag, secretBytes)
			checkErr(err)

			client, _, err := f.Clients(cmd)
			checkErr(err)

			secret := &secretapi.Secret{
				ObjectMeta: kapi.ObjectMeta{
					Name: secretName,
				},
				Type: secretType,
				Data: secretData,
			}
			_, err = client.Secrets(namespace).Create(secret)
			checkErr(err)
		},
	}
	cmd.Flags().StringP("filename", "f", "", "Comma-separated list of filenames that contain the secret data")
	cmd.Flags().StringP("type", "t", "", "Type of secret to create. Valid values are text and binary")
	return cmd
}

func readSecrets(filenames string) ([][]byte, error) {
	result := [][]byte{}
	if filenames == "-" {
		data, err := ioutil.ReadAll(os.Stdin)
		if err != nil {
			return nil, err
		}
		if len(data) == 0 {
			return nil, fmt.Errorf("Read from stdin specified but no data found.")
		}
		result = append(result, data)
		return result, nil
	}
	files := strings.Split(filenames, ",")
	for _, f := range files {
		data, err := ioutil.ReadFile(f)
		if err != nil {
			return nil, err
		}
		result = append(result, data)
	}
	return result, nil
}

func convertData(typeFlag string, byteData [][]byte) ([]string, secretapi.SecretType) {
	data := []string{}
	var secretType secretapi.SecretType
	switch typeFlag {
	case "text":
		secretType = secretapi.TextSecretType
	case "binary":
		secretType = secretapi.Base64SecretType
	default:
		if isAscii(byteData) {
			secretType = secretapi.TextSecretType
		} else {
			secretType = secretapi.Base64SecretType
		}
	}
	switch secretType {
	case secretapi.Base64SecretType:
		for _, b := range byteData {
			data = append(data, base64.StdEncoding.EncodeToString(b))
		}
		return data, secretType
	default:
		for _, b := range byteData {
			data = append(data, string(b))
		}
		return data, secretType
	}
}

func isAscii(byteData [][]byte) bool {
	for _, ba := range byteData {
		for _, b := range ba {
			if b > 127 {
				return false
			}
		}
	}
	return true
}
