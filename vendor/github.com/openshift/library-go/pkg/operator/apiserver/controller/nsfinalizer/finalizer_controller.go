package nsfinalizer

import (
	"context"
	"fmt"
	"reflect"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	kubeinformers "k8s.io/client-go/informers"
	v1 "k8s.io/client-go/kubernetes/typed/core/v1"
	appsv1lister "k8s.io/client-go/listers/apps/v1"
	corev1listers "k8s.io/client-go/listers/core/v1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
)

type finalizerController struct {
	name          string
	namespaceName string

	namespaceGetter v1.NamespacesGetter
	podLister       corev1listers.PodLister
	dsLister        appsv1lister.DaemonSetLister
}

// NewFinalizerController is here because
// When running an aggregated API on the platform, you delete the namespace hosting the aggregated API. Doing that the
// namespace controller starts by doing complete discovery and then deleting all objects, but pods have a grace period,
// so it deletes the rest and requeues. The ns controller starts again and does a complete discovery and.... fails. The
// failure means it refuses to complete the cleanup. Now, we don't actually want to delete the resoruces from our
// aggregated API, only the server plus config if we remove the apiservices to unstick it, GC will start cleaning
// everything. For now, we can unbork 4.0, but clearing the finalizer after the pod and daemonset we created are gone.
func NewFinalizerController(
	namespaceName string,
	kubeInformersForTargetNamespace kubeinformers.SharedInformerFactory,
	namespaceGetter v1.NamespacesGetter,
	eventRecorder events.Recorder,
) factory.Controller {
	fullname := "NamespaceFinalizerController_" + namespaceName
	c := &finalizerController{
		name:            fullname,
		namespaceName:   namespaceName,
		namespaceGetter: namespaceGetter,
		podLister:       kubeInformersForTargetNamespace.Core().V1().Pods().Lister(),
		dsLister:        kubeInformersForTargetNamespace.Apps().V1().DaemonSets().Lister(),
	}

	return factory.New().ResyncEvery(time.Second).WithSync(c.sync).WithInformers(
		kubeInformersForTargetNamespace.Core().V1().Pods().Informer(),
		kubeInformersForTargetNamespace.Apps().V1().DaemonSets().Informer(),
	).ToController(fullname, eventRecorder.WithComponentSuffix("finalizer-controller"))
}

func (c finalizerController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	ns, err := c.namespaceGetter.Namespaces().Get(ctx, c.namespaceName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if ns.DeletionTimestamp == nil {
		return nil
	}

	// allow one minute of grace for most things to terminate.
	// TODO now that we have conditions, we may be able to check specific conditions
	deletedMoreThanAMinute := ns.DeletionTimestamp.Time.Add(1 * time.Minute).Before(time.Now())
	if !deletedMoreThanAMinute {
		syncCtx.Queue().AddAfter(c.namespaceName, 1*time.Minute)
		return nil
	}

	pods, err := c.podLister.Pods(c.namespaceName).List(labels.Everything())
	if err != nil {
		return err
	}
	if len(pods) > 0 {
		return nil
	}
	dses, err := c.dsLister.DaemonSets(c.namespaceName).List(labels.Everything())
	if err != nil {
		return err
	}
	if len(dses) > 0 {
		return nil
	}

	newFinalizers := []corev1.FinalizerName{}
	for _, curr := range ns.Spec.Finalizers {
		if curr == corev1.FinalizerKubernetes {
			continue
		}
		newFinalizers = append(newFinalizers, curr)
	}
	if reflect.DeepEqual(newFinalizers, ns.Spec.Finalizers) {
		return nil
	}
	ns.Spec.Finalizers = newFinalizers

	syncCtx.Recorder().Event("NamespaceFinalization", fmt.Sprintf("clearing namespace finalizer on %q", c.namespaceName))
	_, err = c.namespaceGetter.Namespaces().Finalize(ctx, ns, metav1.UpdateOptions{})
	return err
}
