package index

import (
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/docker/distribution/digest"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
)

type ImageIndex struct {
	client client.ImageInterface

	imageController  cache.Controller
	imageIndex       cache.Indexer
	imageStoreSynced func() bool
}

var (
	ChainIndexName     = "chain"
	LastChainIndexName = "last-chain"
	// TODO: Re-listing images might be expensive on large clusters, this should be
	// configurable?
	ResyncInterval = 10 * time.Minute
)

// NewImageIndex returns a new image index.
func NewImageIndex(client client.ImageInterface, stopChan <-chan struct{}) *ImageIndex {
	c := &ImageIndex{
		client: client,
	}
	c.imageIndex, c.imageController = cache.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
				return c.client.List(options)
			},
			WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
				return c.client.Watch(options)
			},
		},
		&imageapi.Image{},
		ResyncInterval,
		cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addImage,
			UpdateFunc: c.updateImage,
			DeleteFunc: c.deleteImage,
		},
		cache.Indexers{
			ChainIndexName:     chainIndexFunc,
			LastChainIndexName: lastChainIndexFunc,
		},
	)

	go c.imageController.Run(stopChan)
	return c
}

func (c *ImageIndex) WaitForSyncedStores() {
	for !c.imageController.HasSynced() {
		time.Sleep(100 * time.Millisecond)
	}
}

// Ancestors provides a list of images that are based in the provideed image.
func (c *ImageIndex) Ancestors(image *imageapi.Image) ([]*imageapi.Image, error) {
	imageChains := getImageLayerChain(image)
	result := &uniqueImageList{}
	for _, chain := range imageChains {
		objects, err := c.imageIndex.ByIndex(LastChainIndexName, chain)
		if err != nil {
			return nil, err
		}
		for _, obj := range objects {
			if i := obj.(*imageapi.Image); i.Name != image.Name {
				result.Append(i)
			}
		}
	}
	return result.Images, nil
}

// Descendants provides a list of images that the provided image is based on.
// Note that if a base image is tagged multiple times, this will return all the matching
// images.
func (c *ImageIndex) Descendants(image *imageapi.Image) ([]*imageapi.Image, error) {
	chains := getImageLayerChain(image)
	objects, err := c.imageIndex.ByIndex(ChainIndexName, chains[len(chains)-1])
	if err != nil {
		return nil, err
	}
	result := &uniqueImageList{}
	for _, obj := range objects {
		i := obj.(*imageapi.Image)
		imageChain := getImageLayerChain(i)
		// Remove the image itself from the result
		if imageChain[len(imageChain)-1] != chains[len(chains)-1] {
			result.Append(i)
		}
	}
	return result.Images, nil
}

// Siblings returns a list of images that represents the same image but under different
// tag or name.
func (c *ImageIndex) Siblings(image *imageapi.Image) ([]*imageapi.Image, error) {
	chains := getImageLayerChain(image)
	objects, err := c.imageIndex.ByIndex(LastChainIndexName, chains[len(chains)-1])
	if err != nil {
		return nil, err
	}
	result := []*imageapi.Image{}
	for _, obj := range objects {
		i := obj.(*imageapi.Image)
		if i.Name != image.Name {
			result = append(result, i)
		}
	}
	return result, nil
}

func (c *ImageIndex) addImage(obj interface{}) {
	if err := c.imageIndex.Add(obj); err != nil {
		glog.Errorf("failed to add image %+v to index: %v", obj, err)
	}
}

func (c *ImageIndex) updateImage(_, newObj interface{}) {
	if err := c.imageIndex.Update(newObj); err != nil {
		glog.Errorf("failed to update image %+v in index: %v", newObj, err)
	}
}

func (c *ImageIndex) deleteImage(obj interface{}) {
	i, ok := obj.(*imageapi.Image)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			glog.Errorf("couldn't get object from tombstone %#v", obj)
			return
		}
		i, ok = tombstone.Obj.(*imageapi.Image)
		if !ok {
			glog.Errorf("tombstone contained object that is not a Deployment %#v", obj)
			return
		}
	}
	c.imageIndex.Delete(i)
}

type uniqueImageList struct {
	Images []*imageapi.Image
	index  map[string]struct{}
}

func (l *uniqueImageList) Append(image *imageapi.Image) {
	if l.index == nil {
		l.index = map[string]struct{}{}
	}
	if _, exists := l.index[image.Name]; exists {
		return
	}
	l.Images = append(l.Images, image)
	l.index[image.Name] = struct{}{}
}

// getImageLayerChain recalculates the digest from each layer and return a sequence that
// represents the image.
func getImageLayerChain(image *imageapi.Image) []string {
	chain := []string{}
	for i, layer := range image.DockerImageLayers {
		if i > 0 {
			chain = append(chain, digest.FromBytes([]byte(chain[i-1]+"/"+layer.Name)).String())
		} else {
			chain = append(chain, layer.Name)
		}
	}
	return chain
}

// chainIndexFunc provides an index function that indexes the images by the chains that the
// images are build from.
func chainIndexFunc(obj interface{}) ([]string, error) {
	return getImageLayerChain(obj.(*imageapi.Image)), nil
}

// lastChainIndexFunc provides an index function that indexes the images by the last chain
// that indentifies the image itself.
func lastChainIndexFunc(obj interface{}) ([]string, error) {
	chains := getImageLayerChain(obj.(*imageapi.Image))
	return []string{chains[len(chains)-1]}, nil
}
