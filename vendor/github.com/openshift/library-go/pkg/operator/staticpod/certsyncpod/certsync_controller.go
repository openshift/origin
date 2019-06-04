package certsyncpod

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1interface "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/staticpod/controller/revision"
)

type CertSyncController struct {
	destinationDir string
	namespace      string
	configMaps     []revision.RevisionResource
	secrets        []revision.RevisionResource

	configmapGetter corev1interface.ConfigMapInterface
	configMapLister v1.ConfigMapLister
	secretGetter    corev1interface.SecretInterface
	secretLister    v1.SecretLister
	eventRecorder   events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue        workqueue.RateLimitingInterface
	preRunCaches []cache.InformerSynced
}

func NewCertSyncController(targetDir, targetNamespace string, configmaps, secrets []revision.RevisionResource, kubeClient kubernetes.Interface, informers informers.SharedInformerFactory, eventRecorder events.Recorder) (*CertSyncController, error) {
	c := &CertSyncController{
		destinationDir: targetDir,
		namespace:      targetNamespace,
		configMaps:     configmaps,
		secrets:        secrets,
		eventRecorder:  eventRecorder.WithComponentSuffix("cert-sync-controller"),

		configmapGetter: kubeClient.CoreV1().ConfigMaps(targetNamespace),
		configMapLister: informers.Core().V1().ConfigMaps().Lister(),
		secretLister:    informers.Core().V1().Secrets().Lister(),
		secretGetter:    kubeClient.CoreV1().Secrets(targetNamespace),

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
			// Check with the live call it is really missing
			configMap, err = c.configmapGetter.Get(cm.Name, metav1.GetOptions{})
			if err == nil {
				klog.Infof("Caches are stale. They don't see configmap '%s/%s', yet it is present", configMap.Namespace, configMap.Name)
				// We will get re-queued when we observe the change
				continue
			}
			if !apierrors.IsNotFound(err) {
				errors = append(errors, err)
				continue
			}

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

		data := map[string]string{}
		for filename := range configMap.Data {
			fullFilename := filepath.Join(contentDir, filename)

			existingContent, err := ioutil.ReadFile(fullFilename)
			if err != nil {
				if !os.IsNotExist(err) {
					klog.Error(err)
				}
				continue
			}

			data[filename] = string(existingContent)
		}

		// Check if cached configmap differs
		if reflect.DeepEqual(configMap.Data, data) {
			continue
		}

		klog.V(2).Infof("Syncing updated configmap '%s/%s'.", configMap.Namespace, configMap.Name)

		// We need to do a live get here so we don't overwrite a newer file with one from a stale cache
		configMap, err = c.configmapGetter.Get(configMap.Name, metav1.GetOptions{})
		if err != nil {
			// Even if the error is not exists we will act on it when caches catch up
			errors = append(errors, err)
			continue
		}

		// Check if the live configmap differs
		if reflect.DeepEqual(configMap.Data, data) {
			klog.Infof("Caches are stale. The live configmap '%s/%s' is reflected on filesystem, but cached one differs", configMap.Namespace, configMap.Name)
			continue
		}

		klog.Infof("Creating directory %q ...", contentDir)
		if err := os.MkdirAll(contentDir, 0755); err != nil && !os.IsExist(err) {
			errors = append(errors, err)
			continue
		}
		for filename, content := range configMap.Data {
			fullFilename := filepath.Join(contentDir, filename)
			// if the existing is the same, do nothing
			if reflect.DeepEqual(data[fullFilename], content) {
				continue
			}

			klog.Infof("Writing configmap manifest %q ...", fullFilename)
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
			// Check with the live call it is really missing
			secret, err = c.secretGetter.Get(s.Name, metav1.GetOptions{})
			if err == nil {
				klog.Infof("Caches are stale. They don't see secret '%s/%s', yet it is present", secret.Namespace, secret.Name)
				// We will get re-queued when we observe the change
				continue
			}
			if !apierrors.IsNotFound(err) {
				errors = append(errors, err)
				continue
			}

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

		data := map[string][]byte{}
		for filename := range secret.Data {
			fullFilename := filepath.Join(contentDir, filename)

			existingContent, err := ioutil.ReadFile(fullFilename)
			if err != nil {
				if !os.IsNotExist(err) {
					klog.Error(err)
				}
				continue
			}

			data[filename] = existingContent
		}

		// Check if cached secret differs
		if reflect.DeepEqual(secret.Data, data) {
			continue
		}

		klog.V(2).Infof("Syncing updated secret '%s/%s'.", secret.Namespace, secret.Name)

		// We need to do a live get here so we don't overwrite a newer file with one from a stale cache
		secret, err = c.secretGetter.Get(secret.Name, metav1.GetOptions{})
		if err != nil {
			// Even if the error is not exists we will act on it when caches catch up
			errors = append(errors, err)
			continue
		}

		// Check if the live secret differs
		if reflect.DeepEqual(secret.Data, data) {
			klog.Infof("Caches are stale. The live secret '%s/%s' is reflected on filesystem, but cached one differs", secret.Namespace, secret.Name)
			continue
		}

		klog.Infof("Creating directory %q ...", contentDir)
		if err := os.MkdirAll(contentDir, 0755); err != nil && !os.IsExist(err) {
			errors = append(errors, err)
			continue
		}
		for filename, content := range secret.Data {
			// TODO fix permissions
			fullFilename := filepath.Join(contentDir, filename)
			// if the existing is the same, do nothing
			if reflect.DeepEqual(data[fullFilename], content) {
				continue
			}

			klog.Infof("Writing secret manifest %q ...", fullFilename)
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

	klog.Infof("Starting CertSyncer")
	defer klog.Infof("Shutting down CertSyncer")

	if !cache.WaitForCacheSync(stopCh, c.preRunCaches...) {
		klog.Error("failed waiting for caches")
		return
	}
	klog.V(2).Infof("CertSyncer caches synced")

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
