package gnocchi

import (
	"github.com/gophercloud/gophercloud"
)

func initClientOpts(client *gophercloud.ProviderClient, eo gophercloud.EndpointOpts, clientType string) (*gophercloud.ServiceClient, error) {
	sc := new(gophercloud.ServiceClient)
	eo.ApplyDefaults(clientType)
	url, err := client.EndpointLocator(eo)
	if err != nil {
		return sc, err
	}
	sc.ProviderClient = client
	sc.Endpoint = url
	sc.Type = clientType
	return sc, nil
}

// NewGnocchiV1 creates a ServiceClient that may be used with the v1 Gnocchi package.
func NewGnocchiV1(client *gophercloud.ProviderClient, eo gophercloud.EndpointOpts) (*gophercloud.ServiceClient, error) {
	sc, err := initClientOpts(client, eo, "metric")
	sc.ResourceBase = sc.Endpoint + "v1/"
	return sc, err
}
