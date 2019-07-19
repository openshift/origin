package api

import (
	"net/http"

	"k8s.io/apiserver/pkg/authentication/user"
)

func NewResponse(code int, body interface{}, err error) *Response {
	return &Response{Code: code, Body: body, Err: err}
}

func BadRequest(err error) *Response {
	return NewResponse(http.StatusBadRequest, nil, err)
}

func Forbidden(err error) *Response {
	return NewResponse(http.StatusForbidden, nil, err)
}

func InternalServerError(err error) *Response {
	return NewResponse(http.StatusInternalServerError, nil, err)
}

// TemplateInstanceRequester holds the identity of an agent requesting a
// template instantiation.
type TemplateInstanceRequester struct {
	// username uniquely identifies this user among all active users.
	Username string

	// uid is a unique value that identifies this user across time; if this user is
	// deleted and another user by the same name is added, they will have
	// different UIDs.
	UID string

	// groups represent the groups this user is a part of.
	Groups []string

	// extra holds additional information provided by the authenticator.
	Extra map[string]ExtraValue
}

// ExtraValue masks the value so protobuf can generate
type ExtraValue []string

// ConvertUserToTemplateInstanceRequester copies analogous fields from user.Info to TemplateInstanceRequester
func ConvertUserToTemplateInstanceRequester(u user.Info) TemplateInstanceRequester {
	templatereq := TemplateInstanceRequester{}

	if u != nil {
		extra := map[string]ExtraValue{}
		if u.GetExtra() != nil {
			for k, v := range u.GetExtra() {
				extra[k] = ExtraValue(v)
			}
		}

		templatereq.Username = u.GetName()
		templatereq.UID = u.GetUID()
		templatereq.Groups = u.GetGroups()
		templatereq.Extra = extra
	}

	return templatereq
}
