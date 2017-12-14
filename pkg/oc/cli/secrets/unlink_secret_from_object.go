package secrets

import (
	"errors"
	"fmt"
	"io"

	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

const UnlinkSecretRecommendedName = "unlink"

var (
	unlinkSecretLong = templates.LongDesc(`
    Unlink (detach) secrets from a service account

    If a secret is no longer valid for a pod, build or image pull, you may unlink it from a service account.`)

	unlinkSecretExample = templates.Examples(`
    # Unlink a secret currently associated with a service account:
    %[1]s serviceaccount-name secret-name another-secret-name ...`)
)

type UnlinkSecretOptions struct {
	SecretOptions
}

// NewCmdUnlinkSecret creates a command object for detaching one or more secret references from a service account
func NewCmdUnlinkSecret(name, fullName string, f kcmdutil.Factory, out io.Writer) *cobra.Command {
	o := &UnlinkSecretOptions{SecretOptions{Out: out}}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s serviceaccount-name secret-name [another-secret-name] ...", name),
		Short:   "Detach secrets from a ServiceAccount",
		Long:    unlinkSecretLong,
		Example: fmt.Sprintf(unlinkSecretExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
			}
			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
			}

			if err := o.UnlinkSecrets(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	return cmd
}

func (o UnlinkSecretOptions) UnlinkSecrets() error {
	serviceaccount, err := o.GetServiceAccount()
	if err != nil {
		return err
	}

	if err = o.unlinkSecretsFromServiceAccount(serviceaccount); err != nil {
		return err
	}

	return nil
}

// unlinkSecretsFromServiceAccount detaches pull and mount secrets from the service account.
func (o UnlinkSecretOptions) unlinkSecretsFromServiceAccount(serviceaccount *kapi.ServiceAccount) error {
	// All of the requested secrets must be present in either the Mount or Pull secrets
	// If any of them are not present, we'll return an error and push no changes.
	rmSecrets, hasNotFound, err := o.GetSecrets(true)
	if err != nil {
		return err
	}
	rmSecretNames := o.GetSecretNames(rmSecrets)

	newMountSecrets := []kapi.ObjectReference{}
	newPullSecrets := []kapi.LocalObjectReference{}
	updated := false

	// Check the mount secrets
	for _, secret := range serviceaccount.Secrets {
		if !rmSecretNames.Has(secret.Name) {
			// Copy this back in, since it doesn't match the ones we're removing
			newMountSecrets = append(newMountSecrets, secret)
		} else {
			updated = true
		}
	}

	// Check the image pull secrets
	for _, imagePullSecret := range serviceaccount.ImagePullSecrets {
		if !rmSecretNames.Has(imagePullSecret.Name) {
			// Copy this back in, since it doesn't match the one we're removing
			newPullSecrets = append(newPullSecrets, imagePullSecret)
		} else {
			updated = true
		}
	}

	if updated {
		// Save the updated Secret lists back to the server
		serviceaccount.Secrets = newMountSecrets
		serviceaccount.ImagePullSecrets = newPullSecrets
		_, err = o.KubeCoreClient.ServiceAccounts(o.Namespace).Update(serviceaccount)
		if err != nil {
			return err
		}
		if hasNotFound {
			return fmt.Errorf("Unlinked deleted secrets from %s/%s service account", o.Namespace, serviceaccount.Name)
		}
		return nil
	} else {
		return errors.New("No valid secrets found or secrets not linked to service account")
	}
}
