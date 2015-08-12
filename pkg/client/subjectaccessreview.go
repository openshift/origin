package client

import (
	"fmt"

	"k8s.io/kubernetes/pkg/client"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

// SubjectAccessReviewsNamespacer has methods to work with SubjectAccessReview resources in a namespace
type SubjectAccessReviewsNamespacer interface {
	SubjectAccessReviews(namespace string) SubjectAccessReviewInterface
}

type SubjectAccessReviewsImpersonator interface {
	ImpersonateSubjectAccessReviews(namespace, token string) SubjectAccessReviewInterface
}

// ClusterSubjectAccessReviews has methods to work with SubjectAccessReview resources in the cluster scope
type ClusterSubjectAccessReviews interface {
	ClusterSubjectAccessReviews() SubjectAccessReviewInterface
}

// SubjectAccessReviewInterface exposes methods on SubjectAccessReview resources.
type SubjectAccessReviewInterface interface {
	Create(policy *authorizationapi.SubjectAccessReview) (*authorizationapi.SubjectAccessReviewResponse, error)
}

// subjectAccessReviews implements SubjectAccessReviewsNamespacer interface
type subjectAccessReviews struct {
	r     *Client
	ns    string
	token string
}

// newSubjectAccessReviews returns a subjectAccessReviews
func newSubjectAccessReviews(c *Client, namespace, token string) *subjectAccessReviews {
	return &subjectAccessReviews{
		r:     c,
		ns:    namespace,
		token: token,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *subjectAccessReviews) Create(policy *authorizationapi.SubjectAccessReview) (result *authorizationapi.SubjectAccessReviewResponse, err error) {
	result = &authorizationapi.SubjectAccessReviewResponse{}
	err = overrideAuth(c.token, c.r.Post().Namespace(c.ns).Resource("subjectAccessReviews")).Body(policy).Do().Into(result)
	return
}

// clusterSubjectAccessReviews implements ClusterSubjectAccessReviews interface
type clusterSubjectAccessReviews struct {
	r *Client
}

// newClusterSubjectAccessReviews returns a clusterSubjectAccessReviews
func newClusterSubjectAccessReviews(c *Client) *clusterSubjectAccessReviews {
	return &clusterSubjectAccessReviews{
		r: c,
	}
}

// Create creates new policy. Returns the server's representation of the policy and error if one occurs.
func (c *clusterSubjectAccessReviews) Create(policy *authorizationapi.SubjectAccessReview) (result *authorizationapi.SubjectAccessReviewResponse, err error) {
	result = &authorizationapi.SubjectAccessReviewResponse{}
	err = c.r.Post().Resource("subjectAccessReviews").Body(policy).Do().Into(result)
	return
}

func overrideAuth(token string, req *client.Request) *client.Request {
	if len(token) > 0 {
		req.SetHeader("Authorization", fmt.Sprintf("Bearer %s", token))
	}
	return req
}
