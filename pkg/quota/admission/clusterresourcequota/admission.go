package clusterresourcequota

import (
	"errors"
	"io"
	"sort"
	"sync"
	"time"

	utilwait "k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/admission"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kinternalinformers "k8s.io/kubernetes/pkg/client/informers/informers_generated/internalversion"
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"
	resourcequotaapi "k8s.io/kubernetes/plugin/pkg/admission/resourcequota/apis/resourcequota"

	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotainformer "github.com/openshift/origin/pkg/quota/generated/informers/internalversion/quota/internalversion"
	quotaclient "github.com/openshift/origin/pkg/quota/generated/internalclientset"
	quotatypedclient "github.com/openshift/origin/pkg/quota/generated/internalclientset/typed/quota/internalversion"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
)

func Register(plugins *admission.Plugins) {
	plugins.Register("openshift.io/ClusterResourceQuota",
		func(config io.Reader) (admission.Interface, error) {
			return NewClusterResourceQuota()
		})
}

// clusterQuotaAdmission implements an admission controller that can enforce clusterQuota constraints
type clusterQuotaAdmission struct {
	*admission.Handler

	// these are used to create the accessor
	clusterQuotaLister quotalister.ClusterResourceQuotaLister
	namespaceLister    kcorelisters.NamespaceLister
	clusterQuotaSynced func() bool
	namespaceSynced    func() bool
	clusterQuotaClient quotatypedclient.ClusterResourceQuotasGetter
	clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper

	lockFactory LockFactory

	// these are used to create the evaluator
	registry quota.Registry

	init      sync.Once
	evaluator resourcequota.Evaluator
}

var _ oadmission.WantsInternalKubernetesInformers = &clusterQuotaAdmission{}
var _ oadmission.WantsOpenshiftInternalQuotaClient = &clusterQuotaAdmission{}
var _ oadmission.WantsClusterQuota = &clusterQuotaAdmission{}

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
		q.evaluator = resourcequota.NewQuotaEvaluator(clusterQuotaAccessor, install.DefaultIgnoredResources(), q.registry, q.lockAquisition, &resourcequotaapi.Configuration{}, numEvaluatorThreads, utilwait.NeverStop)
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

func (q *clusterQuotaAdmission) SetInternalKubernetesInformers(informers kinternalinformers.SharedInformerFactory) {
	q.namespaceLister = informers.Core().InternalVersion().Namespaces().Lister()
	q.namespaceSynced = informers.Core().InternalVersion().Namespaces().Informer().HasSynced
}

func (q *clusterQuotaAdmission) SetOpenshiftInternalQuotaClient(client quotaclient.Interface) {
	q.clusterQuotaClient = client.Quota()
}

func (q *clusterQuotaAdmission) SetClusterQuota(clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper, informers quotainformer.ClusterResourceQuotaInformer) {
	q.clusterQuotaMapper = clusterQuotaMapper
	q.clusterQuotaLister = informers.Lister()
	q.clusterQuotaSynced = informers.Informer().HasSynced
}

func (q *clusterQuotaAdmission) ValidateInitialization() error {
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
