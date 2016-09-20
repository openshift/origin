package clusterresourcequota

import (
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/cache"
	"k8s.io/kubernetes/pkg/quota"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	oclient "github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

func init() {
	admission.RegisterPlugin("openshift.io/ClusterResourceQuota",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			return NewClusterResourceQuota()
		})
}

// clusterQuotaAdmission implements an admission controller that can enforce clusterQuota constraints
type clusterQuotaAdmission struct {
	*admission.Handler

	// these are used to create the accessor
	clusterQuotaLister *ocache.IndexerToClusterResourceQuotaLister
	namespaceLister    *cache.IndexerToNamespaceLister
	clusterQuotaSynced func() bool
	namespaceSynced    func() bool
	clusterQuotaClient oclient.ClusterResourceQuotasInterface
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper

	lockFactory LockFactory

	// these are used to create the evaluator
	registry quota.Registry

	init      sync.Once
	evaluator resourcequota.Evaluator
}

var _ oadmission.WantsInformers = &clusterQuotaAdmission{}
var _ oadmission.WantsOpenshiftClient = &clusterQuotaAdmission{}
var _ oadmission.WantsClusterQuotaMapper = &clusterQuotaAdmission{}
var _ oadmission.Validator = &clusterQuotaAdmission{}

const (
	timeToWaitForCacheSync = 10 * time.Second
	numEvaluatorThreads    = 10
)

// NewClusterResourceQuota configures an admission controller that can enforce clusterQuota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewClusterResourceQuota() (admission.Interface, error) {
	return &clusterQuotaAdmission{
		Handler:     admission.NewHandler(admission.Create, admission.Update),
		lockFactory: NewDefaultLockFactory(),
	}, nil
}

// Admit makes admission decisions while enforcing clusterQuota
func (q *clusterQuotaAdmission) Admit(a admission.Attributes) (err error) {
	// ignore all operations that correspond to sub-resource actions
	if len(a.GetSubresource()) != 0 {
		return nil
	}
	// ignore cluster level resources
	if len(a.GetNamespace()) == 0 {
		return nil
	}

	if !q.waitForSyncedStore(time.After(timeToWaitForCacheSync)) {
		return admission.NewForbidden(a, errors.New("caches not synchronized"))
	}

	q.init.Do(func() {
		clusterQuotaAccessor := newQuotaAccessor(q.clusterQuotaLister, q.namespaceLister, q.clusterQuotaClient, q.clusterQuotaMapper)
		q.evaluator = resourcequota.NewQuotaEvaluator(clusterQuotaAccessor, q.registry, q.lockAquisition, numEvaluatorThreads, utilwait.NeverStop)
	})

	return q.evaluator.Evaluate(a)
}

func (q *clusterQuotaAdmission) lockAquisition(quotas []kapi.ResourceQuota) func() {
	locks := []sync.Locker{}

	// acquire the locks in alphabetical order because I'm too lazy to think of something clever
	sort.Sort(ByName(quotas))
	for _, quota := range quotas {
		lock := q.lockFactory.GetLock(quota.Name)
		lock.Lock()
		locks = append(locks, lock)
	}

	return func() {
		for i := len(locks) - 1; i >= 0; i-- {
			locks[i].Unlock()
		}
	}
}

func (q *clusterQuotaAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !q.clusterQuotaSynced() || !q.namespaceSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return q.clusterQuotaSynced() && q.namespaceSynced()
		}
	}

	return true
}

func (q *clusterQuotaAdmission) SetOriginQuotaRegistry(registry quota.Registry) {
	q.registry = registry
}

func (q *clusterQuotaAdmission) SetInformers(informers shared.InformerFactory) {
	q.clusterQuotaLister = informers.ClusterResourceQuotas().Lister()
	q.clusterQuotaSynced = informers.ClusterResourceQuotas().Informer().HasSynced
	q.namespaceLister = informers.Namespaces().Lister()
	q.namespaceSynced = informers.Namespaces().Informer().HasSynced
}

func (q *clusterQuotaAdmission) SetOpenshiftClient(client oclient.Interface) {
	q.clusterQuotaClient = client
}

func (q *clusterQuotaAdmission) SetClusterQuotaMapper(clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper) {
	q.clusterQuotaMapper = clusterQuotaMapper
}

func (q *clusterQuotaAdmission) Validate() error {
	if q.clusterQuotaLister == nil {
		return errors.New("missing clusterQuotaLister")
	}
	if q.namespaceLister == nil {
		return errors.New("missing namespaceLister")
	}
	if q.clusterQuotaClient == nil {
		return errors.New("missing clusterQuotaClient")
	}
	if q.clusterQuotaMapper == nil {
		return errors.New("missing clusterQuotaMapper")
	}
	if q.registry == nil {
		return errors.New("missing registry")
	}

	return nil
}

type ByName []kapi.ResourceQuota

func (v ByName) Len() int           { return len(v) }
func (v ByName) Swap(i, j int)      { v[i], v[j] = v[j], v[i] }
func (v ByName) Less(i, j int) bool { return v[i].Name < v[j].Name }
