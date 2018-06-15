package controller

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang/glog"

	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/typed/image/internalversion"
	imageinformer "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

var ErrNotImportable = errors.New("requested image cannot be imported")

// Notifier provides information about when the controller makes a decision
type Notifier interface {
	// Importing is invoked when the controller is going to import an image stream
	Importing(stream *imageapi.ImageStream)
}

type ImageStreamController struct {
	// image stream client
	client imageclient.ImageInterface

	// queue contains replication controllers that need to be synced.
	queue workqueue.RateLimitingInterface

	syncHandler func(isKey string) error

	// lister can list/get image streams from a shared informer's cache
	lister imageinformer.ImageStreamLister
	// listerSynced makes sure the is store is synced before reconciling streams
	listerSynced cache.InformerSynced

	// notifier informs other controllers that an import is being performed
	notifier Notifier
}

func (c *ImageStreamController) SetNotifier(n Notifier) {
	c.notifier = n
}

// Run begins watching and syncing.
func (c *ImageStreamController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting image stream controller")

	// Wait for the stream store to sync before starting any work in this controller.
	if !cache.WaitForCacheSync(stopCh, c.listerSynced) {
		utilruntime.HandleError(fmt.Errorf("timed out waiting for caches to sync"))
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
	glog.Infof("Shutting down image stream controller")
}

func (c *ImageStreamController) addImageStream(obj interface{}) {
	if stream, ok := obj.(*imageapi.ImageStream); ok {
		c.enqueueImageStream(stream)
	}
}

func (c *ImageStreamController) updateImageStream(old, cur interface{}) {
	curStream, ok := cur.(*imageapi.ImageStream)
	if !ok {
		return
	}
	oldStream, ok := old.(*imageapi.ImageStream)
	if !ok {
		return
	}
	// we only compare resource version, since deeper inspection if a stream
	// needs to be re-imported happens in syncImageStream
	//
	// FIXME: this will only be ever true on cache resync
	if curStream.ResourceVersion == oldStream.ResourceVersion {
		return
	}
	c.enqueueImageStream(curStream)
}

func (c *ImageStreamController) enqueueImageStream(stream *imageapi.ImageStream) {
	key, err := kcontroller.KeyFunc(stream)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("Couldn't get key for image stream %#v: %v", stream, err))
		return
	}
	c.queue.Add(key)
}

func (c *ImageStreamController) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *ImageStreamController) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.syncHandler(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("Error syncing image stream %q: %v", key, err))
	c.queue.AddRateLimited(key)

	return true
}

func (c *ImageStreamController) syncImageStream(key string) error {
	startTime := time.Now()
	defer func() {
		glog.V(4).Infof("Finished syncing image stream %q (%v)", key, time.Since(startTime))
	}()

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}
	stream, err := c.lister.ImageStreams(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		glog.V(4).Infof("ImageStream has been deleted: %v", key)
		return nil
	}
	if err != nil {
		return err
	}

	glog.V(3).Infof("Queued import of stream %s/%s...", stream.Namespace, stream.Name)
	return handleImageStream(stream, c.client, c.notifier)
}

// tagImportable is true if the given TagReference is importable by this controller
func tagImportable(tagRef imageapi.TagReference) bool {
	return !(tagRef.From == nil || tagRef.From.Kind != "DockerImage" || tagRef.Reference)
}

// tagNeedsImport is true if the observed tag generation for this tag is older than the
// specified tag generation (if no tag generation is specified, the controller does not
// need to import this tag).
func tagNeedsImport(stream *imageapi.ImageStream, tag string, tagRef imageapi.TagReference, importWhenGenerationNil bool) bool {
	if !tagImportable(tagRef) {
		return false
	}
	if tagRef.Generation == nil {
		return importWhenGenerationNil
	}
	return *tagRef.Generation > imageapi.LatestObservedTagGeneration(stream, tag)
}

