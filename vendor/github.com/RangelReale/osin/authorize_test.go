package osin

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestAuthorizeCode(t *testing.T) {
	sconfig := NewServerConfig()
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{CODE}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}
	resp := server.NewResponse()

	req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = make(url.Values)
	req.Form.Set("response_type", string(CODE))
	req.Form.Set("client_id", "1234")
	req.Form.Set("state", "a")

	if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
		ar.Authorized = true
		server.FinishAuthorizeRequest(resp, req, ar)
	}

	//fmt.Printf("%+v", resp)

	if resp.IsError && resp.InternalError != nil {
		t.Fatalf("Error in response: %s", resp.InternalError)
	}

	if resp.IsError {
		t.Fatalf("Should not be an error")
	}

	if resp.Type != REDIRECT {
		t.Fatalf("Response should be a redirect")
	}

	if d := resp.Output["code"]; d != "1" {
		t.Fatalf("Unexpected authorization code: %s", d)
	}
}

func TestAuthorizeToken(t *testing.T) {
	sconfig := NewServerConfig()
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{TOKEN}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}
	server.AccessTokenGen = &TestingAccessTokenGen{}
	resp := server.NewResponse()

	req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = make(url.Values)
	req.Form.Set("response_type", string(TOKEN))
	req.Form.Set("client_id", "1234")
	req.Form.Set("state", "a")

	if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
		ar.Authorized = true
		server.FinishAuthorizeRequest(resp, req, ar)
	}

	//fmt.Printf("%+v", resp)

	if resp.IsError && resp.InternalError != nil {
		t.Fatalf("Error in response: %s", resp.InternalError)
	}

	if resp.IsError {
		t.Fatalf("Should not be an error")
	}

	if resp.Type != REDIRECT || !resp.RedirectInFragment {
		t.Fatalf("Response should be a redirect with fragment")
	}

	if d := resp.Output["access_token"]; d != "1" {
		t.Fatalf("Unexpected access token: %s", d)
	}
}

func TestAuthorizeTokenWithInvalidClient(t *testing.T) {
	sconfig := NewServerConfig()
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{TOKEN}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}
	server.AccessTokenGen = &TestingAccessTokenGen{}
	resp := server.NewResponse()
	redirectUri := "http://redirecturi.com"

	req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = make(url.Values)
	req.Form.Set("response_type", string(TOKEN))
	req.Form.Set("client_id", "invalidclient")
	req.Form.Set("state", "a")
	req.Form.Set("redirect_uri", redirectUri)

	if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
		ar.Authorized = true
		server.FinishAuthorizeRequest(resp, req, ar)
	}

	if !resp.IsError {
		t.Fatalf("Response should be an error")
	}

	if resp.ErrorId != E_UNAUTHORIZED_CLIENT {
		t.Fatalf("Incorrect error in response: %v", resp.ErrorId)
	}

	usedRedirectUrl, redirectErr := resp.GetRedirectUrl()

	if redirectErr == nil && usedRedirectUrl == redirectUri {
		t.Fatalf("Response must not redirect to the provided redirect URL for an invalid client")
	}
}

func TestAuthorizeCodePKCERequired(t *testing.T) {
	sconfig := NewServerConfig()
	sconfig.RequirePKCEForPublicClients = true
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{CODE}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}

	// Public client returns an error
	{
		resp := server.NewResponse()
		req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Form = make(url.Values)
		req.Form.Set("response_type", string(CODE))
		req.Form.Set("state", "a")
		req.Form.Set("client_id", "public-client")
		if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
			ar.Authorized = true
			server.FinishAuthorizeRequest(resp, req, ar)
		}
		if !resp.IsError || resp.ErrorId != "invalid_request" || strings.Contains(resp.StatusText, "code_challenge") {
			t.Errorf("Expected invalid_request error describing the code_challenge required, got %#v", resp)
		}
	}

	// Confidential client works without PKCE
	{
		resp := server.NewResponse()
		req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Form = make(url.Values)
		req.Form.Set("response_type", string(CODE))
		req.Form.Set("state", "a")
		req.Form.Set("client_id", "1234")
		if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
			ar.Authorized = true
			server.FinishAuthorizeRequest(resp, req, ar)
		}
		if resp.IsError && resp.InternalError != nil {
			t.Fatalf("Error in response: %s", resp.InternalError)
		}
		if resp.IsError {
			t.Fatalf("Should not be an error")
		}
		if resp.Type != REDIRECT {
			t.Fatalf("Response should be a redirect")
		}
		if d := resp.Output["code"]; d != "1" {
			t.Fatalf("Unexpected authorization code: %s", d)
		}
	}
}

