package controller

import (
	"fmt"
	"sync"
	"time"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/cache"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/dockerregistry"
	"github.com/openshift/origin/pkg/image/api"
)

type ImportController struct {
	streams  client.ImageStreamsNamespacer
	mappings client.ImageStreamMappingsNamespacer
	// injected for testing
	client dockerregistry.Client

	stopChan chan struct{}

	imageStreamController *framework.Controller

	work           chan *api.ImageStream
	workingSet     sets.String
	workingSetLock sync.Mutex

	// this should not be larger the capacity of the work channel
	numParallelImports int
}

func NewImportController(isNamespacer client.ImageStreamsNamespacer, ismNamespacer client.ImageStreamMappingsNamespacer, parallelImports int, resyncInterval time.Duration) *ImportController {
	c := &ImportController{
		streams:  isNamespacer,
		mappings: ismNamespacer,

		numParallelImports: parallelImports,
		work:               make(chan *api.ImageStream, 20*parallelImports),
		workingSet:         sets.String{},
	}

	_, c.imageStreamController = framework.NewInformer(
		&cache.ListWatch{
			ListFunc: func() (runtime.Object, error) {
				return c.streams.ImageStreams(kapi.NamespaceAll).List(labels.Everything(), fields.Everything())
			},
			WatchFunc: func(resourceVersion string) (watch.Interface, error) {
				return c.streams.ImageStreams(kapi.NamespaceAll).Watch(labels.Everything(), fields.Everything(), resourceVersion)
			},
		},
		&api.ImageStream{},
		resyncInterval,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    c.imageStreamAdded,
			UpdateFunc: c.imageStreamUpdated,
		},
	)

	return c
}

// Runs controller loops and returns immediately
func (c *ImportController) Run() {
	if c.stopChan == nil {
		c.stopChan = make(chan struct{})
		go c.imageStreamController.Run(c.stopChan)

		for i := 0; i < c.numParallelImports; i++ {
			go util.Until(c.handleImport, time.Second, c.stopChan)
		}
	}
}

// Stop gracefully shuts down this controller
func (c *ImportController) Stop() {
	if c.stopChan != nil {
		close(c.stopChan)
		c.stopChan = nil
	}
}

func (c *ImportController) imageStreamAdded(obj interface{}) {
	imageStream := obj.(*api.ImageStream)
	if needsImport(imageStream) {
		glog.V(5).Infof("trying to add %s to the worklist", workingSetKey(imageStream))
		c.work <- imageStream
		glog.V(3).Infof("added %s to the worklist", workingSetKey(imageStream))

	} else {
		glog.V(5).Infof("not adding %s to the worklist", workingSetKey(imageStream))
	}
}

func (c *ImportController) imageStreamUpdated(oldObj interface{}, newObj interface{}) {
	newImageStream := newObj.(*api.ImageStream)
	if needsImport(newImageStream) {
		glog.V(5).Infof("trying to add %s to the worklist", workingSetKey(newImageStream))
		c.work <- newImageStream
		glog.V(3).Infof("added %s to the worklist", workingSetKey(newImageStream))

	} else {
		glog.V(5).Infof("not adding %s to the worklist", workingSetKey(newImageStream))
	}
}

// needsImport returns true if the provided image stream should have its tags imported.
func needsImport(stream *api.ImageStream) bool {
	return stream.Annotations == nil || len(stream.Annotations[api.DockerImageRepositoryCheckAnnotation]) == 0
}

func (c *ImportController) handleImport() {
	for {
		select {
		case <-c.stopChan:
			return

		case staleImageStream := <-c.work:
			glog.V(1).Infof("popped %s from the worklist", workingSetKey(staleImageStream))

			c.importImageStream(staleImageStream)
		}
	}
}

func (c *ImportController) importImageStream(staleImageStream *api.ImageStream) {
	// if we're already in the workingset, that means that some thread is already trying to do an import for this.
	// This does NOT mean that we shouldn't attempt to do this work, only that we shouldn't attempt to do it now.
	if !c.addToWorkingSet(staleImageStream) {
		// If there isn't any other work in the queue, wait for a while so that we don't hot loop.
		// Then requeue to the end of the channel.  That allows other work to continue without delay
		if len(c.work) == 0 {
			time.Sleep(100 * time.Millisecond)
		}
		glog.V(5).Infof("requeuing %s to the worklist", workingSetKey(staleImageStream))
		c.work <- staleImageStream

		return
	}
	defer c.removeFromWorkingSet(staleImageStream)

	err := kclient.RetryOnConflict(kclient.DefaultBackoff, func() error {
		liveImageStream, err := c.streams.ImageStreams(staleImageStream.Namespace).Get(staleImageStream.Name)
		// no work to do here
		if kapierrors.IsNotFound(err) {
			return nil
		}
		if err != nil {
			return err
		}
		if !needsImport(liveImageStream) {
			return nil
		}

		// if we're notified, do work and then start waiting again.
		return c.Next(liveImageStream)
	})

	if err != nil {
		util.HandleError(err)
	}
}

