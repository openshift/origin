package integration

import (
	"fmt"
	"net/http"
	"reflect"
	"sort"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kubeerr "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/unversioned"
	authuser "k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/restclient"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/registry/generic"
	"k8s.io/kubernetes/pkg/registry/generic/registry"
	"k8s.io/kubernetes/pkg/serviceaccount"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/storage/storagebackend/factory"
	"k8s.io/kubernetes/pkg/util/wait"
	"k8s.io/kubernetes/pkg/watch"

	authenticationclient "github.com/openshift/origin/pkg/auth/client"
	"github.com/openshift/origin/pkg/authorization/authorizer/scope"
	osclient "github.com/openshift/origin/pkg/client"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	oauthapi "github.com/openshift/origin/pkg/oauth/api"
	"github.com/openshift/origin/pkg/oauth/registry/helpers"
	clientauthetcd "github.com/openshift/origin/pkg/oauth/registry/oauthclientauthorization/etcd"
	saoauth "github.com/openshift/origin/pkg/serviceaccounts/oauthclient"
	userapi "github.com/openshift/origin/pkg/user/api"
	testutil "github.com/openshift/origin/test/util"
	testserver "github.com/openshift/origin/test/util/server"
)

type clientAuthorizationTester struct {
	asClusterAdmin osclient.OAuthClientAuthorizationInterface

	destroyFunc factory.DestroyFunc
	rawStorage  storage.Interface
	rawPrefix   string
	ns          string
	sa          kclient.ServiceAccountsInterface
	sc          kclient.SecretsInterface
	user        osclient.UserInterface
	identity    osclient.IdentityInterface
	rc          *restclient.Config
	currentTest string
	t           *testing.T
}

func (o *clientAuthorizationTester) createClientAuthorizations(clients ...*oauthapi.OAuthClientAuthorization) {
	for _, client := range clients {
		if _, err := o.asClusterAdmin.Create(client); err != nil {
			o.t.Fatalf("%s failed: error creating client auth: %#v", o.currentTest, err)
		}
	}
}

func (o *clientAuthorizationTester) oldLocationEtcdCreate(clients ...*oauthapi.OAuthClientAuthorization) {
	ctx := kapi.NewContext()
	for _, client := range clients {
		// old name format
		client.Name = client.UserName + ":" + client.ClientName
		key, err := registry.NoNamespaceKeyFunc(ctx, o.rawPrefix, client.Name)
		if err != nil {
			o.t.Fatalf("%s failed: unexpected key error: %#v", o.currentTest, err)
		}
		if err = o.rawStorage.Create(ctx, key, client, nil, 0); err != nil {
			o.t.Fatalf("%s failed: unexpected create error: %#v", o.currentTest, err)
		}
	}
}

func (o *clientAuthorizationTester) createSA(name string) *kapi.ServiceAccount {
	serviceAccount := &kapi.ServiceAccount{
		ObjectMeta: kapi.ObjectMeta{
			Name:        name,
			Namespace:   o.ns,
			Annotations: map[string]string{saoauth.OAuthRedirectModelAnnotationURIPrefix + "foo": "http://bar"}},
	}
	serviceAccount, err := o.sa.Create(serviceAccount)
	if err != nil {
		o.t.Fatalf("%s failed: error creating SA: %#v", o.currentTest, err)
	}
	secret := &kapi.Secret{
		ObjectMeta: kapi.ObjectMeta{
			GenerateName: serviceAccount.Name,
			Annotations:  map[string]string{kapi.ServiceAccountNameKey: serviceAccount.Name},
		},
		Type: kapi.SecretTypeServiceAccountToken,
	}
	if _, err := o.sc.Create(secret); err != nil {
		o.t.Fatalf("%s failed: error creating secret: %#v", o.currentTest, err)
	}
	return serviceAccount
}

func (o *clientAuthorizationTester) createUser(userName string) (*userapi.User, osclient.SelfOAuthClientAuthorizationInterface) {
	userClient, _, _, err := testutil.GetClientForUser(*o.rc, userName)
	if err != nil {
		o.t.Fatalf("%s failed: error getting user client for %s: %#v", o.currentTest, userName, err)
	}
	self, err := userClient.Users().Get("~")
	if err != nil {
		o.t.Fatalf("%s failed: error getting user UID for %s: %#v", o.currentTest, userName, err)
	}
	return self, userClient.SelfOAuthClientAuthorizations()
}

