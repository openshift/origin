package util

import (
	"context"
	"fmt"
	"sync"

	configv1 "github.com/openshift/api/config/v1"
	configv1client "github.com/openshift/client-go/config/clientset/versioned"
	"github.com/openshift/library-go/pkg/config/clusteroperator/v1helpers"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
)

type OperatorProgressingStatus struct {
	lock sync.Mutex

	// rolloutStableAtBeginning is closed once the operator is confirmed to be stable before progressing
	rolloutStableAtBeginning chan struct{}
	// rolloutStarted is closed once the operator starts progressing
	rolloutStarted chan struct{}
	// rolloutDone is closed once the operator finishes progressing *or* once the operation has failed.
	// If the operation failed, then RolloutError will be non-nil
	rolloutDone chan struct{}

	setErrCalled bool
	rolloutError error
}

// StableBeforeStarting is closed once the operator indicates that it is stable to begin
func (p *OperatorProgressingStatus) StableBeforeStarting() <-chan struct{} {
	return p.rolloutStableAtBeginning
}

// Started is closed once the operator starts progressing
func (p *OperatorProgressingStatus) Started() <-chan struct{} {
	return p.rolloutStarted
}

// Done is closed once the operator finishes progressing *or* once the operation has failed.
// If the operation failed, then Err() will be non-nil
func (p *OperatorProgressingStatus) Done() <-chan struct{} {
	return p.rolloutDone
}

// Err returns whether or not there was failure waiting on the operator status.
// If Done is not yet closed, Err returns nil.
// If Done is closed, Err returns nil if it was successful or non-nil if it was not.
func (p *OperatorProgressingStatus) Err() error {
	select {
	case <-p.Done():
	default:
		return nil
	}

	p.lock.Lock()
	defer p.lock.Unlock()

	err := p.rolloutError
	return err
}

func (p *OperatorProgressingStatus) setErr(err error) error {
	p.lock.Lock()
	defer p.lock.Unlock()
	if p.setErrCalled {
		return fmt.Errorf("setErr already called")
	}

	select {
	case <-p.Done():
		return fmt.Errorf("setErr called AFTER already done")
	default:
	}

	p.rolloutError = err
	return nil
}

func WaitForOperatorProgressingFalse(ctx context.Context, configClient configv1client.Interface, operatorName string) error {
	return waitForOperatorProgressingToBe(ctx, configClient, operatorName, false)
}

func WaitForOperatorProgressingTrue(ctx context.Context, configClient configv1client.Interface, operatorName string) error {
	return waitForOperatorProgressingToBe(ctx, configClient, operatorName, true)
}

func waitForOperatorProgressingToBe(ctx context.Context, configClient configv1client.Interface, operatorName string, desiredProgressing bool) error {
	_, err := watchtools.UntilWithSync(ctx,
		cache.NewListWatchFromClient(configClient.ConfigV1().RESTClient(), "clusteroperators", "", fields.Everything()),
		&configv1.ClusterOperator{},
		nil,
		func(event watch.Event) (bool, error) {
			switch event.Type {
			case watch.Added, watch.Modified:
				operator := event.Object.(*configv1.ClusterOperator)
				if operator.Name != operatorName {
					return false, nil
				}

				if desiredProgressing {
					if v1helpers.IsStatusConditionTrue(operator.Status.Conditions, configv1.OperatorProgressing) {
						return true, nil
					}
					return false, nil
				}

				if v1helpers.IsStatusConditionFalse(operator.Status.Conditions, configv1.OperatorProgressing) {
					return true, nil
				}
				return false, nil

			default:
				return false, nil
			}
		},
	)

	return err
}

// WaitForOperatorToRollout is called *before* a configuration change is made.  This method will close the first returned channel
// when the operator starts progressing and second channel once it is done progressing.  If it fails, it will
func WaitForOperatorToRollout(ctx context.Context, configClient configv1client.Interface, operatorName string) *OperatorProgressingStatus {
	ret := &OperatorProgressingStatus{
		rolloutStableAtBeginning: make(chan struct{}),
		rolloutStarted:           make(chan struct{}),
		rolloutDone:              make(chan struct{}),
	}
	go func() {
		var err error
		defer close(ret.rolloutDone)
		defer func() {
			if err := ret.setErr(err); err != nil {
				panic(err)
			}
		}()

		err = WaitForOperatorProgressingFalse(ctx, configClient, operatorName)
		close(ret.rolloutStableAtBeginning)
		if err != nil {
			// rolloutDone and rolloutErr are set on return by defer
			return
		}

		err = WaitForOperatorProgressingTrue(ctx, configClient, operatorName)
		close(ret.rolloutStarted)
		if err != nil {
			// rolloutDone and rolloutErr are set on return by defer
			return
		}

		err = WaitForOperatorProgressingFalse(ctx, configClient, operatorName)
		// rolloutDone and rolloutErr are set on return by defer
	}()

	return ret
}
