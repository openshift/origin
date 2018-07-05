/*
Copyright 2018 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package idler

import (
	"fmt"
	"log"

	"github.com/kubernetes-sigs/kubebuilder/pkg/controller"
	"github.com/kubernetes-sigs/kubebuilder/pkg/controller/types"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	coreinformers "k8s.io/client-go/informers/core/v1"
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	idling "github.com/openshift/service-idler/pkg/apis/idling/v1alpha2"
	idlingclient "github.com/openshift/service-idler/pkg/client/clientset/versioned/typed/idling/v1alpha2"
	idlinginformer "github.com/openshift/service-idler/pkg/client/informers/externalversions/idling/v1alpha2"
	idlinglister "github.com/openshift/service-idler/pkg/client/listers/idling/v1alpha2"
	"github.com/openshift/service-idler/pkg/inject/args"
)

// TODO: set status conditions?

// IdlerExecutor performs the actual idling and unidling.
// It does not deal with watching or retrying -- that's
// left to the controller.  It decouples routine controller
// logic from easily testible idler-specific logic.
type IdlerExecutor struct {
	// EndpointsActive takes the name and namespace of a service,
	// and indicates whether or not it has endpoints
	EndpointsActive func(ep types.ReconcileKey) (bool, error)

	// ScaleClient fetches and updates scales for idling/unidling
	ScaleClient scale.ScalesGetter

	// UpdateIdler updates the given idler
	UpdateIdler func(idler *idling.Idler) error

	record.EventRecorder
}

// scalesMap saves us time on typing CrossGroupObjectReference a bajillion times.
type scalesMap map[idling.CrossGroupObjectReference]int32

// collectScales fetches scales for all of the given scalable references.
// It will return the scales that it managed to fetch, even on error
func (bc *IdlerExecutor) collectScales(namespace string, targetScalables []idling.CrossGroupObjectReference, u *idling.Idler) (scalesMap, []error) {
	scales := scalesMap{}
	var delayedErrors []error
	for _, target := range targetScalables {
		groupRes := schema.GroupResource{
			Group:    target.Group,
			Resource: target.Resource,
		}
		currScaleObj, err := bc.ScaleClient.Scales(namespace).Get(groupRes, target.Name)
		if err != nil {
			fullErr := fmt.Errorf("unable to fetch scale for target scalable %s %s: %v", groupRes.String(), target.Name, err)
			delayedErrors = append(delayedErrors, fullErr)
			bc.Eventf(u, corev1.EventTypeWarning, "UnableToGetScale", "unable to fetch the scale of %s %s: %v", groupRes.String(), target.Name, err)
			// continue on, we'll try again later
			continue
		}

		currScale := currScaleObj.Spec.Replicas
		if currScale == 0 {
			// If we see something that we own with scale zero,
			// and we don't have a scale record for it, assume the user
			// manually scaled down, and we should ignore it.
			continue
		}
		scales[target] = currScale
	}

	return scales, delayedErrors
}

// mergeScales merges a list of previous scales into a of update-to-date scales,
// favoring the newer scales in case of conflicts
func mergeScales(prevScales, currScales scalesMap) {
	for objRef, prevScale := range prevScales {
		newVal, hasNewVal := currScales[objRef]
		if hasNewVal {
			// warn, but use the new val
			groupRes := schema.GroupResource{
				Group:    objRef.Group,
				Resource: objRef.Resource,
			}
			// TODO: log about the idler name here too...
			log.Printf("found a new non-zero scale %v for target scalable %s %s with previously recorded scale %v, using the new scale", newVal, groupRes.String(), objRef.Name, prevScale)
			continue
		}
		currScales[objRef] = prevScale
	}
}

func (bc *IdlerExecutor) EnsureIdle(cow *CoWIdler) []error {
	// NB: always return an AggregateError

	// put our previous scale records in a form slightly more conducive to looking up on the fly
	prevScales := make(map[idling.CrossGroupObjectReference]int32, len(cow.Status().UnidledScales))
	for _, record := range cow.Status().UnidledScales {
		prevScales[record.CrossGroupObjectReference] = record.PreviousScale
	}

	// NB: order is important here -- it's possible to lose information if we scale before recording,
	// and then recording fails.  In order to ensure that we don't lose any previous scales,
	// *first* we fetch all scales, *then* we place them into the idler object, and *only then*
	// do we actually scale.

	// record all previous scales for scalables that we own
	currScales, delayedErrors := bc.collectScales(cow.ObjectMeta().Namespace, cow.Spec().TargetScalables, cow.original)

	// record scale updates
	// we only actually need to bother updating recorded scales if we will make changes
	if len(currScales) > 0 {
		// merge any previously recorded scales into the current list, so that we don't
		// wipe them out when we do the update
		mergeScales(prevScales, currScales)

		status := cow.WritableStatus()
		status.UnidledScales = make([]idling.UnidleInfo, 0, len(currScales))
		for ref, scale := range currScales {
			status.UnidledScales = append(status.UnidledScales, idling.UnidleInfo{
				CrossGroupObjectReference: ref,
				PreviousScale:             scale,
			})
		}
	}

	// enforce that idle state is correct, even if no updates were performed
	if len(cow.Status().UnidledScales) > 0 {
		cow.WritableStatusIf(!cow.Status().Idled).Idled = true
	}

	// if we've made a change and at least one scalable will be idled,
	// we've started idling, so indicate that by setting Idled to true.
	// Note that technically, we could fail to scale below (and have to retry),
	// but it's ok to prematurely mark that we've started idling
	// (no harm in watchers prematurely turning on idling proxies and such).
	if cow.Updated() {
		if err := bc.UpdateIdler(cow.Full()); err != nil {
			delayedErrors = append(delayedErrors, err)
			// NB: we return immediately because we don't want
			// to try actually executing the scale operations
			// if we fail to mark that idling has started
			bc.Eventf(cow.original, corev1.EventTypeWarning, "UnableToUpdateIdler", "unable to update the idler to store previous scales: %v", err)
			return delayedErrors
		}

		if len(delayedErrors) == 0 && cow.Status().Idled {
			bc.Eventf(cow.original, corev1.EventTypeNormal, "SuccesfullyIdled", "marked idler as idled with %v idled scalables", len(cow.Status().UnidledScales))
		}
	}

	// ensure that all scalables in TargetScalables
	// are scaled to zero.
	for _, target := range cow.Spec().TargetScalables {
		groupRes := schema.GroupResource{
			Group:    target.Group,
			Resource: target.Resource,
		}
		newScale := &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: cow.ObjectMeta().Namespace,
			},
			Spec: autoscalingv1.ScaleSpec{Replicas: 0},
		}
		_, err := bc.ScaleClient.Scales(cow.ObjectMeta().Namespace).Update(groupRes, newScale)
		if err != nil {
			fullErr := fmt.Errorf("unable to update scale for target scalable %s %s: %v", groupRes.String(), target.Name, err)
			delayedErrors = append(delayedErrors, fullErr)
			bc.Eventf(cow.original, corev1.EventTypeWarning, "UnableToUpdateScale", "unable to update the scale of %s %s to 0: %v", groupRes.String(), target.Name, err)
			// continue on, we'll try again later
			continue
		}
	}

	return delayedErrors
}

func (bc *IdlerExecutor) EnsureUnidle(cow *CoWIdler) []error {
	var delayedErrors []error

	// arrange all scale records for easy access
	prevScales := make(map[idling.CrossGroupObjectReference]int32, len(cow.Status().UnidledScales))
	for _, record := range cow.Status().UnidledScales {
		prevScales[record.CrossGroupObjectReference] = record.PreviousScale
	}

	// scale all targets with know previous scales back up
	for _, target := range cow.Spec().TargetScalables {
		prevScale, hasRecord := prevScales[target]
		if !hasRecord {
			// skip any target scalable that we don't know about having scaled...
			continue
		}
		groupRes := schema.GroupResource{
			Group:    target.Group,
			Resource: target.Resource,
		}
		_, err := bc.ScaleClient.Scales(cow.ObjectMeta().Namespace).Update(groupRes, &autoscalingv1.Scale{
			ObjectMeta: metav1.ObjectMeta{
				Name:      target.Name,
				Namespace: cow.ObjectMeta().Namespace,
			},
			Spec: autoscalingv1.ScaleSpec{
				Replicas: prevScale,
			},
		})
		if err != nil {
			delayedErrors = append(delayedErrors, fmt.Errorf("unable restore target scalable %s %s to previous scale %v: %v", groupRes.String(), target.Name, prevScale, err))
			bc.Eventf(cow.original, corev1.EventTypeWarning, "UnableToUpdateScale", "unable to update the scale of %s %s to %v: %v", groupRes.String(), target.Name, prevScale, err)
			continue
		}

		delete(prevScales, target)
	}

	// NB: unlike the idle function, there's no issue if we fail to save our updates here --
	// we'll just see that we have leftover previous scales, and try to reconcile them again.

	// clean up unknown recorded scales, if needed
	if len(prevScales) > 0 {
		knownTargets := make(map[idling.CrossGroupObjectReference]struct{}, len(cow.Spec().TargetScalables))
		for _, target := range cow.Spec().TargetScalables {
			knownTargets[target] = struct{}{}
		}

		for target := range prevScales {
			if _, known := knownTargets[target]; !known {
				// NB: this is actually ok in Go
				delete(prevScales, target)
			}
		}
	}

	// check if we need to update the list of scale records
	if len(prevScales) != len(cow.Status().UnidledScales) {
		// actually copy our idler here
		status := cow.WritableStatus()
		status.UnidledScales = make([]idling.UnidleInfo, 0, len(prevScales))
		for target, scale := range prevScales {
			status.UnidledScales = append(status.UnidledScales, idling.UnidleInfo{
				CrossGroupObjectReference: target,
				PreviousScale:             scale,
			})
		}
	}

	if len(cow.Status().InactiveServiceNames) == 0 {
		// consider ourselves updated
		cow.WritableStatusIf(cow.Status().Idled).Idled = false
	} else {
		// NB: this could cause us to flip if someone manually scales endpoints
		// to zero.  Consumers should be aware of this.
		cow.WritableStatusIf(!cow.Status().Idled).Idled = true
	}

	if cow.Updated() {
		if err := bc.UpdateIdler(cow.Full()); err != nil {
			delayedErrors = append(delayedErrors, err)
		}

		if len(delayedErrors) == 0 && !cow.Status().Idled {
			bc.Eventf(cow.original, corev1.EventTypeNormal, "SuccesfullyUnidled", "finished unidling all scalables", len(cow.Status().UnidledScales))
		}
	}

	return delayedErrors
}

// populateInactiveServices updates the list of active/inactive services.
func (bc *IdlerExecutor) PopulateInactiveServices(u *CoWIdler) []error {
	var delayedErrors []error

	// NB: if the user ever scales the trigger services backing scalables down manually
	// before idling is finished, we'll never fully mark as unidled.  Such is life.

	if !u.Status().Idled {
		// only update if we're currently idled
		// if we're not idled, by definition we should have no active services,
		// but reconcile just in case
		inactiveServiceCount := len(u.Status().InactiveServiceNames)
		if inactiveServiceCount > 0 {
			u.WritableStatus().InactiveServiceNames = nil
			// TODO: log idler name/namespace?
			return []error{fmt.Errorf("idler was unidled, but had %v inactive services", inactiveServiceCount)}
		}
		return nil
	}

	// check if we've *actually* finished idling, which is determined by whether or not
	// all listed trigger services have at least one endpoint subset.
	inactiveSvcInd := 0
	prevInactiveSvcs := u.Status().InactiveServiceNames
	for _, svcName := range u.Spec().TriggerServiceNames {
		active, err := bc.EndpointsActive(types.ReconcileKey{
			Namespace: u.ObjectMeta().Namespace,
			Name:      svcName,
		})
		if err != nil {
			delayedErrors = append(delayedErrors, fmt.Errorf("unable to check service %s/%s for active endpoints: %v", u.ObjectMeta().Namespace, svcName, err))
			// NB: using original here is safe, because all we need is the objectmeta, which is immutable
			bc.Eventf(u.original, corev1.EventTypeWarning, "UnableToCheckEndpoints", "unable to check service %s for active endpoints: %v", svcName, err)
			active = false // just treat errors as inactive services
		}

		if !active {
			status := u.WritableStatusIf(inactiveSvcInd >= len(prevInactiveSvcs) || prevInactiveSvcs[inactiveSvcInd] != svcName)
			status.InactiveServiceNames = append(status.InactiveServiceNames[:inactiveSvcInd], svcName)
			inactiveSvcInd++
		}
	}
	// update the length if it ended up that we have no inactive services
	status := u.WritableStatusIf(inactiveSvcInd != len(u.Status().InactiveServiceNames))
	status.InactiveServiceNames = status.InactiveServiceNames[:inactiveSvcInd]

	return delayedErrors
}

func (bc *IdlerController) Reconcile(k types.ReconcileKey) error {
	log.Printf("reconciling Idler %s/%s\n", k.Namespace, k.Name)

	originalIdler, err := bc.idlerLister.Idlers(k.Namespace).Get(k.Name)
	if err != nil {
		return err
	}

	// set up a copy-on-write object
	cow := NewCoW(originalIdler)
	errs := bc.executor.PopulateInactiveServices(cow)

	// TODO: attach a message to the aggregate error?
	if originalIdler.Spec.WantIdle {
		errs = append(errs, bc.executor.EnsureIdle(cow)...)
	} else {
		errs = append(errs, bc.executor.EnsureUnidle(cow)...)
	}

	// make sure to run the update if we actually need to do so
	errs = append(errs, cow.UpdateIfNeeded(bc.updateIdler))

	return utilerrors.NewAggregate(errs)
}

func (bc *IdlerController) updateIdler(idler *idling.Idler) error {
	_, err := bc.idlerClient.Idlers(idler.Namespace).Update(idler)
	return err
}

func (bc *IdlerController) endpointsActive(epKey types.ReconcileKey) (bool, error) {
	ep, err := bc.endpointsLister.Endpoints(epKey.Namespace).Get(epKey.Name)
	if err != nil {
		return false, err
	}
	// TODO: report unready endpoints, somehow?
	if len(ep.Subsets) > 0 && len(ep.Subsets[0].Addresses) > 0 {
		return true, nil
	}

	return false, nil
}

// +informers:group=core,version=v1,kind=Endpoints
// +rbac:groups="",resources=endpoints,verbs=list;watch;get
// +rbac:groups="",resources=events,verbs=patch;create;update
// +rbac:groups=*,resources=*/scale,verbs=get;update
// +controller:group=idling,version=idling,kind=Idler,resource=idlers
type IdlerController struct {
	// executor actually executes idling or unidling
	executor *IdlerExecutor

	endpointsLister corelisters.EndpointsLister

	idlerIndexer cache.Indexer
	idlerLister  idlinglister.IdlerLister
	idlerClient  idlingclient.IdlingV1alpha2Interface
}

