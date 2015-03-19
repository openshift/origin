package admin

import (
	"errors"
	"fmt"
	"io"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

const CreateMasterCertsCommandName = "create-master-certs"

type CreateMasterCertsOptions struct {
	CertDir    string
	SignerName string

	Hostnames util.StringList

	APIServerURL       string
	PublicAPIServerURL string

	Overwrite bool
}

func NewCommandCreateMasterCerts(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateMasterCertsOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create all certificates for an OpenShift master.  To create node certificates, try openshift admin create-node-config",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), err.Error())
				c.Help()
				return
			}

			if err := options.CreateMasterCerts(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()

	flags.StringVar(&options.CertDir, "cert-dir", "openshift.local.certificates", "The certificate data directory.")
	flags.StringVar(&options.SignerName, "signer-name", DefaultSignerName(), "The name to use for the generated signer.")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

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
	glog.V(2).Infof("Creating all certs with: %#v", o)

	signerCertOptions := CreateSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, DefaultCADir),
		KeyFile:    DefaultKeyFilename(o.CertDir, DefaultCADir),
		SerialFile: DefaultSerialFilename(o.CertDir, DefaultCADir),
		Name:       o.SignerName,
		Overwrite:  o.Overwrite,
	}
	if err := signerCertOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := signerCertOptions.CreateSignerCert(); err != nil {
		return err
	}
	// once we've minted the signer, don't overwrite it
	getSignerCertOptions := GetSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, DefaultCADir),
		KeyFile:    DefaultKeyFilename(o.CertDir, DefaultCADir),
		SerialFile: DefaultSerialFilename(o.CertDir, DefaultCADir),
	}

	if err := o.createServerCerts(&getSignerCertOptions); err != nil {
		return err
	}

	if err := o.createAPIClients(&getSignerCertOptions); err != nil {
		return err
	}

	if err := o.createEtcdClientCerts(&getSignerCertOptions); err != nil {
		return err
	}

	if err := o.createKubeletClientCerts(&getSignerCertOptions); err != nil {
		return err
	}

	return nil
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
			ServerNick:         "master",

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,
			UserNick: clientCertInfo.User,

			KubeConfigFile: path.Join(filepath.Dir(clientCertInfo.CertLocation.CertFile), ".kubeconfig"),
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
