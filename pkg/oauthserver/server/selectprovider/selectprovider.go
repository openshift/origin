package selectprovider

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"net/http"

	"github.com/openshift/origin/pkg/oauthserver/oauth/handlers"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
)

type SelectProviderRenderer interface {
	Render(redirectors []handlers.ProviderInfo, w http.ResponseWriter, req *http.Request)
}

type SelectProvider struct {
	render            SelectProviderRenderer
	forceInterstitial bool
}

var _ = handlers.AuthenticationSelectionHandler(&SelectProvider{})

func NewSelectProvider(render SelectProviderRenderer, forceInterstitial bool) *SelectProvider {
	return &SelectProvider{
		render:            render,
		forceInterstitial: forceInterstitial,
	}
}

type ProviderData struct {
	Providers []handlers.ProviderInfo
}

// NewSelectProviderRenderer creates a select provider renderer that takes in an optional custom template to
// allow branding of the page. Uses the default if customSelectProviderTemplateFile is not set.
func NewSelectProviderRenderer(customSelectProviderTemplateFile string) (*selectProviderTemplateRenderer, error) {
	r := &selectProviderTemplateRenderer{}
	if len(customSelectProviderTemplateFile) > 0 {
		customTemplate, err := template.ParseFiles(customSelectProviderTemplateFile)
		if err != nil {
			return nil, err
		}
		r.selectProviderTemplate = customTemplate
	} else {
		r.selectProviderTemplate = defaultSelectProviderTemplate
	}

	return r, nil
}

func (s *SelectProvider) SelectAuthentication(providers []handlers.ProviderInfo, w http.ResponseWriter, req *http.Request) (*handlers.ProviderInfo, bool, error) {
	if len(providers) == 0 {
		return nil, false, nil
	}

	if len(providers) == 1 && !s.forceInterstitial {
		return &providers[0], false, nil
	}

	s.render.Render(providers, w, req)
	return nil, true, nil
}

func ValidateSelectProviderTemplate(templateContent []byte) []error {
	var allErrs []error

	template, err := template.New("selectProviderTemplateTest").Parse(string(templateContent))
	if err != nil {
		return append(allErrs, err)
	}

	// Execute the template with dummy values and check if they're there.
	providerData := ProviderData{
		Providers: []handlers.ProviderInfo{
			{
				Name: "provider_1",
				URL:  "http://example.com/redirect_1/",
			},
			{
				Name: "provider_2",
				URL:  "http://example.com/redirect_2/",
			},
		},
	}

	var buffer bytes.Buffer
	err = template.Execute(&buffer, providerData)
	if err != nil {
		return append(allErrs, err)
	}
	output := buffer.Bytes()

	// We only care that they are using the URLs we provide, and that they are iterating over all providers
	// for when multiple providers are allowed
	if !bytes.Contains(output, []byte(providerData.Providers[1].URL)) {
		allErrs = append(allErrs, errors.New("template must iterate over all {{.Providers}} and use the {{ .URL }} for each one"))
	}

	return allErrs
}

type selectProviderTemplateRenderer struct {
	selectProviderTemplate *template.Template
}

func (r selectProviderTemplateRenderer) Render(providers []handlers.ProviderInfo, w http.ResponseWriter, req *http.Request) {
	w.Header().Add("Content-Type", "text/html; charset=UTF-8")
	w.WriteHeader(http.StatusOK)
	if err := r.selectProviderTemplate.Execute(w, ProviderData{Providers: providers}); err != nil {
		utilruntime.HandleError(fmt.Errorf("unable to render select provider template: %v", err))
	}
}
