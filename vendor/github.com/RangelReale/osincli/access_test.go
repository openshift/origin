package osincli

import (
	"fmt"
	"testing"
)

func TestGetTokenUrl(t *testing.T) {
	clientConfig := ClientConfig{
		ClientId:     "myclient",
		ClientSecret: "mysecret",
		TokenUrl:     "https://example.com/token",
		AuthorizeUrl: "https://example.com/authorize",
		RedirectUrl:  "/",
		Scope:        "scope1 scope2",
	}

	testcases := map[string]struct {
		Type   AccessRequestType
		Data   AuthorizeData
		Params map[string]string

		URL string
	}{
		"client credentials": {
			Type: CLIENT_CREDENTIALS,
			Data: AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"},
			URL:  "https://example.com/token?grant_type=client_credentials&scope=scope1+scope2",
		},
		"client credentials with custom params": {
			Type:   CLIENT_CREDENTIALS,
			Data:   AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"},
			Params: map[string]string{"scope": "customscope"},
			URL:    "https://example.com/token?grant_type=client_credentials&scope=customscope",
		},
		"code grant": {
			Type: AUTHORIZATION_CODE,
			Data: AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"},
			URL:  "https://example.com/token?code=mycode&grant_type=authorization_code&redirect_uri=%2F",
		},
		"refresh grant": {
			Type: REFRESH_TOKEN,
			Data: AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"},
			URL:  "https://example.com/token?grant_type=refresh_token&refresh_token=mycode",
		},
		"password grant": {
			Type: PASSWORD,
			Data: AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"},
			URL:  "https://example.com/token?grant_type=password&password=mypassword&scope=scope1+scope2&username=myusername",
		},
		"password grant with custom params": {
			Type:   PASSWORD,
			Data:   AuthorizeData{},
			Params: map[string]string{"username": "customuser", "password": "custompw", "scope": "customscope"},
			URL:    "https://example.com/token?grant_type=password&password=custompw&scope=customscope&username=customuser",
		},
	}

	client, err := NewClient(&clientConfig)
	if err != nil {
		t.Fatal(err)
	}

	for k, tc := range testcases {
		req := client.NewAccessRequest(tc.Type, &tc.Data)
		req.CustomParameters = tc.Params
		url := req.GetTokenUrl().String()
		if url != tc.URL {
			t.Errorf("%s: Expected\n%s\ngot\n%s", k, tc.URL, url)
		}
	}
}

func TestGetTokenUrlPKCE(t *testing.T) {
	generatedConfig := &ClientConfig{
		ClientId:     "myclient",
		TokenUrl:     "https://example.com/token",
		AuthorizeUrl: "https://example.com/authorize",
		RedirectUrl:  "/",
	}
	if err := PopulatePKCE(generatedConfig); err != nil {
		t.Fatalf("Unexpected error: %#v", err)
	}
	generatedConfigTokenUrl := fmt.Sprintf(
		"https://example.com/token?code=mycode&code_verifier=%s&grant_type=authorization_code&redirect_uri=%%2F",
		generatedConfig.CodeVerifier)

	for name, test := range map[string]struct {
		Config *ClientConfig
		URL    string
	}{
		"no verifier": {
			Config: &ClientConfig{
				ClientId:     "myclient",
				TokenUrl:     "https://example.com/token",
				AuthorizeUrl: "https://example.com/authorize",
				RedirectUrl:  "/",
			},
			URL: "https://example.com/token?code=mycode&grant_type=authorization_code&redirect_uri=%2F",
		},
		"has verifier": {
			Config: &ClientConfig{
				ClientId:     "myclient",
				TokenUrl:     "https://example.com/token",
				AuthorizeUrl: "https://example.com/authorize",
				RedirectUrl:  "/",
				CodeVerifier: "randomdata",
			},
			URL: "https://example.com/token?code=mycode&code_verifier=randomdata&grant_type=authorization_code&redirect_uri=%2F",
		},
		"has generated verifier": {
			Config: generatedConfig,
			URL:    generatedConfigTokenUrl,
		},
	} {
		client, err := NewClient(test.Config)
		if err != nil {
			t.Fatal(err)
		}
		req := client.NewAccessRequest(AUTHORIZATION_CODE, &AuthorizeData{State: "mystate", Code: "mycode", Username: "myusername", Password: "mypassword"})
		url := req.GetTokenUrl().String()
		if url != test.URL {
			t.Errorf("%s: Expected\n%s\ngot\n%s", name, test.URL, url)
		}
	}
}
