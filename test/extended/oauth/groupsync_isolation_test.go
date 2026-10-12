package oauth

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"testing"

	userv1 "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/test/extended/testdata"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/yaml"
)

type ldapFakeGroups struct {
	userclient.GroupInterface
	items            map[string]*userv1.Group
	listErr          error
	updateErr        error
	deleteErr        map[string]error
	beforeDelete     func(string)
	updated, deleted []string
}

func (client *ldapFakeGroups) List(context.Context, metav1.ListOptions) (*userv1.GroupList, error) {
	if client.listErr != nil {
		return nil, client.listErr
	}
	groups := &userv1.GroupList{}
	for _, group := range client.items {
		groups.Items = append(groups.Items, *group.DeepCopy())
	}
	return groups, nil
}

func (client *ldapFakeGroups) Get(ctx context.Context, name string, options metav1.GetOptions) (*userv1.Group, error) {
	if group, exists := client.items[name]; exists {
		return group.DeepCopy(), nil
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{Group: userv1.GroupName, Resource: "groups"}, name)
}

func (client *ldapFakeGroups) Update(ctx context.Context, group *userv1.Group, options metav1.UpdateOptions) (*userv1.Group, error) {
	if client.updateErr != nil {
		return nil, client.updateErr
	}
	client.updated = append(client.updated, group.Name)
	client.items[group.Name] = group.DeepCopy()
	return group.DeepCopy(), nil
}

func (client *ldapFakeGroups) Delete(ctx context.Context, name string, options metav1.DeleteOptions) error {
	client.deleted = append(client.deleted, name)
	if client.beforeDelete != nil {
		client.beforeDelete(name)
	}
	if err := client.deleteErr[name]; err != nil {
		return err
	}
	group, err := client.Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return err
	}
	if options.Preconditions == nil || options.Preconditions.UID == nil || *options.Preconditions.UID != group.UID {
		return fmt.Errorf("missing or wrong UID precondition for %s", name)
	}
	delete(client.items, name)
	return nil
}

func ldapScopeForTest(t *testing.T) (*ldapGroupScope, *ldapFakeGroups) {
	t.Helper()
	client := &ldapFakeGroups{items: map[string]*userv1.Group{}, deleteErr: map[string]error{}}
	scope, err := newLDAPGroupScope(context.Background(), client, "test-run", "127.19.20.21")
	if err != nil {
		t.Fatal(err)
	}
	return scope, client
}

func ldapFixtureForTest(t *testing.T, scope *ldapGroupScope, schema, name string) ([]byte, []*userv1.Group) {
	t.Helper()
	content := scope.rewrite(string(testdata.MustAsset("test/extended/testdata/ldap/groupsync/" + schema + "/" + name)))
	content = strings.ReplaceAll(content, "LDAP_SERVICE_IP:389", scope.host+":30389")
	content = strings.ReplaceAll(content, "LDAP_SERVICE_IP", scope.host)
	var groups []*userv1.Group
	for _, document := range strings.Split(content, ldapGroupHeader)[1:] {
		group := &userv1.Group{}
		if err := yaml.Unmarshal([]byte(ldapGroupHeader+document), group); err != nil {
			t.Fatal(err)
		}
		group.TypeMeta = metav1.TypeMeta{}
		group.UID = types.UID(group.Name)
		groups = append(groups, group)
	}
	return []byte(content), groups
}

