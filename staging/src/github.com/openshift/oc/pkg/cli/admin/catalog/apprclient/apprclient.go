package apprclient

import (
	"net/url"

	"github.com/openshift/oc/pkg/cli/admin/catalog/openapi"
)

type Options struct {
	// Source refers to the URL of the remote app registry server.
	Source string

	// AuthToken refers to the authorization token required to access operator
	// manifest in private repositories.
	//
	// If not set, it is assumed that the remote registry is public.
	AuthToken string
}

func New(options Options) (Client, error) {
	u, err := url.Parse(options.Source)
	if err != nil {
		return nil, err
	}

	cfg := openapi.NewConfiguration()
	cfg.Host = u.Host
	cfg.BasePath = u.Path
	cfg.Scheme = u.Scheme

	// If a token has been specified then we should pass it along in the headers
	if options.AuthToken != "" {
		cfg.AddDefaultHeader("Authorization", options.AuthToken)
	}

	return &client{
		adapter: &apprApiAdapterImpl{client: openapi.NewAPIClient(cfg)},
		decoder: &blobDecoderImpl{},
	}, nil
}
