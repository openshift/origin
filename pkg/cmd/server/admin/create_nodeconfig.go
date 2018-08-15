package admin

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/master/ports"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	latestconfigapi "github.com/openshift/origin/pkg/cmd/server/apis/config/latest"
	configv1 "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const NodeConfigCommandName = "create-node-config"

type CreateNodeConfigOptions struct {
	SignerCertOptions *SignerCertOptions

	NodeConfigDir string

	NodeName               string
	Hostnames              []string
	VolumeDir              string
	ImageTemplate          variable.ImageTemplate
	AllowDisabledDocker    bool
	DNSBindAddress         string
	DNSDomain              string
	DNSIP                  string
	DNSRecursiveResolvConf string
	ListenAddr             flagtypes.Addr

	KubeletArguments map[string][]string

	ClientCertFile    string
	ClientKeyFile     string
	ServerCertFile    string
	ServerKeyFile     string
	ExpireDays        int
	NodeClientCAFile  string
	APIServerCAFiles  []string
	APIServerURL      string
	NetworkPluginName string

	genericclioptions.IOStreams
}

func NewCommandNodeConfig(commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewDefaultCreateNodeConfigOptions()
	o.IOStreams = streams
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a configuration bundle for a node",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			if _, err := o.CreateNodeFolder(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	BindSignerCertOptions(o.SignerCertOptions, cmd.Flags(), "")

	cmd.Flags().StringVar(&o.NodeConfigDir, "node-dir", o.NodeConfigDir, "The client data directory.")

	cmd.Flags().StringVar(&o.NodeName, "node", o.NodeName, "The name of the node as it appears in etcd.")
	cmd.Flags().StringSliceVar(&o.Hostnames, "hostnames", o.Hostnames, "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	cmd.Flags().StringVar(&o.VolumeDir, "volume-dir", o.VolumeDir, "The volume storage directory.  This path is not relativized.")
	cmd.Flags().StringVar(&o.ImageTemplate.Format, "images", o.ImageTemplate.Format, "When fetching the network container image, use this format. The latest release will be used by default.")
	cmd.Flags().BoolVar(&o.ImageTemplate.Latest, "latest-images", o.ImageTemplate.Latest, "If true, attempt to use the latest images for the cluster instead of the latest release.")
	cmd.Flags().BoolVar(&o.AllowDisabledDocker, "allow-disabled-docker", o.AllowDisabledDocker, "Allow the node to start without docker being available.")
	cmd.Flags().StringVar(&o.DNSBindAddress, "dns-bind-address", o.DNSBindAddress, "An address to bind DNS to.")
	cmd.Flags().StringVar(&o.DNSDomain, "dns-domain", o.DNSDomain, "DNS domain for the cluster.")
	cmd.Flags().StringVar(&o.DNSIP, "dns-ip", o.DNSIP, "DNS server IP for the cluster.")
	cmd.Flags().Var(&o.ListenAddr, "listen", "The address to listen for connections on (scheme://host:port).")

	cmd.Flags().StringVar(&o.ClientCertFile, "client-certificate", o.ClientCertFile, "The client cert file for the node to contact the API.")
	cmd.Flags().StringVar(&o.ClientKeyFile, "client-key", o.ClientKeyFile, "The client key file for the node to contact the API.")
	cmd.Flags().StringVar(&o.ServerCertFile, "server-certificate", o.ServerCertFile, "The server cert file for the node to serve secure traffic.")
	cmd.Flags().StringVar(&o.ServerKeyFile, "server-key", o.ServerKeyFile, "The server key file for the node to serve secure traffic.")
	cmd.Flags().IntVar(&o.ExpireDays, "expire-days", o.ExpireDays, "Validity of the certificates in days (defaults to 2 years). WARNING: extending this above default value is highly discouraged.")
	cmd.Flags().StringVar(&o.NodeClientCAFile, "node-client-certificate-authority", o.NodeClientCAFile, "The file containing signing authorities to use to verify requests to the node. If empty, all requests will be allowed.")
	cmd.Flags().StringVar(&o.APIServerURL, "master", o.APIServerURL, "The API server's URL.")
	cmd.Flags().StringSliceVar(&o.APIServerCAFiles, "certificate-authority", o.APIServerCAFiles, "Files containing signing authorities to use to verify the API server's serving certificate.")
	cmd.Flags().StringVar(&o.NetworkPluginName, "network-plugin", o.NetworkPluginName, "Name of the network plugin to hook to for pod networking.")

	// autocompletion hints
	cmd.MarkFlagFilename("node-dir")
	cmd.MarkFlagFilename("volume-dir")
	cmd.MarkFlagFilename("client-certificate")
	cmd.MarkFlagFilename("client-key")
	cmd.MarkFlagFilename("server-certificate")
	cmd.MarkFlagFilename("server-key")
	cmd.MarkFlagFilename("node-client-certificate-authority")
	cmd.MarkFlagFilename("certificate-authority")

	return cmd
}

func NewDefaultCreateNodeConfigOptions() *CreateNodeConfigOptions {
	options := &CreateNodeConfigOptions{
		SignerCertOptions: NewDefaultSignerCertOptions(),
		ExpireDays:        crypto.DefaultCertificateLifetimeInDays,
	}
	options.VolumeDir = "openshift.local.volumes"
	// TODO: replace me with a proper round trip of config options through decode
	options.DNSDomain = "cluster.local"
	options.APIServerURL = "https://localhost:8443"
	options.APIServerCAFiles = []string{"openshift.local.config/master/ca.crt"}
	options.NodeClientCAFile = "openshift.local.config/master/ca.crt"

	options.ImageTemplate = variable.NewDefaultImageTemplate()

	options.ListenAddr = flagtypes.Addr{Value: "0.0.0.0:10250", DefaultScheme: "https", DefaultPort: 10250, AllowPrefix: true}.Default()
	options.NetworkPluginName = ""

	return options
}

func (o CreateNodeConfigOptions) IsCreateClientCertificate() bool {
	return len(o.ClientCertFile) == 0 && len(o.ClientKeyFile) == 0
}

func (o CreateNodeConfigOptions) IsCreateServerCertificate() bool {
	return len(o.ServerCertFile) == 0 && len(o.ServerKeyFile) == 0 && o.UseTLS()
}

func (o CreateNodeConfigOptions) UseTLS() bool {
	return o.ListenAddr.URL.Scheme == "https"
}

func (o CreateNodeConfigOptions) UseNodeClientCA() bool {
	return o.UseTLS() && len(o.NodeClientCAFile) > 0
}

func (o CreateNodeConfigOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.NodeConfigDir) == 0 {
		return errors.New("--node-dir must be provided")
	}
	if len(o.NodeName) == 0 {
		return errors.New("--node must be provided")
	}
	if len(o.APIServerURL) == 0 {
		return errors.New("--master must be provided")
	}
	if len(o.APIServerCAFiles) == 0 {
		return fmt.Errorf("--certificate-authority must be a valid certificate file")
	} else {
		for _, caFile := range o.APIServerCAFiles {
			if _, err := cert.NewPool(caFile); err != nil {
				return fmt.Errorf("--certificate-authority must be a valid certificate file: %v", err)
			}
		}
	}
	if len(o.Hostnames) == 0 {
		return errors.New("at least one hostname must be provided")
	}

	if len(o.ClientCertFile) != 0 {
		if len(o.ClientKeyFile) == 0 {
			return errors.New("--client-key must be provided if --client-certificate is provided")
		}
	} else if len(o.ClientKeyFile) != 0 {
		return errors.New("--client-certificate must be provided if --client-key is provided")
	}

	if len(o.ServerCertFile) != 0 {
		if len(o.ServerKeyFile) == 0 {
			return errors.New("--server-key must be provided if --server-certificate is provided")
		}
	} else if len(o.ServerKeyFile) != 0 {
		return errors.New("--server-certificate must be provided if --server-key is provided")
	}

	if o.IsCreateClientCertificate() || o.IsCreateServerCertificate() {
		if len(o.SignerCertOptions.KeyFile) == 0 {
			return errors.New("--signer-key must be provided to create certificates")
		}
		if len(o.SignerCertOptions.CertFile) == 0 {
			return errors.New("--signer-cert must be provided to create certificates")
		}
		if len(o.SignerCertOptions.SerialFile) == 0 {
			return errors.New("--signer-serial must be provided to create certificates")
		}
	}

	if o.ExpireDays < 0 {
		return errors.New("expire-days must be valid number of days")
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

func CopyFile(src, dest string, permissions os.FileMode) error {
	// copy the cert and key over
	if content, err := ioutil.ReadFile(src); err != nil {
		return err
	} else if err := ioutil.WriteFile(dest, content, permissions); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) CreateNodeFolder() (string, error) {
	servingCertInfo := DefaultNodeServingCertInfo(o.NodeConfigDir)
	clientCertInfo := DefaultNodeClientCertInfo(o.NodeConfigDir)

	clientCertFile := clientCertInfo.CertFile
	clientKeyFile := clientCertInfo.KeyFile
	apiServerCAFile := DefaultCAFilename(o.NodeConfigDir, CAFilePrefix)

	serverCertFile := servingCertInfo.CertFile
	serverKeyFile := servingCertInfo.KeyFile
	nodeClientCAFile := DefaultCAFilename(o.NodeConfigDir, "node-client-ca")

	kubeConfigFile := DefaultNodeKubeConfigFile(o.NodeConfigDir)
	nodeConfigFile := path.Join(o.NodeConfigDir, "node-config.yaml")
	nodeJSONFile := path.Join(o.NodeConfigDir, "node-registration.json")

	fmt.Fprintf(o.Out, "Generating node credentials ...\n")

	if err := o.MakeClientCert(clientCertFile, clientKeyFile); err != nil {
		return "", err
	}
	if o.UseTLS() {
		if err := o.MakeAndWriteServerCert(serverCertFile, serverKeyFile); err != nil {
			return "", err
		}
		if o.UseNodeClientCA() {
			if err := o.MakeNodeClientCA(nodeClientCAFile); err != nil {
				return "", err
			}
		}
	}
	if err := o.MakeAPIServerCA(apiServerCAFile); err != nil {
		return "", err
	}
	if err := o.MakeKubeConfig(clientCertFile, clientKeyFile, apiServerCAFile, kubeConfigFile); err != nil {
		return "", err
	}
	if err := o.MakeNodeConfig(serverCertFile, serverKeyFile, nodeClientCAFile, kubeConfigFile, nodeConfigFile); err != nil {
		return "", err
	}
	if err := o.MakeNodeJSON(nodeJSONFile); err != nil {
		return "", err
	}

	fmt.Fprintf(o.Out, "Created node config for %s in %s\n", o.NodeName, o.NodeConfigDir)

	return nodeConfigFile, nil
}

func (o CreateNodeConfigOptions) MakeClientCert(clientCertFile, clientKeyFile string) error {
	if o.IsCreateClientCertificate() {
		createNodeClientCert := CreateClientCertOptions{
			SignerCertOptions: o.SignerCertOptions,

			CertFile: clientCertFile,
			KeyFile:  clientKeyFile,

			ExpireDays: o.ExpireDays,

			User:   "system:node:" + o.NodeName,
			Groups: []string{bootstrappolicy.NodesGroup},
			Output: o.Out,
		}

		if err := createNodeClientCert.Validate(nil); err != nil {
			return err
		}
		if _, err := createNodeClientCert.CreateClientCert(); err != nil {
			return err
		}

	} else {
		if err := CopyFile(o.ClientCertFile, clientCertFile, 0644); err != nil {
			return err
		}
		if err := CopyFile(o.ClientKeyFile, clientKeyFile, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeAndWriteServerCert(serverCertFile, serverKeyFile string) error {
	if o.IsCreateServerCertificate() {
		nodeServerCertOptions := CreateServerCertOptions{
			SignerCertOptions: o.SignerCertOptions,

			CertFile: serverCertFile,
			KeyFile:  serverKeyFile,

			ExpireDays: o.ExpireDays,

			Hostnames: o.Hostnames,
			IOStreams: o.IOStreams,
		}

		if err := nodeServerCertOptions.Validate(nil); err != nil {
			return err
		}
		if _, err := nodeServerCertOptions.CreateServerCert(); err != nil {
			return err
		}

	} else {
		if err := CopyFile(o.ServerCertFile, serverCertFile, 0644); err != nil {
			return err
		}
		if err := CopyFile(o.ServerKeyFile, serverKeyFile, 0600); err != nil {
			return err
		}
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeAPIServerCA(clientCopyOfCAFile string) error {
	content, err := readFiles(o.APIServerCAFiles, []byte("\n"))
	if err != nil {
		return err
	}
	return ioutil.WriteFile(clientCopyOfCAFile, content, 0644)
}

func (o CreateNodeConfigOptions) MakeNodeClientCA(clientCopyOfCAFile string) error {
	if err := CopyFile(o.NodeClientCAFile, clientCopyOfCAFile, 0644); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeKubeConfig(clientCertFile, clientKeyFile, clientCopyOfCAFile, kubeConfigFile string) error {
	createKubeConfigOptions := CreateKubeConfigOptions{
		APIServerURL:     o.APIServerURL,
		APIServerCAFiles: []string{clientCopyOfCAFile},

		CertFile: clientCertFile,
		KeyFile:  clientKeyFile,

		ContextNamespace: metav1.NamespaceDefault,

		KubeConfigFile: kubeConfigFile,
		IOStreams:      o.IOStreams,
	}
	if err := createKubeConfigOptions.Validate(nil); err != nil {
		return err
	}
	if _, err := createKubeConfigOptions.CreateKubeConfig(); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeNodeConfig(serverCertFile, serverKeyFile, nodeClientCAFile, kubeConfigFile, nodeConfigFile string) error {
	config := &configapi.NodeConfig{
		NodeName: o.NodeName,

		ServingInfo: configapi.ServingInfo{
			BindAddress: o.ListenAddr.HostPort(ports.KubeletPort),
		},

		VolumeDirectory:     o.VolumeDir,
		AllowDisabledDocker: o.AllowDisabledDocker,

		ImageConfig: configapi.ImageConfig{
			Format: o.ImageTemplate.Format,
			Latest: o.ImageTemplate.Latest,
		},

		DNSBindAddress: o.DNSBindAddress,
		DNSDomain:      o.DNSDomain,
		DNSIP:          o.DNSIP,

		MasterKubeConfig: kubeConfigFile,

		NetworkConfig: configapi.NodeNetworkConfig{
			NetworkPluginName: o.NetworkPluginName,
		},

		KubeletArguments: o.KubeletArguments,

		EnableUnidling: true,
	}

	if o.UseTLS() {
		config.ServingInfo.ServerCert = configapi.CertInfo{
			CertFile: serverCertFile,
			KeyFile:  serverKeyFile,
		}
		config.ServingInfo.ClientCA = nodeClientCAFile
	}

	// Resolve relative to CWD
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	if err := configapi.ResolveNodeConfigPaths(config, cwd); err != nil {
		return err
	}

	// Relativize to config file dir
	base, err := cmdutil.MakeAbs(o.NodeConfigDir, cwd)
	if err != nil {
		return err
	}
	if err := configapi.RelativizeNodeConfigPaths(config, base); err != nil {
		return err
	}

	// Roundtrip the config to v1 and back to ensure proper defaults are set.
	ext, err := configapi.Scheme.ConvertToVersion(config, configv1.LegacySchemeGroupVersion)
	if err != nil {
		return err
	}
	configapi.Scheme.Default(ext)
	internal, err := configapi.Scheme.ConvertToVersion(ext, configapi.SchemeGroupVersion)
	if err != nil {
		return err
	}
	config = internal.(*configapi.NodeConfig)

	// For new configurations, use protobuf.
	configapi.SetProtobufClientDefaults(config.MasterClientConnectionOverrides)

	content, err := latestconfigapi.WriteYAML(internal)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(nodeConfigFile, content, 0644); err != nil {
		return err
	}

	return nil
}

func (o CreateNodeConfigOptions) MakeNodeJSON(nodeJSONFile string) error {
	node := &kapi.Node{}
	node.Name = o.NodeName

	json, err := runtime.Encode(legacyscheme.Codecs.LegacyCodec(configv1.LegacySchemeGroupVersion), node)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(nodeJSONFile, json, 0644); err != nil {
		return err
	}

	return nil
}
