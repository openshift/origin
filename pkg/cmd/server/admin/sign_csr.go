package admin

import (
	"time"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type SignCSROptions struct {
	SignerCertOptions *SignerCertOptions

	ExpireDays int

	CSRFile  string
	CertFile string
}

func (o *SignCSROptions) SignCSR() error {
	request, err := CSRFromFile(o.CSRFile)
	if err != nil {
		return err
	}

	signerCert, err := o.SignerCertOptions.CA()
	if err != nil {
		return err
	}

	certs, err := signerCert.SignRequest(request, o.ExpireDays, time.Now)
	if err != nil {
		return err
	}

	return crypto.WriteCertificates(o.CertFile, certs...)
}
