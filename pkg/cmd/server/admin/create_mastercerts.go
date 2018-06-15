package admin

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/util/parallel"
)

const CreateMasterCertsCommandName = "create-master-certs"

var masterCertLong = templates.LongDesc(`
	Create keys and certificates for a master

	This command creates keys and certs necessary to run a secure master.
	It also creates keys, certificates, and configuration necessary for most
	related infrastructure components that are clients to the master.
	See the related "create-node-config" command for generating per-node config.

	All files are expected or created in standard locations under the cert-dir.

	    openshift.local.config/master/
		    ca.{crt,key,serial.txt}
		    master.server.{crt,key}
			admin.{crt,key,kubeconfig}
			...

	Note that the certificate authority (CA aka "signer") generated automatically
	is self-signed. In production usage, administrators are more likely to
	want to generate signed certificates separately rather than rely on a
	generated CA. Alternatively, start with an existing signed CA and
	have this command use it to generate valid certificates.

	This command would usually only be used once at installation. If you
	need to regenerate the master server cert, DO NOT use --overwrite as this
	would recreate ALL certs including the CA cert, invalidating any existing
	infrastructure or client configuration. Instead, delete/rename the existing
	server cert and run the command to fill it in:

	    mv openshift.local.config/master/master.server.crt{,.old}
	    %[1]s --cert-dir=... \
	            --master=https://internal.master.fqdn:8443 \
	            --public-master=https://external.master.fqdn:8443 \
	            --hostnames=external.master.fqdn,internal.master.fqdn,localhost,127.0.0.1,172.17.42.1,kubernetes.default.local

	Alternatively, use the related "ca create-server-cert" command to explicitly
	create a certificate.

	Regardless of --overwrite, the master server key/cert will be updated
	if --hostnames does not match the current certificate.
	Regardless of --overwrite, .kubeconfig files will be updated every time this
	command is run, so always specify --master (and if needed, --public-master).
	This is designed to match the behavior of "start" which rewrites certs/confs
	for certain configuration changes.`)

type CreateMasterCertsOptions struct {
	CertDir    string
	SignerName string

	ExpireDays       int
	SignerExpireDays int

	APIServerCAFiles []string

	Hostnames []string

	APIServerURL       string
	PublicAPIServerURL string

	Overwrite bool
	Output    io.Writer
}

func NewCommandCreateMasterCerts(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateMasterCertsOptions{
		ExpireDays:       crypto.DefaultCertificateLifetimeInDays,
		SignerExpireDays: crypto.DefaultCACertificateLifetimeInDays,
		Output:           out,
	}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create certificates and keys for a master",
		Long:  fmt.Sprintf(masterCertLong, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(cmd, err.Error()))
			}

			if err := options.CreateMasterCerts(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.CertDir, "cert-dir", "openshift.local.config/master", "The certificate data directory.")
	flags.StringVar(&options.SignerName, "signer-name", DefaultSignerName(), "The name to use for the generated signer.")
	flags.StringSliceVar(&options.APIServerCAFiles, "certificate-authority", options.APIServerCAFiles, "Optional files containing signing authorities to use (in addition to the generated signer) to verify the API server's serving certificate.")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.StringSliceVar(&options.Hostnames, "hostnames", options.Hostnames, "Every hostname or IP that server certs should be valid for (comma-delimited list)")
	flags.BoolVar(&options.Overwrite, "overwrite", false, "Overwrite all existing cert/key/config files (WARNING: includes signer/CA)")

	flags.IntVar(&options.ExpireDays, "expire-days", options.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")
	flags.IntVar(&options.SignerExpireDays, "signer-expire-days", options.SignerExpireDays, "Validity of the CA certificate in days (defaults to 5 years). WARNING: extending this above default value is highly discouraged.")

	// set dynamic value annotation - allows man pages  to be generated and verified
	flags.SetAnnotation("signer-name", "manpage-def-value", []string{"openshift-signer@<current_timestamp>"})

	// autocompletion hints
	cmd.MarkFlagFilename("cert-dir")
	cmd.MarkFlagFilename("certificate-authority")

	return cmd
}

