package secrets

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	coreapiv1 "k8s.io/api/core/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions/printers"
	"k8s.io/kubernetes/pkg/kubectl/scheme"
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

	PrintFlags *genericclioptions.PrintFlags
	Printer    printers.ResourcePrinter

	genericclioptions.IOStreams
}

func NewUnlinkSecretOptions(streams genericclioptions.IOStreams) *UnlinkSecretOptions {
	return &UnlinkSecretOptions{
		PrintFlags:    genericclioptions.NewPrintFlags("updated").WithTypeSetter(scheme.Scheme),
		SecretOptions: SecretOptions{},
		IOStreams:     streams,
	}
}

// NewCmdUnlinkSecret creates a command object for detaching one or more secret references from a service account
func NewCmdUnlinkSecret(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewUnlinkSecretOptions(streams)

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

			if err := o.Run(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	o.PrintFlags.AddFlags(cmd)
	return cmd
}

func (o *UnlinkSecretOptions) Complete(f kcmdutil.Factory, args []string) error {
	if err := o.SecretOptions.Complete(f, args); err != nil {
		return err
	}

	var err error
	o.Printer, err = o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}

	return nil
}

func (o UnlinkSecretOptions) Run() error {
	serviceaccount, err := o.GetServiceAccount()
	if err != nil {
		return err
	}

	if err = o.unlinkSecretsFromServiceAccount(serviceaccount); err != nil {
		return err
	}

	return o.Printer.PrintObj(serviceaccount, o.Out)
}

// unlinkSecretsFromServiceAccount detaches pull and mount secrets from the service account.
func (o UnlinkSecretOptions) unlinkSecretsFromServiceAccount(serviceaccount *coreapiv1.ServiceAccount) error {
	// All of the requested secrets must be present in either the Mount or Pull secrets
	// If any of them are not present, we'll return an error and push no changes.
	rmSecrets, hasNotFound, err := o.GetSecrets(true)
	if err != nil {
		return err
	}
	rmSecretNames := o.GetSecretNames(rmSecrets)

	newMountSecrets := []coreapiv1.ObjectReference{}
	newPullSecrets := []coreapiv1.LocalObjectReference{}
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
		_, err = o.KubeClient.ServiceAccounts(o.Namespace).Update(serviceaccount)
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
