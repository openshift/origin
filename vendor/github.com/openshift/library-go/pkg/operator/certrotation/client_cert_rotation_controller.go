package certrotation

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/user"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/library-go/pkg/operator/events"
)

const (
	// CertificateExpiryAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateExpiryAnnotation = "auth.openshift.io/certificate-expiry-date"
	// CertificateSignedBy contains the common name of the certificate that signed another certificate.
	CertificateSignedBy = "auth.openshift.io/certificate-signed-by"
)

const workQueueKey = "key"

type CertRotationController struct {
	name string

	signingNamespace             string
	signingCertKeyPairValidity   time.Duration
	newSigningPercentage         float32
	signingCertKeyPairSecretName string
	signingLister                corev1listers.SecretLister

	caBundleNamespace     string
	caBundleConfigMapName string
	caBundleLister        corev1listers.ConfigMapLister

	targetNamespace                     string
	targetCertKeyPairValidity           time.Duration
	newTargetPercentage                 float32
	targetCertKeyPairSecretName         string
	targetServingHostnames              []string
	targetServingCertificateExtensionFn []crypto.CertificateExtensionFunc
	targetUserInfo                      user.Info
	targetLister                        corev1listers.SecretLister

	cachesSynced []cache.InformerSynced

	configmapsClient corev1client.ConfigMapsGetter
	secretsClient    corev1client.SecretsGetter
	eventRecorder    events.Recorder

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func newCertRotationController(
	name string,
	signingNamespace string,
	signingCertKeyPairValidity time.Duration,
	newSigningPercentage float32,
	signingCertKeyPairSecretName string,
	caBundleNamespace string,
	caBundleConfigMapName string,
	targetNamespace string,
	targetCertKeyPairValidity time.Duration,
	newTargetPercentage float32,
	targetCertKeyPairSecretName string,

	signingInformer corev1informers.SecretInformer,
	cabundleInformer corev1informers.ConfigMapInformer,
	targetInformer corev1informers.SecretInformer,

	configmapsClient corev1client.ConfigMapsGetter,
	secretsClient corev1client.SecretsGetter,
	eventRecorder events.Recorder,
) *CertRotationController {
	ret := &CertRotationController{
		signingNamespace:             signingNamespace,
		signingCertKeyPairValidity:   signingCertKeyPairValidity,
		newSigningPercentage:         newSigningPercentage,
		signingCertKeyPairSecretName: signingCertKeyPairSecretName,
		signingLister:                signingInformer.Lister(),

		caBundleNamespace:     caBundleNamespace,
		caBundleConfigMapName: caBundleConfigMapName,
		caBundleLister:        cabundleInformer.Lister(),

		targetNamespace:             targetNamespace,
		targetCertKeyPairValidity:   targetCertKeyPairValidity,
		newTargetPercentage:         newTargetPercentage,
		targetCertKeyPairSecretName: targetCertKeyPairSecretName,
		targetLister:                targetInformer.Lister(),

		cachesSynced: []cache.InformerSynced{
			signingInformer.Informer().HasSynced,
			cabundleInformer.Informer().HasSynced,
			targetInformer.Informer().HasSynced,
		},

		configmapsClient: configmapsClient,
		secretsClient:    secretsClient,
		eventRecorder:    eventRecorder,

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}

	signingInformer.Informer().AddEventHandler(ret.eventHandler())
	cabundleInformer.Informer().AddEventHandler(ret.eventHandler())
	targetInformer.Informer().AddEventHandler(ret.eventHandler())

	return ret
}

func NewClientCertRotationController(
	name string,
	signingNamespace string,
	signingCertKeyPairValidity time.Duration,
	newSigningPercentage float32,
	signingCertKeyPairSecretName string,
	caBundleNamespace string,
	caBundleConfigMapName string,
	targetNamespace string,
	targetCertKeyPairValidity time.Duration,
	newTargetPercentage float32,
	targetCertKeyPairSecretName string,
	targetUserInfo user.Info,

	signingInformer corev1informers.SecretInformer,
	cabundleInformer corev1informers.ConfigMapInformer,
	targetInformer corev1informers.SecretInformer,

	configmapsClient corev1client.ConfigMapsGetter,
	secretsClient corev1client.SecretsGetter,
	eventRecorder events.Recorder,
) *CertRotationController {
	ret := newCertRotationController(
		name,
		signingNamespace,
		signingCertKeyPairValidity,
		newSigningPercentage,
		signingCertKeyPairSecretName,
		caBundleNamespace,
		caBundleConfigMapName,
		targetNamespace,
		targetCertKeyPairValidity,
		newTargetPercentage,
		targetCertKeyPairSecretName,

		signingInformer,
		cabundleInformer,
		targetInformer,

		configmapsClient,
		secretsClient,
		eventRecorder,
	)
	ret.targetUserInfo = targetUserInfo

	return ret
}

func NewServingCertRotationController(
	name string,
	signingNamespace string,
	signingCertKeyPairValidity time.Duration,
	newSigningPercentage float32,
	signingCertKeyPairSecretName string,
	caBundleNamespace string,
	caBundleConfigMapName string,
	targetNamespace string,
	targetCertKeyPairValidity time.Duration,
	newTargetPercentage float32,
	targetCertKeyPairSecretName string,
	targetServingHostnames []string,
	targetServingCertificateExtensionFn []crypto.CertificateExtensionFunc,

	signingInformer corev1informers.SecretInformer,
	cabundleInformer corev1informers.ConfigMapInformer,
	targetInformer corev1informers.SecretInformer,

	configmapsClient corev1client.ConfigMapsGetter,
	secretsClient corev1client.SecretsGetter,
	eventRecorder events.Recorder,
) *CertRotationController {
	ret := newCertRotationController(
		name,
		signingNamespace,
		signingCertKeyPairValidity,
		newSigningPercentage,
		signingCertKeyPairSecretName,
		caBundleNamespace,
		caBundleConfigMapName,
		targetNamespace,
		targetCertKeyPairValidity,
		newTargetPercentage,
		targetCertKeyPairSecretName,

		signingInformer,
		cabundleInformer,
		targetInformer,

		configmapsClient,
		secretsClient,
		eventRecorder,
	)
	ret.targetServingHostnames = targetServingHostnames
	ret.targetServingCertificateExtensionFn = targetServingCertificateExtensionFn

	return ret
}

func (c CertRotationController) sync() error {
	signingCertKeyPair, err := c.ensureSigningCertKeyPair()
	if err != nil {
		return err
	}

	if err := c.ensureConfigMapCABundle(signingCertKeyPair); err != nil {
		return err
	}

	if err := c.ensureTargetCertKeyPair(signingCertKeyPair); err != nil {
		return err
	}

	return nil
}

func needNewCertKeyPairForTime(annotations map[string]string, validity time.Duration, renewalPercentage float32) bool {
	signingCertKeyPairExpiry := annotations[CertificateExpiryAnnotation]
	if len(signingCertKeyPairExpiry) == 0 {
		return true
	}
	certExpiry, err := time.Parse(time.RFC3339, signingCertKeyPairExpiry)
	if err != nil {
		glog.Infof("bad expiry: %q", signingCertKeyPairExpiry)
		// just create a new one
		return true
	}

	// If Certificate is not-expired, skip this iteration.
	renewalDuration := -1 * float32(validity) * (1 - renewalPercentage)
	if certExpiry.Add(time.Duration(renewalDuration)).After(time.Now()) {
		return false
	}

	return true
}

func (c *CertRotationController) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	glog.Infof("Starting CertRotationController - %q", c.name)
	defer glog.Infof("Shutting down CertRotationController - %q", c.name)

	if !cache.WaitForCacheSync(stopCh, c.cachesSynced...) {
		utilruntime.HandleError(fmt.Errorf("caches did not sync"))
		return
	}

	// doesn't matter what workers say, only start one.
	go wait.Until(c.runWorker, time.Second, stopCh)

	<-stopCh
}

func (c *CertRotationController) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *CertRotationController) processNextWorkItem() bool {
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

// eventHandler queues the operator to check spec and status
func (c *CertRotationController) eventHandler() cache.ResourceEventHandler {
	return cache.ResourceEventHandlerFuncs{
		AddFunc:    func(obj interface{}) { c.queue.Add(workQueueKey) },
		UpdateFunc: func(old, new interface{}) { c.queue.Add(workQueueKey) },
		DeleteFunc: func(obj interface{}) { c.queue.Add(workQueueKey) },
	}
}
