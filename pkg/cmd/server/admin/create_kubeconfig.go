package admin

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/client/config"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

const CreateKubeConfigCommandName = "create-kubeconfig"

var createKubeConfigLongDesc = templates.LongDesc(`
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
      client-key-data: <contents of --client-key>`)

type CreateKubeConfigOptions struct {
	APIServerURL       string
	PublicAPIServerURL string
	APIServerCAFiles   []string

	CertFile string
	KeyFile  string

	ContextNamespace string

	KubeConfigFile string

	genericclioptions.IOStreams
}

func NewCreateKubeConfigOptions(streams genericclioptions.IOStreams) *CreateKubeConfigOptions {
	return &CreateKubeConfigOptions{
		APIServerURL:     "https://localhost:8443",
		APIServerCAFiles: []string{"openshift.local.config/master/ca.crt"},
		ContextNamespace: metav1.NamespaceDefault,
		KubeConfigFile:   ".kubeconfig",
		IOStreams:        streams,
	}
}

func NewCommandCreateKubeConfig(commandName string, fullName string, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewCreateKubeConfigOptions(streams)
	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Create a basic .kubeconfig file from client certs",
		Long:  createKubeConfigLongDesc,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Validate(args))
			if _, err := o.CreateKubeConfig(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	cmd.Flags().StringVar(&o.APIServerURL, "master", o.APIServerURL, "The API server's URL.")
	cmd.Flags().StringVar(&o.PublicAPIServerURL, "public-master", o.PublicAPIServerURL, "The API public facing server's URL (if applicable).")
	cmd.Flags().StringSliceVar(&o.APIServerCAFiles, "certificate-authority", o.APIServerCAFiles, "Files containing signing authorities to use to verify the API server's serving certificate.")
	cmd.Flags().StringVar(&o.CertFile, "client-certificate", o.CertFile, "The client cert file.")
	cmd.Flags().StringVar(&o.KeyFile, "client-key", o.KeyFile, "The client key file.")
	cmd.Flags().StringVar(&o.ContextNamespace, "namespace", o.ContextNamespace, "Namespace for this context in .kubeconfig.")
	cmd.Flags().StringVar(&o.KubeConfigFile, "kubeconfig", o.KubeConfigFile, "Path for the resulting .kubeconfig file.")

	// autocompletion hints
	cmd.MarkFlagFilename("certificate-authority")
	cmd.MarkFlagFilename("client-certificate")
	cmd.MarkFlagFilename("client-key")
	cmd.MarkFlagFilename("kubeconfig")

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
	if len(o.APIServerCAFiles) == 0 {
		return errors.New("certificate-authority must be provided")
	} else {
		for _, caFile := range o.APIServerCAFiles {
			if _, err := cert.NewPool(caFile); err != nil {
				return fmt.Errorf("certificate-authority must be a valid certificate file: %v", err)
			}
		}
	}
	if len(o.ContextNamespace) == 0 {
		return errors.New("namespace must be provided")
	}
	if len(o.APIServerURL) == 0 {
		return errors.New("master must be provided")
	}

	return nil
}

func (o CreateKubeConfigOptions) CreateKubeConfig() (*clientcmdapi.Config, error) {
	glog.V(4).Infof("creating a .kubeconfig with: %#v", o)

	// read all the referenced filenames
	caData, err := readFiles(o.APIServerCAFiles, []byte("\n"))
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
	certConfig, err := crypto.GetTLSCertificateConfig(o.CertFile, o.KeyFile)
	if err != nil {
		return nil, err
	}

	// determine all the nicknames
	clusterNick, err := config.GetClusterNicknameFromURL(o.APIServerURL)
	if err != nil {
		return nil, err
	}
	userNick, err := config.GetUserNicknameFromCert(clusterNick, certConfig.Certs...)
	if err != nil {
		return nil, err
	}
	contextNick := config.GetContextNickname(o.ContextNamespace, clusterNick, userNick)

	credentials := make(map[string]*clientcmdapi.AuthInfo)
	credentials[userNick] = &clientcmdapi.AuthInfo{
		ClientCertificateData: certData,
		ClientKeyData:         keyData,
	}

	// normalize the provided server to a format expected by config
	o.APIServerURL, err = config.NormalizeServerURL(o.APIServerURL)
	if err != nil {
		return nil, err
	}

	clusters := make(map[string]*clientcmdapi.Cluster)
	clusters[clusterNick] = &clientcmdapi.Cluster{
		Server: o.APIServerURL,
		CertificateAuthorityData: caData,
	}

	contexts := make(map[string]*clientcmdapi.Context)
	contexts[contextNick] = &clientcmdapi.Context{Cluster: clusterNick, AuthInfo: userNick, Namespace: o.ContextNamespace}

	createPublic := (len(o.PublicAPIServerURL) > 0) && o.APIServerURL != o.PublicAPIServerURL
	if createPublic {
		publicClusterNick, err := config.GetClusterNicknameFromURL(o.PublicAPIServerURL)
		if err != nil {
			return nil, err
		}
		publicContextNick := config.GetContextNickname(o.ContextNamespace, publicClusterNick, userNick)

		clusters[publicClusterNick] = &clientcmdapi.Cluster{
			Server: o.PublicAPIServerURL,
			CertificateAuthorityData: caData,
		}
		contexts[publicContextNick] = &clientcmdapi.Context{Cluster: publicClusterNick, AuthInfo: userNick, Namespace: o.ContextNamespace}
	}

	kubeConfig := &clientcmdapi.Config{
		Clusters:       clusters,
		AuthInfos:      credentials,
		Contexts:       contexts,
		CurrentContext: contextNick,
	}

	glog.V(3).Infof("Generating '%s' API client config as %s\n", userNick, o.KubeConfigFile)
	// Ensure the parent dir exists
	if err := os.MkdirAll(filepath.Dir(o.KubeConfigFile), os.FileMode(0755)); err != nil {
		return nil, err
	}
	if err := clientcmd.WriteToFile(*kubeConfig, o.KubeConfigFile); err != nil {
		return nil, err
	}

	return kubeConfig, nil
}
