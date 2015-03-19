package authorizer

import (
	"bytes"
	"errors"
	"io/ioutil"
	"net/http"

	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
)

func IsPersonalAccessReview(a AuthorizationAttributes) (bool, error) {
	req, ok := a.GetRequestAttributes().(*http.Request)
	if !ok {
		return false, errors.New("expected request, but did not get one")
	}

	// TODO once we're integrated with the api installer, we should have direct access to the deserialized content
	// for now, this only happens on subjectaccessreviews with a personal check, pay the double retrieve and decode cost
	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return false, err
	}
	req.Body = ioutil.NopCloser(bytes.NewBuffer(body))

	subjectAccessReview := &authorizationapi.SubjectAccessReview{}
	if err := latest.Codec.DecodeInto(body, subjectAccessReview); err != nil {
		return false, err
	}

	if (len(subjectAccessReview.User) == 0) && (len(subjectAccessReview.Groups) == 0) {
		return true, nil
	}

	return false, nil
}
