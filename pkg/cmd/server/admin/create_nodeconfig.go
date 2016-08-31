package admin

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/server/bootstrappolicy"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apimachinery/registered"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/master/ports"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/crypto"

	"github.com/openshift/origin/pkg/cmd/flagtypes"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	latestconfigapi "github.com/openshift/origin/pkg/cmd/server/api/latest"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/variable"
)

const NodeConfigCommandName = "create-node-config"

type CreateNodeConfigOptions struct {
	SignerCertOptions *SignerCertOptions

	NodeConfigDir string

	NodeName            string
	Hostnames           []string
	VolumeDir           string
	ImageTemplate       variable.ImageTemplate
	AllowDisabledDocker bool
	DNSDomain           string
	DNSIP               string
	ListenAddr          flagtypes.Addr

	ClientCertFile    string
	ClientKeyFile     string
	ServerCertFile    string
	ServerKeyFile     string
	NodeClientCAFile  string
	APIServerCAFiles  []string
	APIServerURL      string
	Output            io.Writer
	NetworkPluginName string
}

func NewCommandNodeConfig(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := NewDefaultCreateNodeConfigOptions()
	options.Output = out

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a configuration bundle for a node",
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.CreateNodeFolder(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	BindSignerCertOptions(options.SignerCertOptions, flags, "")

	flags.StringVar(&options.NodeConfigDir, "node-dir", "", "The client data directory.")

	flags.StringVar(&options.NodeName, "node", "", "The name of the node as it appears in etcd.")
	flags.StringSliceVar(&options.Hostnames, "hostnames", options.Hostnames, "Every hostname or IP you want server certs to be valid for. Comma delimited list")
	flags.StringVar(&options.VolumeDir, "volume-dir", options.VolumeDir, "The volume storage directory.  This path is not relativized.")
	flags.StringVar(&options.ImageTemplate.Format, "images", options.ImageTemplate.Format, "When fetching the network container image, use this format. The latest release will be used by default.")
	flags.BoolVar(&options.ImageTemplate.Latest, "latest-images", options.ImageTemplate.Latest, "If true, attempt to use the latest images for the cluster instead of the latest release.")
	flags.BoolVar(&options.AllowDisabledDocker, "allow-disabled-docker", options.AllowDisabledDocker, "Allow the node to start without docker being available.")
	flags.StringVar(&options.DNSDomain, "dns-domain", options.DNSDomain, "DNS domain for the cluster.")
	flags.StringVar(&options.DNSIP, "dns-ip", options.DNSIP, "DNS server IP for the cluster.")
	flags.Var(&options.ListenAddr, "listen", "The address to listen for connections on (scheme://host:port).")

	flags.StringVar(&options.ClientCertFile, "client-certificate", "", "The client cert file for the node to contact the API.")
	flags.StringVar(&options.ClientKeyFile, "client-key", "", "The client key file for the node to contact the API.")
	flags.StringVar(&options.ServerCertFile, "server-certificate", "", "The server cert file for the node to serve secure traffic.")
	flags.StringVar(&options.ServerKeyFile, "server-key", "", "The server key file for the node to serve secure traffic.")
	flags.StringVar(&options.NodeClientCAFile, "node-client-certificate-authority", options.NodeClientCAFile, "The file containing signing authorities to use to verify requests to the node. If empty, all requests will be allowed.")
	flags.StringVar(&options.APIServerURL, "master", options.APIServerURL, "The API server's URL.")
	flags.StringSliceVar(&options.APIServerCAFiles, "certificate-authority", options.APIServerCAFiles, "Files containing signing authorities to use to verify the API server's serving certificate.")
	flags.StringVar(&options.NetworkPluginName, "network-plugin", options.NetworkPluginName, "Name of the network plugin to hook to for pod networking.")

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
	options := &CreateNodeConfigOptions{SignerCertOptions: NewDefaultSignerCertOptions()}
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
			if _, err := crypto.CertPoolFromFile(caFile); err != nil {
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

func (o CreateNodeConfigOptions) CreateNodeFolder() error {
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

	fmt.Fprintf(o.Output, "Generating node credentials ...\n")

	if err := o.MakeClientCert(clientCertFile, clientKeyFile); err != nil {
		return err
	}
	if o.UseTLS() {
		if err := o.MakeAndWriteServerCert(serverCertFile, serverKeyFile); err != nil {
			return err
		}
		if o.UseNodeClientCA() {
			if err := o.MakeNodeClientCA(nodeClientCAFile); err != nil {
				return err
			}
		}
	}
	if err := o.MakeAPIServerCA(apiServerCAFile); err != nil {
		return err
	}
	if err := o.MakeKubeConfig(clientCertFile, clientKeyFile, apiServerCAFile, kubeConfigFile); err != nil {
		return err
	}
	if err := o.MakeNodeConfig(serverCertFile, serverKeyFile, nodeClientCAFile, kubeConfigFile, nodeConfigFile); err != nil {
		return err
	}
	if err := o.MakeNodeJSON(nodeJSONFile); err != nil {
		return err
	}

	fmt.Fprintf(o.Output, "Created node config for %s in %s\n", o.NodeName, o.NodeConfigDir)

	return nil
}

func (o CreateNodeConfigOptions) MakeClientCert(clientCertFile, clientKeyFile string) error {
	if o.IsCreateClientCertificate() {
		createNodeClientCert := CreateClientCertOptions{
			SignerCertOptions: o.SignerCertOptions,

			CertFile: clientCertFile,
			KeyFile:  clientKeyFile,

			User:   "system:node:" + o.NodeName,
			Groups: []string{bootstrappolicy.NodesGroup},
			Output: o.Output,
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

			Hostnames: o.Hostnames,
			Output:    o.Output,
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

		ContextNamespace: kapi.NamespaceDefault,

		KubeConfigFile: kubeConfigFile,
		Output:         o.Output,
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
			BindAddress: net.JoinHostPort(o.ListenAddr.Host, strconv.Itoa(ports.KubeletPort)),
		},

		VolumeDirectory:     o.VolumeDir,
		AllowDisabledDocker: o.AllowDisabledDocker,

		ImageConfig: configapi.ImageConfig{
			Format: o.ImageTemplate.Format,
			Latest: o.ImageTemplate.Latest,
		},

		DNSDomain: o.DNSDomain,
		DNSIP:     o.DNSIP,

		MasterKubeConfig: kubeConfigFile,

		NetworkConfig: configapi.NodeNetworkConfig{
			NetworkPluginName: o.NetworkPluginName,
		},

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
	ext, err := configapi.Scheme.ConvertToVersion(config, latestconfigapi.Version)
	if err != nil {
		return err
	}
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

	groupMeta := registered.GroupOrDie(kapi.GroupName)

	json, err := runtime.Encode(kapi.Codecs.LegacyCodec(groupMeta.GroupVersions[0]), node)
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(nodeJSONFile, json, 0644); err != nil {
		return err
	}

	return nil
}