func (o *clientAuthorizationTester) assertEqualList(expected *oauthapi.OAuthClientAuthorizationList, actual *oauthapi.OAuthClientAuthorizationList) error {
	zeroIgnoredFields(actual)
	sort.Sort(sortList(*expected))
	sort.Sort(sortList(*actual))
	if !reflect.DeepEqual(expected, actual) {
		return fmt.Errorf("%s EqualList failed\nexpected:\n%#v\ngot:\n%#v", o.currentTest, expected, actual)
	}
	return nil
}

func zeroIgnoredFields(list *oauthapi.OAuthClientAuthorizationList) {
	list.ResourceVersion = ""
	list.SelfLink = ""
	l := list.Items
	for i := range l {
		l[i].Name = ""
		l[i].ResourceVersion = ""
		l[i].UID = ""
		l[i].SelfLink = ""
		l[i].CreationTimestamp = unversioned.Time{}
	}
}

func (o *clientAuthorizationTester) assertEqualSelfList(expected *oauthapi.OAuthClientAuthorizationList, actual *oauthapi.SelfOAuthClientAuthorizationList) error {
	zeroSelfIgnoredFields(actual)
	e := clientauthetcd.ToSelfList(expected).(*oauthapi.SelfOAuthClientAuthorizationList)
	sort.Sort(sortSelfList(*e))
	sort.Sort(sortSelfList(*actual))
	if e.Items == nil {
		e.Items = []oauthapi.SelfOAuthClientAuthorization{} // don't want this to be nil for comparision with actual
	}
	if actual.Items == nil {
		actual.Items = []oauthapi.SelfOAuthClientAuthorization{} // don't want this to be nil for comparision with expected
	}
	if !reflect.DeepEqual(e, actual) {
		return fmt.Errorf("%s EqualSelfList failed\nexpected:\n%#v\ngot:\n%#v", o.currentTest, e, actual)
	}
	return nil
}

func zeroSelfIgnoredFields(list *oauthapi.SelfOAuthClientAuthorizationList) {
	list.ResourceVersion = ""
	list.SelfLink = ""
	l := list.Items
	for i := range l {
		l[i].ResourceVersion = ""
		l[i].UID = ""
		l[i].SelfLink = ""
		l[i].CreationTimestamp = unversioned.Time{}
	}
}

func (o *clientAuthorizationTester) assertGetSuccess(auth osclient.SelfOAuthClientAuthorizationInterface, expected *oauthapi.OAuthClientAuthorizationList, saList ...*kapi.ServiceAccount) error {
	if len(saList) == 0 {
		o.t.Errorf("%s failed: invalid test data %#v %#v", o.currentTest, saList, expected)
	}
	actual := &oauthapi.SelfOAuthClientAuthorizationList{}
	for _, sa := range saList {
		data, err := auth.Get(getSAName(sa))
		if err != nil {
			return fmt.Errorf("%s GetSuccess failed: error getting self client auth: %#v", o.currentTest, err)
		}
		actual.Items = append(actual.Items, *data)
	}
	return o.assertEqualSelfList(expected, actual)
}

func (o *clientAuthorizationTester) assertGetFailure(auth osclient.SelfOAuthClientAuthorizationInterface, saList ...*kapi.ServiceAccount) error {
	if len(saList) == 0 {
		o.t.Errorf("%s failed: invalid test data %#v", o.currentTest, saList)
	}
	for _, sa := range saList {
		if _, err := auth.Get(getSAName(sa)); err == nil || !kubeerr.IsNotFound(err) {
			return fmt.Errorf("%s GetFailure failed: did NOT return NotFound error when getting self client auth: %#v", o.currentTest, err)
		}
	}
	return nil
}

