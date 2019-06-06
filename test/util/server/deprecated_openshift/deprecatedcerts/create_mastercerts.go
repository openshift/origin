package deprecatedcerts

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"path/filepath"

	"k8s.io/klog"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/util/cert"

	"github.com/openshift/oc/pkg/helpers/parallel"
	"github.com/openshift/origin/pkg/oc/cli/admin/createkubeconfig"
)

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

	genericclioptions.IOStreams
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
	klog.V(4).Infof("Creating all certs with: %#v", o)

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
		IOStreams:  o.IOStreams,
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

		createKubeConfigOptions := createkubeconfig.CreateKubeConfigOptions{
			APIServerURL:       o.APIServerURL,
			PublicAPIServerURL: o.PublicAPIServerURL,
			APIServerCAFiles:   append([]string{getSignerCertOptions.CertFile}, o.APIServerCAFiles...),

			CertFile: clientCertInfo.CertLocation.CertFile,
			KeyFile:  clientCertInfo.CertLocation.KeyFile,

			ContextNamespace: metav1.NamespaceDefault,

			KubeConfigFile: DefaultKubeConfigFilename(filepath.Dir(clientCertInfo.CertLocation.CertFile), clientCertInfo.UnqualifiedUser),
			IOStreams:      o.IOStreams,
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
		Output:    o.Out,
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
			IOStreams: o.IOStreams,
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
		IOStreams: o.IOStreams,
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
		IOStreams:  o.IOStreams,

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

// readFiles returns a byte array containing the contents of all the given filenames,
// optionally separated by a delimiter, or an error if any of the files cannot be read
func readFiles(srcFiles []string, separator []byte) ([]byte, error) {
	data := []byte{}
	for _, srcFile := range srcFiles {
		fileData, err := ioutil.ReadFile(srcFile)
		if err != nil {
			return nil, err
		}
		if len(data) > 0 && len(separator) > 0 {
			data = append(data, separator...)
		}
		data = append(data, fileData...)
	}
	return data, nil
}
