package authentication

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	userscheme "github.com/openshift/client-go/user/clientset/versioned/scheme"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	restfake "k8s.io/client-go/rest/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestComponentProxyUserCleanup(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name         string
		createUser   bool
		createGroup  bool
		failResource string
		failVerb     string
	}{
		{name: "complete login", createUser: true, createGroup: true},
		{name: "identity without user"},
		{name: "group without user", createGroup: true},
		{name: "user deleted mid test", createGroup: true},
		{name: "identity list error", createUser: true, createGroup: true, failResource: "identities", failVerb: "list"},
		{name: "identity delete error", createUser: true, createGroup: true, failResource: "identities", failVerb: "delete"},
		{name: "group delete error", createUser: true, createGroup: true, failResource: "groups", failVerb: "delete"},
		{name: "user delete error", createUser: true, createGroup: true, failResource: "users", failVerb: "delete"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			client := newComponentProxyTestClient(t,
				&userv1.User{ObjectMeta: metav1.ObjectMeta{Name: "unrelated-user"}},
				&userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "unrelated-group"}},
				&userv1.Identity{ObjectMeta: metav1.ObjectMeta{Name: "other:subject"}, ProviderName: "other"},
			)
			cleanup, err := newComponentProxyUserCleanup(ctx, client.UserV1(), "test-idp", "test-user", "test-group")
			if err != nil {
				t.Fatal(err)
			}
			objects := []runtime.Object{
				&userv1.Identity{ObjectMeta: metav1.ObjectMeta{Name: "test-idp:subject", UID: "first"}, ProviderName: "test-idp"},
				&userv1.Identity{ObjectMeta: metav1.ObjectMeta{Name: "test-idp:another-subject", UID: "second"}, ProviderName: "test-idp"},
			}
			if testCase.createUser {
				objects = append(objects, &userv1.User{ObjectMeta: metav1.ObjectMeta{Name: "test-user"}})
			}
			if testCase.createGroup {
				objects = append(objects, &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "test-group"}})
			}
			for _, object := range objects {
				if err := client.Tracker().Add(object); err != nil {
					t.Fatal(err)
				}
			}
			if testCase.failResource != "" {
				client.PrependReactor(testCase.failVerb, testCase.failResource, func(action clienttesting.Action) (bool, runtime.Object, error) {
					return true, nil, fmt.Errorf("injected %s failure", testCase.failResource)
				})
			}
			err = cleanup(ctx)
			if testCase.failResource == "" && err != nil {
				t.Fatal(err)
			}
			if testCase.failResource != "" && (err == nil || !strings.Contains(err.Error(), "injected "+testCase.failResource+" failure")) {
				t.Fatalf("expected injected failure, got %v", err)
			}
			for resource, names := range map[string][]string{
				"users": {"test-user"}, "groups": {"test-group"}, "identities": {"test-idp:subject", "test-idp:another-subject"},
			} {
				if resource == testCase.failResource {
					continue
				}
				for _, name := range names {
					_, err := client.Tracker().Get(userv1.SchemeGroupVersion.WithResource(resource), "", name)
					if !apierrors.IsNotFound(err) {
						t.Errorf("%s %s remains: %v", resource, name, err)
					}
				}
			}
			for resource, name := range map[string]string{"users": "unrelated-user", "groups": "unrelated-group", "identities": "other:subject"} {
				if _, err := client.Tracker().Get(userv1.SchemeGroupVersion.WithResource(resource), "", name); err != nil {
					t.Errorf("unrelated %s was affected: %v", resource, err)
				}
			}
			if testCase.failResource == "" {
				if err := cleanup(ctx); err != nil {
					t.Fatalf("repeated cleanup: %v", err)
				}
			}
		})
	}
}

func TestComponentProxyUserCleanupRejectsExistingResources(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name   string
		object runtime.Object
	}{
		{name: "user", object: &userv1.User{ObjectMeta: metav1.ObjectMeta{Name: "test-user"}}},
		{name: "group", object: &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "test-group"}}},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			client := newComponentProxyTestClient(t, testCase.object)
			cleanup, err := newComponentProxyUserCleanup(context.Background(), client.UserV1(), "test-idp", "test-user", "test-group")
			if err == nil || cleanup != nil {
				t.Fatalf("expected collision rejection, got cleanup %v, error %v", cleanup != nil, err)
			}
		})
	}
}

func TestComponentProxyUserCleanupNotFoundDuringDelete(t *testing.T) {
	t.Parallel()
	client := newComponentProxyTestClient(t)
	cleanup, err := newComponentProxyUserCleanup(context.Background(), client.UserV1(), "test-idp", "test-user", "test-group")
	if err != nil {
		t.Fatal(err)
	}
	if err := client.Tracker().Add(&userv1.Identity{ObjectMeta: metav1.ObjectMeta{Name: "test-idp:subject"}, ProviderName: "test-idp"}); err != nil {
		t.Fatal(err)
	}
	client.PrependReactor("delete", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: userv1.GroupName, Resource: action.GetResource().Resource}, action.(clienttesting.DeleteAction).GetName())
	})
	if err := cleanup(context.Background()); err != nil {
		t.Fatalf("NotFound must be tolerated: %v", err)
	}
}

