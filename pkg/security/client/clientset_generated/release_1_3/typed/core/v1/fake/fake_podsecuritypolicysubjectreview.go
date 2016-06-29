package fake

import (
	v1 "github.com/openshift/origin/pkg/security/api/v1"
	api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakePodSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type FakePodSecurityPolicySubjectReviews struct {
	Fake *FakeCore
	ns   string
}

var podsecuritypolicysubjectreviewsResource = unversioned.GroupVersionResource{Group: "", Version: "v1", Resource: "podsecuritypolicysubjectreviews"}

func (c *FakePodSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &v1.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) Update(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (result *v1.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &v1.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) UpdateStatus(podSecurityPolicySubjectReview *v1.PodSecurityPolicySubjectReview) (*v1.PodSecurityPolicySubjectReview, error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateSubresourceAction(podsecuritypolicysubjectreviewsResource, "status", c.ns, podSecurityPolicySubjectReview), &v1.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) Delete(name string, options *api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(podsecuritypolicysubjectreviewsResource, c.ns, name), &v1.PodSecurityPolicySubjectReview{})

	return err
}

func (c *FakePodSecurityPolicySubjectReviews) DeleteCollection(options *api.DeleteOptions, listOptions api.ListOptions) error {
	action := core.NewDeleteCollectionAction(podsecuritypolicysubjectreviewsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &v1.PodSecurityPolicySubjectReviewList{})
	return err
}

func (c *FakePodSecurityPolicySubjectReviews) Get(name string) (result *v1.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(podsecuritypolicysubjectreviewsResource, c.ns, name), &v1.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) List(opts api.ListOptions) (result *v1.PodSecurityPolicySubjectReviewList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(podsecuritypolicysubjectreviewsResource, c.ns, opts), &v1.PodSecurityPolicySubjectReviewList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1.PodSecurityPolicySubjectReviewList), err
}

// Watch returns a watch.Interface that watches the requested podSecurityPolicySubjectReviews.
func (c *FakePodSecurityPolicySubjectReviews) Watch(opts api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(podsecuritypolicysubjectreviewsResource, c.ns, opts))

}
