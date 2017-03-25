package api

import "net/http"

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