func TestLDAPGroupComparisonIsolation(t *testing.T) {
	for _, scenario := range []string{"valid", "missing", "missing label", "missing ownership metadata", "wrong label", "wrong users", "wrong uid", "extra owned", "extra marked"} {
		t.Run(scenario, func(t *testing.T) {
			scope, client := ldapScopeForTest(t)
			expected, groups := ldapFixtureForTest(t, scope, "rfc2307", "valid_all_ldap_sync.yaml")
			for _, group := range groups {
				client.items[group.Name] = group
			}
			unrelated := &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "e2e-proxy-kc-group-unrelated"}, Users: userv1.OptionalNames{"keep"}}
			client.items[unrelated.Name] = unrelated.DeepCopy()
			client.items["group1"] = &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "group1", Labels: map[string]string{"openshift.io/ldap.host": "127.0.0.1"}}, Users: userv1.OptionalNames{"keep"}}
			switch scenario {
			case "missing":
				delete(client.items, groups[0].Name)
			case "missing label":
				groups[0].Labels = nil
			case "missing ownership metadata":
				groups[0].Labels = nil
				groups[0].Annotations = nil
			case "wrong label":
				groups[0].Labels["openshift.io/ldap.host"] = "127.0.0.1"
			case "wrong users":
				groups[0].Users = userv1.OptionalNames{"wrong"}
			case "wrong uid":
				groups[0].Annotations["openshift.io/ldap.uid"] = "wrong"
			case "extra owned":
				client.items["unexpected-test-run"] = &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "unexpected-test-run", Labels: map[string]string{"openshift.io/ldap.host": scope.host}}}
			case "extra marked":
				client.items["unexpected-test-run"] = &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "unexpected-test-run"}}
			}
			err := scope.compare(context.Background(), expected)
			if (err == nil) != (scenario == "valid") {
				t.Fatalf("comparison result for %s: %v", scenario, err)
			}
			if err := scope.cleanup(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(client.items) != 2 || !reflect.DeepEqual(client.items[unrelated.Name], unrelated) || !reflect.DeepEqual(client.items["group1"].Users, userv1.OptionalNames{"keep"}) {
				t.Fatalf("unrelated groups were changed or owned resources remain: %#v", client.items)
			}
			if err := scope.cleanup(context.Background()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestLDAPGroupMembershipResetIsolation(t *testing.T) {
	scope, client := ldapScopeForTest(t)
	_, groups := ldapFixtureForTest(t, scope, "ad", "valid_all_ldap_sync.yaml")
	for _, group := range groups {
		client.items[group.Name] = group
	}
	unrelated := &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "firstgroup"}, Users: userv1.OptionalNames{"keep"}}
	client.items[unrelated.Name] = unrelated.DeepCopy()
	if err := scope.clearUsers(context.Background(), "firstgroup"); err == nil {
		t.Fatal("accepted an unrelated group")
	}
	if err := scope.clearUsers(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, group := range groups {
		if len(client.items[group.Name].Users) != 0 {
			t.Fatal("owned group not reset")
		}
	}
	if !reflect.DeepEqual(client.items[unrelated.Name], unrelated) {
		t.Fatal("unrelated group changed")
	}
	client.updateErr = fmt.Errorf("update denied")
	if err := scope.clearUsers(context.Background(), groups[0].Name); err == nil {
		t.Fatal("update error hidden")
	}
	client.updateErr = nil
	client.items[groups[0].Name].Labels = nil
	if err := scope.clearUsers(context.Background(), groups[0].Name); err == nil {
		t.Fatal("accepted missing ownership metadata")
	}
	delete(client.items, groups[0].Name)
	if err := scope.clearUsers(context.Background(), groups[0].Name); !apierrors.IsNotFound(err) {
		t.Fatalf("missing reset target: %v", err)
	}
}

func TestLDAPGroupMappingAndPruneComparison(t *testing.T) {
	for _, scenario := range []string{"full", "pruned", "retained pruned group", "missing", "missing label", "wrong label", "wrong URL", "wrong users", "unexpected name"} {
		t.Run(scenario, func(t *testing.T) {
			scope, client := ldapScopeForTest(t)
			expected, groups := ldapFixtureForTest(t, scope, "augmented-ad", "valid_all_ldap_sync_user_defined.yaml")
			for _, group := range groups {
				client.items[group.Name] = group
			}
			names := []string{scope.rewrite("firstgroup"), scope.rewrite("secondgroup"), scope.rewrite("thirdgroup")}
			switch scenario {
			case "pruned":
				delete(client.items, names[1])
				names = []string{names[0], names[2]}
			case "retained pruned group":
				names = []string{names[0], names[2]}
			case "missing":
				delete(client.items, names[0])
			case "missing label":
				client.items[names[0]].Labels = nil
			case "wrong label":
				client.items[names[0]].Labels["openshift.io/ldap.host"] = "127.0.0.1"
			case "wrong URL":
				client.items[names[0]].Annotations["openshift.io/ldap.url"] = "another:389"
			case "wrong users":
				client.items[names[0]].Users = nil
			case "unexpected name":
				names = append(names, "not-in-fixture")
			}
			err := scope.compare(context.Background(), expected, names...)
			if (err == nil) != (scenario == "full" || scenario == "pruned") {
				t.Fatalf("%s comparison: %v", scenario, err)
			}
		})
	}
}

func TestLDAPGroupCleanupErrors(t *testing.T) {
	scope, client := ldapScopeForTest(t)
	_, groups := ldapFixtureForTest(t, scope, "ad", "valid_all_ldap_sync.yaml")
	for _, group := range groups {
		client.items[group.Name] = group
	}
	client.deleteErr[groups[0].Name] = fmt.Errorf("denied")
	client.deleteErr[groups[1].Name] = apierrors.NewNotFound(schema.GroupResource{Resource: "groups"}, groups[1].Name)
	err := scope.cleanup(context.Background())
	if err == nil || !strings.Contains(err.Error(), "denied") || len(client.deleted) != 3 {
		t.Fatalf("cleanup did not continue/report errors: %v %v", err, client.deleted)
	}
	if _, exists := client.items[groups[2].Name]; exists {
		t.Fatal("later cleanup did not run")
	}
	client.deleteErr[groups[0].Name] = nil
	if err := scope.cleanup(context.Background()); err != nil {
		t.Fatalf("delete NotFound should be tolerated: %v", err)
	}
	client.listErr = apierrors.NewNotFound(schema.GroupResource{Resource: "groups"}, "")
	if err := scope.cleanup(context.Background()); err == nil {
		t.Fatal("discovery NotFound must not be ignored")
	}
	if err := scope.compare(context.Background(), nil); err == nil {
		t.Fatal("comparison discovery error hidden")
	}
	if err := scope.clearUsers(context.Background()); err == nil {
		t.Fatal("reset discovery error hidden")
	}
}

func TestLDAPGroupCleanupOwnershipRaces(t *testing.T) {
	for _, scenario := range []string{"host collision", "endpoint collision", "replacement"} {
		t.Run(scenario, func(t *testing.T) {
			scope, client := ldapScopeForTest(t)
			_, groups := ldapFixtureForTest(t, scope, "ad", "valid_all_ldap_sync.yaml")
			for _, group := range groups {
				client.items[group.Name] = group
			}
			unrelated := &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "unrelated", UID: "unrelated"}, Users: userv1.OptionalNames{"keep"}}
			switch scenario {
			case "host collision":
				unrelated.Labels = map[string]string{"openshift.io/ldap.host": scope.host}
				unrelated.Annotations = map[string]string{"openshift.io/ldap.url": scope.address()}
			case "endpoint collision":
				unrelated.Annotations = map[string]string{"openshift.io/ldap.url": scope.address()}
			case "replacement":
				unrelated.Name = groups[0].Name
				client.beforeDelete = func(name string) {
					if name == unrelated.Name {
						client.items[name] = unrelated.DeepCopy()
					}
				}
			}
			if scenario != "replacement" {
				client.items[unrelated.Name] = unrelated.DeepCopy()
				if err := scope.clearUsers(context.Background()); err == nil {
					t.Fatal("reset accepted ambiguous ownership")
				}
			}
			if err := scope.cleanup(context.Background()); err == nil {
				t.Fatal("cleanup hid ownership conflict")
			}
			if len(client.items) != 1 || !reflect.DeepEqual(client.items[unrelated.Name], unrelated) {
				t.Fatalf("unrelated group changed or remaining owned groups not cleaned: %#v", client.items)
			}
			client.beforeDelete = nil
			if err := scope.cleanup(context.Background()); err == nil {
				t.Fatal("repeated cleanup hid ownership conflict")
			}
			if len(client.items) != 1 || !reflect.DeepEqual(client.items[unrelated.Name], unrelated) {
				t.Fatal("repeated cleanup adopted an unrelated replacement")
			}
		})
	}
}

