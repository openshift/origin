package errorpage

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
	"github.com/openshift/origin/pkg/util/httprequest"
)

var _ handlers.AuthenticationErrorHandler = &ErrorPage{}
var _ handlers.GrantErrorHandler = &ErrorPage{}

// ErrorPage implements auth and grant error handling by rendering an error page for browser-like clients
type ErrorPage struct {
	render ErrorPageRenderer
}

// NewErrorPageHandler returns an auth and grant error handler using the given renderer
func NewErrorPageHandler(renderer ErrorPageRenderer) *ErrorPage {
	return &ErrorPage{render: renderer}
}

func (p *ErrorPage) AuthenticationError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.Errorf("AuthenticationError: %v", err)
	// Only render html error pages for browser-like things
	if !httprequest.PrefersHTML(req) {
		return false, err
	}

	errorData := ErrorData{}
	errorData.ErrorCode = AuthenticationErrorCode(err)
	errorData.Error = AuthenticationErrorMessage(errorData.ErrorCode)

	p.render.Render(errorData, w, req)
	return true, nil
}

func (p *ErrorPage) GrantError(err error, w http.ResponseWriter, req *http.Request) (bool, error) {
	glog.Errorf("GrantError: %v", err)
	// Only render html error pages for browser-like things
	if !httprequest.PrefersHTML(req) {
		return false, err
	}

	errorData := ErrorData{}
	errorData.ErrorCode = GrantErrorCode(err)
	errorData.Error = GrantErrorMessage(errorData.ErrorCode)

	p.render.Render(errorData, w, req)
	return true, nil
}

// ErrorData holds fields for the error page renderer
type ErrorData struct {
	Error     string
	ErrorCode string
}

// ErrorPageRenderer handles rendering a given error code/message
type ErrorPageRenderer interface {
	Render(data ErrorData, w http.ResponseWriter, req *http.Request)
}

// errorPageTemplateRenderer renders a golang template for requests which indicate they can accept HTML
type errorPageTemplateRenderer struct {
	errorPageTemplate *template.Template
}

// NewErrorPageRenderer creates an error page renderer that takes in an optional custom template to
// allow branding of the page. Uses the default if templateFile is not set.
func NewErrorPageTemplateRenderer(templateFile string) (ErrorPageRenderer, error) {
	r := &errorPageTemplateRenderer{}
	if len(templateFile) > 0 {
		customTemplate, err := template.ParseFiles(templateFile)
		if err != nil {
			return nil, err
		}
		r.errorPageTemplate = customTemplate
	} else {
		r.errorPageTemplate = defaultErrorPageTemplate
	}

	return r, nil
}

func (r *errorPageTemplateRenderer) Render(data ErrorData, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := r.errorPageTemplate.Execute(w, data); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render error page template: %v", err))
	}
}

// ValidateErrorPageTemplate ensures the given template does not error when rendered with an ErrorData object as input
func ValidateErrorPageTemplate(templateContent []byte) []error {
	var allErrs []error

	template, err := template.New("errorPageTemplateTest").Parse(string(templateContent))
	if err != nil {
		return append(allErrs, err)
	}

	var buffer bytes.Buffer
	err = template.Execute(&buffer, ErrorData{})
	if err != nil {
		return append(allErrs, err)
	}

	return allErrs
}
