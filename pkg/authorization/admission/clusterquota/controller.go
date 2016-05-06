package clusterquota

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/quota"
	utilruntime "k8s.io/kubernetes/pkg/util/runtime"
	"k8s.io/kubernetes/pkg/util/sets"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/util/workqueue"

	quotaapi "github.com/openshift/origin/pkg/authorization/api"
	oclient "github.com/openshift/origin/pkg/client"
)

type quotaEvaluator struct {
	client oclient.Interface

	// registry that knows how to measure usage for objects
	registry quota.Registry

	// TODO these are used together to bucket items by namespace and then batch them up for processing.
	// The technique is valuable for rollup activities to avoid fanout and reduce resource contention.
	// We could move this into a library if another component needed it.
	// queue is indexed by namespace, so that we bundle up on a per-namespace basis
	queue      *workqueue.Type
	workLock   sync.Mutex
	work       map[string][]*admissionWaiter
	dirtyWork  map[string][]*admissionWaiter
	inProgress sets.String

	updatedQuotas     map[string]*quotaapi.ClusterResourceQuota
	updatedQuotasLock sync.Mutex

	lockFactory LockFactory
}

type admissionWaiter struct {
	attributes admission.Attributes
	finished   chan struct{}
	result     error
}

type defaultDeny struct{}

func (defaultDeny) Error() string {
	return "DEFAULT DENY"
}

func IsDefaultDeny(err error) bool {
	if err == nil {
		return false
	}

	_, ok := err.(defaultDeny)
	return ok
}

func newAdmissionWaiter(a admission.Attributes) *admissionWaiter {
	return &admissionWaiter{
		attributes: a,
		finished:   make(chan struct{}),
		result:     defaultDeny{},
	}
}

// newQuotaEvaluator configures an admission controller that can enforce quota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func newQuotaEvaluator(client oclient.Interface, registry quota.Registry) (*quotaEvaluator, error) {
	return &quotaEvaluator{
		client:   client,
		registry: registry,

		queue:      workqueue.New(),
		work:       map[string][]*admissionWaiter{},
		dirtyWork:  map[string][]*admissionWaiter{},
		inProgress: sets.String{},

		lockFactory: NewDefaultLockFactory(),

		updatedQuotas: map[string]*quotaapi.ClusterResourceQuota{},
	}, nil
}

// Run begins watching and syncing.
func (e *quotaEvaluator) Run(workers int) {
	defer utilruntime.HandleCrash()

	for i := 0; i < workers; i++ {
		go wait.Until(e.doWork, time.Second, make(chan struct{}))
	}
}

func (e *quotaEvaluator) doWork() {
	for {
		func() {
			ns, admissionAttributes := e.getWork()
			defer e.completeWork(ns)
			if len(admissionAttributes) == 0 {
				return
			}

			e.checkAttributes(ns, admissionAttributes)
		}()
	}
}

// checkAttributes iterates evaluates all the waiting admissionAttributes.  It will always notify all waiters
// before returning.  The default is to deny.
func (e *quotaEvaluator) checkAttributes(ns string, admissionAttributes []*admissionWaiter) {
	// notify all on exit
	defer func() {
		for _, admissionAttribute := range admissionAttributes {
			close(admissionAttribute.finished)
		}
	}()

	quotas, err := e.getQuotas(ns)
	if err != nil {
		for _, admissionAttribute := range admissionAttributes {
			admissionAttribute.result = err
		}
		return
	}
	if len(quotas) == 0 {
		for _, admissionAttribute := range admissionAttributes {
			admissionAttribute.result = nil
		}
		return
	}

	// acquire the locks in alphabetical order because I'm too lazy to think of something clever
	sort.Sort(ByName(quotas))
	for _, quota := range quotas {
		lock := e.lockFactory.GetLock(quota.Name)
		lock.Lock()
		defer lock.Unlock()
	}

	e.checkQuotas(quotas, admissionAttributes, 3)
}

type ByName []api.ResourceQuota

func (v ByName) Len() int           { return len(v) }
func (v ByName) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v ByName) Less(i, j int) bool { return v[i].Name < v[j].Name }

