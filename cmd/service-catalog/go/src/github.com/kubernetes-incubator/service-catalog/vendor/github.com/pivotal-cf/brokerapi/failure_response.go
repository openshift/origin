// Copyright (C) 2016-Present Pivotal Software, Inc. All rights reserved.
// This program and the accompanying materials are made available under the terms of the under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
// Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.

package brokerapi

import (
	"net/http"

	"fmt"

	"code.cloudfoundry.org/lager"
)

// FailureResponse can be returned from any of the `ServiceBroker` interface methods
// which allow an error to be returned. Doing so will provide greater control over
// the HTTP response.
type FailureResponse struct {
	error
	statusCode    int
	loggerAction  string
	emptyResponse bool
	errorKey      string
}

// NewFailureResponse returns a pointer to a new instance of FailureResponse.
// err will by default be used as both a logging message and HTTP response description.
// statusCode is the HTTP status code to be returned, must be 4xx or 5xx
// loggerAction is a short description which will be used as the action if the error is logged.
func NewFailureResponse(err error, statusCode int, loggerAction string) *FailureResponse {
	return &FailureResponse{
		error:        err,
		statusCode:   statusCode,
		loggerAction: loggerAction,
	}
}

// ErrorResponse returns an interface{} which will be JSON encoded and form the body
// of the HTTP response
func (f *FailureResponse) ErrorResponse() interface{} {
	if f.emptyResponse {
		return EmptyResponse{}
	}

	return ErrorResponse{
		Description: f.error.Error(),
		Error:       f.errorKey,
	}
}

// ValidatedStatusCode returns the HTTP response status code. If the code is not 4xx
// or 5xx, an InternalServerError will be returned instead.
func (f *FailureResponse) ValidatedStatusCode(logger lager.Logger) int {
	if f.statusCode < 400 || 600 <= f.statusCode {
		if logger != nil {
			logger.Error("validating-status-code", fmt.Errorf("Invalid failure http response code: 600, expected 4xx or 5xx, returning internal server error: 500."))
		}
		return http.StatusInternalServerError
	}
	return f.statusCode
}

// LoggerAction returns the loggerAction, used as the action when logging
func (f *FailureResponse) LoggerAction() string {
	return f.loggerAction
}

// FailureResponseBuilder provides a fluent set of methods to build a *FailureResponse.
type FailureResponseBuilder struct {
	error
	statusCode    int
	loggerAction  string
	emptyResponse bool
	errorKey      string
}

// NewFailureResponseBuilder returns a pointer to a newly instantiated FailureResponseBuilder
// Accepts required arguments to create a FailureResponse.
func NewFailureResponseBuilder(err error, statusCode int, loggerAction string) *FailureResponseBuilder {
	return &FailureResponseBuilder{
		error:         err,
		statusCode:    statusCode,
		loggerAction:  loggerAction,
		emptyResponse: false,
	}
}

// WithErrorKey adds a custom ErrorKey which will be used in FailureResponse to add an `Error`
// field to the JSON HTTP response body
func (f *FailureResponseBuilder) WithErrorKey(errorKey string) *FailureResponseBuilder {
	f.errorKey = errorKey
	return f
}

// WithEmptyResponse will cause the built FailureResponse to return an empty JSON object as the
// HTTP response body
func (f *FailureResponseBuilder) WithEmptyResponse() *FailureResponseBuilder {
	f.emptyResponse = true
	return f
}

// Build returns the generated FailureResponse built using previously configured variables.
func (f *FailureResponseBuilder) Build() *FailureResponse {
	return &FailureResponse{
		error:         f.error,
		statusCode:    f.statusCode,
		loggerAction:  f.loggerAction,
		emptyResponse: f.emptyResponse,
		errorKey:      f.errorKey,
	}
}