// ProvideController provides a controller that will be run at startup.  Kubebuilder will use codegeneration
// to automatically register this controller in the inject package
func ProvideController(arguments args.InjectArgs) (*controller.GenericController, error) {
	idlerInformer := arguments.ControllerManager.GetInformerProvider(&idling.Idler{}).(idlinginformer.IdlerInformer)
	indexer := idlerInformer.Informer().GetIndexer()
	bc := &IdlerController{
		endpointsLister: arguments.ControllerManager.GetInformerProvider(&corev1.Endpoints{}).(coreinformers.EndpointsInformer).Lister(),
		idlerIndexer:    indexer,
		idlerLister:     idlerInformer.Lister(),
		idlerClient:     arguments.Clientset.IdlingV1alpha2(),
	}

	bc.executor = &IdlerExecutor{
		ScaleClient:     arguments.ScaleClient,
		UpdateIdler:     bc.updateIdler,
		EndpointsActive: bc.endpointsActive,
		EventRecorder:   arguments.CreateRecorder("idler controller"),
	}

	// Create a new controller that will call IdlerController.Reconcile on changes to Idlers
	gc := &controller.GenericController{
		Name:             "IdlerController",
		Reconcile:        bc.Reconcile,
		InformerRegistry: arguments.ControllerManager,
	}
	if err := gc.Watch(&idling.Idler{}); err != nil {
		return gc, err
	}

	idlerInformer.Informer().AddIndexers(cache.Indexers{
		triggerServicesIndex: triggerServicesIndexFunc,
	})
	idlerByEndpts := func(raw interface{}) string {
		endpts := raw.(*corev1.Endpoints)
		idlers, err := bc.idlerIndexer.ByIndex(triggerServicesIndex, endpts.Namespace+"/"+endpts.Name)
		if err != nil {
			log.Printf("unable to fetch idlers for endpoint: %v", err)
			return ""
		}

		if len(idlers) == 0 {
			return ""
		}

		idler := idlers[0].(*idling.Idler)
		if len(idlers) > 1 {
			log.Printf("multiple (%v) idlers for endpoints %s/%s, using the first (%s/%s)", len(idlers), endpts.Namespace, endpts.Name, idler.Namespace, idler.Name)
		}

		return idler.Namespace + "/" + idler.Name
	}
	// TODO: predicates?
	if err := gc.WatchTransformationOf(&corev1.Endpoints{}, idlerByEndpts); err != nil {
		return gc, err
	}

	// NOTE: Informers for Kubernetes resources *MUST* be registered in the pkg/inject package so that they are started.
	return gc, nil
}

const (
	triggerServicesIndex = "triggerServices"
)

func triggerServicesIndexFunc(obj interface{}) ([]string, error) {
	idler, wasIdler := obj.(*idling.Idler)
	if !wasIdler {
		return nil, fmt.Errorf("trigger services indexer received object %v that wasn't an Idler", obj)
	}

	res := make([]string, len(idler.Spec.TriggerServiceNames))
	for i, svcName := range idler.Spec.TriggerServiceNames {
		res[i] = idler.Namespace + "/" + svcName
	}

	return res, nil
}
