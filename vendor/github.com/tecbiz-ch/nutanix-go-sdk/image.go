package nutanix

import (
	"bytes"
	"context"
	"fmt"
	"net/http"

	"github.com/tecbiz-ch/nutanix-go-sdk/pkg/utils"
	"github.com/tecbiz-ch/nutanix-go-sdk/schema"
)

const (
	imageBasePath   = "/images"
	imageListPath   = imageBasePath + "/list"
	imageSinglePath = imageBasePath + "/%s"
	imageUploadPath = imageSinglePath + "/file"
)

// ImageClient is a client for the image API.
type ImageClient struct {
	client *Client
}

// Get retrieves an image by its ID if the input can be parsed as an uuid, otherwise it
// retrieves an image by its name
func (c *ImageClient) Get(ctx context.Context, idOrName string) (*schema.ImageIntent, error) {
	if utils.IsValidUUID(idOrName) {
		return c.GetByUUID(ctx, idOrName)
	}
	return c.GetByName(ctx, idOrName)
}

// GetByUUID retrieves an image by its UUID
func (c *ImageClient) GetByUUID(ctx context.Context, uuid string) (*schema.ImageIntent, error) {
	response := new(schema.ImageIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(imageSinglePath, uuid), http.MethodGet, nil, response)
	return response, err
}

// GetByName retrieves an image by its name
func (c *ImageClient) GetByName(ctx context.Context, name string) (*schema.ImageIntent, error) {
	images, err := c.List(ctx, &schema.DSMetadata{Filter: fmt.Sprintf("name==%s", name)})
	if err != nil {
		return nil, err
	}
	if len(images.Entities) == 0 {
		return nil, fmt.Errorf("image not found: %s", name)
	}
	return images.Entities[0], err
}

// List returns a list of images
func (c *ImageClient) List(ctx context.Context, opts *schema.DSMetadata) (*schema.ImageListIntent, error) {
	response := new(schema.ImageListIntent)
	err := c.client.requestHelper(ctx, imageListPath, http.MethodPost, opts, response)
	return response, err

}

// All returns all images
func (c *ImageClient) All(ctx context.Context) (*schema.ImageListIntent, error) {
	return c.List(ctx, &schema.DSMetadata{Length: utils.Int64Ptr(itemsPerPage), Offset: utils.Int64Ptr(0)})
}

// Upload a qcow2
func (c *ImageClient) Upload(ctx context.Context, uuid string, fileContents []byte) (*schema.ImageIntent, error) {

	file := &schema.File{
		ContentType: mediaTypeUpload,
		Body:        bytes.NewBuffer(fileContents),
	}

	req, err := c.client.NewV3PCRequest(ctx, http.MethodPut, fmt.Sprintf(imageUploadPath, uuid), file)
	if err != nil {
		return nil, err
	}

	err = c.client.Do(req, nil)
	if err != nil {
		return nil, err
	}

	return c.GetByUUID(ctx, uuid)
}

// Create creates a image
func (c *ImageClient) Create(ctx context.Context, createRequest *schema.ImageIntent) (*schema.ImageIntent, error) {
	response := new(schema.ImageIntent)
	err := c.client.requestHelper(ctx, imageBasePath, http.MethodPost, createRequest, response)
	return response, err
}

// Update a image
func (c *ImageClient) Update(ctx context.Context, image *schema.ImageIntent) (*schema.ImageIntent, error) {
	image.Status = nil
	response := new(schema.ImageIntent)
	err := c.client.requestHelper(ctx, fmt.Sprintf(imageSinglePath, image.Metadata.UUID), http.MethodPut, image, response)
	return response, err
}

// Delete deletes a image.
func (c *ImageClient) Delete(ctx context.Context, uuid string) error {
	return c.client.requestHelper(ctx, fmt.Sprintf(imageSinglePath, uuid), http.MethodDelete, nil, nil)
}