// checkQuotas checks the admission atttributes against the passed quotas.  If a quota applies, it will attempt to update it
// AFTER it has checked all the admissionAttributes.  The method breaks down into phase like this:
// 0. make a copy of the quotas to act as a "running" quota so we know what we need to update and can still compare against the
//    originals
// 1. check each admission attribute to see if it fits within *all* the quotas.  If it doesn't fit, mark the waiter as failed
//    and the running quota don't change.  If it did fit, check to see if any quota was changed.  It there was no quota change
//    mark the waiter as succeeded.  If some quota did change, update the running quotas
// 2. If no running quota was changed, return now since no updates are needed.
// 3. for each quota that has changed, attempt an update.  If all updates succeeded, update all unset waiters to success status and return.  If the some
//    updates failed on conflict errors and we have retries left, re-get the failed quota from our cache for the latest version
//    and recurse into this method with the subset.  It's safe for us to evaluate ONLY the subset, because the other quota
//    documents for these waiters have already been evaluated.  Step 1, will mark all the ones that should already have succeeded.
func (e *quotaEvaluator) checkQuotas(quotas []api.ResourceQuota, admissionAttributes []*admissionWaiter, remainingRetries int) {
	// yet another copy to compare against originals to see if we actually have deltas
	originalQuotas := make([]api.ResourceQuota, len(quotas), len(quotas))
	copy(originalQuotas, quotas)

	atLeastOneChanged := false
	for i := range admissionAttributes {
		admissionAttribute := admissionAttributes[i]
		newQuotas, err := e.checkRequest(quotas, admissionAttribute.attributes)
		if err != nil {
			admissionAttribute.result = err
			continue
		}

		// if the new quotas are the same as the old quotas, then this particular one doesn't issue any updates
		// that means that no quota docs applied, so it can get a pass
		atLeastOneChangeForThisWaiter := false
		for j := range newQuotas {
			if !quota.Equals(originalQuotas[j].Status.Used, newQuotas[j].Status.Used) {
				atLeastOneChanged = true
				atLeastOneChangeForThisWaiter = true
				break
			}
		}

		if !atLeastOneChangeForThisWaiter {
			admissionAttribute.result = nil
		}
		quotas = newQuotas
	}

	// if none of the requests changed anything, there's no reason to issue an update, just fail them all now
	if !atLeastOneChanged {
		return
	}

	// now go through and try to issue updates.  Things get a little weird here:
	// 1. check to see if the quota changed.  If not, skip.
	// 2. if the quota changed and the update passes, be happy
	// 3. if the quota changed and the update fails, add the original to a retry list
	var updatedFailedQuotas []api.ResourceQuota
	var lastErr error
	for i := range quotas {
		newQuota := quotas[i]
		// if this quota didn't have its status changed, skip it
		if quota.Equals(originalQuotas[i].Status.Used, newQuota.Status.Used) {
			continue
		}

		clusterQuota := quotaapi.ClusterResourceQuota{}
		clusterQuota.ObjectMeta = newQuota.ObjectMeta
		clusterQuota.Status = newQuota.Status

		if updatedQuota, err := e.client.ClusterResourceQuotas().UpdateStatus(&clusterQuota); err != nil {
			updatedFailedQuotas = append(updatedFailedQuotas, newQuota)
			lastErr = err
		} else {
			// update our cache
			e.updateCache(updatedQuota)
		}
	}

	if len(updatedFailedQuotas) == 0 {
		// all the updates succeeded.  At this point, anything with the default deny error was just waiting to
		// get a successful update, so we can mark and notify
		for _, admissionAttribute := range admissionAttributes {
			if IsDefaultDeny(admissionAttribute.result) {
				admissionAttribute.result = nil
			}
		}
		return
	}

	// at this point, errors are fatal.  Update all waiters without status to failed and return
	if remainingRetries <= 0 {
		for _, admissionAttribute := range admissionAttributes {
			if IsDefaultDeny(admissionAttribute.result) {
				admissionAttribute.result = lastErr
			}
		}
		return
	}

	// this retry logic has the same bug that its possible to be checking against quota in a state that never actually exists where
	// you've added a new documented, then updated an old one, your resource matches both and you're only checking one
	// updates for these quota names failed.  Get the current quotas in the namespace, compare by name, check to see if the
	// resource versions have changed.  If not, we're going to fall through an fail everything.  If they all have, then we can try again
	newQuotas, err := e.getQuotas(quotas[0].Namespace)
	if err != nil {
		// this means that updates failed.  Anything with a default deny error has failed and we need to let them know
		for _, admissionAttribute := range admissionAttributes {
			if IsDefaultDeny(admissionAttribute.result) {
				admissionAttribute.result = lastErr
			}
		}
		return
	}

	// this logic goes through our cache to find the new version of all quotas that failed update.  If something has been removed
	// it is skipped on this retry.  After all, you removed it.
	quotasToCheck := []api.ResourceQuota{}
	for _, newQuota := range newQuotas {
		for _, oldQuota := range updatedFailedQuotas {
			if newQuota.Name == oldQuota.Name {
				quotasToCheck = append(quotasToCheck, newQuota)
				break
			}
		}
	}
	e.checkQuotas(quotasToCheck, admissionAttributes, remainingRetries-1)
}