func TestLDAPGroupOwnershipPreflight(t *testing.T) {
	for _, scenario := range []string{"name", "host", "url", "list error"} {
		t.Run(scenario, func(t *testing.T) {
			scope, client := ldapScopeForTest(t)
			group := &userv1.Group{ObjectMeta: metav1.ObjectMeta{Name: "unrelated"}}
			switch scenario {
			case "name":
				group.Name = scope.rewrite("firstgroup")
			case "host":
				group.Labels = map[string]string{"openshift.io/ldap.host": scope.host}
			case "url":
				group.Annotations = map[string]string{"openshift.io/ldap.url": scope.host + ":30389"}
			case "list error":
				client.listErr = fmt.Errorf("denied")
			}
			client.items[group.Name] = group
			if scope, err := newLDAPGroupScope(context.Background(), client, "test-run", "127.19.20.21"); err == nil || scope != nil {
				t.Fatal("preflight accepted a collision/error")
			}
			if len(client.updated) != 0 || len(client.deleted) != 0 {
				t.Fatal("preflight mutated groups")
			}
		})
	}
}

func TestLDAPGroupFixtureCompatibility(t *testing.T) {
	scope, client := ldapScopeForTest(t)
	var config, scripts corev1.ConfigMap
	if err := yaml.Unmarshal(testdata.MustAsset("test/extended/testdata/ldap/ldapserver-config-cm.yaml"), &config); err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(testdata.MustAsset("test/extended/testdata/ldap/ldapserver-scripts-cm.yaml"), &scripts); err != nil {
		t.Fatal(err)
	}
	ldif := scope.rewrite(config.Data["groupsync.ldif"])
	if !strings.Contains(scripts.Data["run-openldap.sh"], "ldapadd -x -D cn=Manager,dc=example,dc=com -w admin -f /usr/local/etc/ldapconf/groupsync.ldif") {
		t.Fatal("LDAP startup no longer loads the personalized LDIF")
	}
	for _, schema := range []string{"rfc2307", "ad", "augmented-ad"} {
		directory := "test/extended/testdata/ldap/groupsync/" + schema
		files, err := testdata.AssetDir(directory)
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			original := string(testdata.MustAsset(directory + "/" + file))
			transformed := scope.rewrite(original)
			if strings.ReplaceAll(transformed, scope.marker, "") != original {
				t.Fatalf("fixture content other than group identifiers changed: %s/%s", schema, file)
			}
			if strings.HasPrefix(file, "valid_") {
				expected, groups := ldapFixtureForTest(t, scope, schema, file)
				client.items = map[string]*userv1.Group{}
				for _, group := range groups {
					if !scope.names[group.Name] {
						t.Fatalf("unregistered expected group: %s", group.Name)
					}
					client.items[group.Name] = group
				}
				if err := scope.compare(context.Background(), expected); err != nil {
					t.Fatalf("%s/%s: %v", schema, file, err)
				}
			}
			if file == "ldapgroupuids.txt" {
				for _, uid := range strings.Split(transformed, "\n") {
					if !strings.Contains(ldif, uid) {
						t.Fatalf("fixture UID missing from LDAP initialization: %s", uid)
					}
				}
			}
			if strings.HasPrefix(file, "sync-config") {
				var parsed map[string]interface{}
				if err := yaml.Unmarshal([]byte(transformed), &parsed); err != nil {
					t.Fatal(err)
				}
				if mappings, exists := parsed["groupUIDNameMapping"].(map[string]interface{}); exists {
					for uid, name := range mappings {
						if !strings.Contains(ldif, uid) || !scope.names[name.(string)] {
							t.Fatalf("mapping disconnected from fixtures: %s=%v", uid, name)
						}
					}
				}
			}
		}
	}
}