func (o *clientAuthorizationTester) cleanUp() {
	if o.t.Failed() {
		return // don't cleanup on failure
	}
	allAuths, err := o.asClusterAdmin.List(kapi.ListOptions{})
	if err != nil {
		o.t.Fatalf("%s failed: cleanup failed to list auths: %#v", o.currentTest, err)
	}
	for _, auth := range allAuths.Items {
		if err := o.asClusterAdmin.Delete(auth.Name); err != nil {
			o.t.Fatalf("%s failed: cleanup failed to delete auth %#v: %#v", o.currentTest, auth, err)
		}
	}
	allSAs, err := o.sa.List(kapi.ListOptions{})
	if err != nil {
		o.t.Fatalf("%s failed: cleanup failed to list SAs: %#v", o.currentTest, err)
	}
	for _, sa := range allSAs.Items {
		for _, secret := range sa.Secrets {
			if err := o.sc.Delete(secret.Name); err != nil {
				o.t.Fatalf("%s failed: cleanup failed to delete secret %#v: %#v", o.currentTest, secret, err)
			}
		}
		if err := o.sa.Delete(sa.Name); err != nil {
			o.t.Fatalf("%s failed: cleanup failed to delete SA %#v: %#v", o.currentTest, sa, err)
		}
	}
	allUsers, err := o.user.List(kapi.ListOptions{})
	if err != nil {
		o.t.Fatalf("%s failed: cleanup failed to list users: %#v", o.currentTest, err)
	}
	for _, user := range allUsers.Items {
		for _, identity := range user.Identities {
			if err := o.identity.Delete(identity); err != nil {
				o.t.Fatalf("%s failed: cleanup failed to delete identity %s: %#v", o.currentTest, identity, err)
			}
		}
		if err := o.user.Delete(user.Name); err != nil {
			o.t.Fatalf("%s failed: cleanup failed to delete user %#v: %#v", o.currentTest, user, err)
		}
	}
}

func (o *clientAuthorizationTester) backoffAssert(assert func() error) {
	backoff := wait.Backoff{
		Steps:    5,
		Duration: 2 * time.Second,
		Factor:   2.0,
		Jitter:   0.1,
	}
	var assertErr error
	if err := wait.ExponentialBackoff(backoff, func() (bool, error) {
		if assertErr = assert(); assertErr != nil {
			return false, nil
		}
		return true, nil
	}); err != nil {
		o.t.Errorf("%s failed\n%#v\n%#v", o.currentTest, assertErr, err)
	}
}

func (o *clientAuthorizationTester) assert(assert func() error) {
	if err := assert(); err != nil {
		o.t.Errorf("%s failed: %#v", o.currentTest, err)
	}
}

func (o *clientAuthorizationTester) asImpersonatingUser(user *userapi.User) osclient.SelfOAuthClientAuthorizationInterface {
	privilegedConfig := *o.rc
	oldWrapTransport := privilegedConfig.WrapTransport
	privilegedConfig.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		return authenticationclient.NewImpersonatingRoundTripper(&authuser.DefaultInfo{Name: user.GetName()}, oldWrapTransport(rt))
	}
	client, err := osclient.New(&privilegedConfig)
	if err != nil {
		o.t.Fatalf("%s failed: error getting impersonating user client for %#v: %#v", o.currentTest, user, err)
	}
	return client.SelfOAuthClientAuthorizations()
}

func (o *clientAuthorizationTester) runTest(testName string, test func()) {
	defer o.cleanUp()
	o.currentTest = testName
	test()
}

func (o *clientAuthorizationTester) assertEvents(expected *oauthapi.OAuthClientAuthorizationList, w watch.Interface, eventTypes ...watch.EventType) {
	if len(eventTypes) != len(expected.Items) || len(eventTypes) == 0 {
		o.t.Errorf("%s failed: invalid test data %#v %#v", o.currentTest, eventTypes, expected)
	}
	for i, eventType := range eventTypes {
		select {
		case event := <-w.ResultChan():
			if event.Type != eventType {
				o.t.Errorf("%s failed with wrong event type in %#v, exptected %s", o.currentTest, event, eventType)
			}
			expectedSingle := &oauthapi.OAuthClientAuthorizationList{Items: []oauthapi.OAuthClientAuthorization{expected.Items[i]}}
			switch auth := event.Object.(type) {
			case *oauthapi.SelfOAuthClientAuthorization:
				actualSingle := &oauthapi.SelfOAuthClientAuthorizationList{Items: []oauthapi.SelfOAuthClientAuthorization{*auth}}
				if err := o.assertEqualSelfList(expectedSingle, actualSingle); err != nil {
					o.t.Errorf("%s failed: self watch event at index %d does not match %#v", o.currentTest, event, i, err)
				}
			case *oauthapi.OAuthClientAuthorization:
				actualSingle := &oauthapi.OAuthClientAuthorizationList{Items: []oauthapi.OAuthClientAuthorization{*auth}}
				if err := o.assertEqualList(expectedSingle, actualSingle); err != nil {
					o.t.Errorf("%s failed: non-self watch event at index %d does not match %#v", o.currentTest, event, i, err)
				}
			default:
				o.t.Errorf("%s failed: watch event %#v at index %d has unexpected type", o.currentTest, event, i)
			}

		case <-time.After(30 * time.Second):
			o.t.Errorf("%s failed: timeout during watch", o.currentTest)
		}
	}
}

