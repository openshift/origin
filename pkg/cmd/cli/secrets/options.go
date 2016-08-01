package secrets

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	client "k8s.io/kubernetes/pkg/client/unversioned"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
)

// SecretOptions Structure holding state for processing secret linking and
// unlinking.
type SecretOptions struct {
	TargetName  string
	SecretNames []string
	typeFlags   []string

	Namespace string

	Mapper          meta.RESTMapper
	Typer           runtime.ObjectTyper
	ClientMapper    resource.ClientMapper
	ClientInterface client.Interface

	Out io.Writer
}

// Complete Parses the command line arguments and populates SecretOptions
func (o *SecretOptions) Complete(f *kcmdutil.Factory, args []string) error {
	if len(args) < 2 {
		return errors.New("must have service account name and at least one secret name")
	}
	o.TargetName = args[0]
	o.SecretNames = args[1:]

	var err error
	o.ClientInterface, err = f.Client()
	if err != nil {
		return err
	}

	o.Namespace, _, err = f.DefaultNamespace()
	if err != nil {
		return err
	}

	o.Mapper, o.Typer = f.Object(false)
	o.ClientMapper = resource.ClientMapperFunc(f.ClientForMapping)

	return nil
}

// Validate Ensures that all arguments have appropriate values
func (o SecretOptions) Validate() error {
	if len(o.TargetName) == 0 {
		return errors.New("service account name must be present")
	}
	if len(o.SecretNames) == 0 {
		return errors.New("secret name must be present")
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

// GetServiceAccount Retrieve the service account object specified by the command
func (o SecretOptions) GetServiceAccount() (*kapi.ServiceAccount, error) {
	r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper, kapi.Codecs.UniversalDecoder()).
		NamespaceParam(o.Namespace).
		ResourceNames("serviceaccounts", o.TargetName).
		SingleResourceType().
		Do()
	if r.Err() != nil {
		return nil, r.Err()
	}
	obj, err := r.Object()
	if err != nil {
		return nil, err
	}

	switch t := obj.(type) {
	case *kapi.ServiceAccount:
		return t, nil
	default:
		return nil, fmt.Errorf("unhandled object: %#v", t)
	}
}

// GetSecretNames Get a list of the names of the secrets in a set of them
func (o SecretOptions) GetSecretNames(secrets []*kapi.Secret) sets.String {
	names := sets.String{}
	for _, secret := range secrets {
		names.Insert(secret.Name)
	}
	return names
}

// GetMountSecretNames Get a list of the names of the mount secrets associated
// with a service account
func (o SecretOptions) GetMountSecretNames(serviceaccount *kapi.ServiceAccount) sets.String {
	names := sets.String{}
	for _, secret := range serviceaccount.Secrets {
		names.Insert(secret.Name)
	}
	return names
}

// GetPullSecretNames Get a list of the names of the pull secrets associated
// with a service account.
func (o SecretOptions) GetPullSecretNames(serviceaccount *kapi.ServiceAccount) sets.String {
	names := sets.String{}
	for _, secret := range serviceaccount.ImagePullSecrets {
		names.Insert(secret.Name)
	}
	return names
}

// GetOut Retrieve the output writer
func (o SecretOptions) GetOut() io.Writer {
	if o.Out == nil {
		return ioutil.Discard
	}

	return o.Out
}

// GetSecrets Return a list of secret objects in the default namespace
func (o SecretOptions) GetSecrets() ([]*kapi.Secret, bool, error) {
	secrets := []*kapi.Secret{}
	failLater := false

	for _, secretName := range o.SecretNames {
		r := resource.NewBuilder(o.Mapper, o.Typer, o.ClientMapper, kapi.Codecs.UniversalDecoder()).
			NamespaceParam(o.Namespace).
			ResourceNames("secrets", secretName).
			SingleResourceType().
			Do()
		if r.Err() != nil {
			return nil, false, r.Err()
		}
		obj, err := r.Object()
		if err != nil {
			fmt.Fprintf(os.Stderr, "secrets \"%s\" not found\n", secretName)
			// Missing secrets are non-fatal but the command should not return
			// success.
			failLater = true
			continue
		}
		switch t := obj.(type) {
		case *kapi.Secret:
			secrets = append(secrets, t)
		default:
			return nil, false, fmt.Errorf("unhandled object: %#v", t)
		}
	}

	if len(secrets) == 0 {
		return nil, false, errors.New("No valid secrets found")
	}

	return secrets, failLater, nil
}
