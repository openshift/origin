package openshiftkubeapiserver

import (
	"fmt"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/golang/glog"

	"k8s.io/api/core/v1"
	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	listersv1 "k8s.io/client-go/listers/core/v1"
	restclient "k8s.io/client-go/rest"
	cache "k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

// serviceCABundleRoundTripper creates a new RoundTripper (or uses cachedTransport) for each request with serverName and
// caBundle input into the TLSClientConfig. It is expected that caBundle is kept up to date and cachedTransport
// cleared by the serviceCABundleUpdater controller.
type serviceCABundleRoundTripper struct {
	serverName      string
	caBundle        atomic.Value
	cachedTransport http.RoundTripper
}

func (r *serviceCABundleRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt := r.cachedTransport
	if rt == nil {
		caBundle, ok := r.caBundle.Load().([]byte)
		if !ok || len(caBundle) == 0 {
			return nil, fmt.Errorf("error loading caBundle")
		}
		newRestConfig := &restclient.Config{
			TLSClientConfig: restclient.TLSClientConfig{
				ServerName: r.serverName,
				CAData:     caBundle,
			},
		}
		var err error
		rt, err = restclient.TransportFor(newRestConfig)
		if err != nil {
			return nil, err
		}
		r.cachedTransport = rt
	}
	return rt.RoundTrip(req)
}

// These must align with the service-ca operator configuration
const (
	caBundleDataKey              = "cabundle.crt"
	serviceCABundleNamespace     = "openshift-service-cert-signer"
	serviceCABundleConfigMapName = "signing-cabundle"
)

// serviceCABundleUpdater runs a simple controller to keep rt.caBundle updated with CAs from the service-ca controller.
type serviceCABundleUpdater struct {
	// Initial CA bundle that CA updates are tacked on to.
	startingHandlerCA []byte
	// RoundTripper that utilizes the updated CA bundle.
	rt *serviceCABundleRoundTripper

	lister      listersv1.ConfigMapLister
	queue       workqueue.RateLimitingInterface
	hasSynced   cache.InformerSynced
	syncHandler func(serviceKey string) error
}

func (u *serviceCABundleUpdater) isServiceCABundleConfigMap(obj interface{}) bool {
	configMap, ok := obj.(*v1.ConfigMap)
	if !ok {
		return false
	}
	return configMap.Namespace == serviceCABundleNamespace && configMap.Name == serviceCABundleConfigMapName
}

// addCABundle is the informer's AddFunc.
func (u *serviceCABundleUpdater) addCABundle(obj interface{}) {
	cm, ok := obj.(*v1.ConfigMap)
	if !ok {
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(cm)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", cm, err)
		return
	}

	glog.V(4).Infof("serviceCABundleUpdater controller: queuing an add of %v", key)
	u.queue.Add(key)
}

// updateCABundle is the informer's UpdateFunc.
func (u *serviceCABundleUpdater) updateCABundle(old, cur interface{}) {
	cm, ok := cur.(*v1.ConfigMap)
	if !ok {
		return
	}

	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(cm)
	if err != nil {
		glog.Errorf("Couldn't get key for object %+v: %v", cm, err)
		return
	}

	glog.V(4).Infof("serviceCABundleUpdater controller: queuing an update of %v", key)
	u.queue.Add(key)
}

// processNextWorkItem processes the queued items.
func (u *serviceCABundleUpdater) processNextWorkItem() bool {
	key, quit := u.queue.Get()
	if quit {
		return false
	}
	defer u.queue.Done(key)

	err := u.syncHandler(key.(string))
	if err == nil {
		u.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", key, err))
	u.queue.AddRateLimited(key)

	return true
}

// Run runs the controller until stopCh is closed.
func (u *serviceCABundleUpdater) Run(stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer u.queue.ShutDown()
	glog.V(2).Infof("starting serviceCABundleUpdater controller")

	if !cache.WaitForCacheSync(stopCh, u.hasSynced) {
		return
	}

	go wait.Until(u.runWorker, time.Second, stopCh)
	<-stopCh
	glog.V(2).Infof("stopping serviceCABundleUpdater controller")
}

func (u *serviceCABundleUpdater) runWorker() {
	for u.processNextWorkItem() {
	}
}

// Updates the RoundTripper CA bundle and clears the cache for the next request.
func (u *serviceCABundleUpdater) updateRoundTripper(caBundle []byte) {
	u.rt.caBundle.Store(caBundle)
	u.rt.cachedTransport = nil
}

func (u *serviceCABundleUpdater) getRoundTripperCABundle() interface{} {
	return u.rt.caBundle.Load()
}

// syncCABundle will update the RoundTripper's CA bundle by combining the starting CA with the updated CA from the
// service CA configMap.
func (u *serviceCABundleUpdater) syncCABundle(key string) error {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		return err
	}

	sharedConfigMap, err := u.lister.ConfigMaps(namespace).Get(name)
	if kapierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	data, ok := sharedConfigMap.Data[caBundleDataKey]
	if !ok {
		return nil
	}

	combinedCA := make([]byte, len(u.startingHandlerCA))
	copy(combinedCA, u.startingHandlerCA)
	combinedCA = append(combinedCA, data...)

	curBundle, ok := u.getRoundTripperCABundle().([]byte)
	if !ok {
		return fmt.Errorf("error loading caBundle")
	}

	if string(curBundle) == string(combinedCA) {
		return nil
	}

	u.updateRoundTripper(combinedCA)

	glog.V(4).Infof("serviceCABundleUpdater controller: updated proxy transport CA bundle")
	return nil
}

// NewServiceCABundleUpdater creates a new serviceCABundleUpdater controller.
func NewServiceCABundleUpdater(kubeInformers informers.SharedInformerFactory, serverName string, caBundle []byte) (*serviceCABundleUpdater, error) {
	initialBundle := atomic.Value{}
	initialBundle.Store(caBundle)

	roundTripper := &serviceCABundleRoundTripper{
		serverName:      serverName,
		caBundle:        initialBundle,
		cachedTransport: nil,
	}

	updater := &serviceCABundleUpdater{
		rt:                roundTripper,
		lister:            kubeInformers.Core().V1().ConfigMaps().Lister(),
		startingHandlerCA: caBundle,
		queue:             workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
	}

	kubeInformers.Core().V1().ConfigMaps().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			Handler: &cache.ResourceEventHandlerFuncs{
				AddFunc:    updater.addCABundle,
				UpdateFunc: updater.updateCABundle,
			},
			FilterFunc: updater.isServiceCABundleConfigMap,
		},
	)

	updater.hasSynced = kubeInformers.Core().V1().ConfigMaps().Informer().HasSynced
	updater.syncHandler = updater.syncCABundle
	return updater, nil
}
