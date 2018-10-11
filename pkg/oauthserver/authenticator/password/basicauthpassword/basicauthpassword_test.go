package basicauthpassword

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestUnmarshal(t *testing.T) {
	expectedSubject := "12345"
	expectedName := "My Name"
	expectedEmail := "mylogin@example.com"
	expectedPreferredUsername := "myusername"
	expectedGroups := []string{"group1", "group2", "group3"}

	// These keys are the published interface for the basicauthpassword IDP
	// The keys for this test should not be changed as that indicates a breaking API change
	data := fmt.Sprintf(`
	{
		"sub":"%s",
		"name": "%s",
		"email": "%s",
		"preferred_username": "%s",
		"groups": %s,
		"additional_field": "should be ignored"
	}`, expectedSubject, expectedName, expectedEmail, expectedPreferredUsername,
		strings.Replace(fmt.Sprintf("%q", expectedGroups), " ", ",", -1))

	user := &RemoteUserData{}
	err := json.Unmarshal([]byte(data), user)
	if err != nil {
		t.Fatal(err)
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
	if !reflect.DeepEqual(user.Groups, expectedGroups) {
		t.Errorf("Expected %s, got %v", expectedGroups, user.Groups)
	}
}
