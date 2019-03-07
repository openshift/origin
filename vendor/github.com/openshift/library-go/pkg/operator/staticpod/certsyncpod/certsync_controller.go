package certsyncpod

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
)

type CertSyncController struct {
	destinationDir string
	namespace      string
	configMaps     []revision.RevisionResource
	secrets        []revision.RevisionResource

	configMapLister v1.ConfigMapLister
	secretLister    v1.SecretLister
	eventRecorder   events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue        workqueue.RateLimitingInterface
	preRunCaches []cache.InformerSynced
}

func NewCertSyncController(targetDir, targetNamespace string, configmaps, secrets []revision.RevisionResource, informers informers.SharedInformerFactory, eventRecorder events.Recorder) (*CertSyncController, error) {
	c := &CertSyncController{
		destinationDir: targetDir,
		namespace:      targetNamespace,
		configMaps:     configmaps,
		secrets:        secrets,
		eventRecorder:  eventRecorder.WithComponentSuffix("cert-sync-controller"),

		configMapLister: informers.Core().V1().ConfigMaps().Lister(),
		secretLister:    informers.Core().V1().Secrets().Lister(),

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "CertSyncController"),
		preRunCaches: []cache.InformerSynced{
			informers.Core().V1().ConfigMaps().Informer().HasSynced,
			informers.Core().V1().Secrets().Informer().HasSynced,
		},
	}

	informers.Core().V1().ConfigMaps().Informer().AddEventHandler(c.eventHandler())
	informers.Core().V1().Secrets().Informer().AddEventHandler(c.eventHandler())

	return c, nil
}

func getConfigMapDir(targetDir, configMapName string) string {
	return filepath.Join(targetDir, "configmaps", configMapName)
}

func getSecretDir(targetDir, secretName string) string {
	return filepath.Join(targetDir, "secrets", secretName)
}

func (c *CertSyncController) sync() error {
	errors := []error{}

	for _, cm := range c.configMaps {
		configMap, err := c.configMapLister.ConfigMaps(c.namespace).Get(cm.Name)
		switch {
		case apierrors.IsNotFound(err) && !cm.Optional:
			errors = append(errors, err)
			continue
		case apierrors.IsNotFound(err) && cm.Optional:
			// remove missing content
			if err := os.RemoveAll(getConfigMapDir(c.destinationDir, cm.Name)); err != nil {
				errors = append(errors, err)
			}
			continue
		case err != nil:
			errors = append(errors, err)
			continue
		}

		contentDir := getConfigMapDir(c.destinationDir, cm.Name)
		glog.Infof("Creating directory %q ...", contentDir)
		if err := os.MkdirAll(contentDir, 0755); err != nil && !os.IsExist(err) {
			errors = append(errors, err)
			continue
		}
		for filename, content := range configMap.Data {
			fullFilename := filepath.Join(contentDir, filename)
			// if the existing is the same, do nothing
			if existingContent, err := ioutil.ReadFile(fullFilename); err == nil && reflect.DeepEqual(existingContent, []byte(content)) {
				continue
			}

			glog.Infof("Writing configmap manifest %q ...", fullFilename)
			if err := ioutil.WriteFile(fullFilename, []byte(content), 0644); err != nil {
				errors = append(errors, err)
				continue
			}
		}
	}

	for _, s := range c.secrets {
		secret, err := c.secretLister.Secrets(c.namespace).Get(s.Name)
		switch {
		case apierrors.IsNotFound(err) && !s.Optional:
			errors = append(errors, err)
			continue
		case apierrors.IsNotFound(err) && s.Optional:
			// remove missing content
			if err := os.RemoveAll(getSecretDir(c.destinationDir, s.Name)); err != nil {
				errors = append(errors, err)
			}
			continue
		case err != nil:
			errors = append(errors, err)
			continue
		}

		contentDir := getSecretDir(c.destinationDir, s.Name)
		glog.Infof("Creating directory %q ...", contentDir)
		if err := os.MkdirAll(contentDir, 0755); err != nil && !os.IsExist(err) {
			errors = append(errors, err)
			continue
		}
		for filename, content := range secret.Data {
			// TODO fix permissions
			fullFilename := filepath.Join(contentDir, filename)
			// if the existing is the same, do nothing
			if existingContent, err := ioutil.ReadFile(fullFilename); err == nil && reflect.DeepEqual(existingContent, content) {
				continue
			}

			glog.Infof("Writing secret manifest %q ...", fullFilename)
			if err := ioutil.WriteFile(fullFilename, content, 0644); err != nil {
				errors = append(errors, err)
				continue
			}
		}
	}

	return utilerrors.NewAggregate(errors)
}

// Run starts the kube-apiserver and blocks until stopCh is closed.
func (c *CertSyncController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting CertSyncer")
	defer glog.Infof("Shutting down CertSyncer")

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *CertSyncController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *CertSyncController) processNextWorkItem() bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.sync()
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

const workQueueKey = "key"

// eventHandler queues the operator to check spec and status
func (c *CertSyncController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
