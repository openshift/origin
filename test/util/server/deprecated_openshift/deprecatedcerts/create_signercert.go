package deprecatedcerts

import (
	"errors"

	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/crypto"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

type CreateSignerCertOptions struct {
	CertFile   string
	KeyFile    string
	SerialFile string
	ExpireDays int
	Name       string

	Overwrite bool

	genericclioptions.IOStreams
}

func (o CreateSignerCertOptions) Validate(args []string) error {
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
	if len(o.Name) == 0 {
		return errors.New("name must be provided")
	}

	return nil
}

func (o CreateSignerCertOptions) CreateSignerCert() (*crypto.CA, error) {
	klog.V(4).Infof("Creating a signer cert with: %#v", o)
	var ca *crypto.CA
	var err error
	written := true
	if o.Overwrite {
		ca, err = crypto.MakeSelfSignedCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name, o.ExpireDays)
	} else {
		ca, written, err = crypto.EnsureCA(o.CertFile, o.KeyFile, o.SerialFile, o.Name, o.ExpireDays)
	}
	if written {
		klog.V(3).Infof("Generated new CA for %s: cert in %s and key in %s\n", o.Name, o.CertFile, o.KeyFile)
	} else {
		klog.V(3).Infof("Keeping existing CA cert at %s and key at %s\n", o.CertFile, o.KeyFile)
	}
	return ca, err
}