func (o CreateMasterCertsOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
	}
	if len(o.CertDir) == 0 {
		return errors.New("cert-dir must be provided")
	}
	if len(o.SignerName) == 0 {
		return errors.New("signer-name must be provided")
	}
	if o.ExpireDays <= 0 {
		return errors.New("expire-days must be valid number of days")
	}
	if o.SignerExpireDays <= 0 {
		return errors.New("signer-expire-days must be valid number of days")
	}
	if len(o.APIServerURL) == 0 {
		return errors.New("master must be provided")
	} else if u, err := url.Parse(o.APIServerURL); err != nil {
		return errors.New("master must be a valid URL (e.g. https://10.0.0.1:8443)")
	} else if len(u.Scheme) == 0 {
		return errors.New("master must be a valid URL (e.g. https://10.0.0.1:8443)")
	}

	if len(o.PublicAPIServerURL) == 0 {
		// not required
	} else if u, err := url.Parse(o.PublicAPIServerURL); err != nil {
		return errors.New("public master must be a valid URL (e.g. https://example.com:8443)")
	} else if len(u.Scheme) == 0 {
		return errors.New("public master must be a valid URL (e.g. https://example.com:8443)")
	}

	for _, caFile := range o.APIServerCAFiles {
		if _, err := cert.NewPool(caFile); err != nil {
			return fmt.Errorf("certificate authority must be a valid certificate file: %v", err)
		}
	}

	return nil
}

func (o CreateMasterCertsOptions) CreateMasterCerts() error {
	glog.V(4).Infof("Creating all certs with: %#v", o)

	getSignerCertOptions, err := o.createNewSigner(CAFilePrefix)
	if err != nil {
		return err
	}

	frontProxyOptions := o
	frontProxyOptions.SignerName = DefaultFrontProxySignerName()
	getFrontProxySignerCertOptions, err := frontProxyOptions.createNewSigner(FrontProxyCAFilePrefix)
	if err != nil {
		return err
	}

	errs := parallel.Run(
		func() error { return o.createCABundle(getSignerCertOptions) },
		func() error { return o.createServerCerts(getSignerCertOptions) },
		func() error { return o.createAPIClients(getSignerCertOptions) },
		func() error { return o.createEtcdClientCerts(getSignerCertOptions) },
		func() error { return o.createKubeletClientCerts(getSignerCertOptions) },
		func() error { return o.createProxyClientCerts(getSignerCertOptions) },
		func() error { return o.createServiceAccountKeys() },
		func() error { return o.createServiceSigningCA(getSignerCertOptions) },
		func() error { return frontProxyOptions.createAggregatorClientCerts(getFrontProxySignerCertOptions) },
	)
	return utilerrors.NewAggregate(errs)
}

func (o CreateMasterCertsOptions) createNewSigner(prefix string) (*SignerCertOptions, error) {
	signerCertOptions := CreateSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, prefix),
		KeyFile:    DefaultKeyFilename(o.CertDir, prefix),
		SerialFile: DefaultSerialFilename(o.CertDir, prefix),
		ExpireDays: o.SignerExpireDays,
		Name:       o.SignerName,
		Overwrite:  o.Overwrite,
		Output:     o.Output,
	}
	if err := signerCertOptions.Validate(nil); err != nil {
		return nil, err
	}
	if _, err := signerCertOptions.CreateSignerCert(); err != nil {
		return nil, err
	}
	// once we've minted the signer, don't overwrite it
	return &SignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, prefix),
		KeyFile:    DefaultKeyFilename(o.CertDir, prefix),
		SerialFile: DefaultSerialFilename(o.CertDir, prefix),
	}, nil

}

func (o CreateMasterCertsOptions) createAPIClients(getSignerCertOptions *SignerCertOptions) error {
	for _, clientCertInfo := range DefaultAPIClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}

		createKubeConfigOptions := CreateKubeConfigOptions{
			APIServerURL:       o.APIServerURL,
			PublicAPIServerURL: o.PublicAPIServerURL,
			APIServerCAFiles:   append([]string{getSignerCertOptions.CertFile}, o.APIServerCAFiles...),

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,

			ContextNamespace: metav1.NamespaceDefault,

			KubeConfigFile: DefaultKubeConfigFilename(filepath.Dir(clientCertInfo.CertLocation.CertFile), clientCertInfo.UnqualifiedUser),
			Output:         o.Output,
		}
		if err := createKubeConfigOptions.Validate(nil); err != nil {
			return err
		}
		if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createAggregatorClientCerts(getSignerCertOptions *SignerCertOptions) error {
	if err := o.createClientCert(DefaultAggregatorClientCertInfo(o.CertDir), getSignerCertOptions); err != nil {
		return err
	}
	return nil
}

