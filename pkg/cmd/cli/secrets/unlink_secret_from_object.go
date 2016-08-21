package secrets

import (
	"errors"
	"fmt"
	"io"

	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/spf13/cobra"
)

const (
	UnlinkSecretRecommendedName = "unlink"

	// TODO: move to examples
	unlinkSecretLong = `
Unlink (detach) secrets from a service account

If a secret is no longer valid for a pod, build or image pull, you may unlink it from a service account.`

	unlinkSecretExample = ` # Unlink a secret currently associated with a service account:
%[1]s serviceaccount-name secret-name another-secret-name ...`
)

type UnlinkSecretOptions struct {
	SecretOptions
}

// NewCmdUnlinkSecret creates a command object for detaching one or more secret references from a service account
func NewCmdUnlinkSecret(name, fullName string, f *kcmdutil.Factory, out io.Writer) *cobra.Command {
	o := &UnlinkSecretOptions{SecretOptions{Out: out}}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s serviceaccount-name secret-name [another-secret-name] ...", name),
		Short:   "Detach secrets from a ServiceAccount",
		Long:    unlinkSecretLong,
		Example: fmt.Sprintf(unlinkSecretExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := o.Complete(f, args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}
			if err := o.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
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
	rmSecrets, failLater, err := o.GetSecrets()
	if err != nil {
		return err
	}
	rmSecretNames := o.GetSecretNames(rmSecrets)

	newMountSecrets := []kapi.ObjectReference{}
	newPullSecrets := []kapi.LocalObjectReference{}

	// Check the mount secrets
	for i := len(serviceaccount.Secrets) - 1; i >= 0; i-- {
		found := false
		for _, secretname := range rmSecretNames.List() {
			if secretname == serviceaccount.Secrets[i].Name {
				found = true
				// Skip adding this to the updated list
			}
		}

		if !found {
			// Copy this back in, since it doesn't match the ones we're removing
			newMountSecrets = append(newMountSecrets, serviceaccount.Secrets[i])
		}
	}

	// Check the image pull secrets
	for i := len(serviceaccount.ImagePullSecrets) - 1; i >= 0; i-- {
		found := false
		for _, secretname := range rmSecretNames.List() {
			if secretname == serviceaccount.ImagePullSecrets[i].Name {
				found = true
				// Skip adding this to the updated list
			}
		}
		if !found {
			// Copy this back in, since it doesn't match the one we're removing
			newPullSecrets = append(newPullSecrets, serviceaccount.ImagePullSecrets[i])
		}
	}

	// Save the updated Secret lists back to the server
	serviceaccount.Secrets = newMountSecrets
	serviceaccount.ImagePullSecrets = newPullSecrets
	_, err = o.ClientInterface.ServiceAccounts(o.Namespace).Update(serviceaccount)
	if err != nil {
		return err
	}

	if failLater {
		return errors.New("Some secrets could not be unlinked")
	}

	return nil
}
