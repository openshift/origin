package admin

import (
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/openshift/origin/pkg/util/parallel"
)

const CreateMasterCertsCommandName = "create-master-certs"
const masterCertLong = `Create keys and certificates for an OpenShift master

This command creates keys and certs necessary to run a secure OpenShift master.
It also creates keys, certificates, and configuration necessary for most
related infrastructure components that are clients to the master.
See the related "create-node-config" command for generating per-node config.

All files are expected or created in standard locations under the cert-dir.

    openshift.local.config/master/
	    ca.{crt,key,serial.txt}
	    master.server.{crt,key}
		openshift-router.{crt,key,kubeconfig}
		admin.{crt,key,kubeconfig}
		...

Note that the certificate authority (CA aka "signer") generated automatically
is self-signed. In production usage, administrators are more likely to
want to generate signed certificates separately rather than rely on an
OpenShift-generated CA. Alternatively, start with an existing signed CA and
have this command use it to generate valid certificates.

This command would usually only be used once at installation. If you
need to regenerate the master server cert, DO NOT use --overwrite as this
would recreate ALL certs including the CA cert, invalidating any existing
infrastructure or client configuration. Instead, delete/rename the existing
server cert and run the command to fill it in:

    $ mv openshift.local.config/master/master.server.crt{,.old}
    $ %[1]s --cert-dir=... \
            --master=https://internal.master.fqdn:8443 \
            --public-master=https://external.master.fqdn:8443 \
            --hostnames=external.master.fqdn,internal.master.fqdn,localhost,127.0.0.1,172.17.42.1,kubernetes.default.local

Alternatively, use the related "create-server-cert" command to explicitly
create a certificate.

Regardless of --overwrite, the master server key/cert will be updated 
if --hostnames does not match the current certificate.
Regardless of --overwrite, .kubeconfig files will be updated every time this
command is run, so always specify --master (and if needed, --public-master).
This is designed to match the behavior of "openshift start" which rewrites
certs/confs for certain configuration changes.
`

type CreateMasterCertsOptions struct {
	CertDir    string
	SignerName string

	Hostnames util.StringList

	APIServerURL       string
	PublicAPIServerURL string

	Overwrite bool
	Output    io.Writer
}

func NewCommandCreateMasterCerts(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateMasterCertsOptions{Output: out}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create certificates for an OpenShift master",
		Long:  fmt.Sprintf(masterCertLong, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.CreateMasterCerts(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.CertDir, "cert-dir", "openshift.local.config/master", "The certificate data directory.")
	flags.StringVar(&options.SignerName, "signer-name", DefaultSignerName(), "The name to use for the generated signer.")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP that server certs should be valid for (comma-delimited list)")
	flags.BoolVar(&options.Overwrite, "overwrite", false, "Overwrite all existing cert/key/config files (WARNING: includes signer/CA)")

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
	if len(o.APIServerURL) == 0 {
		return errors.New("master must be provided")
	}

	return nil
}

func (o CreateMasterCertsOptions) CreateMasterCerts() error {
	glog.V(4).Infof("Creating all certs with: %#v", o)

	signerCertOptions := CreateSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, CAFilePrefix),
		KeyFile:    DefaultKeyFilename(o.CertDir, CAFilePrefix),
		SerialFile: DefaultSerialFilename(o.CertDir, CAFilePrefix),
		Name:       o.SignerName,
		Overwrite:  o.Overwrite,
		Output:     o.Output,
	}
	if err := signerCertOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := signerCertOptions.CreateSignerCert(); err != nil {
		return err
	}
	// once we've minted the signer, don't overwrite it
	getSignerCertOptions := GetSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, CAFilePrefix),
		KeyFile:    DefaultKeyFilename(o.CertDir, CAFilePrefix),
		SerialFile: DefaultSerialFilename(o.CertDir, CAFilePrefix),
	}

	errs := parallel.Run(
		func() error { return o.createServerCerts(&getSignerCertOptions) },
		func() error { return o.createAPIClients(&getSignerCertOptions) },
		func() error { return o.createEtcdClientCerts(&getSignerCertOptions) },
		func() error { return o.createKubeletClientCerts(&getSignerCertOptions) },
		func() error { return o.createServiceAccountKeys() },
	)
	return utilerrors.NewAggregate(errs)
}

func (o CreateMasterCertsOptions) createAPIClients(getSignerCertOptions *GetSignerCertOptions) error {
	for _, clientCertInfo := range DefaultAPIClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}

		createKubeConfigOptions := CreateKubeConfigOptions{
			APIServerURL:       o.APIServerURL,
			PublicAPIServerURL: o.PublicAPIServerURL,
			APIServerCAFile:    getSignerCertOptions.CertFile,

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,

			ContextNamespace: kapi.NamespaceDefault,

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

func (o CreateMasterCertsOptions) createEtcdClientCerts(getSignerCertOptions *GetSignerCertOptions) error {
	for _, clientCertInfo := range DefaultEtcdClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createKubeletClientCerts(getSignerCertOptions *GetSignerCertOptions) error {
	for _, clientCertInfo := range DefaultKubeletClientCerts(o.CertDir) {
		if err := o.createClientCert(clientCertInfo, getSignerCertOptions); err != nil {
			return err
		}
	}
	return nil
}

func (o CreateMasterCertsOptions) createClientCert(clientCertInfo ClientCertInfo, getSignerCertOptions *GetSignerCertOptions) error {
	clientCertOptions := CreateClientCertOptions{
		GetSignerCertOptions: getSignerCertOptions,

		CertFile: clientCertInfo.CertLocation.CertFile,
		KeyFile:  clientCertInfo.CertLocation.KeyFile,

		User:      clientCertInfo.User,
		Groups:    util.StringList(clientCertInfo.Groups.List()),
		Overwrite: o.Overwrite,
		Output:    o.Output,
	}
	if _, err := clientCertOptions.CreateClientCert(); err != nil {
		return err
	}
	return nil
}

func (o CreateMasterCertsOptions) createServerCerts(getSignerCertOptions *GetSignerCertOptions) error {
	for _, serverCertInfo := range DefaultServerCerts(o.CertDir) {
		serverCertOptions := CreateServerCertOptions{
			GetSignerCertOptions: getSignerCertOptions,

			CertFile: serverCertInfo.CertFile,
			KeyFile:  serverCertInfo.KeyFile,

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
