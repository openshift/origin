package clusterresourcequota

import (
	"errors"
	"io"
	"sync"
	"time"

	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"

	"k8s.io/kubernetes/pkg/admission"
	"k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/quota/install"
	utilwait "k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/plugin/pkg/admission/resourcequota"

	oclient "github.com/openshift/origin/pkg/client"
	ocache "github.com/openshift/origin/pkg/client/cache"
	oadmission "github.com/openshift/origin/pkg/cmd/server/admission"
	"github.com/openshift/origin/pkg/controller/shared"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
)

func init() {
	admission.RegisterPlugin("ClusterResourceQuota",
		func(client clientset.Interface, config io.Reader) (admission.Interface, error) {
			registry := install.NewRegistry(client)
			// TODO: expose a stop channel in admission factory
			return NewClusterResourceQuota(registry)
		})
}

// quotaAdmission implements an admission controller that can enforce quota constraints
type quotaAdmission struct {
	*admission.Handler

	// these are used to create the accessor
	quotaLister     *ocache.IndexerToClusterResourceQuotaLister
	namespaceLister *ocache.IndexerToNamespaceLister
	quotaSynced     func() bool
	namespaceSynced func() bool
	quotaClient     oclient.ClusterResourceQuotasInterface
	quotaMapper     clusterquotamapping.ClusterQuotaMapper

	// these are used to create the evaluator
	registry quota.Registry

	init      sync.Once
	evaluator resourcequota.Evaluator
}

var _ oadmission.WantsInformers = &quotaAdmission{}
var _ oadmission.WantsOpenshiftClient = &quotaAdmission{}
var _ oadmission.WantsClusterQuotaMapper = &quotaAdmission{}
var _ oadmission.Validator = &quotaAdmission{}

// NewResourceQuota configures an admission controller that can enforce quota constraints
// using the provided registry.  The registry must have the capability to handle group/kinds that
// are persisted by the server this admission controller is intercepting
func NewClusterResourceQuota(registry quota.Registry) (admission.Interface, error) {
	return &quotaAdmission{
		Handler:  admission.NewHandler(admission.Create),
		registry: registry,
	}, nil
}

// Admit makes admission decisions while enforcing quota
func (q *quotaAdmission) Admit(a admission.Attributes) (err error) {
	// ignore all operations that correspond to sub-resource actions
	if len(a.GetSubresource()) != 0 {
		return nil
	}
	// ignore cluster level resources
	if len(a.GetNamespace()) == 0 {
		return nil
	}

	if !q.waitForSyncedStore(time.After(10 * time.Second)) {
		return admission.NewForbidden(a, errors.New("caches not synchronized"))
	}

	q.init.Do(func() {
		quotaAccessor := newQuotaAccessor(q.quotaLister, q.namespaceLister, q.quotaClient, q.quotaMapper)
		q.evaluator = resourcequota.NewQuotaEvaluator(quotaAccessor, q.registry, nil, 10, utilwait.NeverStop)
	})

	return q.evaluator.Evaluate(a)
}

func (q *quotaAdmission) waitForSyncedStore(timeout <-chan time.Time) bool {
	for !q.quotaSynced() || !q.namespaceSynced() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-timeout:
			return q.quotaSynced() && q.namespaceSynced()
		}
	}

	return true
}

func (q *quotaAdmission) SetInformers(informers shared.InformerFactory) {
	q.quotaLister = informers.ClusterResourceQuotas().Lister()
	q.quotaSynced = informers.ClusterResourceQuotas().Informer().HasSynced
	q.namespaceLister = informers.Namespaces().Lister()
	q.namespaceSynced = informers.Namespaces().Informer().HasSynced
}

func (q *quotaAdmission) SetOpenshiftClient(client oclient.Interface) {
	q.quotaClient = client
}

func (q *quotaAdmission) SetClusterQuotaMapper(clusterQuotaMapper clusterquotamapping.ClusterQuotaMapper) {
	q.quotaMapper = clusterQuotaMapper
}

func (q *quotaAdmission) Validate() error {
	if q.quotaLister == nil {
		return errors.New("missing quotaLister")
	}
	if q.namespaceLister == nil {
		return errors.New("missing namespaceLister")
	}
	if q.quotaClient == nil {
		return errors.New("missing quotaClient")
	}
	if q.quotaMapper == nil {
		return errors.New("missing quotaMapper")
	}

	return nil
}
