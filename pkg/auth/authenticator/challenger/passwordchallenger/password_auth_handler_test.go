package passwordchallenger

import (
	"net/http"
	"testing"
)

func TestAuthChallengeNeeded(t *testing.T) {
	handler := NewBasicAuthChallenger("testing-realm")
	req := &http.Request{}
	header, err := handler.AuthenticationChallenge(req)

	if err != nil {
		t.Errorf("Unexepcted error: %v", err)
	}

	if value, ok := header["Www-Authenticate"]; ok {
		expectedValue := "Basic realm=\"testing-realm\""
		if value[0] != expectedValue {
			t.Errorf("Expected %v, got %v", expectedValue, value)
		}
	} else {
		t.Error("Did not get back header")

	}

}
