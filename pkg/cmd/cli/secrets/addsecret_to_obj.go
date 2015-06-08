package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/spf13/cobra"
)

const (
	AddSecretRecommendedName = "add"

	addSecretLong = `Add secrets to a ServiceAccount

After you have created a secret, you probably want to make use of that secret inside of a pod, for a build, or as an image pull secret.  In order to do that, you must add your secret to a service account.

To use your secret inside of a pod or as a push, pull, or source secret for a build, you must add a 'mount' secret to your service account like this:

  $ %s serviceaccount/sa-name secrets/secret-name secrets/another-secret-name

To use your secret as an image pull secret, you must add a 'pull' secret to your service account like this:

  $ %s serviceaccount/sa-name secrets/secret-name --for=pull
`
)

type SecretType string

var (
	PullType  SecretType = "pull"
	MountType SecretType = "mount"
)

type AddSecretOptions struct {
	TargetName  string
	SecretNames []string

	Type SecretType

	Namespace string

	Mapper          meta.RESTMapper
	Typer           runtime.ObjectTyper
	ClientMapper    resource.ClientMapper
	ClientInterface client.Interface

	Out io.Writer
}

// NewCmdAddSecret creates a command object for adding a secret reference to a service account
func NewCmdAddSecret(name, fullName string, f *cmdutil.Factory, out io.Writer) *cobra.Command {
	o := &AddSecretOptions{Out: out}
	typeFlag := "mount"

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s serviceaccounts/sa-name secrets/secret-name [secrets/another-secret-name]...", name),
		Short: "Add secrets to a ServiceAccount",
		Long:  fmt.Sprintf(addSecretLong, fullName, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args, typeFlag); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := o.Validate(); err != nil {
				cmdutil.CheckErr(err)
			}

			if err := o.AddSecrets(); err != nil {
				cmdutil.CheckErr(err)
			}

		},
	}

	cmd.Flags().StringVar(&typeFlag, "for", typeFlag, "type of secret to add: mount or pull")

	return cmd
}

func (o *AddSecretOptions) Complete(f *cmdutil.Factory, args []string, typeFlag string) error {
	if len(args) < 2 {
		return errors.New("must have service account name and at least one secret name")
	}
	o.TargetName = args[0]
	o.SecretNames = args[1:]

	loweredTypeFlag := strings.ToLower(typeFlag)
	switch loweredTypeFlag {
	case string(PullType):
		o.Type = PullType
	case string(MountType):
		o.Type = MountType
	default:
		return fmt.Errorf("unknown for: %v", typeFlag)
	}

	var err error
	o.ClientInterface, err = f.Client()
	if err != nil {
		return err
	}

	o.Namespace, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Mapper, o.Typer = f.Object()
	o.ClientMapper = f.ClientMapperForCommand()

	return nil
}

func (o AddSecretOptions) Validate() error {
	if len(o.TargetName) == 0 {
		return errors.New("service account name must be present")
	}
	if len(o.SecretNames) == 0 {
		return errors.New("secret name must be present")
	}
	if len(o.Type) == 0 {
		return errors.New("for must be present")
	}
	if o.Mapper == nil {
		return errors.New("Mapper must be present")
	}
	if o.Typer == nil {
		return errors.New("Typer must be present")
	}
	if o.ClientMapper == nil {
		return errors.New("ClientMapper must be present")
	}
	if o.ClientInterface == nil {
		return errors.New("ClientInterface must be present")
	}

	return nil
}

func (o AddSecretOptions) AddSecrets() error {
	r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper).
		NamespaceParam(o.Namespace).
		ResourceTypeOrNameArgs(false, o.TargetName).
		SingleResourceType().
		Do()
	if r.Err() != nil {
		return r.Err()
	}
	obj, err := r.Object()
	if err != nil {
		return err
	}

	switch t := obj.(type) {
	case *api.ServiceAccount:
		switch o.Type {
		case PullType:
			_, err := o.AddSecretsToSAPullSecrets(t)
			return err
		case MountType:
			_, err := o.AddSecretsToSAMountableSecrets(t)
			return err
		default:
			return fmt.Errorf("%v is not handled for ServiceAccounts", o.Type)
		}
	default:
		return fmt.Errorf("unhandled object: %#v", t)
	}

	return nil
}

func (o AddSecretOptions) getSecrets() ([]*api.Secret, error) {
	r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper).
		NamespaceParam(o.Namespace).
		ResourceTypeOrNameArgs(false, o.SecretNames...).
		SingleResourceType().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}
	infos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	secrets := []*api.Secret{}
	for i := range infos {
		info := infos[i]

		switch t := info.Object.(type) {
		case *api.Secret:
			secrets = append(secrets, t)
		default:
			return nil, fmt.Errorf("unhandled object: %#v", t)
		}
	}

	return secrets, nil
}

func (o AddSecretOptions) AddSecretsToSAMountableSecrets(serviceAccount *api.ServiceAccount) (*api.ServiceAccount, error) {
	secrets, err := o.getSecrets()
	if err != nil {
		return nil, err
	}
	if len(secrets) == 0 {
		return nil, errors.New("no secrets found")
	}

	currentSecrets := util.StringSet{}
	for _, secretRef := range serviceAccount.Secrets {
		currentSecrets.Insert(secretRef.Name)
	}

	for _, secret := range secrets {
		if currentSecrets.Has(secret.Name) {
			continue
		}

		serviceAccount.Secrets = append(serviceAccount.Secrets, api.ObjectReference{Name: secret.Name})
		currentSecrets.Insert(secret.Name)
	}

	return o.ClientInterface.ServiceAccounts(o.Namespace).Update(serviceAccount)
}

func (o AddSecretOptions) AddSecretsToSAPullSecrets(serviceAccount *api.ServiceAccount) (*api.ServiceAccount, error) {
	secrets, err := o.getSecrets()
	if err != nil {
		return nil, err
	}

	currentSecrets := util.StringSet{}
	for _, secretRef := range serviceAccount.ImagePullSecrets {
		currentSecrets.Insert(secretRef.Name)
	}

	for _, secret := range secrets {
		if currentSecrets.Has(secret.Name) {
			continue
		}

		serviceAccount.ImagePullSecrets = append(serviceAccount.ImagePullSecrets, api.LocalObjectReference{Name: secret.Name})
		currentSecrets.Insert(secret.Name)
	}

	return o.ClientInterface.ServiceAccounts(o.Namespace).Update(serviceAccount)
}

func (o AddSecretOptions) GetOut() io.Writer {
	if o.Out == nil {
		return ioutil.Discard
	}

	return o.Out
}
