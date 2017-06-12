package admin

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
	"github.com/openshift/origin/pkg/cmd/templates"
)

const CreateServerCertCommandName = "create-server-cert"

type CreateServerCertOptions struct {
	Phases []string

	SignerCertOptions *SignerCertOptions

	CSRFile  string
	CertFile string
	KeyFile  string

	ExpireDays int

	Hostnames []string
	Overwrite bool
	Output    io.Writer
}

var createServerLong = templates.LongDesc(`
	Create a key and server certificate

	Create a key and server certificate valid for the specified hostnames,
	signed by the specified CA. These are useful for securing infrastructure
	components such as the router, authentication server, etc.

	Example: Creating a secure router certificate.

	    CA=openshift.local.config/master
			%[1]s --signer-cert=$CA/ca.crt \
		          --signer-key=$CA/ca.key --signer-serial=$CA/ca.serial.txt \
		          --hostnames='*.cloudapps.example.com' \
		          --cert=cloudapps.crt --key=cloudapps.key
	    cat cloudapps.crt cloudapps.key $CA/ca.crt > cloudapps.router.pem
	`)

func NewCommandCreateServerCert(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateServerCertOptions{
		Phases:            NewDefaultPhaseOptions(),
		SignerCertOptions: NewDefaultSignerCertOptions(),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
		Output:            out,
	}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a signed server certificate and key",
		Long:  fmt.Sprintf(createServerLong, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.CreateServerCert(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()
	BindPhaseOptions(&options.Phases, flags, "")
	BindSignerCertOptions(options.SignerCertOptions, flags, "")

	flags.StringVar(&options.CSRFile, "csr", "", "The csr file. Choose a name that indicates what the service is. Defaults to the value of --cert with .csr appended.")
	flags.StringVar(&options.CertFile, "cert", "", "The certificate file. Choose a name that indicates what the service is.")
	flags.StringVar(&options.KeyFile, "key", "", "The key file. Choose a name that indicates what the service is.")

	flags.StringSliceVar(&options.Hostnames, "hostnames", options.Hostnames, "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificate in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")

	// autocompletion hints
	cmd.MarkFlagFilename("csr")
	cmd.MarkFlagFilename("cert")
	cmd.MarkFlagFilename("key")

	return cmd
}

func (o *CreateServerCertOptions) Complete(args []string) error {
	if len(o.CSRFile) == 0 && len(o.CertFile) > 0 {
		o.CSRFile = o.CertFile + ".csr"
	}
	return nil
}

func (o CreateServerCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if err := ValidatePhases(o.Phases); err != nil {
		return fmt.Errorf("server cert phase error: %v", err)
	}
	if hasPhase(PhaseCSR, o.Phases) || hasPhase(PhaseVerify, o.Phases) || hasPhase(PhasePackage, o.Phases) {
		if len(o.Hostnames) == 0 {
			return errors.New("at least one hostname must be provided")
		}
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.CSRFile) == 0 {
		return errors.New("server csr must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}

	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}

	if hasPhase(PhaseSign, o.Phases) {
		if o.SignerCertOptions == nil {
			return errors.New("signer options are required")
		}
		if err := o.SignerCertOptions.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func (o CreateServerCertOptions) CreateServerCert() error {
	glog.V(4).Infof("Creating a server cert with: %#v", o)

	if hasPhase(PhaseKey, o.Phases) {
		createKey := &CreateKeyPairOptions{
			PrivateKeyFile: o.KeyFile,
			Overwrite:      o.Overwrite,
			Output:         o.Output,
		}
		if err := createKey.CreateKeyPair(); err != nil {
			return err
		}
	}

	if hasPhase(PhaseCSR, o.Phases) {
		createServerCSR := &CreateServerCSROptions{
			PrivateKeyFile: o.KeyFile,
			CSRFile:        o.CSRFile,
			Hostnames:      o.Hostnames,
		}
		if err := createServerCSR.CreateServerCSR(); err != nil {
			return fmt.Errorf("error creating %s: %v", o.CSRFile, err)
		}
	}

	if hasPhase(PhaseSign, o.Phases) {
		// sign if invalid or overwrite is true
		var needsSigning bool
		switch {
		case o.Overwrite:
			needsSigning = true
		default:
			verifyServerCert := &VerifyServerCertOptions{
				CertFile:       o.CertFile,
				PrivateKeyFile: o.KeyFile,
				CSRFile:        o.CSRFile,
				Hostnames:      o.Hostnames,
				CAFile:         o.SignerCertOptions.CertFile,
			}
			if err := verifyServerCert.VerifyServerCert(); err != nil {
				needsSigning = true
			}
		}

		if needsSigning {
			signCSR := &SignCSROptions{
				SignerCertOptions: o.SignerCertOptions,
				CSRFile:           o.CSRFile,
				CertFile:          o.CertFile,
				ExpireDays:        o.ExpireDays,
			}
			if err := signCSR.SignCSR(); err != nil {
				return fmt.Errorf("error signing %s: %v", o.CSRFile, err)
			}
			glog.V(3).Infof("Generated new server certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
		}
	}

	if hasPhase(PhaseVerify, o.Phases) {
		verifyServerCert := &VerifyServerCertOptions{
			CertFile:       o.CertFile,
			PrivateKeyFile: o.KeyFile,
			CSRFile:        o.CSRFile,
			Hostnames:      o.Hostnames,
		}

		// If we signed it, verify the signature
		if hasPhase(PhaseSign, o.Phases) {
			verifyServerCert.CAFile = o.SignerCertOptions.CertFile
		}

		if err := verifyServerCert.VerifyServerCert(); err != nil {
			return fmt.Errorf("error verifying %s: %v", o.CertFile, err)
		}
	}

	if hasPhase(PhasePackage, o.Phases) {
		// no-op for server certs
	}

	return nil
}
