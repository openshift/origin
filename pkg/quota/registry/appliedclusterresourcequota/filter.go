package appliedclusterresourcequota

import (
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
	kcorelisters "k8s.io/kubernetes/pkg/client/listers/core/internalversion"

	oapi "github.com/openshift/origin/pkg/api"
	quotaapi "github.com/openshift/origin/pkg/quota/apis/quota"
	"github.com/openshift/origin/pkg/quota/controller/clusterquotamapping"
	quotalister "github.com/openshift/origin/pkg/quota/generated/listers/quota/internalversion"
)

type AppliedClusterResourceQuotaREST struct {
	quotaMapper     clusterquotamapping.ClusterQuotaMapper
	quotaLister     quotalister.ClusterResourceQuotaLister
	namespaceLister kcorelisters.NamespaceLister
}

func NewREST(quotaMapper clusterquotamapping.ClusterQuotaMapper, quotaLister quotalister.ClusterResourceQuotaLister, namespaceLister kcorelisters.NamespaceLister) *AppliedClusterResourceQuotaREST {
	return &AppliedClusterResourceQuotaREST{
		quotaMapper:     quotaMapper,
		quotaLister:     quotaLister,
		namespaceLister: namespaceLister,
	}
}

var _ rest.Getter = &AppliedClusterResourceQuotaREST{}
var _ rest.Lister = &AppliedClusterResourceQuotaREST{}

func (r *AppliedClusterResourceQuotaREST) New() runtime.Object {
	return &quotaapi.AppliedClusterResourceQuota{}
}

func (r *AppliedClusterResourceQuotaREST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace is required")
	}

	quotaNames, _ := r.quotaMapper.GetClusterQuotasFor(namespace)
	quotaNamesSet := sets.NewString(quotaNames...)
	if !quotaNamesSet.Has(name) {
		return nil, kapierrors.NewNotFound(quotaapi.Resource("appliedclusterresourcequota"), name)
	}

	clusterQuota, err := r.quotaLister.Get(name)
	if err != nil {
		return nil, err
	}

	return quotaapi.ConvertClusterResourceQuotaToAppliedClusterResourceQuota(clusterQuota), nil
}

func (r *AppliedClusterResourceQuotaREST) NewList() runtime.Object {
	return &quotaapi.AppliedClusterResourceQuotaList{}
}

func (r *AppliedClusterResourceQuotaREST) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("namespace is required")
	}

	// TODO max resource version?  watch?
	list := &quotaapi.AppliedClusterResourceQuotaList{}
	matcher := matcher(oapi.InternalListOptionsToSelectors(options))
	quotaNames, _ := r.quotaMapper.GetClusterQuotasFor(namespace)

	for _, name := range quotaNames {
		quota, err := r.quotaLister.Get(name)
		if err != nil {
			continue
		}
		if matches, err := matcher.Matches(quota); err != nil || !matches {
			continue
		}
		list.Items = append(list.Items, *quotaapi.ConvertClusterResourceQuotaToAppliedClusterResourceQuota(quota))
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
