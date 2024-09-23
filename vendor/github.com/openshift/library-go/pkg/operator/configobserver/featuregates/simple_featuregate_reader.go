package featuregates

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"sync"
	"time"

	configv1 "github.com/openshift/api/config/v1"

	v1 "github.com/openshift/client-go/config/informers/externalversions/config/v1"
	configlistersv1 "github.com/openshift/client-go/config/listers/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

type FeatureGateChangeHandlerFunc func(featureChange FeatureChange)

// FeatureGateAccess is used to get a list of enabled and disabled featuregates.
// Create a new instance using NewFeatureGateAccess.
// To create one for unit testing, use NewHardcodedFeatureGateAccess.
type FeatureGateAccess interface {
	// SetChangeHandler can only be called before Run.
	// The default change handler will exit 0 when the set of featuregates changes.
	// That is usually the easiest and simplest thing for an *operator* to do.
	// This also discourages direct operand reading since all operands restarting simultaneously is bad.
	// This function allows changing that default behavior to something else (perhaps a channel notification for
	// all impacted controllers in an operator.
	// I doubt this will be worth the effort in the majority of cases.
	SetChangeHandler(featureGateChangeHandlerFn FeatureGateChangeHandlerFunc)

	// Run starts a go func that continously watches the set of featuregates enabled in the cluster.
	Run(ctx context.Context)
	// InitialFeatureGatesObserved returns a channel that is closed once the featuregates have
	// been observed. Once closed, the CurrentFeatureGates method will return the current set of
	// featuregates and will never return a non-nil error.
	InitialFeatureGatesObserved() <-chan struct{}
	// CurrentFeatureGates returns the list of enabled and disabled featuregates.
	// It returns an error if the current set of featuregates is not known.
	CurrentFeatureGates() (FeatureGate, error)
	// AreInitialFeatureGatesObserved returns true if the initial featuregates have been observed.
	AreInitialFeatureGatesObserved() bool
}

type Features struct {
	Enabled  []configv1.FeatureGateName
	Disabled []configv1.FeatureGateName
}

type FeatureChange struct {
	Previous *Features
	New      Features
}

type defaultFeatureGateAccess struct {
	desiredVersion              string
	missingVersionMarker        string
	clusterVersionLister        configlistersv1.ClusterVersionLister
	featureGateLister           configlistersv1.FeatureGateLister
	initialFeatureGatesObserved chan struct{}

	featureGateChangeHandlerFn FeatureGateChangeHandlerFunc

	lock            sync.Mutex
	started         bool
	initialFeatures Features
	currentFeatures Features

	queue         workqueue.RateLimitingInterface
	eventRecorder events.Recorder
}

// NewFeatureGateAccess returns a controller that keeps the list of enabled/disabled featuregates up to date.
// desiredVersion is the version of this operator that would be set on the clusteroperator.status.versions.
// missingVersionMarker is the stub version provided by the operator.  If that is also the desired version,
// then the most either the desired clusterVersion or most recent version will be used.
// clusterVersionInformer is used when desiredVersion and missingVersionMarker are the same to derive the "best" version
// of featuregates to use.
// featureGateInformer is used to track changes to the featureGates once they are initially set.
// By default, when the enabled/disabled list  of featuregates changes, os.Exit is called.  This behavior can be
// overridden by calling SetChangeHandler to whatever you wish the behavior to be.
// A common construct is:
/* go
featureGateAccessor := NewFeatureGateAccess(args)
go featureGateAccessor.Run(ctx)

select{
case <- featureGateAccessor.InitialFeatureGatesObserved():
	featureGates, _ := featureGateAccessor.CurrentFeatureGates()
	klog.Infof("FeatureGates initialized: knownFeatureGates=%v", featureGates.KnownFeatures())
case <- time.After(1*time.Minute):
	klog.Errorf("timed out waiting for FeatureGate detection")
	return fmt.Errorf("timed out waiting for FeatureGate detection")
}

// whatever other initialization you have to do, at this point you have FeatureGates to drive your behavior.
*/
// That construct is easy.  It is better to use the .spec.observedConfiguration construct common in library-go operators
// to avoid gating your general startup on FeatureGate determination, but if you haven't already got that mechanism
// this construct is easy.
func NewFeatureGateAccess(
	desiredVersion, missingVersionMarker string,
	clusterVersionInformer v1.ClusterVersionInformer,
	featureGateInformer v1.FeatureGateInformer,
	eventRecorder events.Recorder) FeatureGateAccess {
	c := &defaultFeatureGateAccess{
		desiredVersion:              desiredVersion,
		missingVersionMarker:        missingVersionMarker,
		clusterVersionLister:        clusterVersionInformer.Lister(),
		featureGateLister:           featureGateInformer.Lister(),
		initialFeatureGatesObserved: make(chan struct{}),
		queue:                       workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "feature-gate-detector"),
		eventRecorder:               eventRecorder,
	}
	c.SetChangeHandler(ForceExit)

	// we aren't expecting many
	clusterVersionInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add("cluster")
		},
		UpdateFunc: func(old, cur interface{}) {
			c.queue.Add("cluster")
		},
		DeleteFunc: func(uncast interface{}) {
			c.queue.Add("cluster")
		},
	})
	featureGateInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.queue.Add("cluster")
		},
		UpdateFunc: func(old, cur interface{}) {
			c.queue.Add("cluster")
		},
		DeleteFunc: func(uncast interface{}) {
			c.queue.Add("cluster")
		},
	})

	return c
}

