package signature

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/kubernetes/pkg/controller"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	informers "github.com/openshift/origin/pkg/image/generated/informers/internalversion/image/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset"
	imagelister "github.com/openshift/origin/pkg/image/generated/listers/image/internalversion"
)

type SignatureDownloader interface {
	DownloadImageSignatures(*imageapi.Image) ([]imageapi.ImageSignature, error)
}

type SignatureImportController struct {
	imageClient imageclient.Interface
	imageLister imagelister.ImageLister

	imageHasSynced cache.InformerSynced

	queue workqueue.RateLimitingInterface

	// signatureImportLimit limits amount of signatures we will import.
	// By default this is set to 3 signatures.
	signatureImportLimit int

	fetcher SignatureDownloader
}

func NewSignatureImportController(ctx context.Context, imageClient imageclient.Interface, imageInformer informers.ImageInformer, resyncInterval, fetchTimeout time.Duration, limit int) *SignatureImportController {
	controller := &SignatureImportController{
		queue:                workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		imageClient:          imageClient,
		imageLister:          imageInformer.Lister(),
		imageHasSynced:       imageInformer.Informer().HasSynced,
		signatureImportLimit: limit,
	}
	controller.fetcher = NewContainerImageSignatureDownloader(ctx, fetchTimeout)

	imageInformer.Informer().AddEventHandlerWithResyncPeriod(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			image := obj.(*imageapi.Image)
			glog.V(4).Infof("Adding image %s", image.Name)
			controller.enqueueImage(obj)
		},
		UpdateFunc: func(old, cur interface{}) {
			image := cur.(*imageapi.Image)
			glog.V(4).Infof("Updating image %s", image.Name)
			controller.enqueueImage(cur)
		},
	}, resyncInterval)

	return controller
}

func (s *SignatureImportController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer s.queue.ShutDown()

	if !cache.WaitForCacheSync(stopCh, s.imageHasSynced) {
		return
	}

	glog.V(5).Infof("Starting workers")
	for i := 0; i < workers; i++ {
		go wait.Until(s.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Infof("Shutting down")

}

func (s *SignatureImportController) worker() {
	for {
		if !s.work() {
			return
		}
	}
}

// work returns true if the worker thread should continue
func (s *SignatureImportController) work() bool {
	key, quit := s.queue.Get()
	if quit {
		return false
	}
	defer s.queue.Done(key)

	err := s.syncImageSignatures(key.(string))
	if err != nil {
		if _, ok := err.(GetSignaturesError); !ok {
			utilruntime.HandleError(fmt.Errorf("error syncing image %s, it will be retried: %v", key.(string), err))
		}

		if s.queue.NumRequeues(key) < 5 {
			s.queue.AddRateLimited(key)
		}
		return true
	}

	s.queue.Forget(key)
	return true
}

func (s *SignatureImportController) enqueueImage(obj interface{}) {
	_, ok := obj.(*imageapi.Image)
	if !ok {
		return
	}
	key, err := controller.KeyFunc(obj)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", obj, err)
		return
	}
	s.queue.Add(key)
}

func (s *SignatureImportController) syncImageSignatures(key string) error {
	glog.V(4).Infof("Initiating download of signatures for %s", key)
	image, err := s.imageLister.Get(key)
	if err != nil {
		glog.V(4).Infof("Unable to get image %v: %v", key, err)
		return err
	}

	if image.Annotations[imageapi.ManagedByOpenShiftAnnotation] == "true" {
		glog.V(4).Infof("Skipping downloading signatures for image %s because it's a managed image", image.Name)
		return nil
	}

	currentSignatures, err := s.fetcher.DownloadImageSignatures(image)
	if err != nil {
		glog.V(4).Infof("Failed to fetch image %s signatures: %v", image.Name, err)
		return err
	}

	// Having no signatures means no-op (we don't remove stored signatures when
	// the sig-store no longer have them).
	if len(currentSignatures) == 0 {
		glog.V(4).Infof("No signatures dowloaded for %s", image.Name)
		return nil
	}

	newImage := image.DeepCopy()
	shouldUpdate := false

	// Only add new signatures, do not override existing stored signatures as that
	// can void their verification status.
	for _, c := range currentSignatures {
		found := false
		for _, s := range newImage.Signatures {
			if s.Name == c.Name {
				found = true
				break
			}
		}
		if !found {
			newImage.Signatures = append(newImage.Signatures, c)
			shouldUpdate = true
		}
	}

	if len(newImage.Signatures) > s.signatureImportLimit {
		glog.V(2).Infof("Image %s reached signature limit (max:%d, want:%d)", newImage.Name, s.signatureImportLimit, len(newImage.Signatures))
		return nil
	}

	// Avoid unnecessary updates to images.
	if !shouldUpdate {
		return nil
	}
	glog.V(4).Infof("Image %s now has %d signatures", newImage.Name, len(newImage.Signatures))

	_, err = s.imageClient.Image().Images().Update(newImage)
	return err
}
