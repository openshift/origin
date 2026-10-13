package nutanix

import (
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	clusterBasePath   = "/clusters"
	clusterListPath   = clusterBasePath + "/list"
	clusterSinglePath = clusterBasePath + "/%s"
)

// ClusterClient is a client for the cluster API.
type ClusterClient struct {
	client *Client
}

// Get retrieves an cluster by its ID if the input can be parsed as an string, otherwise it
// retrieves an cluster by its name. If the cluster does not exist, nil is returned.
func (c *ClusterClient) Get(ctx context.Context, idOrName string) (*schema.ClusterIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an cluster by its UUID. If the cluster does not exist, nil is returned.
func (c *ClusterClient) GetByUUID(ctx context.Context, uuid string) (*schema.ClusterIntent, error) {
	response := new(schema.ClusterIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(clusterSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an cluster by its name. If the cluster does not exist, nil is returned.
func (c *ClusterClient) GetByName(ctx context.Context, name string) (*schema.ClusterIntent, error) {
	// filter not working. always returned all entries
	list, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("clu_name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(list.Entities) == 0 {
		return nil, fmt.Errorf("cluster not found: %s, error: %v", name, err)
	}
	for _, cluster := range list.Entities {
		if cluster.Spec.Name == name {
			return cluster, nil
		}
	}
	return nil, fmt.Errorf("cluster not found: %s", name)
}

// List returns a list of clusters for a specific page.
func (c *ClusterClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.ClusterListIntent, error) {
	response := new(schema.ClusterListIntent)
	err := c.client.requestHelper(ctx, clusterListPath, http.MethodPost, opts, response)
	return response, err
}

// All returns all clusters.
func (c *ClusterClient) All(ctx context.Context) (*schema.ClusterListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}
