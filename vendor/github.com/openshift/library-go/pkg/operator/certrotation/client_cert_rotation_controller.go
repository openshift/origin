package certrotation

import (
	"context"
	"fmt"
	"time"

	operatorv1 "github.com/openshift/api/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/condition"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	// RunOnceContextKey is a context value key that can be used to call the controller Sync() and make it only run the syncWorker once and report error.
	RunOnceContextKey = "cert-rotation-controller.openshift.io/run-once"
)

// StatusReporter knows how to report the status of cert rotation
type StatusReporter interface {
	Report(ctx context.Context, controllerName string, syncErr error) (updated bool, updateErr error)
}

var _ StatusReporter = (*StaticPodConditionStatusReporter)(nil)

type StaticPodConditionStatusReporter struct {
	// Plumbing:
	OperatorClient v1helpers.StaticPodOperatorClient
}

func (s *StaticPodConditionStatusReporter) Report(ctx context.Context, controllerName string, syncErr error) (bool, error) {
	newCondition := operatorv1.OperatorCondition{
		Type:   fmt.Sprintf(condition.CertRotationDegradedConditionTypeFmt, controllerName),
		Status: operatorv1.ConditionFalse,
	}
	if syncErr != nil {
		newCondition.Status = operatorv1.ConditionTrue
		newCondition.Reason = "RotationError"
		newCondition.Message = syncErr.Error()
	}
	_, updated, updateErr := v1helpers.UpdateStaticPodStatus(ctx, s.OperatorClient, v1helpers.UpdateStaticPodConditionFn(newCondition))
	return updated, updateErr
}

// CertRotationController does:
//
// 1) continuously create a self-signed signing CA (via RotatedSigningCASecret) and store it in a secret.
// 2) maintain a CA bundle ConfigMap with all not yet expired CA certs.
// 3) continuously create a target cert and key signed by the latest signing CA and store it in a secret.
type CertRotationController struct {
	// controller name
	Name string
	// RotatedSigningCASecret rotates a self-signed signing CA stored in a secret.
	RotatedSigningCASecret RotatedSigningCASecret
	// CABundleConfigMap maintains a CA bundle config map, by adding new CA certs coming from rotatedSigningCASecret, and by removing expired old ones.
	CABundleConfigMap CABundleConfigMap
	// RotatedSelfSignedCertKeySecret rotates a key and cert signed by a signing CA and stores it in a secret.
	RotatedSelfSignedCertKeySecret RotatedSelfSignedCertKeySecret

	// Plumbing:
	StatusReporter StatusReporter
}

func NewCertRotationController(
	name string,
	rotatedSigningCASecret RotatedSigningCASecret,
	caBundleConfigMap CABundleConfigMap,
	rotatedSelfSignedCertKeySecret RotatedSelfSignedCertKeySecret,
	recorder events.Recorder,
	reporter StatusReporter,
) factory.Controller {
	c := &CertRotationController{
		Name:                           name,
		RotatedSigningCASecret:         rotatedSigningCASecret,
		CABundleConfigMap:              caBundleConfigMap,
		RotatedSelfSignedCertKeySecret: rotatedSelfSignedCertKeySecret,
		StatusReporter:                 reporter,
	}
	return factory.New().
		ResyncEvery(time.Minute).
		WithSync(c.Sync).
		WithFilteredEventsInformers(
			func(obj interface{}) bool {
				if cm, ok := obj.(*corev1.ConfigMap); ok {
					return cm.Namespace == caBundleConfigMap.Namespace && cm.Name == caBundleConfigMap.Name
				}
				if secret, ok := obj.(*corev1.Secret); ok {
					if secret.Namespace == rotatedSigningCASecret.Namespace && secret.Name == rotatedSigningCASecret.Name {
						return true
					}
					if secret.Namespace == rotatedSelfSignedCertKeySecret.Namespace && secret.Name == rotatedSelfSignedCertKeySecret.Name {
						return true
					}
					return false
				}
				return true
			},
			rotatedSigningCASecret.Informer.Informer(),
			caBundleConfigMap.Informer.Informer(),
			rotatedSelfSignedCertKeySecret.Informer.Informer(),
		).
		WithPostStartHooks(
			c.targetCertRecheckerPostRunHook,
		).
		ToController(
			"CertRotationController", // don't change what is passed here unless you also remove the old FooDegraded condition
			recorder.WithComponentSuffix("cert-rotation-controller").WithComponentSuffix(name),
		)
}

func (c CertRotationController) Sync(ctx context.Context, syncCtx factory.SyncContext) error {
	syncErr := c.SyncWorker(ctx)

	// running this function with RunOnceContextKey value context will make this "run-once" without updating status.
	isRunOnce, ok := ctx.Value(RunOnceContextKey).(bool)
	if ok && isRunOnce {
		return syncErr
	}

	updated, updateErr := c.StatusReporter.Report(ctx, c.Name, syncErr)
	if updateErr != nil {
		return updateErr
	}
	if updated && syncErr != nil {
		syncCtx.Recorder().Warningf("RotationError", syncErr.Error())
	}

	return syncErr
}

func (c CertRotationController) getSigningCertKeyPairLocation() string {
	return fmt.Sprintf("%s/%s", c.RotatedSelfSignedCertKeySecret.Namespace, c.RotatedSelfSignedCertKeySecret.Name)
}

func (c CertRotationController) SyncWorker(ctx context.Context) error {
	signingCertKeyPair, _, err := c.RotatedSigningCASecret.EnsureSigningCertKeyPair(ctx)
	if err != nil || signingCertKeyPair == nil {
		return err
	}

	cabundleCerts, err := c.CABundleConfigMap.EnsureConfigMapCABundle(ctx, signingCertKeyPair, c.getSigningCertKeyPairLocation())
	if err != nil {
		return err
	}

	if _, err := c.RotatedSelfSignedCertKeySecret.EnsureTargetCertKeyPair(ctx, signingCertKeyPair, cabundleCerts); err != nil {
		return err
	}

	return nil
}

func (c CertRotationController) targetCertRecheckerPostRunHook(ctx context.Context, syncCtx factory.SyncContext) error {
	// If we have a need to force rechecking the cert, use this channel to do it.
	refresher, ok := c.RotatedSelfSignedCertKeySecret.CertCreator.(TargetCertRechecker)
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
