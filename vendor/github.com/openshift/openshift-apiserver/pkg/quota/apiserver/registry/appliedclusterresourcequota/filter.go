package appliedclusterresourcequota

import (
	"context"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	"github.com/openshift/api/quota"
	quotalister "github.com/openshift/client-go/quota/listers/quota/v1"
	"github.com/openshift/library-go/pkg/quota/clusterquotamapping"
	"github.com/openshift/openshift-apiserver/pkg/api/apihelpers"
	printersinternal "github.com/openshift/openshift-apiserver/pkg/printers/internalversion"
	quotaapi "github.com/openshift/openshift-apiserver/pkg/quota/apis/quota"
	quotav1conversions "github.com/openshift/openshift-apiserver/pkg/quota/apis/quota/v1"
)

type AppliedClusterResourceQuotaREST struct {
	quotaMapper clusterquotamapping.ClusterQuotaMapper
	quotaLister quotalister.ClusterResourceQuotaLister
	rest.TableConvertor
}

func NewREST(quotaMapper clusterquotamapping.ClusterQuotaMapper, quotaLister quotalister.ClusterResourceQuotaLister) *AppliedClusterResourceQuotaREST {
	return &AppliedClusterResourceQuotaREST{
		quotaMapper:    quotaMapper,
		quotaLister:    quotaLister,
		TableConvertor: printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},
	}
}

var _ rest.Getter = &AppliedClusterResourceQuotaREST{}
var _ rest.Lister = &AppliedClusterResourceQuotaREST{}
var _ rest.Scoper = &AppliedClusterResourceQuotaREST{}

func (r *AppliedClusterResourceQuotaREST) New() runtime.Object {
	return &quotaapi.AppliedClusterResourceQuota{}
}

func (s *AppliedClusterResourceQuotaREST) NamespaceScoped() bool {
	return true
}

func (r *AppliedClusterResourceQuotaREST) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace is required")
	}

	quotaNames, _ := r.quotaMapper.GetClusterQuotasFor(namespace)
	quotaNamesSet := sets.NewString(quotaNames...)
	if !quotaNamesSet.Has(name) {
		return nil, kapierrors.NewNotFound(quota.Resource("appliedclusterresourcequota"), name)
	}

	clusterQuota, err := r.quotaLister.Get(name)
	if err != nil {
		return nil, err
	}

	return quotaapi.ConvertV1ClusterResourceQuotaToV1AppliedClusterResourceQuota(clusterQuota), nil
}

func (r *AppliedClusterResourceQuotaREST) NewList() runtime.Object {
	return &quotaapi.AppliedClusterResourceQuotaList{}
}

func (r *AppliedClusterResourceQuotaREST) List(ctx context.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace is required")
	}

	// TODO max resource version?  watch?
	list := &quotaapi.AppliedClusterResourceQuotaList{}
	matcher := matcher(apihelpers.InternalListOptionsToSelectors(options))
	quotaNames, _ := r.quotaMapper.GetClusterQuotasFor(namespace)

	for _, name := range quotaNames {
		quota, err := r.quotaLister.Get(name)
		if err != nil {
			continue
		}
		if matches, err := matcher.Matches(quota); err != nil || !matches {
			continue
		}
		internalAppliedQuota, err := quotav1conversions.ConvertV1ClusterResourceQuotaToInternalAppliedClusterResourceQuota(quota)
		if err != nil {
			return nil, err
		}
		list.Items = append(list.Items, *internalAppliedQuota)
	}

	return list, nil
}

// Matcher returns a generic matcher for a given label and field selector.
func matcher(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: storage.DefaultClusterScopedAttr,
	}
}
