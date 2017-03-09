package admin

import (
	"errors"
	"io"

	"github.com/golang/glog"

	"k8s.io/apiserver/pkg/authentication/user"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateClientCertOptions struct {
	SignerCertOptions *SignerCertOptions

	CertFile string
	KeyFile  string

	ExpireDays int

	User   string
	Groups []string

	Overwrite bool
	Output    io.Writer
}

func (o CreateClientCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.CertFile) == 0 {
		return errors.New("cert must be provided")
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

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateClientCertOptions) CreateClientCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(4).Infof("Creating a client cert with: %#v and %#v", o, o.SignerCertOptions)

	signerCert, err := o.SignerCertOptions.CA()
	if err != nil {
		return nil, err
	}

	var cert *crypto.TLSCertificateConfig
	written := true
	userInfo := &user.DefaultInfo{Name: o.User, Groups: o.Groups}
	if o.Overwrite {
		cert, err = signerCert.MakeClientCertificate(o.CertFile, o.KeyFile, userInfo, o.ExpireDays)
	} else {
		cert, written, err = signerCert.EnsureClientCertificate(o.CertFile, o.KeyFile, userInfo, o.ExpireDays)
	}
	if written {
		glog.V(3).Infof("Generated new client cert as %s and key as %s\n", o.CertFile, o.KeyFile)
	} else {
		glog.V(3).Infof("Keeping existing client cert at %s and key at %s\n", o.CertFile, o.KeyFile)
	}
	return cert, err
}