// needsImport returns true if the provided image stream should have tags imported. Partial is returned
// as true if the spec.dockerImageRepository does not need to be refreshed (if only tags have to be imported).
func needsImport(stream *imageapi.ImageStream) (ok bool, partial bool) {
	if stream.Annotations == nil || len(stream.Annotations[imageapi.DockerImageRepositoryCheckAnnotation]) == 0 {
		if len(stream.Spec.DockerImageRepository) > 0 {
			return true, false
		}
		// for backwards compatibility, if any of our tags are importable, trigger a partial import when the
		// annotation is cleared.
		for _, tagRef := range stream.Spec.Tags {
			if tagImportable(tagRef) {
				return true, true
			}
		}
	}
	// find any tags with a newer generation than their status
	for tag, tagRef := range stream.Spec.Tags {
		if tagNeedsImport(stream, tag, tagRef, false) {
			return true, true
		}
	}
	return false, false
}

// Processes the given image stream, looking for streams that have DockerImageRepository
// set but have not yet been marked as "ready". If transient errors occur, err is returned but
// the image stream is not modified (so it will be tried again later). If a permanent
// failure occurs the image is marked with an annotation and conditions are set on the status
// tags. The tags of the original spec image are left as is (those are updated through status).
//
// There are 3 scenarios:
//
// 1. spec.DockerImageRepository defined without any tags results in all tags being imported
//    from upstream image repository
//
// 2. spec.DockerImageRepository + tags defined - import all tags from upstream image repository,
//    and all the specified which (if name matches) will overwrite the default ones.
//    Additionally:
//    for kind == DockerImage import or reference underlying image, exact tag (not provided means latest),
//    for kind != DockerImage reference tag from the same or other ImageStream
//
// 3. spec.DockerImageRepository not defined - import tags per each definition.
//
// Notifier, if passed, will be invoked if the stream is going to be imported.
func handleImageStream(stream *imageapi.ImageStream, client imageclient.ImageInterface, notifier Notifier) error {
	ok, partial := needsImport(stream)
	if !ok {
		return nil
	}
	glog.V(3).Infof("Importing stream %s/%s partial=%t...", stream.Namespace, stream.Name, partial)

	if notifier != nil {
		notifier.Importing(stream)
	}

	isi := &imageapi.ImageStreamImport{
		ObjectMeta: metav1.ObjectMeta{
			Name:            stream.Name,
			Namespace:       stream.Namespace,
			ResourceVersion: stream.ResourceVersion,
			UID:             stream.UID,
		},
		Spec: imageapi.ImageStreamImportSpec{Import: true},
	}
	for tag, tagRef := range stream.Spec.Tags {
		if tagImportable(tagRef) &&
			(tagNeedsImport(stream, tag, tagRef, true) || !partial) {
			isi.Spec.Images = append(isi.Spec.Images, imageapi.ImageImportSpec{
				From:            kapi.ObjectReference{Kind: "DockerImage", Name: tagRef.From.Name},
				To:              &kapi.LocalObjectReference{Name: tag},
				ImportPolicy:    tagRef.ImportPolicy,
				ReferencePolicy: tagRef.ReferencePolicy,
			})
		}
	}
	if repo := stream.Spec.DockerImageRepository; !partial && len(repo) > 0 {
		insecure := stream.Annotations[imageapi.InsecureRepositoryAnnotation] == "true"
		isi.Spec.Repository = &imageapi.RepositoryImportSpec{
			From:         kapi.ObjectReference{Kind: "DockerImage", Name: repo},
			ImportPolicy: imageapi.TagImportPolicy{Insecure: insecure},
		}
	}
	result, err := client.ImageStreamImports(stream.Namespace).Create(isi)
	if err != nil {
		if apierrs.IsNotFound(err) && isStatusErrorKind(err, "imageStream") {
			return ErrNotImportable
		}
		glog.V(4).Infof("Import stream %s/%s partial=%t error: %v", stream.Namespace, stream.Name, partial, err)
	} else {
		glog.V(5).Infof("Import stream %s/%s partial=%t import: %#v", stream.Namespace, stream.Name, partial, result.Status.Import)
	}
	return err
}

// isStatusErrorKind returns true if this error describes the provided kind.
func isStatusErrorKind(err error, kind string) bool {
	if s, ok := err.(apierrs.APIStatus); ok {
		if details := s.Status().Details; details != nil {
			return kind == details.Kind
		}
	}
	return false
}
