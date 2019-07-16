package apprclient

import (
	"context"

	"github.com/antihax/optional"

	"github.com/openshift/oc/pkg/cli/admin/catalog/openapi"
)

const (
	mediaType = "helm"
)

// This interface (internal to this package) encapsulates nitty gritty details of go-appr client bindings
type apprApiAdapter interface {
	// ListPackages returns a list of package(s) available to the user.
	// When namespace is specified, only package(s) associated with the given namespace are returned.
	// If namespace is empty then visible package(s) across all namespaces are returned.
	ListPackages(namespace string) ([]openapi.PackageDescription, error)

	// GetPackageMetadata returns metadata associated with a given package
	GetPackageMetadata(namespace string, repository string, release string) (*openapi.Package, error)

	// DownloadOperatorManifest downloads the blob associated with a given digest that directly corresponds to a package release
	DownloadOperatorManifest(namespace string, repository string, digest string) ([]byte, error)
}

type apprApiAdapterImpl struct {
	client   *openapi.APIClient
	basePath string
}

func (a *apprApiAdapterImpl) ListPackages(namespace string) ([]openapi.PackageDescription, error) {
	opts := openapi.ListPackagesOpts{
		Namespace: optional.EmptyString(),
		Query:     optional.EmptyString(),
		MediaType: optional.NewString(mediaType),
	}

	if namespace != "" {
		opts.Namespace = optional.NewString(namespace)
	}

	packages, _, err := a.client.PackageApi.ListPackages(context.TODO(), &opts)
	if err != nil {
		return nil, err
	}

	return packages, nil
}

func (a *apprApiAdapterImpl) GetPackageMetadata(namespace string, repository string, release string) (*openapi.Package, error) {
	pkg, _, err := a.client.PackageApi.ShowPackage(context.TODO(), namespace, repository, release, mediaType)
	if err != nil {
		return nil, err
	}

	return &pkg, nil
}

func (a *apprApiAdapterImpl) DownloadOperatorManifest(namespace string, repository string, digest string) ([]byte, error) {
	bytes, _, err := a.client.BlobsApi.PullBlob(context.TODO(), namespace, repository, digest)
	if err != nil {
		return nil, err
	}
	return bytes, nil
}
