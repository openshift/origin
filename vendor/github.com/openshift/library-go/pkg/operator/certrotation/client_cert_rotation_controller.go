package certrotation

import (
	"context"
	"fmt"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
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
	// RunOnceContextKey is a context value key that can be used to call the controller Sync() and make it only run the syncWorker once and report error.
	RunOnceContextKey = "cert-rotation-controller.openshift.io/run-once"
)

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
}

func NewCertRotationController(
	name string,
	signingRotation SigningRotation,
	caBundleRotation CABundleRotation,
	targetRotation TargetRotation,
	operatorClient v1helpers.StaticPodOperatorClient,
	recorder events.Recorder,
) factory.Controller {
	c := &CertRotationController{
		name:             name,
		SigningRotation:  signingRotation,
		CABundleRotation: caBundleRotation,
		TargetRotation:   targetRotation,
		OperatorClient:   operatorClient,
	}
	return factory.New().
		ResyncEvery(30*time.Second).
		WithSync(c.Sync).
		WithInformers(
			signingRotation.Informer.Informer(),
			caBundleRotation.Informer.Informer(),
			targetRotation.Informer.Informer(),
		).
		WithPostStartHooks(
			c.targetCertRecheckerPostRunHook,
		).
		ToController("CertRotationController", recorder.WithComponentSuffix("cert-rotation-controller"))
}

func (c CertRotationController) Sync(ctx context.Context, syncCtx factory.SyncContext) error {
	syncErr := c.syncWorker()

	// running this function with RunOnceContextKey value context will make this "run-once" without updating status.
	isRunOnce, ok := ctx.Value(RunOnceContextKey).(bool)
	if ok && isRunOnce {
		return syncErr
	}

	newCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf(condition.CertRotationDegradedConditionTypeFmt, c.name),
		Status: operatorv1.ConditionFalse,
	}
	if syncErr != nil {
		newCondition.Status = operatorv1.ConditionTrue
		newCondition.Reason = "RotationError"
		newCondition.Message = syncErr.Error()
	}
	_, updated, updateErr := v1helpers.UpdateStaticPodStatus(c.OperatorClient, v1helpers.UpdateStaticPodConditionFn(newCondition))
	if updateErr != nil {
		return updateErr
	}
	if updated && syncErr != nil {
		syncCtx.Recorder().Warningf("RotationError", newCondition.Message)
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

func (c CertRotationController) targetCertRecheckerPostRunHook(ctx context.Context, syncCtx factory.SyncContext) error {
	// If we have a need to force rechecking the cert, use this channel to do it.
	refresher, ok := c.TargetRotation.CertCreator.(TargetCertRechecker)
	if !ok {
		return nil
	}
	targetRefresh := refresher.RecheckChannel()
	go wait.Until(func() {
		for {
			select {
			case <-targetRefresh:
				syncCtx.Queue().Add(factory.DefaultQueueKey)
			case <-ctx.Done():
				return
			}
		}
	}, time.Minute, ctx.Done())

	<-ctx.Done()
	return nil
}
