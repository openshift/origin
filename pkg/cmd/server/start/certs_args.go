package start

import (
	"github.com/spf13/pflag"
)

type CertArgs struct {
	CertDir        string
	CreateCerts    bool
	OverwriteCerts bool
}

func BindCertArgs(args *CertArgs, flags *pflag.FlagSet, prefix string) {
	flags.BoolVar(&args.CreateCerts, prefix+"create-certs", true, "Create any missing certificates required for launch or for writing the config file.")
	flags.StringVar(&args.CertDir, prefix+"cert-dir", "openshift.local.certificates", "The certificate data directory.")
}

func NewDefaultCertArgs() *CertArgs {
	return &CertArgs{CreateCerts: true}
}
