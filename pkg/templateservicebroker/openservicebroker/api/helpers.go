package api

import (
	"net/http"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
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

// ConvertUserToTemplateInstanceRequester copies analogous fields from user.Info to TemplateInstanceRequester
func ConvertUserToTemplateInstanceRequester(u user.Info) templateapi.TemplateInstanceRequester {
	templatereq := templateapi.TemplateInstanceRequester{}

	if u != nil {
		extra := map[string]templateapi.ExtraValue{}
		if u.GetExtra() != nil {
			for k, v := range u.GetExtra() {
				extra[k] = templateapi.ExtraValue(v)
			}
		}

		templatereq.Username = u.GetName()
		templatereq.UID = u.GetUID()
		templatereq.Groups = u.GetGroups()
		templatereq.Extra = extra
	}

	return templatereq
}

// ConvertTemplateInstanceRequesterToUser copies analogous fields from TemplateInstanceRequester to user.Info
func ConvertTemplateInstanceRequesterToUser(templateReq *templateapi.TemplateInstanceRequester) user.Info {
	u := user.DefaultInfo{}
	u.Extra = map[string][]string{}

	if templateReq != nil {
		u.Name = templateReq.Username
		u.UID = templateReq.UID
		u.Groups = templateReq.Groups
		if templateReq.Extra != nil {
			for k, v := range templateReq.Extra {
				u.Extra[k] = []string(v)
			}
		}
	}

	return &u
}
