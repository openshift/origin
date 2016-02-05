package authorizer

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func IsPersonalAccessReview(a AuthorizationAttributes) (bool, error) {
	switch extendedAttributes := a.GetRequestAttributes().(type) {
	case *http.Request:
		return isPersonalAccessReviewFromRequest(a, extendedAttributes)

	case *authorizationapi.SubjectAccessReview:
		return isPersonalAccessReviewFromSAR(extendedAttributes), nil

	case *authorizationapi.LocalSubjectAccessReview:
		return isPersonalAccessReviewFromLocalSAR(extendedAttributes), nil

	default:
		return false, fmt.Errorf("unexpected request attributes for checking personal access review: %v", extendedAttributes)

	}
}

// isPersonalAccessReviewFromRequest this variant handles the case where we have an httpRequest
func isPersonalAccessReviewFromRequest(a AuthorizationAttributes, req *http.Request) (bool, error) {
	// TODO once we're integrated with the api installer, we should have direct access to the deserialized content
	// for now, this only happens on subjectaccessreviews with a personal check, pay the double retrieve and decode cost
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return false, err
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), body)
	if err != nil {
		return false, err
	}
	switch castObj := obj.(type) {
	case *authorizationapi.SubjectAccessReview:
		return isPersonalAccessReviewFromSAR(castObj), nil

	case *authorizationapi.LocalSubjectAccessReview:
		return isPersonalAccessReviewFromLocalSAR(castObj), nil

	default:
		return false, nil
	}
}

// isPersonalAccessReviewFromSAR this variant handles the case where we have an SAR
func isPersonalAccessReviewFromSAR(sar *authorizationapi.SubjectAccessReview) bool {
	if len(sar.User) == 0 && len(sar.Groups) == 0 {
		return true
	}

	return false
}

// isPersonalAccessReviewFromLocalSAR this variant handles the case where we have a local SAR
func isPersonalAccessReviewFromLocalSAR(sar *authorizationapi.LocalSubjectAccessReview) bool {
	if len(sar.User) == 0 && len(sar.Groups) == 0 {
		return true
	}

	return false
}