type componentProxyTestClient struct {
	clienttesting.Fake
	tracker clienttesting.ObjectTracker
}

func newComponentProxyTestClient(t *testing.T, objects ...runtime.Object) *componentProxyTestClient {
	t.Helper()
	client := &componentProxyTestClient{tracker: clienttesting.NewObjectTracker(userscheme.Scheme, userscheme.Codecs.UniversalDecoder())}
	for _, object := range objects {
		if err := client.tracker.Add(object); err != nil {
			t.Fatal(err)
		}
	}
	client.AddReactor("*", "*", clienttesting.ObjectReaction(client.tracker))
	return client
}

func (client *componentProxyTestClient) Tracker() clienttesting.ObjectTracker {
	return client.tracker
}

func (client *componentProxyTestClient) UserV1() userv1client.UserV1Interface {
	return userv1client.New(&restfake.RESTClient{
		GroupVersion:         userv1.SchemeGroupVersion,
		NegotiatedSerializer: userscheme.Codecs.WithoutConversion(),
		VersionedAPIPath:     "/apis/user.openshift.io/v1",
		Client: restfake.CreateHTTPClient(func(request *http.Request) (*http.Response, error) {
			var action clienttesting.Action
			name := path.Base(request.URL.Path)
			resource := userv1.SchemeGroupVersion.WithResource(path.Base(path.Dir(request.URL.Path)))
			switch {
			case request.Method == http.MethodGet && name == "identities":
				action = clienttesting.NewRootListAction(userv1.SchemeGroupVersion.WithResource("identities"), userv1.SchemeGroupVersion.WithKind("Identity"), metav1.ListOptions{})
			case request.Method == http.MethodGet:
				action = clienttesting.NewRootGetAction(resource, name)
			case request.Method == http.MethodDelete:
				var options metav1.DeleteOptions
				if err := json.NewDecoder(request.Body).Decode(&options); err != nil {
					return nil, err
				}
				action = clienttesting.NewRootDeleteActionWithOptions(resource, name, options)
			default:
				return nil, fmt.Errorf("unexpected request: %s %s", request.Method, request.URL.Path)
			}
			object, err := client.Invokes(action, &metav1.Status{Status: "Success"})
			statusCode := http.StatusOK
			if err != nil {
				statusCode = http.StatusInternalServerError
				if apierrors.IsNotFound(err) {
					statusCode = http.StatusNotFound
				}
				object = &metav1.Status{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Status"}, Status: "Failure", Code: int32(statusCode), Message: err.Error()}
			}
			body, err := json.Marshal(object)
			return &http.Response{StatusCode: statusCode, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}, err
		}),
	})
}

func TestComponentProxyUserCleanupContinuesAndAggregates(t *testing.T) {
	t.Parallel()
	client := newComponentProxyTestClient(t)
	cleanup, err := newComponentProxyUserCleanup(context.Background(), client.UserV1(), "test-idp", "test-user", "test-group")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"test-idp:first", "test-idp:second"} {
		if err := client.Tracker().Add(&userv1.Identity{ObjectMeta: metav1.ObjectMeta{Name: name, UID: types.UID(name)}, ProviderName: "test-idp"}); err != nil {
			t.Fatal(err)
		}
	}
	client.PrependReactor("delete", "*", func(action clienttesting.Action) (bool, runtime.Object, error) {
		deletion := action.(clienttesting.DeleteAction)
		if action.GetResource().Resource == "identities" {
			preconditions := deletion.GetDeleteOptions().Preconditions
			if preconditions == nil || preconditions.UID == nil || *preconditions.UID != types.UID(deletion.GetName()) {
				t.Error("identity deletion must use the observed UID")
			}
		}
		if deletion.GetName() == "test-idp:first" || deletion.GetName() == "test-group" {
			return true, nil, fmt.Errorf("injected failure for %s", deletion.GetName())
		}
		return false, nil, nil
	})
	err = cleanup(context.Background())
	if err == nil || !strings.Contains(err.Error(), "test-idp:first") || !strings.Contains(err.Error(), "test-group") {
		t.Fatalf("expected both errors, got %v", err)
	}
	if _, err := client.Tracker().Get(userv1.SchemeGroupVersion.WithResource("identities"), "", "test-idp:second"); !apierrors.IsNotFound(err) {
		t.Fatalf("second identity was not deleted: %v", err)
	}
}

func TestComponentProxyUserCleanupPreflightErrors(t *testing.T) {
	t.Parallel()
	for _, resource := range []string{"users", "groups"} {
		t.Run(resource, func(t *testing.T) {
			t.Parallel()
			client := newComponentProxyTestClient(t)
			client.PrependReactor("get", resource, func(action clienttesting.Action) (bool, runtime.Object, error) {
				return true, nil, fmt.Errorf("injected GET failure")
			})
			cleanup, err := newComponentProxyUserCleanup(context.Background(), client.UserV1(), "test-idp", "test-user", "test-group")
			if err == nil || cleanup != nil {
				t.Fatalf("expected error and no cleanup, got %v", err)
			}
			for _, action := range client.Actions() {
				if action.GetVerb() != "get" {
					t.Errorf("unexpected preflight action: %v", action)
				}
			}
		})
	}
}
