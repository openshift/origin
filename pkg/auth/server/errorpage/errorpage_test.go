package errorpage

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorPage(t *testing.T) {
	testCases := map[string]struct {
		Headers       http.Header
		ExpectHandled bool
	}{
		"should not handle non-browser": {
			Headers:       http.Header{},
			ExpectHandled: false,
		},
		"should handle html-accepting browser": {
			Headers:       http.Header{"Accept": []string{"text/html"}},
			ExpectHandled: true,
		},
		"should handle mozilla browser": {
			Headers:       http.Header{"User-Agent": []string{"Mozilla/5.0"}},
			ExpectHandled: true,
		},
		"should not handle mozilla browser requesting json": {
			Headers:       http.Header{"Accept": []string{"application/json"}, "User-Agent": []string{"Mozilla/5.0"}},
			ExpectHandled: false,
		},
	}

	for k, testCase := range testCases {
		renderer, err := NewErrorPageTemplateRenderer("")
		if err != nil {
			t.Errorf("%s: unexpected error: %v", k, err)
			continue
		}
		handler := NewErrorPageHandler(renderer)

		{
			resp := httptest.NewRecorder()
			handled, err := handler.AuthenticationError(nil, resp, &http.Request{Header: testCase.Headers})
			if err != nil {
				t.Errorf("%s: unexpected error: %#v", k, err)
				continue
			}
			if testCase.ExpectHandled != handled {
				t.Errorf("%s: unexpected value for 'handled': %#v", k, handled)
				continue
			}
		}

		{
			resp := httptest.NewRecorder()
			handled, err := handler.GrantError(nil, resp, &http.Request{Header: testCase.Headers})
			if err != nil {
				t.Errorf("%s: unexpected error: %#v", k, err)
				continue
			}
			if testCase.ExpectHandled != handled {
				t.Errorf("%s: unexpected value for 'handled': %#v", k, handled)
				continue
			}
		}
	}
}

func TestValidateErrorPageTemplate(t *testing.T) {
	testCases := map[string]struct {
		Template      string
		TemplateValid bool
	}{
		"broken template": {
			Template:      `Test {{ .BadField }}`,
			TemplateValid: false,
		},
		"default template": {
			Template:      defaultErrorPageTemplateString,
			TemplateValid: true,
		},
		"template example": {
			Template:      ErrorPageTemplateExample,
			TemplateValid: true,
		},
		"original template example": {
			Template:      originalErrorPageTemplateExample,
			TemplateValid: true,
		},
	}

	for k, testCase := range testCases {
		allErrs := ValidateErrorPageTemplate([]byte(testCase.Template))
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
const originalErrorPageTemplateExample = `<!DOCTYPE html>
<!--

This template can be modified and used to customize the error page. To replace
the error page, set master configuration option oauthConfig.templates.error to
the path of the template file.

oauthConfig:
  templates:
    error: templates/error-template.html

The Error field contains an error message, which is human readable, and subject to change.
Default error messages are intentionally generic to avoid leaking information about authentication errors.

The ErrorCode field contains a programmatic error code, which may be (but is not limited to):
- mapping_claim_error
- mapping_lookup_error
- authentication_error
- grant_error
-->
<html>
  <head>
    <title>Error</title>
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
	<!-- example of handling a particular error code in a special way -->
	{{ if eq .ErrorCode "mapping_claim_error" }}
		Could not create your user. Contact your administrator to resolve this issue.
	{{ else }}
		{{ .Error }}
	{{ end }}
	</div>

  </body>
</html>
`
