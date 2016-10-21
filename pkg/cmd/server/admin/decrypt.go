package admin

import (
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/openshift/origin/pkg/cmd/util/term"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/templates"
	pemutil "github.com/openshift/origin/pkg/cmd/util/pem"
)

const DecryptCommandName = "decrypt"

type DecryptOptions struct {
	// EncryptedFile is a file containing an encrypted PEM block.
	EncryptedFile string
	// EncryptedData is a byte slice containing an encrypted PEM block.
	EncryptedData []byte
	// EncryptedReader is used to read an encrypted PEM block if no EncryptedFile or EncryptedData is provided. Cannot be a terminal reader.
	EncryptedReader io.Reader

	// DecryptedFile is a destination file to write decrypted data to.
	DecryptedFile string
	// DecryptedWriter is used to write decrypted data to if no DecryptedFile is provided
	DecryptedWriter io.Writer

	// KeyFile is a file containing a PEM block with the password to use to decrypt the data
	KeyFile string
}

var decryptExample = templates.Examples(`
	# Decrypt an encrypted file to a cleartext file:
	%[1]s --key=secret.key --in=secret.encrypted --out=secret.decrypted

	# Decrypt from stdin to stdout:
	%[1]s --key=secret.key < secret2.encrypted > secret2.decrypted`)

func NewCommandDecrypt(commandName string, fullName, encryptFullName string, out io.Writer) *cobra.Command {
	options := &DecryptOptions{
		EncryptedReader: os.Stdin,
		DecryptedWriter: out,
	}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   fmt.Sprintf("Decrypt data encrypted with %q", encryptFullName),
		Example: fmt.Sprintf(decryptExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Validate(args))
			kcmdutil.CheckErr(options.Decrypt())
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.EncryptedFile, "in", options.EncryptedFile, fmt.Sprintf("File containing encrypted data, in the format written by %q.", encryptFullName))
	flags.StringVar(&options.DecryptedFile, "out", options.DecryptedFile, "File to write the decrypted data to. Written to stdout if omitted.")

	flags.StringVar(&options.KeyFile, "key", options.KeyFile, fmt.Sprintf("The file to read the decrypting key from. Must be a PEM file in the format written by %q.", encryptFullName))

	// autocompletion hints
	cmd.MarkFlagFilename("in")
	cmd.MarkFlagFilename("out")
	cmd.MarkFlagFilename("key")

	return cmd
}

func (o *DecryptOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}

	if len(o.EncryptedFile) == 0 && len(o.EncryptedData) == 0 && (o.EncryptedReader == nil || term.IsTerminalReader(o.EncryptedReader)) {
		return errors.New("no input data specified")
	}
	if len(o.EncryptedFile) > 0 && len(o.EncryptedData) > 0 {
		return errors.New("cannot specify both an input file and data")
	}

	if len(o.KeyFile) == 0 {
		return errors.New("no key specified")
	}

	return nil
}

func (o *DecryptOptions) Decrypt() error {
	// Get PEM data block
	var data []byte
	switch {
	case len(o.EncryptedFile) > 0:
		if d, err := ioutil.ReadFile(o.EncryptedFile); err != nil {
			return err
		} else {
			data = d
		}
	case len(o.EncryptedData) > 0:
		data = o.EncryptedData
	case o.EncryptedReader != nil && !term.IsTerminalReader(o.EncryptedReader):
		if d, err := ioutil.ReadAll(o.EncryptedReader); err != nil {
			return err
		} else {
			data = d
		}
	}
	if len(data) == 0 {
		return fmt.Errorf("no input data specified")
	}
	dataBlock, ok := pemutil.BlockFromBytes(data, configapi.StringSourceEncryptedBlockType)
	if !ok {
		return fmt.Errorf("input does not contain a valid PEM block of type %q", configapi.StringSourceEncryptedBlockType)
	}

	// Get password
	keyBlock, ok, err := pemutil.BlockFromFile(o.KeyFile, configapi.StringSourceKeyBlockType)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%s does not contain a valid PEM block of type %q", o.KeyFile, configapi.StringSourceKeyBlockType)
	}
	if len(keyBlock.Bytes) == 0 {
		return fmt.Errorf("%s does not contain a key", o.KeyFile)
	}
	password := keyBlock.Bytes

	// Decrypt
	plaintext, err := x509.DecryptPEMBlock(dataBlock, password)
	if err != nil {
		return err
	}

	// Write decrypted data
	switch {
	case len(o.DecryptedFile) > 0:
		if err := ioutil.WriteFile(o.DecryptedFile, plaintext, os.FileMode(0600)); err != nil {
			return err
		}
	case o.DecryptedWriter != nil:
		fmt.Fprint(o.DecryptedWriter, string(plaintext))
		if term.IsTerminalWriter(o.DecryptedWriter) {
			fmt.Fprintln(o.DecryptedWriter)
		}
	}

	return nil
}
