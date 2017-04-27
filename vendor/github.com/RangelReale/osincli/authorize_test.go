package osincli

import (
	"fmt"
	"testing"
)

func TestGetAuthorizeUrl(t *testing.T) {
	generatedConfig := &ClientConfig{
		ClientId:     "myclient",
		TokenUrl:     "https://example.com/token",
		AuthorizeUrl: "https://example.com/authorize",
		RedirectUrl:  "/",
	}
	if err := PopulatePKCE(generatedConfig); err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	generatedConfigAuthorizeUrl := fmt.Sprintf(
		"https://example.com/authorize?client_id=myclient&code_challenge=%s&code_challenge_method=%s&redirect_uri=%%2F&response_type=code",
		generatedConfig.CodeChallenge,
		generatedConfig.CodeChallengeMethod)

	for name, test := range map[string]struct {
		Config *ClientConfig
		URL    string
	}{
		"no challenge": {
			Config: &ClientConfig{
				ClientId:     "myclient",
				TokenUrl:     "https://example.com/token",
				AuthorizeUrl: "https://example.com/authorize",
				RedirectUrl:  "/",
			},
			URL: "https://example.com/authorize?client_id=myclient&redirect_uri=%2F&response_type=code",
		},
		"has challenge": {
			Config: &ClientConfig{
				ClientId:            "myclient",
				TokenUrl:            "https://example.com/token",
				AuthorizeUrl:        "https://example.com/authorize",
				RedirectUrl:         "/",
				CodeChallenge:       "randomstuff",
				CodeChallengeMethod: "method",
			},
			URL: "https://example.com/authorize?client_id=myclient&code_challenge=randomstuff&code_challenge_method=method&redirect_uri=%2F&response_type=code",
		},
		"has generated challenge": {
			Config: generatedConfig,
			URL:    generatedConfigAuthorizeUrl,
		},
	} {
		client, err := NewClient(test.Config)
		if err != nil {
			t.Fatalf("Unexpected error: %#v", err)
		}
		if url := client.NewAuthorizeRequest(CODE).GetAuthorizeUrl().String(); url != test.URL {
			t.Errorf("%s: Expected\n%s\ngot\n%s", name, test.URL, url)
		}
	}
}