func workingSetKey(imageStream *api.ImageStream) string {
	return imageStream.Namespace + "/" + imageStream.Name
}

// addToWorkingSet returns true if the image stream was added, false if it was
// already present
func (c *ImportController) addToWorkingSet(imageStream *api.ImageStream) bool {
	c.workingSetLock.Lock()
	defer c.workingSetLock.Unlock()

	if c.workingSet.Has(workingSetKey(imageStream)) {
		return false
	}

	c.workingSet.Insert(workingSetKey(imageStream))
	return true
}

func (c *ImportController) removeFromWorkingSet(imageStream *api.ImageStream) {
	c.workingSetLock.Lock()
	defer c.workingSetLock.Unlock()
	c.workingSet.Delete(workingSetKey(imageStream))
}

// Next processes the given image stream, looking for streams that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image stream is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation. The tags of the original spec image
// are left as is (those are updated through status).
// There are 3 use cases here:
// 1. spec.DockerImageRepository defined without any tags results in all tags being imported
//    from upstream image repository
// 2. spec.DockerImageRepository + tags defined - import all tags from upstream image repository,
//    and all the specified which (if name matches) will overwrite the default ones.
//    Additionally:
//    for kind == DockerImage import or reference underlying image, iow. exact tag (not provided means latest),
//    for kind != DockerImage reference tag from the same or other ImageStream
// 3. spec.DockerImageRepository not defined - import tags per its definition.
// Current behavior of the controller is to process import as far as possible, but
// we still want to keep backwards compatibility and retries, for that we'll return
// error in the following cases:
// 1. connection failure to upstream image repository
// 2. reading tags when error is different from RepositoryNotFound or RegistryNotFound
// 3. image retrieving when error is different from RepositoryNotFound, RegistryNotFound or ImageNotFound
// 4. ImageStreamMapping save error
// 5. error when marking ImageStream as imported
func (c *ImportController) Next(stream *api.ImageStream) error {
	glog.V(4).Infof("Importing stream %s/%s...", stream.Namespace, stream.Name)

	insecure := stream.Annotations[api.InsecureRepositoryAnnotation] == "true"
	client := c.client
	if client == nil {
		client = dockerregistry.NewClient(5 * time.Second)
	}

	var errlist []error
	toImport, retry, err := getTags(stream, client, insecure)
	// return here, only if there is an error and nothing to import
	if err != nil && len(toImport) == 0 {
		if retry {
			return err
		}
		return c.done(stream, err.Error())
	}
	if err != nil {
		errlist = append(errlist, err)
	}

	retry, err = c.importTags(stream, toImport, client, insecure)
	if err != nil {
		if retry {
			return err
		}
		errlist = append(errlist, err)
	}

	if len(errlist) > 0 {
		return c.done(stream, kerrors.NewAggregate(errlist).Error())
	}

	return c.done(stream, "")
}

// getTags returns a map of tags to be imported, a flag saying if we should retry
// imports, meaning not setting the import annotation and an error if one occurs.
// Tags explicitly defined will overwrite those from default upstream image repository.
func getTags(stream *api.ImageStream, client dockerregistry.Client, insecure bool) (map[string]api.DockerImageReference, bool, error) {
	imports := make(map[string]api.DockerImageReference)
	references := sets.NewString()

	// read explicitly defined tags
	for tagName, specTag := range stream.Spec.Tags {
		if specTag.From == nil {
			continue
		}
		if specTag.From.Kind != "DockerImage" || specTag.Reference {
			references.Insert(tagName)
			continue
		}
		ref, err := api.ParseDockerImageReference(specTag.From.Name)
		if err != nil {
			glog.V(2).Infof("error parsing DockerImage %s: %v", specTag.From.Name, err)
			continue
		}
		imports[tagName] = ref.DockerClientDefaults()
	}

	if len(stream.Spec.DockerImageRepository) == 0 {
		return imports, false, nil
	}

	// read tags from default upstream image repository
	streamRef, err := api.ParseDockerImageReference(stream.Spec.DockerImageRepository)
	if err != nil {
		return imports, false, err
	}
	glog.V(5).Infof("Connecting to %s...", streamRef.Registry)
	conn, err := client.Connect(streamRef.Registry, insecure)
	if err != nil {
		glog.V(5).Infof("Error connecting to %s: %v", streamRef.Registry, err)
		// retry-able error no. 1
		return imports, true, err
	}
	glog.V(5).Infof("Fetching tags for %s/%s...", streamRef.Namespace, streamRef.Name)
	tags, err := conn.ImageTags(streamRef.Namespace, streamRef.Name)
	switch {
	case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err):
		glog.V(5).Infof("Error fetching tags for %s/%s: %v", streamRef.Namespace, streamRef.Name, err)
		return imports, false, err
	case err != nil:
		// retry-able error no. 2
		glog.V(5).Infof("Error fetching tags for %s/%s: %v", streamRef.Namespace, streamRef.Name, err)
		return imports, true, err
	}
	glog.V(5).Infof("Got tags for %s/%s: %#v", streamRef.Namespace, streamRef.Name, tags)
	for tag, image := range tags {
		if _, ok := imports[tag]; ok || references.Has(tag) {
			continue
		}
		idTagPresent := false
		// this for loop is for backwards compatibility with v1 repo, where
		// there was no image id returned with tags, like v2 does right now.
		for t2, i2 := range tags {
			if i2 == image && t2 == image {
				idTagPresent = true
				break
			}
		}
		ref := streamRef
		if idTagPresent {
			ref.Tag = image
		} else {
			ref.Tag = tag
		}
		ref.ID = image
		imports[tag] = ref
	}

	return imports, false, nil
}

