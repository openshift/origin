package util

import (
	"errors"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/authentication/user"
	"k8s.io/kubernetes/pkg/apis/authorization"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/authorization/internalversion"
)

// AddUserToSAR adds the requisite user information to a SubjectAccessReview.
// It returns the modified SubjectAccessReview.
func AddUserToSAR(user user.Info, sar *authorization.SubjectAccessReview) *authorization.SubjectAccessReview {
	sar.Spec.User = user.GetName()
	// reminiscent of the bad old days of C.  Copies copy the min number of elements of both source and dest
	sar.Spec.Groups = make([]string, len(user.GetGroups()))
	copy(sar.Spec.Groups, user.GetGroups())
	sar.Spec.Extra = map[string]authorization.ExtraValue{}

	for k, v := range user.GetExtra() {
		sar.Spec.Extra[k] = authorization.ExtraValue(v)
	}

	return sar
}

// Authorize verifies that a given user is permitted to carry out a given
// action.  If this cannot be determined, or if the user is not permitted, an
// error is returned.
func Authorize(sarClient internalversion.SubjectAccessReviewInterface, user user.Info, resourceAttributes *authorization.ResourceAttributes) error {
	sar := AddUserToSAR(user, &authorization.SubjectAccessReview{
		Spec: authorization.SubjectAccessReviewSpec{
			ResourceAttributes: resourceAttributes,
		},
	})

	resp, err := sarClient.Create(sar)
	if err == nil && resp != nil && resp.Status.Allowed {
		return nil
	}

	if err == nil {
		err = errors.New(resp.Status.Reason)
	}
	return kerrors.NewForbidden(schema.GroupResource{Group: resourceAttributes.Group, Resource: resourceAttributes.Resource}, resourceAttributes.Name, err)
}
