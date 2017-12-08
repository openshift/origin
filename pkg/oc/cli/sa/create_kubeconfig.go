package sa

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/serviceaccounts"
)

const (
	CreateKubeconfigRecommendedName = "create-kubeconfig"

	createKubeconfigShort = `Generate a kubeconfig file for a service account`

	createKubeconfigUsage = `%s SA-NAME`
)

var (
	createKubeconfigLong = templates.LongDesc(`
    Generate a kubeconfig file that will utilize this service account.

    The kubeconfig file will reference the service account token and use the current server,
    namespace, and cluster contact info. If the service account has multiple tokens, the
    first token found will be returned. The generated file will be output to STDOUT.

    Service account API tokens are used by service accounts to authenticate to the API.
    Client actions using a service account token will be executed as if the service account
    itself were making the actions.`)

	createKubeconfigExamples = templates.Examples(`
    # Create a kubeconfig file for service account 'default'
    %[1]s 'default' > default.kubeconfig`)
)

type CreateKubeconfigOptions struct {
	SAName           string
	SAClient         kcoreclient.ServiceAccountInterface
	SecretsClient    kcoreclient.SecretInterface
	RawConfig        clientcmdapi.Config
	ContextNamespace string

	Out io.Writer
	Err io.Writer
}

func NewCommandCreateKubeconfig(name, fullname string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &CreateKubeconfigOptions{
		Out: out,
		Err: os.Stderr,
	}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf(createKubeconfigUsage, name),
		Short:   createKubeconfigShort,
		Long:    createKubeconfigLong,
		Example: fmt.Sprintf(createKubeconfigExamples, fullname),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Complete(args, f, cmd))
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}
	cmd.Flags().StringVar(&options.ContextNamespace, "with-namespace", "", "Namespace for this context in .kubeconfig.")
	return cmd
}

func (o *CreateKubeconfigOptions) Complete(args []string, f *clientcmd.Factory, cmd *cobra.Command) error {
	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, fmt.Sprintf("expected one service account name as an argument, got %q", args))
	}

	o.SAName = args[0]

	client, err := f.ClientSet()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.RawConfig, err = f.OpenShiftClientConfig().RawConfig()
	if err != nil {
		return err
	}

	if len(o.ContextNamespace) == 0 {
		o.ContextNamespace = namespace
	}

	o.SAClient = client.Core().ServiceAccounts(namespace)
	o.SecretsClient = client.Core().Secrets(namespace)
	return nil
}

func (o *CreateKubeconfigOptions) Validate() error {
	if o.SAName == "" {
		return errors.New("service account name cannot be empty")
	}

	if o.SAClient == nil || o.SecretsClient == nil {
		return errors.New("API clients must not be nil in order to create a new service account token")
	}

	if o.Out == nil || o.Err == nil {
		return errors.New("cannot proceed if output or error writers are nil")
	}

	return nil
}

func (o *CreateKubeconfigOptions) Run() error {
	serviceAccount, err := o.SAClient.Get(o.SAName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for _, reference := range serviceAccount.Secrets {
		secret, err := o.SecretsClient.Get(reference.Name, metav1.GetOptions{})
		if err != nil {
			continue
		}

		if serviceaccounts.IsValidServiceAccountToken(serviceAccount, secret) {
			token, exists := secret.Data[kapi.ServiceAccountTokenKey]
			if !exists {
				return fmt.Errorf("service account token %q for service account %q did not contain token data", secret.Name, serviceAccount.Name)
			}

			cfg := &o.RawConfig
			if err := clientcmdapi.MinifyConfig(cfg); err != nil {
				return fmt.Errorf("invalid configuration, unable to create new config file: %v", err)
			}

			ctx := cfg.Contexts[cfg.CurrentContext]
			ctx.Namespace = o.ContextNamespace
			// rename the current context
			cfg.CurrentContext = o.SAName
			cfg.Contexts = map[string]*clientcmdapi.Context{
				cfg.CurrentContext: ctx,
			}
			// use the server name
			ctx.AuthInfo = o.SAName
			cfg.AuthInfos = map[string]*clientcmdapi.AuthInfo{
				ctx.AuthInfo: {
					Token: string(token),
				},
			}
			out, err := kclientcmd.Write(*cfg)
			if err != nil {
				return err
			}
			fmt.Fprintf(o.Out, string(out))
			return nil
		}
	}
	return fmt.Errorf("could not find a service account token for service account %q", serviceAccount.Name)
}