// checkRequest verifies that the request does not exceed any quota constraint. it returns back a copy of quotas not yet persisted
// that capture what the usage would be if the request succeeded.  It return an error if the is insufficient quota to satisfy the request
func (e *quotaEvaluator) checkRequest(quotas []api.ResourceQuota, a admission.Attributes) ([]api.ResourceQuota, error) {
	namespace := a.GetNamespace()
	name := a.GetName()

	evaluators := e.registry.Evaluators()
	evaluator, found := evaluators[a.GetKind().GroupKind()]
	if !found {
		return quotas, nil
	}

	op := a.GetOperation()
	operationResources := evaluator.OperationResources(op)
	if len(operationResources) == 0 {
		return quotas, nil
	}

	// find the set of quotas that are pertinent to this request
	// reject if we match the quota, but usage is not calculated yet
	// reject if the input object does not satisfy quota constraints
	// if there are no pertinent quotas, we can just return
	inputObject := a.GetObject()
	interestingQuotaIndexes := []int{}
	for i := range quotas {
		resourceQuota := quotas[i]
		match := evaluator.Matches(&resourceQuota, inputObject)
		if !match {
			continue
		}

		hardResources := quota.ResourceNames(resourceQuota.Status.Hard)
		evaluatorResources := evaluator.MatchesResources()
		requiredResources := quota.Intersection(hardResources, evaluatorResources)
		err := evaluator.Constraints(requiredResources, inputObject)
		if err != nil {
			return nil, admission.NewForbidden(a, fmt.Errorf("Failed quota: %s: %v", resourceQuota.Name, err))
		}
		if !hasUsageStats(&resourceQuota) {
			return nil, admission.NewForbidden(a, fmt.Errorf("Status unknown for quota: %s", resourceQuota.Name))
		}

		interestingQuotaIndexes = append(interestingQuotaIndexes, i)
	}
	if len(interestingQuotaIndexes) == 0 {
		return quotas, nil
	}

	// Usage of some resources cannot be counted in isolation. For example when
	// the resource represents a number of unique references to external
	// resource. In such a case an evaluator needs to process other objects in
	// the same namespace which needs to be known.
	if accessor, err := meta.Accessor(inputObject); namespace != "" && err == nil {
		if accessor.GetNamespace() == "" {
			accessor.SetNamespace(namespace)
		}
	}

	// there is at least one quota that definitely matches our object
	// as a result, we need to measure the usage of this object for quota
	// on updates, we need to subtract the previous measured usage
	// if usage shows no change, just return since it has no impact on quota
	deltaUsage := evaluator.Usage(inputObject)
	if admission.Update == op {
		prevItem, err := evaluator.Get(namespace, name)
		if err != nil {
			return nil, admission.NewForbidden(a, fmt.Errorf("Unable to get previous: %v", err))
		}
		prevUsage := evaluator.Usage(prevItem)
		deltaUsage = quota.Subtract(deltaUsage, prevUsage)
	}
	if quota.IsZero(deltaUsage) {
		return quotas, nil
	}

	for _, index := range interestingQuotaIndexes {
		resourceQuota := quotas[index]

		hardResources := quota.ResourceNames(resourceQuota.Status.Hard)
		requestedUsage := quota.Mask(deltaUsage, hardResources)
		newUsage := quota.Add(resourceQuota.Status.Used, requestedUsage)
		if allowed, exceeded := quota.LessThanOrEqual(newUsage, resourceQuota.Status.Hard); !allowed {
			failedRequestedUsage := quota.Mask(requestedUsage, exceeded)
			failedUsed := quota.Mask(resourceQuota.Status.Used, exceeded)
			failedHard := quota.Mask(resourceQuota.Status.Hard, exceeded)
			return nil, admission.NewForbidden(a,
				fmt.Errorf("Exceeded quota: %s, requested: %s, used: %s, limited: %s",
					resourceQuota.Name,
					prettyPrint(failedRequestedUsage),
					prettyPrint(failedUsed),
					prettyPrint(failedHard)))
		}

		// update to the new usage number
		quotas[index].Status.Used = newUsage
	}

	return quotas, nil
}

func (e *quotaEvaluator) evaluate(a admission.Attributes) error {
	waiter := newAdmissionWaiter(a)

	e.addWork(waiter)

	// wait for completion or timeout
	select {
	case <-waiter.finished:
	case <-time.After(10 * time.Second):
		return fmt.Errorf("timeout")
	}

	return waiter.result
}

