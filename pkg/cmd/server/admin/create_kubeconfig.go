package admin

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
)

const CreateKubeConfigCommandName = "create-kubeconfig"

type CreateKubeConfigOptions struct {
	APIServerURL       string
	PublicAPIServerURL string
	APIServerCAFile    string
	ServerNick         string

	CertFile string
	KeyFile  string
	UserNick string

	ContextNick      string
	ContextNamespace string

	KubeConfigFile string
}

func NewCommandCreateKubeConfig(commandName string, fullName string, out io.Writer) *cobra.Command {
	options := &CreateKubeConfigOptions{}

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a basic .kubeconfig file from client certs",
		Long: `
Create's a .kubeconfig file at <--kubeconfig> that looks like this:

clusters:
- cluster:
    certificate-authority-data: <contents of --certificate-authority>
    server: <--master>
  name: <--cluster>
- cluster:
    certificate-authority-data: <contents of --certificate-authority>
    server: <--public-master>
  name: public-<--cluster>
contexts:
- context:
    cluster: <--cluster>
    user: <--user>
    namespace: <--namespace>
  name: <--context>
- context:
    cluster: public-<--cluster>
    user: <--user>
    namespace: <--namespace>
  name: public-<--context>
current-context: <--context>
kind: Config
users:
- name: <--user>
  user:
    client-certificate-data: <contents of --client-certificate>
    client-key-data: <contents of --client-key>
`,
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), err.Error())
				c.Help()
				return
			}

			if _, err := options.CreateKubeConfig(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()

	flags.StringVar(&options.APIServerURL, "master", "https://localhost:8443", "The API server's URL.")
	flags.StringVar(&options.PublicAPIServerURL, "public-master", "", "The API public facing server's URL (if applicable).")
	flags.StringVar(&options.APIServerCAFile, "certificate-authority", "openshift.local.certificates/ca/cert.crt", "Path to the API server's CA file.")
	flags.StringVar(&options.ServerNick, "cluster", "master", "Nick name for this server in .kubeconfig.")
	flags.StringVar(&options.CertFile, "client-certificate", "", "The client cert file.")
	flags.StringVar(&options.KeyFile, "client-key", "", "The client key file.")
	flags.StringVar(&options.UserNick, "user", "user", "Nick name for this user in .kubeconfig.")
	flags.StringVar(&options.ContextNick, "context", "", "Nick name for this context in .kubeconfig.")
	flags.StringVar(&options.ContextNamespace, "namespace", kapi.NamespaceDefault, "Namespace for this context in .kubeconfig.")
	flags.StringVar(&options.KubeConfigFile, "kubeconfig", ".kubeconfig", "Path for the resulting .kubeconfig file.")

	return cmd
}

func (o CreateKubeConfigOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.KubeConfigFile) == 0 {
		return errors.New("kubeconfig must be provided")
	}
	if len(o.CertFile) == 0 {
		return errors.New("client-certificate must be provided")
	}
	if len(o.KeyFile) == 0 {
		return errors.New("client-key must be provided")
	}
	if len(o.APIServerCAFile) == 0 {
		return errors.New("certificate-authority must be provided")
	}
	if len(o.ServerNick) == 0 {
		return errors.New("cluster must be provided")
	}
	if len(o.UserNick) == 0 {
		return errors.New("user-nick must be provided")
	}
	if len(o.ContextNamespace) == 0 {
		return errors.New("namespace must be provided")
	}

	return nil
}

func (o CreateKubeConfigOptions) CreateKubeConfig() (*clientcmdapi.Config, error) {
	glog.V(2).Infof("creating a .kubeconfig with: %#v", o)

	// if you don't specify a context nick, assign it to the context namespace
	if len(o.ContextNick) == 0 {
		o.ContextNick = o.ContextNamespace
	}

	caData, err := ioutil.ReadFile(o.APIServerCAFile)
	if err != nil {
		return nil, err
	}
	certData, err := ioutil.ReadFile(o.CertFile)
	if err != nil {
		return nil, err
	}
	keyData, err := ioutil.ReadFile(o.KeyFile)
	if err != nil {
		return nil, err
	}

	credentials := make(map[string]clientcmdapi.AuthInfo)
	credentials[o.UserNick] = clientcmdapi.AuthInfo{
		ClientCertificateData: certData,
		ClientKeyData:         keyData,
	}

	clusters := make(map[string]clientcmdapi.Cluster)
	clusters[o.ServerNick] = clientcmdapi.Cluster{
		Server: o.APIServerURL,
		CertificateAuthorityData: caData,
	}

	contexts := make(map[string]clientcmdapi.Context)
	contexts[o.ContextNick] = clientcmdapi.Context{Cluster: o.ServerNick, AuthInfo: o.UserNick, Namespace: o.ContextNamespace}

	createPublic := len(o.PublicAPIServerURL) > 0
	if createPublic {
		publicClusterNick := "public-" + o.ServerNick
		publicContextNick := "public-" + o.ContextNick
		clusters[publicClusterNick] = clientcmdapi.Cluster{
			Server: o.PublicAPIServerURL,
			CertificateAuthorityData: caData,
		}
		contexts[publicContextNick] = clientcmdapi.Context{Cluster: publicClusterNick, AuthInfo: o.UserNick, Namespace: o.ContextNamespace}
	}

	kubeConfig := &clientcmdapi.Config{
		Clusters:       clusters,
		AuthInfos:      credentials,
		Contexts:       contexts,
		CurrentContext: o.ContextNick,
	}

	// Ensure the parent dir exists
	if err := os.MkdirAll(filepath.Dir(o.KubeConfigFile), os.FileMode(0755)); err != nil {
		return nil, err
	}
	if err := clientcmd.WriteToFile(*kubeConfig, o.KubeConfigFile); err != nil {
		return nil, err
	}

	return kubeConfig, nil
}
