package fake

import (
	api "github.com/openshift/origin/pkg/security/api"
	pkg_api "k8s.io/kubernetes/pkg/api"
	unversioned "k8s.io/kubernetes/pkg/api/unversioned"
	core "k8s.io/kubernetes/pkg/client/testing/core"
	watch "k8s.io/kubernetes/pkg/watch"
)

// FakePodSecurityPolicySubjectReviews implements PodSecurityPolicySubjectReviewInterface
type FakePodSecurityPolicySubjectReviews struct {
	Fake *FakeCore
	ns   string
}

var podsecuritypolicysubjectreviewsResource = unversioned.GroupVersionResource{Group: "", Version: "", Resource: "podsecuritypolicysubjectreviews"}

func (c *FakePodSecurityPolicySubjectReviews) Create(podSecurityPolicySubjectReview *api.PodSecurityPolicySubjectReview) (result *api.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewCreateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &api.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) Update(podSecurityPolicySubjectReview *api.PodSecurityPolicySubjectReview) (result *api.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewUpdateAction(podsecuritypolicysubjectreviewsResource, c.ns, podSecurityPolicySubjectReview), &api.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) Delete(name string, options *pkg_api.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(core.NewDeleteAction(podsecuritypolicysubjectreviewsResource, c.ns, name), &api.PodSecurityPolicySubjectReview{})

	return err
}

func (c *FakePodSecurityPolicySubjectReviews) DeleteCollection(options *pkg_api.DeleteOptions, listOptions pkg_api.ListOptions) error {
	action := core.NewDeleteCollectionAction(podsecuritypolicysubjectreviewsResource, c.ns, listOptions)

	_, err := c.Fake.Invokes(action, &api.PodSecurityPolicySubjectReviewList{})
	return err
}

func (c *FakePodSecurityPolicySubjectReviews) Get(name string) (result *api.PodSecurityPolicySubjectReview, err error) {
	obj, err := c.Fake.
		Invokes(core.NewGetAction(podsecuritypolicysubjectreviewsResource, c.ns, name), &api.PodSecurityPolicySubjectReview{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PodSecurityPolicySubjectReview), err
}

func (c *FakePodSecurityPolicySubjectReviews) List(opts pkg_api.ListOptions) (result *api.PodSecurityPolicySubjectReviewList, err error) {
	obj, err := c.Fake.
		Invokes(core.NewListAction(podsecuritypolicysubjectreviewsResource, c.ns, opts), &api.PodSecurityPolicySubjectReviewList{})

	if obj == nil {
		return nil, err
	}
	return obj.(*api.PodSecurityPolicySubjectReviewList), err
}

// Watch returns a watch.Interface that watches the requested podSecurityPolicySubjectReviews.
func (c *FakePodSecurityPolicySubjectReviews) Watch(opts pkg_api.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(core.NewWatchAction(podsecuritypolicysubjectreviewsResource, c.ns, opts))

}
