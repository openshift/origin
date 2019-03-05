package certrotation

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	operatorv1 "github.com/openshift/api/operator/v1"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const (
	// CertificateNotBeforeAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateNotBeforeAnnotation = "auth.openshift.io/certificate-not-before"
	// CertificateNotAfterAnnotation contains the certificate expiration date in RFC3339 format.
	CertificateNotAfterAnnotation = "auth.openshift.io/certificate-not-after"
	// CertificateIssuer contains the common name of the certificate that signed another certificate.
	CertificateIssuer = "auth.openshift.io/certificate-issuer"
	// CertificateHostnames contains the hostnames used by a signer.
	CertificateHostnames = "auth.openshift.io/certificate-hostnames"
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
	OperatorClient   v1helpers.StaticPodOperatorClient

	cachesSynced []cache.InformerSynced

	// queue only ever has one item, but it has nice error handling backoff/retry semantics
	queue workqueue.RateLimitingInterface
}

func NewCertRotationController(
	name string,
	signingRotation SigningRotation,
	caBundleRotation CABundleRotation,
	targetRotation TargetRotation,
	operatorClient v1helpers.StaticPodOperatorClient,
) (*CertRotationController, error) {
	ret := &CertRotationController{
		name: name,

		SigningRotation:  signingRotation,
		CABundleRotation: caBundleRotation,
		TargetRotation:   targetRotation,
		OperatorClient:   operatorClient,

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

	return ret, nil
}

func (c CertRotationController) sync() error {
	syncErr := c.syncWorker()

	condition := operatorv1.OperatorCondition{
		Type:   "CertRotation_" + c.name + "_Failing",
		Status: operatorv1.ConditionFalse,
	}
	if syncErr != nil {
		condition.Status = operatorv1.ConditionTrue
		condition.Reason = "RotationError"
		condition.Message = syncErr.Error()
	}
	if _, _, updateErr := v1helpers.UpdateStaticPodStatus(c.OperatorClient, v1helpers.UpdateStaticPodConditionFn(condition)); updateErr != nil {
		return updateErr
	}

	return syncErr
}

func (c CertRotationController) syncWorker() error {
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

	// start a time based thread to ensure we stay up to date
	go wait.Until(func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()

		for {
			c.queue.Add(workQueueKey)
			select {
			case <-ticker.C:
			case <-stopCh:
				return
			}
		}

	}, time.Minute, stopCh)

	// if we have a need to force rechecking the cert, use this channel to do it.
	if refresher, ok := c.TargetRotation.CertCreator.(TargetCertRechecker); ok {
		targetRefresh := refresher.RecheckChannel()
		go wait.Until(func() {
			for {
				select {
				case <-targetRefresh:
					c.queue.Add(workQueueKey)
				case <-stopCh:
					return
				}
			}

		}, time.Minute, stopCh)
	}

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

	utilruntime.HandleError(fmt.Errorf("%v: %v failed with: %v", c.name, dsKey, err))
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