func newOAuthClientAuthorizationHandler(t *testing.T) *clientAuthorizationTester {
	masterConfig, err := testserver.DefaultMasterOptions()
	if err != nil {
		t.Fatalf("error getting master config: %#v", err)
	}

	kubeConfigFile, err := testserver.StartConfiguredMaster(masterConfig)
	if err != nil {
		t.Fatalf("error starting server: %#v", err)
	}
	kubeClient, err := testutil.GetClusterAdminKubeClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting client: %#v", err)
	}
	openshiftClient, err := testutil.GetClusterAdminClient(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client: %#v", err)
	}

	clusterAdminConfig, err := testutil.GetClusterAdminClientConfig(kubeConfigFile)
	if err != nil {
		t.Fatalf("error getting openshift client config: %v", err)
	}

	optsGetter := originrest.StorageOptions(*masterConfig)
	opts, err := optsGetter.GetRESTOptions(oauthapi.Resource("oauthclientauthorizations"))
	if err != nil {
		t.Fatalf("error getting oauthclientauthorizations RESTOptions: %#v", err)
	}

	clientAuthRawStorage, clientAuthDestroyFunc := generic.NewRawStorage(opts.StorageConfig)

	ns, err := kubeClient.Namespaces().Create(&kapi.Namespace{ObjectMeta: kapi.ObjectMeta{Name: "oauthclientauthorizationtest"}})
	if err != nil {
		t.Fatalf("error creating test namespace: %#v", err)
	}

	return &clientAuthorizationTester{
		asClusterAdmin: openshiftClient.OAuthClientAuthorizations(),

		destroyFunc: clientAuthDestroyFunc,
		rawStorage:  clientAuthRawStorage,
		rawPrefix:   opts.ResourcePrefix,
		ns:          ns.Name,
		rc:          clusterAdminConfig,
		user:        openshiftClient.Users(),
		identity:    openshiftClient.Identities(),
		sa:          kubeClient.ServiceAccounts(ns.Name),
		sc:          kubeClient.Secrets(ns.Name),
		currentTest: "ran newOAuthClientAuthorizationHandler",
		t:           t,
	}
}

func newOAuthClientAuthorization(sa *kapi.ServiceAccount, user *userapi.User, scopes ...string) *oauthapi.OAuthClientAuthorization {
	client := &oauthapi.OAuthClientAuthorization{
		ClientName: getSAName(sa),
		UserName:   user.GetName(),
		UserUID:    string(user.GetUID()),
		Scopes:     scopes,
	}
	return client
}

func getSAName(sa *kapi.ServiceAccount) string {
	return serviceaccount.MakeUsername(sa.Namespace, sa.Name)
}

func newOAuthClientAuthorizationList(in ...*oauthapi.OAuthClientAuthorization) *oauthapi.OAuthClientAuthorizationList {
	out := []oauthapi.OAuthClientAuthorization{}
	for _, client := range in {
		out = append(out, *client)
	}
	return &oauthapi.OAuthClientAuthorizationList{
		Items: out,
	}
}

type sortList oauthapi.OAuthClientAuthorizationList

func (s sortList) Len() int      { return len(s.Items) }
func (s sortList) Swap(i, j int) { s.Items[i], s.Items[j] = s.Items[j], s.Items[i] }
func (s sortList) Less(i, j int) bool {
	return helpers.MakeClientAuthorizationName(s.Items[i].UserName, s.Items[i].ClientName) <
		helpers.MakeClientAuthorizationName(s.Items[j].UserName, s.Items[j].ClientName)
}

type sortSelfList oauthapi.SelfOAuthClientAuthorizationList

