package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

const (
	// DefaultScope is the default scope used for getting token
	// having role specific to the hierarchy matching the scope.
	DefaultScope = "https://management.core.windows.net//.default"
)

// getToken returns the token which can be used for operations
// on the resources.
func getToken(cred azcore.TokenCredential) (string, error) {
	token, err := cred.GetToken(context.Background(), policy.TokenRequestOptions{Scopes: []string{DefaultScope}})
	if err != nil {
		return "", err
	}

	return token.Token, nil
}

// NewCred returns credential capable of providing an OAuth token
// which can be used for obtaining clients for operations on the
// resources.
func NewCred(config file) (cred azcore.TokenCredential, err error) {
	cred, err = azidentity.NewClientSecretCredential(config.TenantID,
		config.ClientID,
		config.ClientSecret,
		nil)
	return
}
