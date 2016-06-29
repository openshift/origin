package v1

import (
	v1 "github.com/openshift/origin/pkg/security/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	watch "k8s.io/kubernetes/pkg/watch"
)

// PodSecurityPolicySubjectReviewsGetter has a method to return a PodSecurityPolicySubjectReviewInterface.
// A group's client should implement this interface.
type PodSecurityPolicySubjectReviewsGetter interface {
	PodSecurityPolicySubjectReviews(namespace string) PodSecurityPolicySubjectReviewInterface
}

// PodSecurityPolicySubjectReviewInterface has methods to work with PodSecurityPolicySubjectReview resources.
type PodSecurityPolicySubjectReviewInterface interface {
	Create(*v1.PodSecurityPolicySubjectReview) (*v1.PodSecurityPolicySubjectReview, error)
	Update(*v1.PodSecurityPolicySubjectReview) (*v1.PodSecurityPolicySubjectReview, error)
	UpdateStatus(*v1.PodSecurityPolicySubjectReview) (*v1.PodSecurityPolicySubjectReview, error)
	Delete(name string, options *api.DeleteOptions) error
	DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error
	Get(name string) (*v1.PodSecurityPolicySubjectReview, error)
	List(opts api.ListOptions) (*v1.PodSecurityPolicySubjectReviewList, error)
	Watch(opts api.ListOptions) (watch.Interface, error)
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
func (c *podSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	result = &v1.PodSecurityPolicySubjectReview{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}

// Update takes the representation of a podSecurityPolicySubjectReview and updates it. Returns the server's representation of the podSecurityPolicySubjectReview, and an error, if there is any.
func (c *podSecurityPolicySubjectReviews) Update(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	result = &v1.PodSecurityPolicySubjectReview{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(podSecurityPolicySubjectReview.Name).
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}

func (c *podSecurityPolicySubjectReviews) UpdateStatus(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	result = &v1.PodSecurityPolicySubjectReview{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(podSecurityPolicySubjectReview.Name).
		SubResource("status").
		Body(podSecurityPolicySubjectReview).
		Do().
		Into(result)
	return
}

// Delete takes name of the podSecurityPolicySubjectReview and deletes it. Returns an error if one occurs.
func (c *podSecurityPolicySubjectReviews) Delete(name string, options *api.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *podSecurityPolicySubjectReviews) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&listOptions, api.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Get takes name of the podSecurityPolicySubjectReview, and returns the corresponding podSecurityPolicySubjectReview object, and an error if there is any.
func (c *podSecurityPolicySubjectReviews) Get(name string) (result *v1.PodSecurityPolicySubjectReview, err error) {
	result = &v1.PodSecurityPolicySubjectReview{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		Name(name).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of PodSecurityPolicySubjectReviews that match those selectors.
func (c *podSecurityPolicySubjectReviews) List(opts api.ListOptions) (result *v1.PodSecurityPolicySubjectReviewList, err error) {
	result = &v1.PodSecurityPolicySubjectReviewList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&opts, api.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested podSecurityPolicySubjectReviews.
func (c *podSecurityPolicySubjectReviews) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.client.Get().
		Prefix("watch").
		Namespace(c.ns).
		Resource("podsecuritypolicysubjectreviews").
		VersionedParams(&opts, api.ParameterCodec).
		Watch()
}
