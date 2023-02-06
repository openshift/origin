package azure

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/services/resources/mgmt/2017-05-10/resources"
	"github.com/Azure/go-autorest/autorest/to"
)

// ResourceMetadata is for holding slicker version of the
// resource metadata.
type ResourceMetadata struct {
	Name string
	Type string
	Tags map[string]string
}

// ListResources returns a list of resources in a resourcegroup and the resourcegroup
// with the metadata(name, type, tags).
func ListResources(ctx context.Context, resourceGroupName string) ([]ResourceMetadata, error) {
	config, err := getAuthFile()
	if err != nil {
		return nil, err
	}

	cred, err := NewCred(*config)
	if err != nil {
		return nil, err
	}

	var list []ResourceMetadata
	rg, err := getResourceGroup(ctx, cred, config.SubscriptionID, resourceGroupName)
	if err != nil {
		return nil, err
	}

	client, err := New(cred, config.SubscriptionID)
	if err != nil {
		return nil, err
	}
	rlist, err := listByResourceGroup(ctx, client, resourceGroupName)
	if err != nil {
		return nil, err
	}

	list = append(list, rg)
	list = append(list, rlist...)
	return list, nil
}

// listByResourceGroup returns a list of resources in the requested resourcegroup
// with the metadata(name, type, tags).
func listByResourceGroup(ctx context.Context, c resources.Client, resourceGroupName string) ([]ResourceMetadata, error) {
	result, err := c.ListByResourceGroup(ctx, resourceGroupName, "", "", nil)
	if err != nil {
		return nil, err
	}

	var responses []resources.GenericResourceExpanded
	responses = append(responses, result.Values()...)

	for result.NotDone() {
		if err = result.NextWithContext(ctx); err != nil {
			return nil, err
		}
		responses = append(responses, result.Values()...)
	}

	return getSlickerResourceList(responses), nil
}

// getSlickerResourceList returns a list of streamlined metadata of
// each resource present in the passed list.
func getSlickerResourceList(list []resources.GenericResourceExpanded) []ResourceMetadata {
	var slickList = make([]ResourceMetadata, 0, len(list))

	for _, item := range list {
		slickList = append(slickList, ResourceMetadata{
			Name: to.String(item.Name),
			Type: to.String(item.Type),
			Tags: to.StringMap(item.Tags),
		})
	}

	return slickList
}

// getResourceGroup returns the streamlined metadata(name, type, tags)
// of the requested resourcegroup.
func getResourceGroup(ctx context.Context, cred azcore.TokenCredential, subscriptionID, resourceGroupName string) (ResourceMetadata, error) {
	client, err := armresources.NewResourceGroupsClient(subscriptionID, cred, nil)
	if err != nil {
		return ResourceMetadata{}, err
	}

	rg, err := client.Get(ctx, resourceGroupName, nil)
	if err != nil {
		return ResourceMetadata{}, err
	}

	return ResourceMetadata{
		Name: to.String(rg.Name),
		Type: to.String(rg.Type),
		Tags: to.StringMap(rg.Tags),
	}, nil
}
