package authorizer

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/apiserver/request"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

type personalSARRequestInfoResolver struct {
	// infoFactory is used to determine info for the request
	infoFactory RequestInfoFactory
}

func NewPersonalSARRequestInfoResolver(infoFactory RequestInfoFactory) RequestInfoFactory {
	return &personalSARRequestInfoResolver{
		infoFactory: infoFactory,
	}
}

func (a *personalSARRequestInfoResolver) NewRequestInfo(req *http.Request) (*request.RequestInfo, error) {
	requestInfo, err := a.infoFactory.NewRequestInfo(req)
	if err != nil {
		return requestInfo, err
	}

	// only match SAR and LSAR requests for personal review
	switch {
	case !requestInfo.IsResourceRequest:
		return requestInfo, nil

	case len(requestInfo.APIGroup) != 0:
		return requestInfo, nil

	case len(requestInfo.Subresource) != 0:
		return requestInfo, nil

	case strings.ToLower(requestInfo.Verb) != "create":
		return requestInfo, nil

	case strings.ToLower(requestInfo.Resource) != "subjectaccessreviews" && strings.ToLower(requestInfo.Resource) != "localsubjectaccessreviews":
		return requestInfo, nil
	}

	// at this point we're probably running a SAR or LSAR.  Decode the body and check.  This is expensive.
	isSelfSAR, err := isPersonalAccessReviewFromRequest(req)
	if err != nil {
		return nil, err
	}
	if !isSelfSAR {
		return requestInfo, nil
	}

	// if we do have a self-SAR, rewrite the requestInfo to indicate this is a selfsubjectaccessreviews.authorization.k8s.io request
	requestInfo.APIGroup = "authorization.k8s.io"
	requestInfo.Resource = "selfsubjectaccessreviews"

	return requestInfo, nil
}

// isPersonalAccessReviewFromRequest this variant handles the case where we have an httpRequest
func isPersonalAccessReviewFromRequest(req *http.Request) (bool, error) {
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
		return IsPersonalAccessReviewFromSAR(castObj), nil

	case *authorizationapi.LocalSubjectAccessReview:
		return isPersonalAccessReviewFromLocalSAR(castObj), nil

	default:
		return false, nil
	}
}

// IsPersonalAccessReviewFromSAR this variant handles the case where we have an SAR
func IsPersonalAccessReviewFromSAR(sar *authorizationapi.SubjectAccessReview) bool {
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
