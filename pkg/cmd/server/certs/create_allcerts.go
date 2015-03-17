package certs

import (
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type CreateAllCertsOptions struct {
	CertDir    string
	SignerName string

	Hostnames util.StringList
	NodeList  util.StringList

	APIServerURL       string
	PublicAPIServerURL string

	Overwrite bool
}

func NewCommandCreateAllCerts() *cobra.Command {
	options := &CreateAllCertsOptions{}

	cmd := &cobra.Command{
		Use:   "create-all-certs",
		Short: "Create all certificates for OpenShift All-In-One",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if err := options.CreateAllCerts(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.CertDir, "cert-dir", "openshift.local.certificates", "The certificate data directory.")
	flags.StringVar(&options.SignerName, "signer-name", DefaultSignerName(), "The name to use for the generated signer.")

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.Var(&options.Hostnames, "hostnames", "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.Var(&options.NodeList, "nodes", "The names of all static nodes you'd like to generate certificates for. Comma delimited list")
	flags.BoolVar(&options.Overwrite, "overwrite", true, "Overwrite existing cert files if found.  If false, any existing file will be left as-is.")

	return cmd
}

func (o CreateAllCertsOptions) Validate(args []string) error {
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

func (o CreateAllCertsOptions) CreateAllCerts() error {
	glog.V(2).Infof("Creating all certs with: %#v", o)

	signerCertOptions := CreateSignerCertOptions{
		CertFile:   DefaultCertFilename(o.CertDir, DefaultCADir),
		KeyFile:    DefaultKeyFilename(o.CertDir, DefaultCADir),
		SerialFile: DefaultSerialFilename(o.CertDir, DefaultCADir),
		Name:       o.SignerName,
		Overwrite:  o.Overwrite,
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

	for _, clientCertInfo := range DefaultClientCerts(o.CertDir) {
		clientCertOptions := CreateClientCertOptions{
			GetSignerCertOptions: &getSignerCertOptions,

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,

			User:      clientCertInfo.User,
			Groups:    util.StringList(clientCertInfo.Groups.List()),
			Overwrite: o.Overwrite,
		}
		if _, err := clientCertOptions.CreateClientCert(); err != nil {
			return err
		}

		createKubeConfigOptions := CreateKubeConfigOptions{
			APIServerURL:       o.APIServerURL,
			PublicAPIServerURL: o.PublicAPIServerURL,
			APIServerCAFile:    getSignerCertOptions.CertFile,
			ServerNick:         "master",

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,
			UserNick: clientCertInfo.SubDir,

			KubeConfigFile: path.Join(filepath.Dir(clientCertOptions.CertFile), ".kubeconfig"),
		}
		if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
			return err
		}
	}

	for _, nodeName := range o.NodeList {
		serverCertInfo := DefaultNodeServingCertInfo(o.CertDir, nodeName)
		nodeServerCertOptions := CreateServerCertOptions{
			GetSignerCertOptions: &getSignerCertOptions,

			CertFile: serverCertInfo.CertFile,
			KeyFile:  serverCertInfo.KeyFile,

			Hostnames: []string{nodeName},
			Overwrite: o.Overwrite,
		}

		if _, err := nodeServerCertOptions.CreateServerCert(); err != nil {
			return err
		}

		username := "node-" + nodeName
		nodeCertOptions := CreateNodeClientCertOptions{
			GetSignerCertOptions: &getSignerCertOptions,

			CertFile: DefaultCertFilename(o.CertDir, username),
			KeyFile:  DefaultKeyFilename(o.CertDir, username),

			NodeName:  nodeName,
			Overwrite: o.Overwrite,
		}
		if _, err := nodeCertOptions.CreateNodeClientCert(); err != nil {
			return err
		}

		createKubeConfigOptions := CreateKubeConfigOptions{
			APIServerURL:       o.APIServerURL,
			PublicAPIServerURL: o.PublicAPIServerURL,
			APIServerCAFile:    getSignerCertOptions.CertFile,
			ServerNick:         "master",

			CertFile: nodeCertOptions.CertFile,
			KeyFile:  nodeCertOptions.KeyFile,
			UserNick: username,

			KubeConfigFile: path.Join(filepath.Dir(nodeCertOptions.CertFile), ".kubeconfig"),
		}
		if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
			return err
		}
	}

	for _, serverCertInfo := range DefaultServerCerts(o.CertDir) {
		serverCertOptions := CreateServerCertOptions{
			GetSignerCertOptions: &getSignerCertOptions,

			CertFile: serverCertInfo.CertFile,
			KeyFile:  serverCertInfo.KeyFile,

			Hostnames: o.Hostnames,
			Overwrite: o.Overwrite,
		}

		if _, err := serverCertOptions.CreateServerCert(); err != nil {
			return err
		}
	}

	return nil
}
