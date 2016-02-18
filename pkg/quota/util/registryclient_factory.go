package util

import (
	"fmt"

	"github.com/golang/glog"

	kclient "k8s.io/kubernetes/pkg/client/unversioned"

	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/serviceaccounts"
)

type InternalRegistryClientFactory interface {
	GetClient() (dockerregistry.Client, error)
}

// NewInternalRegistryClientFactoryForServiceAccount returns a factory that allows to create a client for
// internal Docker registry. kClient must be allowed to get secrets and serviceaccounts objects. Given
// serviceAccount must be authorized to get imagestreams/layers objects and belongs to given namespace.
func NewInternalRegistryClientFactoryForServiceAccount(kClient kclient.Interface, namespace, serviceAccount string, caBundles ...string) InternalRegistryClientFactory {
	return &internalRegistryClientFactory{
		kClient:        kClient,
		saNamespace:    namespace,
		serviceAccount: serviceAccount,
		caBundles:      caBundles,
	}
}

type internalRegistryClientFactory struct {
	kClient        kclient.Interface
	saNamespace    string
	serviceAccount string
	caBundles      []string
	rClient        dockerregistry.Client
}

func (f *internalRegistryClientFactory) GetClient() (dockerregistry.Client, error) {
	if f.rClient != nil {
		return f.rClient, nil
	}
	tokenRetriever := serviceaccounts.ClientLookupTokenRetriever{Client: f.kClient}
	token, err := tokenRetriever.GetToken(f.saNamespace, f.serviceAccount)
	if err != nil {
		return nil, fmt.Errorf("failed to get token of service account %s/%s for registry client: %v", f.saNamespace, f.serviceAccount, err)
	}

	glog.V(4).Infof("creating internal docker registry client for serviceaccount %s/%s", f.saNamespace, f.serviceAccount)

	f.rClient, err = dockerregistry.NewInternalRegistryClient(token, f.caBundles...)
	if err != nil {
		return nil, fmt.Errorf("Unable to initialize internal registry client: %v", err)
	}
	return f.rClient, nil
}