func ForceExit(featureChange FeatureChange) {
	if featureChange.Previous != nil {
		os.Exit(0)
	}
}

func (c *defaultFeatureGateAccess) SetChangeHandler(featureGateChangeHandlerFn FeatureGateChangeHandlerFunc) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.started {
		panic("programmer error, cannot update the change handler after starting")
	}
	c.featureGateChangeHandlerFn = featureGateChangeHandlerFn
}

func (c *defaultFeatureGateAccess) Run(ctx context.Context) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting feature-gate-detector")
	defer klog.Infof("Shutting down feature-gate-detector")

	go wait.UntilWithContext(ctx, c.runWorker, time.Second)

	<-ctx.Done()
}

func (c *defaultFeatureGateAccess) syncHandler(ctx context.Context) error {
	desiredVersion := c.desiredVersion
	if c.missingVersionMarker == c.desiredVersion {
		clusterVersion, err := c.clusterVersionLister.Get("version")
		if apierrors.IsNotFound(err) {
			return nil // we will be re-triggered when it is created
		}
		if err != nil {
			return err
		}

		desiredVersion = clusterVersion.Status.Desired.Version
		if len(desiredVersion) == 0 && len(clusterVersion.Status.History) > 0 {
			desiredVersion = clusterVersion.Status.History[0].Version
		}
	}

	featureGate, err := c.featureGateLister.Get("cluster")
	if apierrors.IsNotFound(err) {
		return nil // we will be re-triggered when it is created
	}
	if err != nil {
		return err
	}

	features, err := featuresFromFeatureGate(featureGate, desiredVersion)
	if err != nil {
		return fmt.Errorf("unable to determine features: %w", err)
	}

	c.setFeatureGates(features)

	return nil
}

func (c *defaultFeatureGateAccess) setFeatureGates(features Features) {
	c.lock.Lock()
	defer c.lock.Unlock()

	var previousFeatures *Features
	if c.AreInitialFeatureGatesObserved() {
		t := c.currentFeatures
		previousFeatures = &t
	}

	c.currentFeatures = features

	if !c.AreInitialFeatureGatesObserved() {
		c.initialFeatures = features
		close(c.initialFeatureGatesObserved)
		c.eventRecorder.Eventf("FeatureGatesInitialized", "FeatureGates updated to %#v", c.currentFeatures)
	}

	if previousFeatures == nil || !reflect.DeepEqual(*previousFeatures, c.currentFeatures) {
		if previousFeatures != nil {
			c.eventRecorder.Eventf("FeatureGatesModified", "FeatureGates updated to %#v", c.currentFeatures)
		}

		c.featureGateChangeHandlerFn(FeatureChange{
			Previous: previousFeatures,
			New:      c.currentFeatures,
		})
	}
}

func (c *defaultFeatureGateAccess) InitialFeatureGatesObserved() <-chan struct{} {
	return c.initialFeatureGatesObserved
}

func (c *defaultFeatureGateAccess) AreInitialFeatureGatesObserved() bool {
	select {
	case <-c.InitialFeatureGatesObserved():
		return true
	default:
		return false
	}
}

func (c *defaultFeatureGateAccess) CurrentFeatureGates() (FeatureGate, error) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if !c.AreInitialFeatureGatesObserved() {
		return nil, fmt.Errorf("featureGates not yet observed")
	}
	retEnabled := make([]configv1.FeatureGateName, len(c.currentFeatures.Enabled))
	retDisabled := make([]configv1.FeatureGateName, len(c.currentFeatures.Disabled))
	copy(retEnabled, c.currentFeatures.Enabled)
	copy(retDisabled, c.currentFeatures.Disabled)

	return NewFeatureGate(retEnabled, retDisabled), nil
}

func (c *defaultFeatureGateAccess) runWorker(ctx context.Context) {
	for c.processNextWorkItem(ctx) {
	}
}

func (c *defaultFeatureGateAccess) processNextWorkItem(ctx context.Context) bool {
	dsKey, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(dsKey)

	err := c.syncHandler(ctx)
	if err == nil {
		c.queue.Forget(dsKey)
		return true
	}

	utilruntime.HandleError(fmt.Errorf("%v failed with : %v", dsKey, err))
	c.queue.AddRateLimited(dsKey)

	return true
}

func featuresFromFeatureGate(featureGate *configv1.FeatureGate, desiredVersion string) (Features, error) {
	found := false
	features := Features{}
	for _, featureGateValues := range featureGate.Status.FeatureGates {
		if featureGateValues.Version != desiredVersion {
			continue
		}
		found = true
		for _, enabled := range featureGateValues.Enabled {
			features.Enabled = append(features.Enabled, enabled.Name)
		}
		for _, disabled := range featureGateValues.Disabled {
			features.Disabled = append(features.Disabled, disabled.Name)
		}
		break
	}

	if !found {
		return Features{}, fmt.Errorf("missing desired version %q in featuregates.config.openshift.io/cluster", desiredVersion)
	}

	return features, nil
}
