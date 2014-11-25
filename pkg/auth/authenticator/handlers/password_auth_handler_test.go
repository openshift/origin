package handlers

import (
	"net/http"
	"testing"
)

func TestAuthChallengeNeeded(t *testing.T) {
	handler := NewBasicPasswordAuthHandler("testing-realm")
	req := &http.Request{}
	header, err := handler.AuthenticationChallengeNeeded(req)

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