func (o CreateMasterCertsOptions) createEtcdClientCerts(getSignerCertOptions *SignerCertOptions) error {
	for _, clientCertInfo := range DefaultEtcdClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createProxyClientCerts(getSignerCertOptions *SignerCertOptions) error {
	for _, clientCertInfo := range DefaultProxyClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createKubeletClientCerts(getSignerCertOptions *SignerCertOptions) error {
	for _, clientCertInfo := range DefaultKubeletClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createClientCert(clientCertInfo ClientCertInfo, getSignerCertOptions *SignerCertOptions) error {
	clientCertOptions := CreateClientCertOptions{
		SignerCertOptions: getSignerCertOptions,

		CertFile: clientCertInfo.CertLocation.CertFile,
		KeyFile:  clientCertInfo.CertLocation.KeyFile,

		ExpireDays: o.ExpireDays,

		User:      clientCertInfo.User,
		Groups:    clientCertInfo.Groups.List(),
		Overwrite: o.Overwrite,
		Output:    o.Output,
	}
	if err := clientCertOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := clientCertOptions.CreateClientCert(); err != nil {
		return err
	}
	return nil
}

func (o CreateMasterCertsOptions) createCABundle(getSignerCertOptions *SignerCertOptions) error {
	caFiles := []string{getSignerCertOptions.CertFile}
	caFiles = append(caFiles, o.APIServerCAFiles...)
	caData, err := readFiles(caFiles, []byte("\n"))
	if err != nil {
		return err
	}

	// ensure parent dir
	if err := os.MkdirAll(o.CertDir, os.FileMode(0755)); err != nil {
		return err
	}
	return ioutil.WriteFile(DefaultCABundleFile(o.CertDir), caData, 0644)
}

func (o CreateMasterCertsOptions) createServerCerts(getSignerCertOptions *SignerCertOptions) error {
	for _, serverCertInfo := range DefaultServerCerts(o.CertDir) {
		serverCertOptions := CreateServerCertOptions{
			SignerCertOptions: getSignerCertOptions,

			CertFile: serverCertInfo.CertFile,
			KeyFile:  serverCertInfo.KeyFile,

			ExpireDays: o.ExpireDays,

			Hostnames: o.Hostnames,
			Overwrite: o.Overwrite,
			Output:    o.Output,
		}
		if err := serverCertOptions.Validate(nil); err != nil {
			return err
		}
		if _, err := serverCertOptions.CreateServerCert(); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createServiceAccountKeys() error {
	keypairOptions := CreateKeyPairOptions{
		PublicKeyFile:  DefaultServiceAccountPublicKeyFile(o.CertDir),
		PrivateKeyFile: DefaultServiceAccountPrivateKeyFile(o.CertDir),

		Overwrite: o.Overwrite,
		Output:    o.Output,
	}
	if err := keypairOptions.Validate(nil); err != nil {
		return err
	}
	if err := keypairOptions.CreateKeyPair(); err != nil {
		return err
	}
	return nil
}

func (o CreateMasterCertsOptions) createServiceSigningCA(getSignerCertOptions *SignerCertOptions) error {
	caInfo := DefaultServiceSignerCAInfo(o.CertDir)

	caOptions := CreateSignerCertOptions{
		CertFile:   caInfo.CertFile,
		KeyFile:    caInfo.KeyFile,
		SerialFile: "", // we want the random cert serial for this one
		ExpireDays: o.SignerExpireDays,
		Name:       DefaultServiceServingCertSignerName(),
		Output:     o.Output,

		Overwrite: o.Overwrite,
	}
	if err := caOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := caOptions.CreateSignerCert(); err != nil {
		return err
	}
	return nil
}
