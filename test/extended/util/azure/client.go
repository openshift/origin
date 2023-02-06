package azure

import (
	"fmt"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest"
)

const (
	// DefaultBaseURI is the default URI used for the service Resources
	DefaultBaseURI = resources.DefaultBaseURI
)

// New returns Client which provides operations for working with resources and resource groups.
func New(cred azcore.TokenCredential, subscriptionID string) (resources.Client, error) {
	c := resources.Client{
		BaseClient: resources.BaseClient{
			Client:         autorest.NewClientWithUserAgent(resources.UserAgent()),
			BaseURI:        DefaultBaseURI,
			SubscriptionID: subscriptionID,
		},
	}

	token, err := getToken(cred)
	if err != nil {
		return c, err
	}

	c.Client.Authorizer = autorest.NewAPIKeyAuthorizerWithHeaders(map[string]interface{}{
		"Authorization": fmt.Sprintf("Bearer %s", token),
	})

	return c, nil
}
