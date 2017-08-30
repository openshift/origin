package internalversion

import (
	quota "github.com/openshift/origin/pkg/quota/apis/quota"
	scheme "github.com/openshift/origin/pkg/quota/generated/internalclientset/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	rest "k8s.io/client-go/rest"
)

// AppliedClusterResourceQuotasGetter has a method to return a AppliedClusterResourceQuotaInterface.
// A group's client should implement this interface.
type AppliedClusterResourceQuotasGetter interface {
	AppliedClusterResourceQuotas(namespace string) AppliedClusterResourceQuotaInterface
}

// AppliedClusterResourceQuotaInterface has methods to work with AppliedClusterResourceQuota resources.
type AppliedClusterResourceQuotaInterface interface {
	Get(name string, options v1.GetOptions) (*quota.AppliedClusterResourceQuota, error)
	List(opts v1.ListOptions) (*quota.AppliedClusterResourceQuotaList, error)
	AppliedClusterResourceQuotaExpansion
}

// appliedClusterResourceQuotas implements AppliedClusterResourceQuotaInterface
type appliedClusterResourceQuotas struct {
	client rest.Interface
	ns     string
}

// newAppliedClusterResourceQuotas returns a AppliedClusterResourceQuotas
func newAppliedClusterResourceQuotas(c *QuotaClient, namespace string) *appliedClusterResourceQuotas {
	return &appliedClusterResourceQuotas{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the appliedClusterResourceQuota, and returns the corresponding appliedClusterResourceQuota object, and an error if there is any.
func (c *appliedClusterResourceQuotas) Get(name string, options v1.GetOptions) (result *quota.AppliedClusterResourceQuota, err error) {
	result = &quota.AppliedClusterResourceQuota{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("appliedclusterresourcequotas").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of AppliedClusterResourceQuotas that match those selectors.
func (c *appliedClusterResourceQuotas) List(opts v1.ListOptions) (result *quota.AppliedClusterResourceQuotaList, err error) {
	result = &quota.AppliedClusterResourceQuotaList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("appliedclusterresourcequotas").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}
