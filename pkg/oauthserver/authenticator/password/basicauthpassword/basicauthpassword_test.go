package basicauthpassword

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	expectedSubject := "12345"
	expectedName := "My Name"
	expectedEmail := "mylogin@example.com"
	expectedPreferredUsername := "myusername"

	// These keys are the published interface for the basicauthpassword IDP
	// The keys for this test should not be changed unless all corresponding docs are also updated
	data := fmt.Sprintf(`
	{
		"sub":"%s",
		"name": "%s",
		"email": "%s",
		"preferred_username": "%s",
		"additional_field": "should be ignored"
	}`, expectedSubject, expectedName, expectedEmail, expectedPreferredUsername)

	user := &RemoteUserData{}
	err := json.Unmarshal([]byte(data), user)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if user.Subject != expectedSubject {
		t.Errorf("Expected %s, got %s", expectedSubject, user.Subject)
	}
	if user.Name != expectedName {
		t.Errorf("Expected %s, got %s", expectedName, user.Name)
	}
	if user.Email != expectedEmail {
		t.Errorf("Expected %s, got %s", expectedEmail, user.Email)
	}
	if user.PreferredUsername != expectedPreferredUsername {
		t.Errorf("Expected %s, got %s", expectedPreferredUsername, user.PreferredUsername)
	}

}