// importTags imports tags specified in a map from given ImageStream. Returns flag
// saying if we should retry imports, meaning not setting the import annotation
// and an error if one occurs.
func (c *ImportController) importTags(stream *api.ImageStream, imports map[string]api.DockerImageReference, client dockerregistry.Client, insecure bool) (bool, error) {
	retrieved := make(map[string]*dockerregistry.Image)
	var errlist []error
	shouldRetry := false
	for tag, ref := range imports {
		image, retry, err := c.importTag(stream, tag, ref, retrieved[ref.ID], client, insecure)
		if err != nil {
			if retry {
				shouldRetry = retry
			}
			errlist = append(errlist, err)
			continue
		}
		// save image object for next tag imports, this is to avoid re-downloading the default image registry
		if len(ref.ID) > 0 {
			retrieved[ref.ID] = image
		}
	}
	return shouldRetry, kerrors.NewAggregate(errlist)
}

// importTag import single tag from given ImageStream. Returns retrieved image (for later reuse),
// a flag saying if we should retry imports and an error if one occurs.
func (c *ImportController) importTag(stream *api.ImageStream, tag string, ref api.DockerImageReference, dockerImage *dockerregistry.Image, client dockerregistry.Client, insecure bool) (*dockerregistry.Image, bool, error) {
	glog.V(5).Infof("Importing tag %s from %s/%s...", tag, stream.Namespace, stream.Name)
	if dockerImage == nil {
		// TODO insecure applies to the stream's spec.dockerImageRepository, not necessarily to an external one!
		conn, err := client.Connect(ref.Registry, insecure)
		if err != nil {
			// retry-able error no. 3
			return nil, true, err
		}
		if len(ref.ID) > 0 {
			dockerImage, err = conn.ImageByID(ref.Namespace, ref.Name, ref.ID)
		} else {
			dockerImage, err = conn.ImageByTag(ref.Namespace, ref.Name, ref.Tag)
		}
		switch {
		case dockerregistry.IsRepositoryNotFound(err), dockerregistry.IsRegistryNotFound(err), dockerregistry.IsImageNotFound(err), dockerregistry.IsTagNotFound(err):
			return nil, false, err
		case err != nil:
			// retry-able error no. 4
			return nil, true, err
		}
	}
	var image api.DockerImage
	if err := kapi.Scheme.Convert(&dockerImage.Image, &image); err != nil {
		return nil, false, fmt.Errorf("could not convert image: %#v", err)
	}

	// prefer to pull by ID always
	if dockerImage.PullByID {
		// if the registry indicates the image is pullable by ID, clear the tag
		ref.Tag = ""
		ref.ID = dockerImage.ID
	}

	mapping := &api.ImageStreamMapping{
		ObjectMeta: kapi.ObjectMeta{
			Name:      stream.Name,
			Namespace: stream.Namespace,
		},
		Tag: tag,
		Image: api.Image{
			ObjectMeta: kapi.ObjectMeta{
				Name: dockerImage.ID,
			},
			DockerImageReference: ref.String(),
			DockerImageMetadata:  image,
		},
	}
	if err := c.mappings.ImageStreamMappings(stream.Namespace).Create(mapping); err != nil {
		// retry-able no. 5
		return nil, true, err
	}
	return dockerImage, false, nil
}

// done marks the stream as being processed due to an error or failure condition.
func (c *ImportController) done(stream *api.ImageStream, reason string) error {
	if len(reason) == 0 {
		reason = unversioned.Now().UTC().Format(time.RFC3339)
	} else if len(reason) > 300 {
		// cut down the reason up to 300 characters max.
		reason = reason[:300]
	}
	if stream.Annotations == nil {
		stream.Annotations = make(map[string]string)
	}
	stream.Annotations[api.DockerImageRepositoryCheckAnnotation] = reason
	if _, err := c.streams.ImageStreams(stream.Namespace).Update(stream); err != nil {
		return err
	}
	return nil
}
