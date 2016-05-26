package imagechange

import (
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/client/record"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kcontroller "k8s.io/kubernetes/pkg/controller"
	"k8s.io/kubernetes/pkg/controller/framework"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/flowcontrol"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/golang/glog"
	osclient "github.com/openshift/origin/pkg/client"
	controller "github.com/openshift/origin/pkg/controller"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// ImageChangeControllerFactory can create an ImageChangeController which
// watches all ImageStream changes.
type ImageChangeControllerFactory struct {
	// Client is an OpenShift client.
	Client osclient.Interface
}

// Create creates an ImageChangeController.
func (factory *ImageChangeControllerFactory) Create() controller.RunnableController {
	imageStreamLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return factory.Client.ImageStreams(kapi.NamespaceAll).List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return factory.Client.ImageStreams(kapi.NamespaceAll).Watch(options)
		},
	}
	queue := cache.NewFIFO(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(imageStreamLW, &imageapi.ImageStream{}, queue, 2*time.Minute).Run()

	deploymentConfigLW := &cache.ListWatch{
		ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).List(options)
		},
		WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
			return factory.Client.DeploymentConfigs(kapi.NamespaceAll).Watch(options)
		},
	}
	store := cache.NewStore(cache.MetaNamespaceKeyFunc)
	cache.NewReflector(deploymentConfigLW, &deployapi.DeploymentConfig{}, store, 2*time.Minute).Run()

	return &controller.RetryController{
		Queue: queue,
		RetryManager: controller.NewQueueRetryManager(
			queue,
			cache.MetaNamespaceKeyFunc,
			func(obj interface{}, err error, retries controller.Retry) bool {
				utilruntime.HandleError(err)
				if _, isFatal := err.(fatalError); isFatal {
					return false
				}
				if retries.Count > 0 {
					return false
				}
				return true
			},
			flowcontrol.NewTokenBucketRateLimiter(1, 10),
		),
	}
}

const (
	// FullImageStreamResyncPeriod means we'll attempt to reconcile image streams
	// every two minutes.
	FullImageStreamResyncPeriod = 2 * time.Minute
)

// NewImageChangeController creates a new ImageChangeController.
func NewImageChangeController(oc osclient.Interface, kc kclient.Interface) *ImageChangeController {
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartRecordingToSink(kc.Events(""))
	recorder := eventBroadcaster.NewRecorder(kapi.EventSource{Component: "imagechange-controller"})

	c := &ImageChangeController{
		queue: workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),

		dn: oc,

		recorder: recorder,
	}

	c.streamStore.Indexer, c.streamController = framework.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return oc.ImageStreams(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return oc.ImageStreams(kapi.NamespaceAll).Watch(options)
			},
		},
		&imageapi.ImageStream{},
		FullImageStreamResyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc:    c.addImageStream,
			UpdateFunc: c.updateImageStream,
		},
		// TODO: Use cache.NamespaceIndex once the rebase lands
		cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc},
	)

	// TODO: Use SharedIndexInformer once the rebase lands
	c.dcStore.Indexer, c.dcController = framework.NewIndexerInformer(
		&cache.ListWatch{
			ListFunc: func(options kapi.ListOptions) (runtime.Object, error) {
				return oc.DeploymentConfigs(kapi.NamespaceAll).List(options)
			},
			WatchFunc: func(options kapi.ListOptions) (watch.Interface, error) {
				return oc.DeploymentConfigs(kapi.NamespaceAll).Watch(options)
			},
		},
		&deployapi.DeploymentConfig{},
		FullImageStreamResyncPeriod,
		framework.ResourceEventHandlerFuncs{
			AddFunc: c.addDeploymentConfig,
		},
		// TODO: Use cache.NamespaceIndex once the rebase lands
		cache.Indexers{"namespace": cache.MetaNamespaceIndexFunc},
	)

	return c
}

// Run begins watching and syncing.
func (c *ImageChangeController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	go c.streamController.Run(stopCh)
	go c.dcController.Run(stopCh)
	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}
	<-stopCh
	glog.Infof("Shutting down imagechange controller")
	c.queue.ShutDown()
}

func (c *ImageChangeController) addImageStream(obj interface{}) {
	stream := obj.(*imageapi.ImageStream)
	glog.V(4).Infof("Adding image stream %q", stream.Name)
	c.enqueueImageStream(stream)
}

func (c *ImageChangeController) updateImageStream(old, cur interface{}) {
	stream := cur.(*imageapi.ImageStream)
	glog.V(4).Infof("Updating image stream %q", stream.Name)
	c.enqueueImageStream(stream)
}

// addDeploymentConfig ensures that when a new deployment config is added, any existing image streams
// that are referenced by this deployment config will be reconciled.
func (c *ImageChangeController) addDeploymentConfig(obj interface{}) {
	dc := obj.(*deployapi.DeploymentConfig)
	streams, err := c.streamStore.GetStreamsForDeploymentConfig(dc)
	if err != nil {
		glog.Infof(err.Error())
		return
	}
	for i := range streams {
		stream := streams[i]
		glog.V(4).Infof("Enqueueing image stream %q for new deployment config %q", imageapi.LabelForStream(stream), deployutil.LabelForDeploymentConfig(dc))
		c.enqueueImageStream(stream)
	}
}

func (c *ImageChangeController) enqueueImageStream(stream *imageapi.ImageStream) {
	key, err := kcontroller.KeyFunc(stream)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", stream, err)
		return
	}
	c.queue.Add(key)
}

func (c *ImageChangeController) worker() {
	for {
		if quit := c.work(); quit {
			return
		}
	}
}

func (c *ImageChangeController) work() bool {
	key, quit := c.queue.Get()
	if quit {
		return true
	}
	defer c.queue.Done(key)

	stream, err := c.getByKey(key.(string))
	if err != nil {
		glog.Error(err.Error())
	}

	if stream == nil {
		return false
	}

	if err := c.Handle(stream); err != nil {
		utilruntime.HandleError(err)
	}
	return false
}

func (c *ImageChangeController) getByKey(key string) (*imageapi.ImageStream, error) {
	obj, exists, err := c.streamStore.Indexer.GetByKey(key)
	if err != nil {
		glog.Infof("Unable to retrieve image stream %q from store: %v", key, err)
		c.queue.Add(key)
		return nil, err
	}
	if !exists {
		glog.Infof("Image stream %q has been deleted", key)
		return nil, nil
	}
	return obj.(*imageapi.ImageStream), nil
}
