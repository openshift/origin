package deprecatedcerts

import (
	"errors"

	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/crypto"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type CreateServerCertOptions struct {
	SignerCertOptions *SignerCertOptions

	CertFile string
	KeyFile  string

	ExpireDays int

	Hostnames []string
	Overwrite bool

	genericclioptions.IOStreams
}

func (o CreateServerCertOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
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

	if o.SignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.SignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateServerCertOptions) CreateServerCert() (*crypto.TLSCertificateConfig, error) {
	klog.V(4).Infof("Creating a server cert with: %#v", o)

	signerCert, err := o.SignerCertOptions.CA()
	if err != nil {
		return nil, err
	}

	var ca *crypto.TLSCertificateConfig
	written := true
	if o.Overwrite {
		ca, err = signerCert.MakeAndWriteServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	} else {
		ca, written, err = signerCert.EnsureServerCert(o.CertFile, o.KeyFile, sets.NewString([]string(o.Hostnames)...), o.ExpireDays)
	}
	if written {
		klog.V(3).Infof("Generated new server certificate as %s, key as %s\n", o.CertFile, o.KeyFile)
	} else {
		klog.V(3).Infof("Keeping existing server certificate at %s, key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
