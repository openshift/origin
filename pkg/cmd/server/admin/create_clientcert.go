package admin

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
)

type CreateClientCertOptions struct {
	Phases []string

	SignerCertOptions *SignerCertOptions

	CSRFile  string
	CertFile string
	KeyFile  string

	ExpireDays int

	User   string
	Groups []string

	Overwrite bool
	Output    io.Writer
}

func (o *CreateClientCertOptions) Complete(args []string) error {
	if len(o.CSRFile) == 0 && len(o.CertFile) > 0 {
		o.CSRFile = o.CertFile + ".csr"
	}
	return nil
}

func (o CreateClientCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if err := ValidatePhases(o.Phases); err != nil {
		return fmt.Errorf("client cert phase error: %v", err)
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
	}
	if len(o.CSRFile) == 0 {
		return errors.New("client csr must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("key must be provided")
	}
	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if len(o.User) == 0 {
		return errors.New("user must be provided")
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

func (o CreateClientCertOptions) CreateClientCert() error {
	glog.V(4).Infof("Creating a client cert with: %#v and %#v", o, o.SignerCertOptions)

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
		createClientCSR := &CreateClientCSROptions{
			PrivateKeyFile: o.KeyFile,
			CSRFile:        o.CSRFile,
			User:           o.User,
			Groups:         o.Groups,
		}
		if err := createClientCSR.CreateClientCSR(); err != nil {
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
			verifyClientCert := &VerifyClientCertOptions{
				CertFile:       o.CertFile,
				PrivateKeyFile: o.KeyFile,
				CSRFile:        o.CSRFile,
				User:           o.User,
				Groups:         o.Groups,
				CAFile:         o.SignerCertOptions.CertFile,
			}
			if err := verifyClientCert.VerifyClientCert(); err != nil {
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
			glog.V(3).Infof("Generated new client certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
		}
	}

	if hasPhase(PhaseVerify, o.Phases) {
		verifyClientCert := &VerifyClientCertOptions{
			CertFile:       o.CertFile,
			PrivateKeyFile: o.KeyFile,
			CSRFile:        o.CSRFile,
			User:           o.User,
			Groups:         o.Groups,
		}

		// If we signed it, verify the signature
		if hasPhase(PhaseSign, o.Phases) {
			verifyClientCert.CAFile = o.SignerCertOptions.CertFile
		}

		if err := verifyClientCert.VerifyClientCert(); err != nil {
			return fmt.Errorf("error verifying %s: %v", o.CertFile, err)
		}
	}

	if hasPhase(PhasePackage, o.Phases) {
		// no-op for client certs
	}

	return nil
}
