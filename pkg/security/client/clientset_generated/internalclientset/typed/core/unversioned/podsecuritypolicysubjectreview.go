package unversioned

import (
	api "github.com/openshift/origin/pkg/security/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// PodSecurityPolicySubjectReviewsGetter has a method to return a PodSecurityPolicySubjectReviewInterface.
// A group's client should implement this interface.
type PodSecurityPolicySubjectReviewsGetter interface {
	PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface
}

// PodSecurityPolicySubjectReviewInterface has methods to work with PodSecurityPolicySubjectReview resources.
type PodSecurityPolicySubjectReviewInterface interface {
	Create(*api.PodSecurityPolicySubjectReview) (*api.PodSecurityPolicySubjectReview, error)
	Update(*api.PodSecurityPolicySubjectReview) (*api.PodSecurityPolicySubjectReview, error)
	Delete(name string, options *pkg_api.DeleteOptions) error
	DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error
	Get(name string) (*api.PodSecurityPolicySubjectReview, error)
	List(opts pkg_api.ListOptions) (*api.PodSecurityPolicySubjectReviewList, error)
	Watch(opts pkg_api.ListOptions) (watch.Interface, error)
	PodSecurityPolicySubjectReviewExpansion
}

// podSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type podSecurityPolicySubjectReviews struct {
	client *CoreClient
	ns     string
}

// newPodSecurityPolicySubjectReviews returns a PodSecurityPolicySubjectReviews
func newPodSecurityPolicySubjectReviews(c *CoreClient, namespace string) *podSecurityPolicySubjectReviews {
	return &podSecurityPolicySubjectReviews{
		client: c,
		ns:     namespace,
	}
}

// Create takes the representation of a podSecurityPolicySubjectReview and creates it.  Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *podSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *api.PodSecurityPolicySubjectReview) (result *api.PodSecurityPolicySubjectReview, err error) {
	result = &api.PodSecurityPolicySubjectReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}

// Update takes the representation of a podSecurityPolicySubjectReview and updates it. Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *podSecurityPolicySubjectReviews) Update(podSecurityPolicySubjectReview *api.PodSecurityPolicySubjectReview) (result *api.PodSecurityPolicySubjectReview, err error) {
	result = &api.PodSecurityPolicySubjectReview{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(podSecurityPolicySubjectReview.Name).
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}

// Delete takes name of the podSecurityPolicySubjectReview and deletes it. Returns an error if one occurs.
func (c *podSecurityPolicySubjectReviews) Delete(name string, options *pkg_api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *podSecurityPolicySubjectReviews) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&listOptions, pkg_api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the podSecurityPolicySubjectReview, and returns the corresponding podSecurityPolicySubjectReview object, and an error if there is any.
func (c *podSecurityPolicySubjectReviews) Get(name string) (result *api.PodSecurityPolicySubjectReview, err error) {
	result = &api.PodSecurityPolicySubjectReview{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PodSecurityPolicySubjectReviews that match those selectors.
func (c *podSecurityPolicySubjectReviews) List(opts pkg_api.ListOptions) (result *api.PodSecurityPolicySubjectReviewList, err error) {
	result = &api.PodSecurityPolicySubjectReviewList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested podSecurityPolicySubjectReviews.
func (c *podSecurityPolicySubjectReviews) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&opts, pkg_api.ParameterCodec).
		Watch()
}