func (s sortSelfList) Len() int           { return len(s.Items) }
func (s sortSelfList) Swap(i, j int)      { s.Items[i], s.Items[j] = s.Items[j], s.Items[i] }
func (s sortSelfList) Less(i, j int) bool { return s.Items[i].ClientName < s.Items[j].ClientName }

// TestOAuthClientAuthorizationStorage makes sure that OAuthClientAuthorizations stored at both
// the old and new locations are accessible by the cluster admin.  It also checks to make sure that
// the user can "get", "list", "watch", "delete", "deletecollection" their (new location) SelfOAuthClientAuthorizations.
func TestOAuthClientAuthorizationStorage(t *testing.T) {
	testutil.RequireEtcd(t)
	defer testutil.DumpEtcdOnFailure(t)
	clientTester := newOAuthClientAuthorizationHandler(t)
	defer clientTester.destroyFunc()

	clientTester.runTest("user can get and list their new client authorizations", func() {
		sa1 := clientTester.createSA("sa1")
		sa2 := clientTester.createSA("sa2")
		user1, user1Auth := clientTester.createUser("user1")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expected, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1Auth, expected, sa1, sa2) })
	})

	clientTester.runTest("user can delete their new client authorizations", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		expectedBeforeDelete := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedBeforeDelete, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1Auth, expectedBeforeDelete, sa1) })

		if err := user1Auth.Delete(getSAName(sa1)); err != nil {
			t.Errorf("%s failed during delete: %#v", clientTester.currentTest, err)
		}

		expectedAfterDelete := newOAuthClientAuthorizationList(
		// should be empty
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedAfterDelete, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetFailure(user1Auth, sa1) })
	})

	clientTester.runTest("user cannot see other user's client authorizations", func() {
		sa1 := clientTester.createSA("sa1")
		sa2 := clientTester.createSA("sa2")
		user1, user1Auth := clientTester.createUser("user1")
		user2, _ := clientTester.createUser("user2")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user2, scope.UserInfo),
		)

		expectedUser1 := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedUser1, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1Auth, expectedUser1, sa1) })
		clientTester.assert(func() error { return clientTester.assertGetFailure(user1Auth, sa2) })
	})

	clientTester.runTest("user cannot see client authorizations stored in the old location", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")

		clientTester.oldLocationEtcdCreate(
			newOAuthClientAuthorization(sa1, user1, scope.UserInfo),
		)

		expected := newOAuthClientAuthorizationList(
		// should be empty
		)

		clientTester.assert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expected, actual)
		})
		clientTester.assert(func() error { return clientTester.assertGetFailure(user1Auth, sa1) })
	})

	clientTester.runTest("cluster admin can see client authorizations stored in the both old and new location", func() {
		sa1 := clientTester.createSA("sa1")
		sa2 := clientTester.createSA("sa2")
		user1, _ := clientTester.createUser("user1")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)
		clientTester.oldLocationEtcdCreate(
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)

		clientTester.backoffAssert(func() error {
			actual, err := clientTester.asClusterAdmin.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualList(expected, actual)
		})
	})

	clientTester.runTest("cluster admin deletes are reflected to the user", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		expectedBeforeDelete := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedBeforeDelete, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1Auth, expectedBeforeDelete, sa1) })

		name := helpers.MakeClientAuthorizationName(user1.GetName(), getSAName(sa1))
		if err := clientTester.asClusterAdmin.Delete(name); err != nil {
			t.Errorf("%s failed during delete of %s: %#v", clientTester.currentTest, name, err)
		}

		expectedAfterDelete := newOAuthClientAuthorizationList(
		// should be empty
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedAfterDelete, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetFailure(user1Auth, sa1) })
	})

	clientTester.runTest("user cannot see client authorizations for a different UID + their name but cluster admin can see all via non-self and impersonation", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		expectedOldUID := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		clientTester.backoffAssert(func() error {
			actual, err := user1Auth.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedOldUID, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1Auth, expectedOldUID, sa1) })

		clientTester.backoffAssert(func() error {
			actual, err := clientTester.asClusterAdmin.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualList(expectedOldUID, actual)
		})

		// delete and recreate user1 so he has a different UID
		for _, identity := range user1.Identities {
			if err := clientTester.identity.Delete(identity); err != nil {
				t.Errorf("%s failed to delete identity %s: %#v", clientTester.currentTest, identity, err)
			}
		}
		if err := clientTester.user.Delete(user1.GetName()); err != nil {
			t.Errorf("%s failed to delete user %#v: %#v", clientTester.currentTest, user1, err)
		}
		user1New, user1AuthNew := clientTester.createUser("user1")
		if user1.GetUID() == user1New.GetUID() {
			t.Errorf("%s failed to create user with new UID: %#v", clientTester.currentTest, user1New)
		}

		expectedNewUID := newOAuthClientAuthorizationList(
		// should be empty
		)

		clientTester.assert(func() error {
			actual, err := user1AuthNew.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedNewUID, actual)
		})
		clientTester.assert(func() error { return clientTester.assertGetFailure(user1AuthNew, sa1) })

		user1AuthImpersonate := clientTester.asImpersonatingUser(user1)
		clientTester.backoffAssert(func() error {
			actual, err := user1AuthImpersonate.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing self client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualSelfList(expectedOldUID, actual)
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1AuthImpersonate, expectedOldUID, sa1) })

		clientTester.backoffAssert(func() error {
			actual, err := clientTester.asClusterAdmin.List(kapi.ListOptions{})
			if err != nil {
				return fmt.Errorf("%s failed: error listing client auths: %#v", clientTester.currentTest, err)
			}
			return clientTester.assertEqualList(expectedOldUID, actual)
		})
	})

	clientTester.runTest("user can watch their own client autorizations", func() {
		sa1 := clientTester.createSA("sa1")
		sa2 := clientTester.createSA("sa2")
		user1, user1Auth := clientTester.createUser("user1")

		user1Watch, err := user1Auth.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user1Watch.Stop()

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)
		if err := user1Auth.Delete(getSAName(sa2)); err != nil {
			t.Errorf("%s failed during delete: %#v", clientTester.currentTest, err)
		}
		newSA1User1Auth, err := clientTester.asClusterAdmin.Get(helpers.MakeClientAuthorizationName(user1.GetName(), getSAName(sa1)))
		if err != nil {
			t.Errorf("%s failed during get: %#v", clientTester.currentTest, err)
		}
		newSA1User1Auth.Scopes = []string{scope.UserInfo}
		if _, err := clientTester.asClusterAdmin.Update(newSA1User1Auth); err != nil {
			t.Errorf("%s failed during update: %#v", clientTester.currentTest, err)
		}

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
			newOAuthClientAuthorization(sa1, user1, scope.UserInfo),
		)

		clientTester.assertEvents(expected, user1Watch,
			watch.Added,
			watch.Added,
			watch.Deleted,
			watch.Modified,
		)
	})

	clientTester.runTest("user cannot see other user's client autorizations during a watch", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")
		user2, user2Auth := clientTester.createUser("user2")

		user1Watch, err := user1Auth.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user1Watch.Stop()
		user2Watch, err := user2Auth.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user2Watch.Stop()

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa1, user2, scope.UserInfo),
		)

		expectedUser1 := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)
		expectedUser2 := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user2, scope.UserInfo),
		)

		clientTester.assertEvents(expectedUser1, user1Watch,
			watch.Added,
		)
		clientTester.assertEvents(expectedUser2, user2Watch,
			watch.Added,
		)
	})

	clientTester.runTest("cluster admin watch sees all events", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")
		user2, user2Auth := clientTester.createUser("user2")

		clusterAdminWatch, err := clientTester.asClusterAdmin.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer clusterAdminWatch.Stop()

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserInfo),
			newOAuthClientAuthorization(sa1, user2, scope.UserInfo),
		)
		if err := user1Auth.Delete(getSAName(sa1)); err != nil {
			t.Errorf("%s failed during delete: %#v", clientTester.currentTest, err)
		}
		if err := user2Auth.Delete(getSAName(sa1)); err != nil {
			t.Errorf("%s failed during delete: %#v", clientTester.currentTest, err)
		}

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserInfo),
			newOAuthClientAuthorization(sa1, user2, scope.UserInfo),
			newOAuthClientAuthorization(sa1, user1, scope.UserInfo),
			newOAuthClientAuthorization(sa1, user2, scope.UserInfo),
		)

		clientTester.assertEvents(expected, clusterAdminWatch,
			watch.Added,
			watch.Added,
			watch.Deleted,
			watch.Deleted,
		)
	})

	clientTester.runTest("impersonation allows watch on self auth with different UID", func() {
		sa1 := clientTester.createSA("sa1")
		sa2 := clientTester.createSA("sa2")
		user1, user1Auth := clientTester.createUser("user1")
		user1WithDifferentUID := *user1
		user1WithDifferentUID.UID = "foo"

		user1Watch, err := user1Auth.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user1Watch.Stop()

		user1AuthImpersonate := clientTester.asImpersonatingUser(user1)
		user1ImpersonateWatch, err := user1AuthImpersonate.Watch(kapi.ListOptions{})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user1ImpersonateWatch.Stop()

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, &user1WithDifferentUID, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)

		expectedUser1 := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)
		clientTester.assertEvents(expectedUser1, user1Watch,
			watch.Added,
		)

		expectedUser1Impersonate := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, &user1WithDifferentUID, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa2, user1, scope.UserInfo),
		)
		clientTester.assertEvents(expectedUser1Impersonate, user1ImpersonateWatch,
			watch.Added,
			watch.Added,
		)
	})

	clientTester.runTest("impersonation allows get and delete on self auth with different UID", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")
		user1WithDifferentUID := *user1
		user1WithDifferentUID.UID = "foo"

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, &user1WithDifferentUID, scope.UserListAllProjects),
		)

		clientTester.assert(func() error { return clientTester.assertGetFailure(user1Auth, sa1) })
		clientTester.assert(func() error {
			if err := user1Auth.Delete(getSAName(sa1)); err == nil || !kubeerr.IsNotFound(err) {
				return fmt.Errorf("%s failed: did NOT return NotFound error when deleting self client auth: %#v", clientTester.currentTest, err)
			}
			return nil
		})

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, &user1WithDifferentUID, scope.UserListAllProjects),
		)

		user1AuthImpersonate := clientTester.asImpersonatingUser(user1)
		clientTester.backoffAssert(func() error { return clientTester.assertGetSuccess(user1AuthImpersonate, expected, sa1) })
		clientTester.backoffAssert(func() error {
			if err := user1AuthImpersonate.Delete(getSAName(sa1)); err != nil {
				return fmt.Errorf("%s failed: error when deleting self client auth: %#v", clientTester.currentTest, err)
			}
			return nil
		})
		clientTester.backoffAssert(func() error { return clientTester.assertGetFailure(user1AuthImpersonate, sa1) })
	})

	clientTester.runTest("user can see watch events from resource version 0", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")

		// only do a single add because order is undefined with ResourceVersion 0
		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		expected := newOAuthClientAuthorizationList(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
		)

		user1Watch, err := user1Auth.Watch(kapi.ListOptions{ResourceVersion: "0"})
		if err != nil {
			t.Errorf("%s failed to watch: %#v", clientTester.currentTest, err)
		}
		defer user1Watch.Stop()

		clientTester.assertEvents(expected, user1Watch,
			watch.Added,
		)
	})

	clientTester.runTest("user cannot get or delete using fully qualified name", func() {
		sa1 := clientTester.createSA("sa1")
		user1, user1Auth := clientTester.createUser("user1")
		user2, _ := clientTester.createUser("user2")

		clientTester.createClientAuthorizations(
			newOAuthClientAuthorization(sa1, user1, scope.UserListAllProjects),
			newOAuthClientAuthorization(sa1, user2, scope.UserListAllProjects),
		)

		clientTester.assert(func() error {
			for _, user := range []*userapi.User{user1, user2} {
				name := helpers.MakeClientAuthorizationName(user.GetName(), getSAName(sa1))
				if _, err := user1Auth.Get(name); err == nil || !kubeerr.IsBadRequest(err) {
					return fmt.Errorf("%s failed: did NOT return BadRequest error when getting fully qualified self client auth for user %#v: %#v", clientTester.currentTest, user, err)
				}
				if err := user1Auth.Delete(name); err == nil || !kubeerr.IsBadRequest(err) {
					return fmt.Errorf("%s failed: did NOT return BadRequest error when deleting fully qualified self client auth for user %#v: %#v", clientTester.currentTest, user, err)
				}
			}
			return nil
		})
	})
}
