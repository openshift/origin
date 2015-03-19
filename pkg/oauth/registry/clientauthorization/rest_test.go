package clientauthorization

import (
	"errors"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	oapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/test"
)

func TestCreateValidationError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.OAuthClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "authTokenName"},
		// ClientName: "clientName",// Missing required field
		UserName: "userName",
		UserUID:  "userUID",
	}

	ctx := api.NewContext()
	_, err := storage.Create(ctx, clientAuth)
	if err == nil {
		t.Errorf("Expected validation error")
	}
}

func TestCreateStorageError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	registry.Err = errors.New("Sample Error")

	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.OAuthClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
		ClientName: "clientName",
		UserName:   "userName",
		UserUID:    "userUID",
	}

	ctx := api.NewContext()
	_, err := storage.Create(ctx, clientAuth)
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	storage := REST{
		registry: registry,
	}
	clientAuth := &oapi.OAuthClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
		ClientName: "clientName",
		UserName:   "userName",
		UserUID:    "userUID",
	}

	ctx := api.NewContext()
	obj, err := storage.Create(ctx, clientAuth)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *api.Status:
		t.Errorf("Got back unexpected status: %#v", r)
	case *oapi.OAuthClientAuthorization:
		// expected case
	default:
		t.Errorf("Got unexpected type: %#v", r)
	}
}

func TestGetError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}
	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	_, err := storage.Get(ctx, "name")
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestGetValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorization = &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "clientName"}}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuth, err := storage.Get(ctx, "name")
	if err != nil {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	if clientAuth != registry.ClientAuthorization {
		t.Errorf("got unexpected clientAuthorization: %v", clientAuth)
		return
	}
}

func TestListError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	_, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err == nil {
		t.Errorf("expected error")
		return
	}
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
}

func TestListEmpty(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorizations = &oapi.OAuthClientAuthorizationList{}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuths, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch clientAuths := clientAuths.(type) {
	case *oapi.OAuthClientAuthorizationList:
		if len(clientAuths.Items) != 0 {
			t.Errorf("expected empty list, got %#v", clientAuths)
		}
	default:
		t.Errorf("expected clientAuthList, got: %v", clientAuths)
		return
	}
}

func TestList(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorizations = &oapi.OAuthClientAuthorizationList{
		Items: []oapi.OAuthClientAuthorization{
			{},
			{},
		},
	}
	storage := REST{
		registry: registry,
	}
	ctx := api.NewContext()
	clientAuths, err := storage.List(ctx, labels.Everything(), fields.Everything())
	if err != registry.Err {
		t.Errorf("got unexpected error: %v", err)
		return
	}
	switch clientAuths := clientAuths.(type) {
	case *oapi.OAuthClientAuthorizationList:
		if len(clientAuths.Items) != 2 {
			t.Errorf("expected list with 2 items, got %#v", clientAuths)
		}
	default:
		t.Errorf("expected clientAuthList, got: %v", clientAuths)
		return
	}
}

func TestUpdateNotSupported(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Storage Error")
	storage := REST{
		registry: registry,
	}
	client := &oapi.OAuthClientAuthorization{
		ObjectMeta: api.ObjectMeta{Name: "clientName"},
		ClientName: "test",
		UserName:   "test",
		UserUID:    "test",
	}

	ctx := api.NewContext()
	_, created, err := storage.Update(ctx, client)
	if err != registry.Err || created {
		t.Errorf("unexpected err: %v", err)
		return
	}
}

func TestDeleteError(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.Err = errors.New("Sample Error")
	storage := REST{
		registry: registry,
	}

	ctx := api.NewContext()
	_, err := storage.Delete(ctx, "foo")
	if err != registry.Err {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestDeleteValid(t *testing.T) {
	registry := &test.ClientAuthorizationRegistry{}

	registry.ClientAuthorization = &oapi.OAuthClientAuthorization{ObjectMeta: api.ObjectMeta{Name: "foo"}}
	storage := REST{
		registry: registry,
	}

	ctx := api.NewContext()
	obj, err := storage.Delete(ctx, "foo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	switch r := obj.(type) {
	case *api.Status:
		if r.Status != "Success" {
			t.Fatalf("Got back non-success status: %#v", obj)
		}
	default:
		t.Fatalf("Got back non-status result: %v", obj)
	}
}
