package foo

// GatewaysClient ...
type GatewaysClient struct {
	BaseClient
}

// NewGatewaysClient ...
func NewGatewaysClient(subscriptionID string) GatewaysClient {
	return NewGatewaysClientWithBaseURI(DefaultBaseURI, subscriptionID)
}

// NewGatewaysClientWithBaseURI ...
func NewGatewaysClientWithBaseURI(baseURI string, subscriptionID string) GatewaysClient {
	return GatewaysClient{NewWithBaseURI(baseURI, subscriptionID)}
}
