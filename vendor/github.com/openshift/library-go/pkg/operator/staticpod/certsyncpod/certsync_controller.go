package certsyncpod

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1interface "k8s.io/client-go/kubernetes/typed/core/v1"
	v1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/klog"

	"github.com/openshift/library-go/pkg/controller/factory"
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
}

func NewCertSyncController(targetDir, targetNamespace string, configmaps, secrets []revision.RevisionResource, kubeClient kubernetes.Interface, informers informers.SharedInformerFactory, eventRecorder events.Recorder) factory.Controller {
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
	}

	return factory.New().WithInformers(informers.Core().V1().ConfigMaps().Informer(), informers.Core().V1().Secrets().Informer()).WithSync(c.sync).ToController("CertSyncController", eventRecorder)
}

func getConfigMapDir(targetDir, configMapName string) string {
	return filepath.Join(targetDir, "configmaps", configMapName)
}

func getSecretDir(targetDir, secretName string) string {
	return filepath.Join(targetDir, "secrets", secretName)
}

func (c *CertSyncController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	errors := []error{}

	klog.Infof("Syncing configmaps: %v", c.configMaps)
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
				c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed removing file for configmap: %s/%s: %v", c.namespace, cm.Name, err)
				errors = append(errors, err)
			}
			c.eventRecorder.Eventf("CertificateRemoved", "Removed file for configmap: %s/%s", c.namespace, cm.Name)
			continue

		case err != nil:
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed getting configmap: %s/%s: %v", c.namespace, cm.Name, err)
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
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed getting configmap: %s/%s: %v", c.namespace, cm.Name, err)
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
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed creating directory for configmap: %s/%s: %v", configMap.Namespace, configMap.Name, err)
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
				c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed writing file for configmap: %s/%s: %v", configMap.Namespace, configMap.Name, err)
				errors = append(errors, err)
				continue
			}
		}
		c.eventRecorder.Eventf("CertificateUpdated", "Wrote updated configmap: %s/%s", configMap.Namespace, configMap.Name)
	}

	klog.Infof("Syncing secrets: %v", c.secrets)
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

			// check if the secret file exists, skip firing events if it does not
			secretFile := getSecretDir(c.destinationDir, s.Name)
			if _, err := os.Stat(secretFile); os.IsNotExist(err) {
				continue
			}

			// remove missing content
			if err := os.RemoveAll(secretFile); err != nil {
				c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed removing file for missing secret: %s/%s: %v", c.namespace, s.Name, err)
				errors = append(errors, err)
				continue
			}
			c.eventRecorder.Warningf("CertificateRemoved", "Removed file for missing secret: %s/%s", c.namespace, s.Name)
			continue

		case err != nil:
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed getting secret: %s/%s: %v", c.namespace, s.Name, err)
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
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed getting secret: %s/%s: %v", c.namespace, s.Name, err)
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
			c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed creating directory for secret: %s/%s: %v", secret.Namespace, secret.Name, err)
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
				c.eventRecorder.Warningf("CertificateUpdateFailed", "Failed writing file for secret: %s/%s: %v", secret.Namespace, secret.Name, err)
				errors = append(errors, err)
				continue
			}
		}
		c.eventRecorder.Eventf("CertificateUpdated", "Wrote updated secret: %s/%s", secret.Namespace, secret.Name)
	}

	return utilerrors.NewAggregate(errors)
}
