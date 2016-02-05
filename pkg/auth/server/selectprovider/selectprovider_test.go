package selectprovider

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openshift/origin/pkg/auth/oauth/handlers"
)

func TestSelectAuthentication(t *testing.T) {
	testCases := map[string]struct {
		ForceInterstitial      bool
		Providers              []handlers.ProviderInfo
		ExpectSelectedProvider bool
		ExpectHandled          bool
		ExpectContains         []string
	}{
		"should select single provider": {
			ForceInterstitial: false,
			Providers: []handlers.ProviderInfo{
				{
					Name: "provider_1",
					URL:  "http://example.com/redirect_1/",
				},
			},
			ExpectSelectedProvider: true,
			ExpectHandled:          false,
		},
		"should return empty provider info when no providers": {
			ForceInterstitial:      false,
			Providers:              []handlers.ProviderInfo{},
			ExpectSelectedProvider: false,
			ExpectHandled:          false,
		},
		"should render select provider when forced": {
			ForceInterstitial: true,
			Providers: []handlers.ProviderInfo{
				{
					Name: "provider_1",
					URL:  "http://example.com/redirect_1/",
				},
			},
			ExpectSelectedProvider: false,
			ExpectHandled:          true,
			ExpectContains: []string{
				`http://example.com/redirect_1/`,
			},
		},
		"should render select provider when multiple providers": {
			ForceInterstitial: false,
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
			ExpectSelectedProvider: false,
			ExpectHandled:          true,
			ExpectContains: []string{
				`http://example.com/redirect_1/`,
				`http://example.com/redirect_2/`,
			},
		},
	}

	for k, testCase := range testCases {
		selectProviderRenderer, err := NewSelectProviderRenderer("")
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		selectProvider := NewSelectProvider(selectProviderRenderer, testCase.ForceInterstitial)
		resp := httptest.NewRecorder()
		provider, handled, err := selectProvider.SelectAuthentication(testCase.Providers, resp, &http.Request{})

		if err != nil {
			t.Errorf("%s: unexpected error: %#v", k, err)
			continue
		}

		if testCase.ExpectHandled != handled {
			t.Errorf("%s: unexpected value for 'handled': %#v", k, handled)
			continue
		}

		if testCase.ExpectSelectedProvider && provider == nil {
			t.Errorf("%s: expected a provider to be selected", k)
			continue
		}

		if len(testCase.ExpectContains) > 0 {
			data, _ := ioutil.ReadAll(resp.Body)
			body := string(data)
			for i := range testCase.ExpectContains {
				if !strings.Contains(body, testCase.ExpectContains[i]) {
					t.Errorf("%s: did not find expected value %s: %s", k, testCase.ExpectContains[i], body)
					continue
				}
			}
		}
	}
}

func TestValidateSelectProviderTemplate(t *testing.T) {
	testCases := map[string]struct {
		Template      string
		TemplateValid bool
	}{
		"default provider selection template": {
			Template:      defaultSelectProviderTemplateString,
			TemplateValid: true,
		},
		"provider selection template example": {
			Template:      SelectProviderTemplateExample,
			TemplateValid: true,
		},
		"original provider selection template example": {
			Template:      originalSelectProviderTemplateExample,
			TemplateValid: true,
		},
		"template only prints first provider URL": {
			Template:      invalidSelectProviderTemplate,
			TemplateValid: false,
		},
	}

	for k, testCase := range testCases {
		allErrs := ValidateSelectProviderTemplate([]byte(testCase.Template))
		if testCase.TemplateValid {
			for _, err := range allErrs {
				t.Errorf("%s: template validation failed when it should have succeeded: %v", k, err)
			}
		} else if len(allErrs) == 0 {
			t.Errorf("%s: template validation succeeded when it should have failed", k)
		}
	}
}

// Make sure the original version of the default template always validates
// this is to avoid breaking existing customized templates.
const originalSelectProviderTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the provider selection page. To replace
the provider selection page, set master configuration option oauthConfig.templates.providerSelection to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    providerSelection: templates/select-provider-template.html

The Name is unique for each provider and can be used for provider specific customizations like
the example below.  The Name matches the name of an identity provider in the master configuration.
-->
<html>
  <head>
    <title>Login</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }
    </style>
  </head>
  <body>

    {{ range $provider := .Providers }}
      <div>
        <!-- This is an example of customizing display for a particular provider based on its Name -->
        {{ if eq $provider.Name "anypassword" }}
          <a href="{{$provider.URL}}">Log in</a> with any username and password
        {{ else }}
          <a href="{{$provider.URL}}">{{$provider.Name}}</a>
        {{ end }}
      </div>
    {{ end }}

  </body>
</html>
`

// This template only prints the first provider URL and should fail validation.
const invalidSelectProviderTemplate = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the provider selection page. To replace
the provider selection page, set master configuration option oauthConfig.templates.providerSelection to
the path of the template file. Don't remove parameters in curly braces below.

oauthConfig:
  templates:
    providerSelection: templates/select-provider-template.html

The ID is unique for each provider and can be used for provider specific customizations like
the example below.
-->
<html>
  <head>
    <title>Login</title>
    <style type="text/css">
      body {
        font-family: "Open Sans", Helvetica, Arial, sans-serif;
        font-size: 14px;
        margin: 15px;
      }
    </style>
  </head>
  <body>
    <div>
      <a href="{{(index .Providers 0).URL}}">Log In</a>
    </div>
  </body>
</html>
`
