package libkpod

import (
	"github.com/containers/storage/pkg/archive"
	"github.com/kubernetes-incubator/cri-o/libpod/images"
	"github.com/kubernetes-incubator/cri-o/libpod/layers"
	"github.com/pkg/errors"
)

// GetDiff returns the differences between the two images, layers, or containers
func (c *ContainerServer) GetDiff(from, to string) ([]archive.Change, error) {
	toLayer, err := c.getLayerID(to)
	if err != nil {
		return nil, err
	}
	fromLayer := ""
	if from != "" {
		fromLayer, err = c.getLayerID(from)
		if err != nil {
			return nil, err
		}
	}
	return c.Store().Changes(fromLayer, toLayer)
}

// GetLayerID gets a full layer id given a full or partial id
// If the id matches a container or image, the id of the top layer is returned
// If the id matches a layer, the top layer id is returned
func (c *ContainerServer) getLayerID(id string) (string, error) {
	var toLayer string
	toImage, err := images.FindImage(c.store, id)
	if err != nil {
		toCtr, err := c.store.Container(id)
		if err != nil {
			toLayer, err = layers.FullID(c.store, id)
			if err != nil {
				return "", errors.Errorf("layer, image, or container %s does not exist", id)
			}
		} else {
			toLayer = toCtr.LayerID
		}
	} else {
		toLayer = toImage.TopLayer
	}
	return toLayer, nil
}

func (c *ContainerServer) getLayerParent(layerID string) (string, error) {
	layer, err := c.store.Layer(layerID)
	if err != nil {
		return "", err
	}
	return layer.Parent, nil
}
