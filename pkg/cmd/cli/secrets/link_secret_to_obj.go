package secrets

import (
	"errors"
	"fmt"
	"io"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

const (
	// LinkSecretRecommendedName `oc secrets link`
	LinkSecretRecommendedName = "link"

	// TODO: move to examples
	linkSecretLong = `
Link secrets to a service account

Linking a secret enables a service account to automatically use that secret for some forms of authentication`

	linkSecretExample = `  # Add an image pull secret to a service account to automatically use it for pulling pod images:
  %[1]s serviceaccount-name pull-secret --for=pull

  # Add an image pull secret to a service account to automatically use it for both pulling and pushing build images:
  %[1]s builder builder-image-secret --for=pull,mount

  # If the cluster's serviceAccountConfig is operating with limitSecretReferences: True, secrets must be added to the pod's service account whitelist in order to be available to the pod:
  %[1]s pod-sa pod-secret`
)

type LinkSecretOptions struct {
	SecretOptions

	ForMount bool
	ForPull  bool

	typeFlags []string
}

// NewCmdLinkSecret creates a command object for linking a secret reference to a service account
func NewCmdLinkSecret(name, fullName string, f *kcmdutil.Factory, out io.Writer) *cobra.Command {
	o := &LinkSecretOptions{SecretOptions{Out: out}, false, false, nil}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s serviceaccounts-name secret-name [another-secret-name]...", name),
		Short:   "Link secrets to a ServiceAccount",
		Long:    linkSecretLong,
		Example: fmt.Sprintf(linkSecretExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			if err := o.LinkSecrets(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	cmd.Flags().StringSliceVar(&o.typeFlags, "for", []string{"mount"}, "type of secret to link: mount or pull")

	return cmd
}

func (o *LinkSecretOptions) Complete(f *kcmdutil.Factory, args []string) error {
	if err := o.SecretOptions.Complete(f, args); err != nil {
		return err
	}

	if len(o.typeFlags) == 0 {
		o.ForMount = true
	} else {
		for _, flag := range o.typeFlags {
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

	return nil
}

func (o LinkSecretOptions) Validate() error {
	if err := o.SecretOptions.Validate(); err != nil {
		return err
	}

	if !o.ForPull && !o.ForMount {
		return errors.New("for must be present")
	}

	return nil
}

func (o LinkSecretOptions) LinkSecrets() error {
	serviceaccount, err := o.GetServiceAccount()
	if err != nil {
		return err
	}

	err = o.linkSecretsToServiceAccount(serviceaccount)
	if err != nil {
		return err
	}

	return nil
}

// TODO: when Secrets in kapi.ServiceAccount get changed to MountSecrets and represented by LocalObjectReferences, this can be
// refactored to reuse the addition code better
// linkSecretsToServiceAccount links secrets to the service account, either as pull secrets, mount secrets, or both.
func (o LinkSecretOptions) linkSecretsToServiceAccount(serviceaccount *kapi.ServiceAccount) error {
	updated := false
	newSecrets, failLater, err := o.GetSecrets()
	if err != nil {
		return err
	}
	newSecretNames := o.GetSecretNames(newSecrets)

	if o.ForMount {
		currentSecrets := o.GetMountSecretNames(serviceaccount)
		secretsToLink := newSecretNames.Difference(currentSecrets)
		for _, secretName := range secretsToLink.List() {
			serviceaccount.Secrets = append(serviceaccount.Secrets, kapi.ObjectReference{Name: secretName})
			updated = true
		}
	}
	if o.ForPull {
		currentSecrets := o.GetPullSecretNames(serviceaccount)
		secretsToLink := newSecretNames.Difference(currentSecrets)
		for _, secretName := range secretsToLink.List() {
			serviceaccount.ImagePullSecrets = append(serviceaccount.ImagePullSecrets, kapi.LocalObjectReference{Name: secretName})
			updated = true
		}
	}
	if updated {
		_, err = o.ClientInterface.ServiceAccounts(o.Namespace).Update(serviceaccount)
		return err
	}

	if failLater {
		return errors.New("Some secrets could not be linked")
	}

	return nil
}
