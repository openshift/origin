package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

const (
	AddSecretRecommendedName = "add"

	// TODO: move to examples
	addSecretLong = `
Add secrets to a ServiceAccount

After you have created a secret, you probably want to make use of that secret inside of a pod, for a build, or as an image pull secret.  In order to do that, you must add your secret to a service account.`

	addSecretExample = `  // To use your secret inside of a pod or as a push, pull, or source secret for a build, you must add a 'mount' secret to your service account like this:
  $ %[1]s serviceaccount/sa-name secrets/secret-name secrets/another-secret-name

  // To use your secret as an image pull secret, you must add a 'pull' secret to your service account like this:
  $ %[1]s serviceaccount/sa-name secrets/secret-name --for=pull

  // To use your secret for image pulls or inside a pod:
  $ %[1]s serviceaccount/sa-name secrets/secret-name --for=pull,mount`
)

type AddSecretOptions struct {
	TargetName  string
	SecretNames []string

	ForMount bool
	ForPull  bool

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
	var typeFlags util.StringList

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s serviceaccounts/sa-name secrets/secret-name [secrets/another-secret-name]...", name),
		Short:   "Add secrets to a ServiceAccount",
		Long:    addSecretLong,
		Example: fmt.Sprintf(addSecretExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args, typeFlags); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(c, err.Error()))
			}

			if err := o.Validate(); err != nil {
				cmdutil.CheckErr(cmdutil.UsageError(c, err.Error()))
			}

			if err := o.AddSecrets(); err != nil {
				cmdutil.CheckErr(err)
			}

		},
	}

	forFlag := &pflag.Flag{
		Name:     "for",
		Usage:    "type of secret to add: mount or pull",
		Value:    &typeFlags,
		DefValue: "mount",
	}
	cmd.Flags().AddFlag(forFlag)

	return cmd
}

func (o *AddSecretOptions) Complete(f *cmdutil.Factory, args []string, typeFlags []string) error {
	if len(args) < 2 {
		return errors.New("must have service account name and at least one secret name")
	}
	o.TargetName = args[0]
	o.SecretNames = args[1:]

	if len(typeFlags) == 0 {
		o.ForMount = true
	} else {
		for _, flag := range typeFlags {
			loweredValue := strings.ToLower(flag)
			switch loweredValue {
			case "pull":
				o.ForPull = true
			case "mount":
				o.ForMount = true
			default:
				return fmt.Errorf("unknown for: %v", flag)
			}
		}
	}

	var err error
	o.ClientInterface, err = f.Client()
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.DefaultNamespace()
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
	if !o.ForPull && !o.ForMount {
		return errors.New("for must be present")
	}
	if o.Mapper == nil {
		return errors.New("mapper must be present")
	}
	if o.Typer == nil {
		return errors.New("typer must be present")
	}
	if o.ClientMapper == nil {
		return errors.New("clientMapper must be present")
	}
	if o.ClientInterface == nil {
		return errors.New("clientInterface must be present")
	}

	return nil
}

func (o AddSecretOptions) AddSecrets() error {
	r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper).
		NamespaceParam(o.Namespace).
		ResourceNames("serviceaccounts", o.TargetName).
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
	case *kapi.ServiceAccount:
		err = o.addSecretsToServiceAccount(t)
		if err != nil {
			return err
		}
	default:
		return fmt.Errorf("unhandled object: %#v", t)
	}

	return nil
}

// TODO: when Secrets in kapi.ServiceAccount get changed to MountSecrets and represented by LocalObjectReferences, this can be
// refactored to reuse the addition code better
// addSecretsToServiceAccount adds secrets to the service account, either as pull secrets, mount secrets, or both.
func (o AddSecretOptions) addSecretsToServiceAccount(serviceaccount *kapi.ServiceAccount) error {
	updated := false
	newSecrets, err := o.getSecrets()
	if err != nil {
		return err
	}
	newSecretNames := getSecretNames(newSecrets)

	if o.ForMount {
		currentSecrets := getMountSecretNames(serviceaccount)
		secretsToAdd := newSecretNames.Difference(currentSecrets)
		for _, secretName := range secretsToAdd.List() {
			serviceaccount.Secrets = append(serviceaccount.Secrets, kapi.ObjectReference{Name: secretName})
			updated = true
		}
	}
	if o.ForPull {
		currentSecrets := getPullSecretNames(serviceaccount)
		secretsToAdd := newSecretNames.Difference(currentSecrets)
		for _, secretName := range secretsToAdd.List() {
			serviceaccount.ImagePullSecrets = append(serviceaccount.ImagePullSecrets, kapi.LocalObjectReference{Name: secretName})
			updated = true
		}
	}
	if updated {
		_, err = o.ClientInterface.ServiceAccounts(o.Namespace).Update(serviceaccount)
		return err
	}
	return nil
}

func (o AddSecretOptions) getSecrets() ([]*kapi.Secret, error) {
	r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper).
		NamespaceParam(o.Namespace).
		ResourceNames("secrets", o.SecretNames...).
		SingleResourceType().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}
	infos, err := r.Infos()
	if err != nil {
		return nil, err
	}

	secrets := []*kapi.Secret{}
	for i := range infos {
		info := infos[i]

		switch t := info.Object.(type) {
		case *kapi.Secret:
			secrets = append(secrets, t)
		default:
			return nil, fmt.Errorf("unhandled object: %#v", t)
		}
	}

	return secrets, nil
}

func getSecretNames(secrets []*kapi.Secret) sets.String {
	names := sets.String{}
	for _, secret := range secrets {
		names.Insert(secret.Name)
	}
	return names
}

func getMountSecretNames(serviceaccount *kapi.ServiceAccount) sets.String {
	names := sets.String{}
	for _, secret := range serviceaccount.Secrets {
		names.Insert(secret.Name)
	}
	return names
}

func getPullSecretNames(serviceaccount *kapi.ServiceAccount) sets.String {
	names := sets.String{}
	for _, secret := range serviceaccount.ImagePullSecrets {
		names.Insert(secret.Name)
	}
	return names
}

func (o AddSecretOptions) GetOut() io.Writer {
	if o.Out == nil {
		return ioutil.Discard
	}

	return o.Out
}
