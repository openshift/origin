package client

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/authorization/api"
)

type ClusterResourceQuotasInterface interface {
	ClusterResourceQuotas() ClusterResourceQuotaInterface
}

type ClusterResourceQuotaInterface interface {
	List(opts kapi.ListOptions) (*api.ClusterResourceQuotaList, error)
	Get(name string) (*api.ClusterResourceQuota, error)
	Create(clusterResourceQuota *api.ClusterResourceQuota) (*api.ClusterResourceQuota, error)
	Update(clusterResourceQuota *api.ClusterResourceQuota) (*api.ClusterResourceQuota, error)
	Delete(name string) error
	Watch(opts kapi.ListOptions) (watch.Interface, error)
	UpdateStatus(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error)
}

type clusterResourceQuotas struct {
	r *Client
}

func newClusterResourceQuotas(c *Client) *clusterResourceQuotas {
	return &clusterResourceQuotas{
		r: c,
	}
}

func (c *clusterResourceQuotas) List(opts kapi.ListOptions) (result *api.ClusterResourceQuotaList, err error) {
	result = &api.ClusterResourceQuotaList{}
	err = c.r.Get().
		Resource("clusterResourceQuotas").
		VersionedParams(&opts, kapi.ParameterCodec).
		Do().
		Into(result)
	return
}

func (c *clusterResourceQuotas) Get(name string) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.r.Get().Resource("clusterResourceQuotas").Name(name).Do().Into(result)
	return
}

func (c *clusterResourceQuotas) Create(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.r.Post().Resource("clusterResourceQuotas").Body(clusterResourceQuota).Do().Into(result)
	return
}

func (c *clusterResourceQuotas) Update(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.r.Put().Resource("clusterResourceQuotas").Name(clusterResourceQuota.Name).Body(clusterResourceQuota).Do().Into(result)
	return
}

func (c *clusterResourceQuotas) Delete(name string) error {
	return c.r.Delete().Resource("clusterResourceQuotas").Name(name).Do().Error()
}

func (c *clusterResourceQuotas) Watch(opts kapi.ListOptions) (watch.Interface, error) {
	return c.r.Get().
		Prefix("watch").
		Resource("clusterResourceQuotas").
		VersionedParams(&opts, kapi.ParameterCodec).
		Watch()
}

func (c *clusterResourceQuotas) UpdateStatus(clusterResourceQuota *api.ClusterResourceQuota) (result *api.ClusterResourceQuota, err error) {
	result = &api.ClusterResourceQuota{}
	err = c.r.Put().Resource("clusterResourceQuotas").Name(clusterResourceQuota.Name).SubResource("status").Body(clusterResourceQuota).Do().Into(result)
	return
}
