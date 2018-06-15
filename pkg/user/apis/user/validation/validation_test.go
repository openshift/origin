package validation

import (
	"testing"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
)

func TestValidateGroup(t *testing.T) {
	validObj := func() *userapi.Group {
		return &userapi.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myname",
			},
			Users: []string{"myuser"},
		}
	}

	if errs := ValidateGroup(validObj()); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	emptyUser := validObj()
	emptyUser.Users = []string{""}
	if errs := ValidateGroup(emptyUser); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidUser := validObj()
	invalidUser.Users = []string{"bad:user:name"}
	if errs := ValidateGroup(invalidUser); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidName := validObj()
	invalidName.Name = "bad:group:name"
	if errs := ValidateGroup(invalidName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateGroupUpdate(t *testing.T) {
	validObj := func() *userapi.Group {
		return &userapi.Group{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "myname",
				ResourceVersion: "1",
			},
			Users: []string{"myuser"},
		}
	}

	oldObj := validObj()

	if errs := ValidateGroupUpdate(validObj(), oldObj); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	emptyUser := validObj()
	emptyUser.Users = []string{""}
	if errs := ValidateGroupUpdate(emptyUser, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidUser := validObj()
	invalidUser.Users = []string{"bad:user:name"}
	if errs := ValidateGroupUpdate(invalidUser, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidName := validObj()
	invalidName.Name = "bad:group:name"
	if errs := ValidateGroupUpdate(invalidName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateUser(t *testing.T) {
	validObj := func() *userapi.User {
		return &userapi.User{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myname",
			},
			Identities: []string{"myprovider:mylogin"},
			Groups:     []string{"mygroup"},
		}
	}

	if errs := ValidateUser(validObj()); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	emptyIdentity := validObj()
	emptyIdentity.Identities = []string{""}
	if errs := ValidateUser(emptyIdentity); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidIdentity := validObj()
	invalidIdentity.Identities = []string{"foo"}
	if errs := ValidateUser(invalidIdentity); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyGroup := validObj()
	emptyGroup.Groups = []string{""}
	if errs := ValidateUser(emptyGroup); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidGroup := validObj()
	invalidGroup.Groups = []string{"bad:group:name"}
	if errs := ValidateUser(invalidGroup); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateUserUpdate(t *testing.T) {

	validObj := func() *userapi.User {
		return &userapi.User{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "myname",
				ResourceVersion: "1",
			},
			Identities: []string{"myprovider:mylogin"},
			Groups:     []string{"mygroup"},
		}
	}

	oldObj := validObj()

	if errs := ValidateUserUpdate(validObj(), oldObj); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	emptyIdentity := validObj()
	emptyIdentity.Identities = []string{""}
	if errs := ValidateUserUpdate(emptyIdentity, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidIdentity := validObj()
	invalidIdentity.Identities = []string{"foo"}
	if errs := ValidateUserUpdate(invalidIdentity, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyGroup := validObj()
	emptyGroup.Groups = []string{""}
	if errs := ValidateUserUpdate(emptyGroup, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidGroup := validObj()
	invalidGroup.Groups = []string{"bad:group:name"}
	if errs := ValidateUserUpdate(invalidGroup, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

}

func TestValidateIdentity(t *testing.T) {
	validObj := func() *userapi.Identity {
		return &userapi.Identity{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myprovider:myproviderusername",
			},
			ProviderName:     "myprovider",
			ProviderUserName: "myproviderusername",
			User:             kapi.ObjectReference{Name: "myuser", UID: "myuseruid"},
		}
	}

	if errs := ValidateIdentity(validObj()); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	noUserUID := validObj()
	noUserUID.User.UID = ""
	if errs := ValidateIdentity(noUserUID); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyProvider := validObj()
	emptyProvider.ProviderName = ""
	if errs := ValidateIdentity(emptyProvider); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidProvider := validObj()
	invalidProvider.ProviderName = "foo:bar"
	if errs := ValidateIdentity(invalidProvider); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyProviderUserName := validObj()
	emptyProviderUserName.ProviderUserName = ""
	if errs := ValidateIdentity(emptyProviderUserName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidProviderUserName := validObj()
	invalidProviderUserName.ProviderUserName = "user:name"
	if errs := ValidateIdentity(invalidProviderUserName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	mismatchName := validObj()
	mismatchName.ProviderUserName = "myproviderusername2"
	if errs := ValidateIdentity(mismatchName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateIdentityUpdate(t *testing.T) {
	validObj := func() *userapi.Identity {
		return &userapi.Identity{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "myprovider:myproviderusername",
				ResourceVersion: "1",
			},
			ProviderName:     "myprovider",
			ProviderUserName: "myproviderusername",
			User:             kapi.ObjectReference{Name: "myuser", UID: "myuseruid"},
		}
	}

	oldObj := validObj()

	if errs := ValidateIdentityUpdate(validObj(), oldObj); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	noUserUID := validObj()
	noUserUID.User.UID = ""
	if errs := ValidateIdentityUpdate(noUserUID, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyProvider := validObj()
	emptyProvider.ProviderName = ""
	if errs := ValidateIdentityUpdate(emptyProvider, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidProvider := validObj()
	invalidProvider.ProviderName = "foo:bar"
	if errs := ValidateIdentityUpdate(invalidProvider, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyProviderUserName := validObj()
	emptyProviderUserName.ProviderUserName = ""
	if errs := ValidateIdentityUpdate(emptyProviderUserName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	invalidProviderUserName := validObj()
	invalidProviderUserName.ProviderUserName = "user:name"
	if errs := ValidateIdentityUpdate(invalidProviderUserName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	mismatchName := validObj()
	mismatchName.ProviderUserName = "myproviderusername2"
	if errs := ValidateIdentityUpdate(mismatchName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateUserIdentityMapping(t *testing.T) {
	validObj := func() *userapi.UserIdentityMapping {
		return &userapi.UserIdentityMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name: "myprovider:myproviderusername",
			},
			Identity: kapi.ObjectReference{Name: "myprovider:myproviderusername"},
			User:     kapi.ObjectReference{Name: "myuser"},
		}
	}

	if errs := ValidateUserIdentityMapping(validObj()); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	mismatchName := validObj()
	mismatchName.Identity.Name = "myprovider:myproviderusername2"
	if errs := ValidateUserIdentityMapping(mismatchName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyIdentityName := validObj()
	emptyIdentityName.Identity.Name = ""
	if errs := ValidateUserIdentityMapping(emptyIdentityName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyUserName := validObj()
	emptyUserName.Identity.Name = ""
	if errs := ValidateUserIdentityMapping(emptyUserName); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}

func TestValidateUserIdentityMappingUpdate(t *testing.T) {
	validObj := func() *userapi.UserIdentityMapping {
		return &userapi.UserIdentityMapping{
			ObjectMeta: metav1.ObjectMeta{
				Name:            "myprovider:myproviderusername",
				ResourceVersion: "1",
			},
			Identity: kapi.ObjectReference{Name: "myprovider:myproviderusername"},
			User:     kapi.ObjectReference{Name: "myuser"},
		}
	}

	oldObj := validObj()

	if errs := ValidateUserIdentityMappingUpdate(validObj(), oldObj); len(errs) > 0 {
		t.Errorf("Expected no errors, got %v", errs)
	}

	mismatchName := validObj()
	mismatchName.Identity.Name = "myprovider:myproviderusername2"
	if errs := ValidateUserIdentityMappingUpdate(mismatchName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyIdentityName := validObj()
	emptyIdentityName.Identity.Name = ""
	if errs := ValidateUserIdentityMappingUpdate(emptyIdentityName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}

	emptyUserName := validObj()
	emptyUserName.Identity.Name = ""
	if errs := ValidateUserIdentityMappingUpdate(emptyUserName, oldObj); len(errs) == 0 {
		t.Errorf("Expected error, got none")
	}
}
