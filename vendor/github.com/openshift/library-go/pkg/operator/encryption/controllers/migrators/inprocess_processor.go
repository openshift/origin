package migrators

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/pager"
	"k8s.io/klog"
)

const (
	defaultConcurrency = 10
)

// workerFunc function that is executed by workers to process a single item
type workerFunc func(*unstructured.Unstructured) error

// listProcessor represents a type that processes resources in parallel.
// It retrieves resources from the server in batches and distributes among set of workers.
type listProcessor struct {
	concurrency   int
	workerFn      workerFunc
	dynamicClient dynamic.Interface
	ctx           context.Context
}

// newListProcessor creates a new instance of listProcessor
func newListProcessor(ctx context.Context, dynamicClient dynamic.Interface, workerFn workerFunc) *listProcessor {
	return &listProcessor{
		concurrency:   defaultConcurrency,
		workerFn:      workerFn,
		dynamicClient: dynamicClient,
		ctx:           ctx,
	}
}

// run starts processing all the instance of the given GVR in batches.
// Note that this operation block until all resources have been process, we can't get the next page or the context has been cancelled
func (p *listProcessor) run(gvr schema.GroupVersionResource) error {
	listPager := pager.New(pager.SimplePageFunc(func(opts metav1.ListOptions) (runtime.Object, error) {
		for {
			allResource, err := p.dynamicClient.Resource(gvr).List(context.TODO(), opts)
			if err != nil {
				klog.Warningf("List of %v failed: %v", gvr, err)
				if errors.IsResourceExpired(err) {
					token, err := inconsistentContinueToken(err)
					if err != nil {
						return nil, err
					}
					opts.Continue = token
					klog.V(2).Infof("Relisting %v after handling expired token", gvr)
					continue
				} else if retryable := canRetry(err); retryable == nil || *retryable == false {
					return nil, err // not retryable or we don't know. Return error and controller will restart migration.
				} else {
					if seconds, delay := errors.SuggestsClientDelay(err); delay {
						time.Sleep(time.Duration(seconds) * time.Second)
					}
					klog.V(2).Infof("Relisting %v after retryable error: %v", gvr, err)
					continue
				}
			}

			migrationStarted := time.Now()
			klog.V(2).Infof("Migrating %d objects of %v", len(allResource.Items), gvr)
			if err = p.processList(allResource, gvr); err != nil {
				klog.Warningf("Migration of %v failed after %v: %v", gvr, time.Now().Sub(migrationStarted), err)
				return nil, err
			}
			klog.V(2).Infof("Migration of %d objects of %v finished in %v", len(allResource.Items), gvr, time.Now().Sub(migrationStarted))

			allResource.Items = nil // do not accumulate items, this fakes the visitor pattern
			return allResource, nil // leave the rest of the list intact to preserve continue token
		}
	}))
	listPager.FullListIfExpired = false // prevent memory explosion from full list

	migrationStarted := time.Now()
	if _, _, err := listPager.List(p.ctx, metav1.ListOptions{}); err != nil {
		metrics.ObserveFailedMigration(gvr.String())
		return err
	}
	migrationDuration := time.Now().Sub(migrationStarted)
	klog.V(2).Infof("Migration for %v finished in %v", gvr, migrationDuration)
	metrics.ObserveSucceededMigration(gvr.String())
	metrics.ObserveSucceededMigrationDuration(migrationDuration.Seconds(), gvr.String())
	return nil
}

func (p *listProcessor) processList(l *unstructured.UnstructuredList, gvr schema.GroupVersionResource) error {
	workCh := make(chan *unstructured.Unstructured, p.concurrency)
	ctx, cancel := context.WithCancel(p.ctx)
	defer cancel()

	processed := 0
	go func() {
		defer utilruntime.HandleCrash()
		defer close(workCh)
		for i := range l.Items {
			select {
			case workCh <- &l.Items[i]:
				processed++
			case <-ctx.Done():
				return
			}
		}
	}()

	var wg sync.WaitGroup
	errCh := make(chan error, p.concurrency)
	for i := 0; i < p.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := p.worker(workCh, gvr); err != nil {
				errCh <- err
				cancel() // stop everything when the first worker errors
			}
		}()
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return utilerrors.NewAggregate(errs)
	}
	if processed < len(l.Items) {
		return fmt.Errorf("context cancelled")
	}
	return nil
}

func (p *listProcessor) worker(workCh <-chan *unstructured.Unstructured, gvr schema.GroupVersionResource) (result error) {
	defer func() {
		if r := recover(); r != nil {
			if err, ok := r.(error); ok {
				result = err
			} else {
				result = fmt.Errorf("panic: %v", r)
			}
		}
	}()

	for item := range workCh {
		err := p.workerFn(item)
		metrics.ObserveObjectsMigrated(1, gvr.String())
		if err != nil {
			return err
		}
	}

	return nil
}

// inconsistentContinueToken extracts the continue token from the response which might be used to retrieve the remainder of the results
//
// Note:
// continuing with the provided token might result in an inconsistent list. Objects that were created,
// modified, or deleted between the time the first chunk was returned and now may show up in the list.
func inconsistentContinueToken(err error) (string, error) {
	status, ok := err.(errors.APIStatus)
	if !ok {
		return "", fmt.Errorf("expected error to implement the APIStatus interface, got %v", reflect.TypeOf(err))
	}
	token := status.Status().ListMeta.Continue
	if len(token) == 0 {
		return "", fmt.Errorf("expected non empty continue token")
	}
	return token, nil
}
