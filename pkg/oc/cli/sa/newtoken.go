package sa

import (
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"time"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	api "k8s.io/kubernetes/pkg/apis/core"
	kcoreclient "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/core/internalversion"
	"k8s.io/kubernetes/pkg/kubectl"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/term"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	"github.com/openshift/origin/pkg/serviceaccounts"
	osautil "github.com/openshift/origin/pkg/serviceaccounts/util"
)

const (
	NewServiceAccountTokenRecommendedName = "new-token"

	newServiceAccountTokenShort = `Generate a new token for a service account.`

	newServiceAccountTokenUsage = `%s SA-NAME`
)

var (
	newServiceAccountTokenLong = templates.LongDesc(`
    Generate a new token for a service account.

    Service account API tokens are used by service accounts to authenticate to the API.
    This command will generate a new token, which could be used to compartmentalize service
    account actions by executing them with distinct tokens, to rotate out pre-existing token
    on the service account, or for use by an external client. If a label is provided, it will
    be applied to any created token so that tokens created with this command can be idenitifed.`)

	newServiceAccountTokenExamples = templates.Examples(`
    # Generate a new token for service account 'default'
    %[1]s 'default'

    # Generate a new token for service account 'default' and apply
    # labels 'foo' and 'bar' to the new token for identification
    # %[1]s 'default' --labels foo=foo-value,bar=bar-value`)
)

type NewServiceAccountTokenOptions struct {
	SAName        string
	SAClient      kcoreclient.ServiceAccountInterface
	SecretsClient kcoreclient.SecretInterface

	Labels map[string]string

	Timeout time.Duration

	Out io.Writer
	Err io.Writer
}

func NewCommandNewServiceAccountToken(name, fullname string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &NewServiceAccountTokenOptions{
		Out:    out,
		Err:    os.Stderr,
		Labels: map[string]string{},
	}

	var requestedLabels string
	newServiceAccountTokenCommand := &cobra.Command{
		Use:     fmt.Sprintf(newServiceAccountTokenUsage, name),
		Short:   newServiceAccountTokenShort,
		Long:    newServiceAccountTokenLong,
		Example: fmt.Sprintf(newServiceAccountTokenExamples, fullname),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(options.Complete(args, requestedLabels, f, cmd))
			cmdutil.CheckErr(options.Validate())
			cmdutil.CheckErr(options.Run())
		},
	}

	newServiceAccountTokenCommand.Flags().DurationVar(&options.Timeout, "timeout", 30*time.Second, "the maximum time allowed to generate a token")
	newServiceAccountTokenCommand.Flags().StringVarP(&requestedLabels, "labels", "l", "", "labels to set in all resources for this application, given as a comma-delimited list of key-value pairs")
	return newServiceAccountTokenCommand
}

func (o *NewServiceAccountTokenOptions) Complete(args []string, requestedLabels string, f *clientcmd.Factory, cmd *cobra.Command) error {
	if len(args) != 1 {
		return cmdutil.UsageErrorf(cmd, fmt.Sprintf("expected one service account name as an argument, got %q", args))
	}

	o.SAName = args[0]

	if len(requestedLabels) > 0 {
		labels, err := kubectl.ParseLabels(requestedLabels)
		if err != nil {
			return cmdutil.UsageErrorf(cmd, err.Error())
		}
		o.Labels = labels
	}

	client, err := f.ClientSet()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return fmt.Errorf("could not retrieve default namespace: %v", err)
	}

	o.SAClient = client.Core().ServiceAccounts(namespace)
	o.SecretsClient = client.Core().Secrets(namespace)
	return nil
}

func (o *NewServiceAccountTokenOptions) Validate() error {
	if o.SAName == "" {
		return errors.New("service account name cannot be empty")
	}

	if o.SAClient == nil || o.SecretsClient == nil {
		return errors.New("API clients must not be nil in order to create a new service account token")
	}

	if o.Timeout <= 0 {
		return errors.New("a positive amount of time must be allotted for the timeout")
	}

	if o.Out == nil || o.Err == nil {
		return errors.New("cannot proceed if output or error writers are nil")
	}

	return nil
}

// Run creates a new token secret, waits for the service account token controller to fulfill it, then adds the token to the service account
func (o *NewServiceAccountTokenOptions) Run() error {
	serviceAccount, err := o.SAClient.Get(o.SAName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	tokenSecret := &api.Secret{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: osautil.GetTokenSecretNamePrefix(serviceAccount),
			Namespace:    serviceAccount.Namespace,
			Labels:       o.Labels,
			Annotations: map[string]string{
				api.ServiceAccountNameKey: serviceAccount.Name,
			},
		},
		Type: api.SecretTypeServiceAccountToken,
		Data: map[string][]byte{},
	}

	persistedToken, err := o.SecretsClient.Create(tokenSecret)
	if err != nil {
		return err
	}

	// we need to wait for the service account token controller to make the new token valid
	tokenSecret, err = waitForToken(persistedToken, serviceAccount, o.Timeout, o.SecretsClient)
	if err != nil {
		return err
	}

	token, exists := tokenSecret.Data[api.ServiceAccountTokenKey]
	if !exists {
		return fmt.Errorf("service account token %q did not contain token data", tokenSecret.Name)
	}

	fmt.Fprintf(o.Out, string(token))
	if term.IsTerminalWriter(o.Out) {
		// pretty-print for a TTY
		fmt.Fprintf(o.Out, "\n")
	}
	return nil
}

// waitForToken uses `cmd.Until` to wait for the service account controller to fulfill the token request
func waitForToken(token *api.Secret, serviceAccount *api.ServiceAccount, timeout time.Duration, client kcoreclient.SecretInterface) (*api.Secret, error) {
	// there is no provided rounding function, so we use Round(x) === Floor(x + 0.5)
	timeoutSeconds := int64(math.Floor(timeout.Seconds() + 0.5))

	options := metav1.ListOptions{
		FieldSelector:   fields.SelectorFromSet(fields.Set(map[string]string{"metadata.name": token.Name})).String(),
		Watch:           true,
		ResourceVersion: token.ResourceVersion,
		TimeoutSeconds:  &timeoutSeconds,
	}

	watcher, err := client.Watch(options)
	if err != nil {
		return nil, fmt.Errorf("could not begin watch for token: %v", err)
	}

	event, err := watch.Until(timeout, watcher, func(event watch.Event) (bool, error) {
		if event.Type == watch.Error {
			return false, fmt.Errorf("encountered error while watching for token: %v", event.Object)
		}

		eventToken, ok := event.Object.(*api.Secret)
		if !ok {
			return false, nil
		}

		if eventToken.Name != token.Name {
			return false, nil
		}

		switch event.Type {
		case watch.Modified:
			if serviceaccounts.IsValidServiceAccountToken(serviceAccount, eventToken) {
				return true, nil
			}
		case watch.Deleted:
			return false, errors.New("token was deleted before fulfillment by service account token controller")
		case watch.Added:
			return false, errors.New("unxepected action: token was added after initial creation")
		}
		return false, nil
	})
	if err != nil {
		return nil, err
	}

	return event.Object.(*api.Secret), nil
}