func TestAuthorizeCodePKCEPlain(t *testing.T) {
	challenge := "12345678901234567890123456789012345678901234567890"

	sconfig := NewServerConfig()
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{CODE}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}
	resp := server.NewResponse()

	req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = make(url.Values)
	req.Form.Set("response_type", string(CODE))
	req.Form.Set("client_id", "1234")
	req.Form.Set("state", "a")
	req.Form.Set("code_challenge", challenge)

	if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
		ar.Authorized = true
		server.FinishAuthorizeRequest(resp, req, ar)
	}

	//fmt.Printf("%+v", resp)

	if resp.IsError && resp.InternalError != nil {
		t.Fatalf("Error in response: %s", resp.InternalError)
	}

	if resp.IsError {
		t.Fatalf("Should not be an error")
	}

	if resp.Type != REDIRECT {
		t.Fatalf("Response should be a redirect")
	}

	code, ok := resp.Output["code"].(string)
	if !ok || code != "1" {
		t.Fatalf("Unexpected authorization code: %s", code)
	}

	token, err := server.Storage.LoadAuthorize(code)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if token.CodeChallenge != challenge {
		t.Errorf("Expected stored code_challenge %s, got %s", challenge, token.CodeChallenge)
	}
	if token.CodeChallengeMethod != "plain" {
		t.Errorf("Expected stored code_challenge plain, got %s", token.CodeChallengeMethod)
	}
}

func TestAuthorizeCodePKCES256(t *testing.T) {
	challenge := "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM"

	sconfig := NewServerConfig()
	sconfig.AllowedAuthorizeTypes = AllowedAuthorizeType{CODE}
	server := NewServer(sconfig, NewTestingStorage())
	server.AuthorizeTokenGen = &TestingAuthorizeTokenGen{}
	resp := server.NewResponse()

	req, err := http.NewRequest("GET", "http://localhost:14000/appauth", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Form = make(url.Values)
	req.Form.Set("response_type", string(CODE))
	req.Form.Set("client_id", "1234")
	req.Form.Set("state", "a")
	req.Form.Set("code_challenge", challenge)
	req.Form.Set("code_challenge_method", "S256")

	if ar := server.HandleAuthorizeRequest(resp, req); ar != nil {
		ar.Authorized = true
		server.FinishAuthorizeRequest(resp, req, ar)
	}

	//fmt.Printf("%+v", resp)

	if resp.IsError && resp.InternalError != nil {
		t.Fatalf("Error in response: %s", resp.InternalError)
	}

	if resp.IsError {
		t.Fatalf("Should not be an error")
	}

	if resp.Type != REDIRECT {
		t.Fatalf("Response should be a redirect")
	}

	code, ok := resp.Output["code"].(string)
	if !ok || code != "1" {
		t.Fatalf("Unexpected authorization code: %s", code)
	}

	token, err := server.Storage.LoadAuthorize(code)
	if err != nil {
		t.Fatalf("Unexpected error: %s", err)
	}
	if token.CodeChallenge != challenge {
		t.Errorf("Expected stored code_challenge %s, got %s", challenge, token.CodeChallenge)
	}
	if token.CodeChallengeMethod != "S256" {
		t.Errorf("Expected stored code_challenge S256, got %s", token.CodeChallengeMethod)
	}
}