func (e *quotaEvaluator) addWork(a *admissionWaiter) {
	e.workLock.Lock()
	defer e.workLock.Unlock()

	ns := a.attributes.GetNamespace()
	// this Add can trigger a Get BEFORE the work is added to a list, but this is ok because the getWork routine
	// waits the worklock before retrieving the work to do, so the writes in this method will be observed
	e.queue.Add(ns)

	if e.inProgress.Has(ns) {
		e.dirtyWork[ns] = append(e.dirtyWork[ns], a)
		return
	}

	e.work[ns] = append(e.work[ns], a)
}

func (e *quotaEvaluator) completeWork(ns string) {
	e.workLock.Lock()
	defer e.workLock.Unlock()

	e.queue.Done(ns)
	e.work[ns] = e.dirtyWork[ns]
	delete(e.dirtyWork, ns)
	e.inProgress.Delete(ns)
}

func (e *quotaEvaluator) getWork() (string, []*admissionWaiter) {
	uncastNS, _ := e.queue.Get()
	ns := uncastNS.(string)

	e.workLock.Lock()
	defer e.workLock.Unlock()
	// at this point, we know we have a coherent view of e.work.  It is entirely possible
	// that our workqueue has another item requeued to it, but we'll pick it up early.  This ok
	// because the next time will go into our dirty list

	work := e.work[ns]
	delete(e.work, ns)
	delete(e.dirtyWork, ns)

	if len(work) != 0 {
		e.inProgress.Insert(ns)
		return ns, work
	}

	e.queue.Done(ns)
	e.inProgress.Delete(ns)
	return ns, []*admissionWaiter{}
}

func (e *quotaEvaluator) getQuotas(namespace string) ([]api.ResourceQuota, error) {
	// trust me to do somethign clever here with a pre-computed cache ala project cache
	cached := e.checkCache(&quotaapi.ClusterResourceQuota{ObjectMeta: api.ObjectMeta{Name: "overall"}})
	if len(cached.ResourceVersion) != 0 {
		resourceQuotas := []api.ResourceQuota{}
		// always make a copy.  We're going to muck around with this and we should never mutate the originals
		clusterQuota := cached
		item := api.ResourceQuota{}
		item.ObjectMeta = clusterQuota.ObjectMeta
		item.Namespace = namespace
		item.Spec = clusterQuota.Spec.Quota
		item.Status = clusterQuota.Status

		if len(item.Status.Hard) == 0 {
			item.Status.Hard = item.Spec.Hard
		}
		if len(item.Status.Used) == 0 {
			item.Status.Used = api.ResourceList{}
			for name := range item.Spec.Hard {
				item.Status.Used[name] = resource.MustParse("0")
			}
		}

		resourceQuotas = append(resourceQuotas, item)

		return resourceQuotas, nil
	}

	liveList, err := e.client.ClusterResourceQuotas().List(api.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("Error resolving quota.")
	}

	resourceQuotas := []api.ResourceQuota{}
	for i := range liveList.Items {
		// always make a copy.  We're going to muck around with this and we should never mutate the originals
		clusterQuota := liveList.Items[i]
		item := api.ResourceQuota{}
		item.ObjectMeta = clusterQuota.ObjectMeta
		item.Namespace = namespace
		item.Spec = clusterQuota.Spec.Quota
		item.Status = clusterQuota.Status

		if len(item.Status.Hard) == 0 {
			item.Status.Hard = item.Spec.Hard
		}
		if len(item.Status.Used) == 0 {
			item.Status.Used = api.ResourceList{}
			for name := range item.Spec.Hard {
				item.Status.Used[name] = resource.MustParse("0")
			}
		}

		resourceQuotas = append(resourceQuotas, item)
	}

	return resourceQuotas, nil
}

func (e *quotaEvaluator) updateCache(quota *quotaapi.ClusterResourceQuota) {
	e.updatedQuotasLock.Lock()
	defer e.updatedQuotasLock.Unlock()

	key := quota.Namespace + "/" + quota.Name
	e.updatedQuotas[key] = quota
}

// checkCache compares the passes quota against the value in the look-aside cache and returns the newer
// if the cache is out of date, it deletes the stale entry.  This only works because of etcd resourceVersions
// being monotonically increasing integers
func (e *quotaEvaluator) checkCache(quota *quotaapi.ClusterResourceQuota) *quotaapi.ClusterResourceQuota {
	e.updatedQuotasLock.Lock()
	defer e.updatedQuotasLock.Unlock()

	key := quota.Namespace + "/" + quota.Name
	cachedQuota, exists := e.updatedQuotas[key]
	if !exists {
		return quota
	}

	return cachedQuota
}
