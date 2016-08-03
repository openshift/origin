package clusterresourcequota

import (
	"errors"
	"fmt"
	"io"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	oclient "github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func init() {
	admission.RegisterPlugin("ImageBlacklist",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewImageBlacklister()
		})
}

// imageBlacklistAdmission implements an admission controller that can enforce imageStream constraints
type imageBlacklistAdmission struct {
	*admission.Handler

	// these are used to create the accessor
	imageStreamLister *ocache.StoreToImageStreamLister
	imageStreamSynced func() bool

	imagesClient oclient.ImagesInterfacer

	internalRegistryNames sets.String
	blacklistAnnotation   string
}

var _ oadmission.WantsInformers = &imageBlacklistAdmission{}
var _ oadmission.WantsOpenshiftClient = &imageBlacklistAdmission{}
var _ oadmission.Validator = &imageBlacklistAdmission{}

const (
	timeToWaitForCacheSync = 10 * time.Second
)

func NewImageBlacklister() (admission.Interface, error) {
	return &imageBlacklistAdmission{
		Handler:               admission.NewHandler(admission.Create, admission.Update),
		internalRegistryNames: sets.NewString("registry.default.svc", "172.30.106.197"),
		blacklistAnnotation:   "images.openshift.io/deny-execution",
	}, nil
}

// Admit makes admission decisions while enforcing imageStream
func (q *imageBlacklistAdmission) Admit(a admission.Attributes) (err error) {
	if a.GetResource().GroupResource() != kapi.Resource("pods") {
		return nil
	}
	// ignore all operations that correspond to sub-resource actions
	if len(a.GetSubresource()) != 0 {
		return nil
	}

	if !q.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("caches not synchronized"))
	}

	imageRefs, err := getPullSpecsToCheck(a)
	if err != nil {
		return err
	}

	for _, imageRef := range imageRefs {
		imageDigest := imageRef.ID
		// if the pull spec wasn't an ID, then pull the latest ID from the imagestream
		if len(imageDigest) == 0 {
			if !q.internalRegistryNames.Has(imageRef.Registry) {
				continue
			}

			imageStream, err := q.imageStreamLister.ImageStreams(imageRef.Namespace).Get(imageRef.Name)
			if kapierrors.IsNotFound(err) {
				continue
			}
			if err != nil {
				return admission.NewForbidden(a, err)
			}
			tagName := "latest"
			if len(imageRef.Tag) > 0 {
				tagName = imageRef.Tag
			}
			tag, exists := imageStream.Status.Tags[tagName]
			if !exists {
				continue
			}
			if len(tag.Items) == 0 {
				continue
			}
			imageDigest = tag.Items[0].Image
		}

		image, err := q.imagesClient.Images().Get(imageDigest)
		if kapierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return admission.NewForbidden(a, err)
		}
		if image.Annotations[q.blacklistAnnotation] == "true" {
			return admission.NewForbidden(a, fmt.Errorf("image %v has been marked as disallowed", imageRef))
		}
	}

	return nil
}

// getPullSpecsToCheck finds the pull specs we need to check
// for creates, we need to check every pull spec
// for updates, we only need to check the differences
func getPullSpecsToCheck(a admission.Attributes) ([]imageapi.DockerImageReference, error) {
	pod, ok := a.GetObject().(*kapi.Pod)
	if !ok {
		return nil, admission.NewForbidden(a, fmt.Errorf("expected pod, got %T", a.GetObject()))
	}

	pullSpecs := sets.String{}
	for _, container := range pod.Spec.InitContainers {
		pullSpecs.Insert(container.Image)
	}
	for _, container := range pod.Spec.Containers {
		pullSpecs.Insert(container.Image)
	}

	// for updates, we only want the diff
	if a.GetOperation() == admission.Update {
		oldPod, ok := a.GetOldObject().(*kapi.Pod)
		if !ok {
			return nil, admission.NewForbidden(a, fmt.Errorf("expected pod, got %T", a.GetObject()))
		}
		for _, container := range oldPod.Spec.InitContainers {
			pullSpecs.Delete(container.Image)
		}
		for _, container := range oldPod.Spec.Containers {
			pullSpecs.Delete(container.Image)
		}
	}

	ret := []imageapi.DockerImageReference{}
	for pullSpec := range pullSpecs {
		dockerImageRef, err := imageapi.ParseDockerImageReference(pullSpec)
		if err != nil {
			return nil, admission.NewForbidden(a, err)
		}
		ret = append(ret, dockerImageRef)
	}

	return ret, nil
}

func (q *imageBlacklistAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !q.imageStreamSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return q.imageStreamSynced()
		}
	}

	return true
}

func (q *imageBlacklistAdmission) SetInformers(informers shared.InformerFactory) {
	q.imageStreamLister = informers.ImageStreams().Lister()
	q.imageStreamSynced = informers.ImageStreams().Informer().HasSynced
}

func (q *imageBlacklistAdmission) SetOpenshiftClient(client oclient.Interface) {
	q.imagesClient = client
}

func (q *imageBlacklistAdmission) Validate() error {
	if q.imageStreamLister == nil {
		return errors.New("missing imageStreamLister")
	}
	if q.imagesClient == nil {
		return errors.New("missing imagesClient")
	}

	return nil
}
