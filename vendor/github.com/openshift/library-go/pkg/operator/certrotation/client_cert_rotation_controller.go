package certrotation

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// CertificateExpiryAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateExpiryAnnotation = "auth.openshift.io/certificate-expiry-date"
	// CertificateSignedBy contains the common name of the certificate that signed another certificate.
	CertificateSignedBy = "auth.openshift.io/certificate-signed-by"
)

const workQueueKey = "key"

// CertRotationController does:
//
// 1) continuously create a self-signed signing CA (via SigningRotation).
//    It creates the next one when a given percentage of the validity of the old CA has passed.
// 2) maintain a CA bundle with all not yet expired CA certs.
// 3) continuously create a target cert and key signed by the latest signing CA
//    It creates the next one when a given percentage of the validity of the previous cert has
//    passed, or when a new CA has been created.
type CertRotationController struct {
	name string

	SigningRotation  SigningRotation
	CABundleRotation CABundleRotation
	TargetRotation   TargetRotation

	cachesSynced []cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewCertRotationController(
	name string,
	signingRotation SigningRotation,
	caBundleRotation CABundleRotation,
	targetRotation TargetRotation,
) *CertRotationController {
	ret := &CertRotationController{
		SigningRotation:  signingRotation,
		CABundleRotation: caBundleRotation,
		TargetRotation:   targetRotation,

		cachesSynced: []cache.InformerSynced{
			signingRotation.Informer.Informer().HasSynced,
			caBundleRotation.Informer.Informer().HasSynced,
			targetRotation.Informer.Informer().HasSynced,
		},

		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), name),
	}

	signingRotation.Informer.Informer().AddEventHandler(ret.eventHandler())
	caBundleRotation.Informer.Informer().AddEventHandler(ret.eventHandler())
	targetRotation.Informer.Informer().AddEventHandler(ret.eventHandler())

	return ret
}

func (c CertRotationController) sync() error {
	signingCertKeyPair, err := c.SigningRotation.ensureSigningCertKeyPair()
	if err != nil {
		return err
	}

	cabundleCerts, err := c.CABundleRotation.ensureConfigMapCABundle(signingCertKeyPair)
	if err != nil {
		return err
	}

	if err := c.TargetRotation.ensureTargetCertKeyPair(signingCertKeyPair, cabundleCerts); err != nil {
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
