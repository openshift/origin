package admin

import (
	"errors"

	"github.com/golang/glog"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/auth/user"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/cmd/server/crypto"
)

type CreateClientCertOptions struct {
	GetSignerCertOptions *GetSignerCertOptions

	CertFile string
	KeyFile  string

	User   string
	Groups util.StringList

	Overwrite bool
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
	if len(o.User) == 0 {
		return errors.New("user must be provided")
	}

	if o.GetSignerCertOptions == nil {
		return errors.New("signer options are required")
	}
	if err := o.GetSignerCertOptions.Validate(); err != nil {
		return err
	}

	return nil
}

func (o CreateClientCertOptions) CreateClientCert() (*crypto.TLSCertificateConfig, error) {
	glog.V(2).Infof("Creating a client cert with: %#v and %#v", o, o.GetSignerCertOptions)

	signerCert, err := o.GetSignerCertOptions.GetSignerCert()
	if err != nil {
		return nil, err
	}

	userInfo := &user.DefaultInfo{Name: o.User, Groups: o.Groups}
	if o.Overwrite {
		return signerCert.MakeClientCertificate(o.CertFile, o.KeyFile, userInfo)
	} else {
		return signerCert.EnsureClientCertificate(o.CertFile, o.KeyFile, userInfo)
	}
}
