package oauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	userv1 "github.com/openshift/api/user/v1"
	userclient "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
	"github.com/openshift/origin/test/extended/testdata"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	kdiff "k8s.io/apimachinery/pkg/util/diff"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"sigs.k8s.io/yaml"
)

const ldapGroupHeader = "apiVersion: user.openshift.io/v1\nkind: Group\n"
const ldapGroupPort = "30389"

type ldapGroupScope struct {
	client      userclient.GroupInterface
	host        string
	marker      string
	names       map[string]bool
	cleanupUIDs map[string]types.UID
	replacer    *strings.Replacer
}

func newLDAPGroupScope(ctx context.Context, client userclient.GroupInterface, token, host string) (*ldapGroupScope, error) {
	if token == "" || host == "" || host == "127.0.0.1" {
		return nil, fmt.Errorf("LDAP group isolation requires a unique token and host")
	}
	scope := &ldapGroupScope{client: client, host: host, marker: "-" + token, names: map[string]bool{}, cleanupUIDs: map[string]types.UID{}}
	var replacements []string
	for _, name := range []string{"group1", "group2", "group3", "firstgroup", "secondgroup", "thirdgroup"} {
		replacements = append(replacements, name, name+scope.marker)
	}
	scope.replacer = strings.NewReplacer(replacements...)
	for _, schema := range []string{"rfc2307", "ad", "augmented-ad"} {
		directory := "test/extended/testdata/ldap/groupsync/" + schema
		files, err := testdata.AssetDir(directory)
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			if !strings.HasPrefix(file, "valid_") {
				continue
			}
			content := scope.rewrite(string(testdata.MustAsset(directory + "/" + file)))
			for _, document := range strings.Split(content, ldapGroupHeader)[1:] {
				var group userv1.Group
				if err := yaml.Unmarshal([]byte(ldapGroupHeader+document), &group); err != nil {
					return nil, err
				}
				if !strings.Contains(group.Name, scope.marker) {
					return nil, fmt.Errorf("fixture group %q has no run marker", group.Name)
				}
				scope.names[group.Name] = true
			}
		}
	}
	groups, err := scope.groups(ctx)
	if err != nil {
		return nil, fmt.Errorf("checking LDAP group isolation: %w", err)
	}
	if len(groups) != 0 {
		return nil, fmt.Errorf("LDAP group ownership collides with existing group %q", groups[0].Name)
	}
	return scope, nil
}

func (scope *ldapGroupScope) rewrite(content string) string {
	return scope.replacer.Replace(content)
}

func (scope *ldapGroupScope) address() string {
	return scope.host + ":" + ldapGroupPort
}

func (scope *ldapGroupScope) owns(group userv1.Group) bool {
	return scope.names[group.Name] || strings.Contains(group.Name, scope.marker) ||
		group.Labels["openshift.io/ldap.host"] == scope.host ||
		group.Annotations["openshift.io/ldap.url"] == scope.address()
}

func (scope *ldapGroupScope) groups(ctx context.Context) ([]userv1.Group, error) {
	groups, err := scope.client.List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	var owned []userv1.Group
	for _, group := range groups.Items {
		if scope.owns(group) {
			owned = append(owned, group)
		}
	}
	sort.Slice(owned, func(first, second int) bool { return owned[first].Name < owned[second].Name })
	return owned, nil
}

func (scope *ldapGroupScope) cleanup(ctx context.Context) error {
	groups, err := scope.groups(ctx)
	if err != nil {
		return fmt.Errorf("discovering LDAP cleanup targets: %w", err)
	}
	var errs []error
	for _, group := range groups {
		if !strings.Contains(group.Name, scope.marker) {
			errs = append(errs, fmt.Errorf("refusing to delete LDAP group %q without run marker", group.Name))
			continue
		}
		uid := group.UID
		if previous, exists := scope.cleanupUIDs[group.Name]; exists && previous != uid {
			errs = append(errs, fmt.Errorf("refusing to delete replacement LDAP group %q", group.Name))
			continue
		}
		scope.cleanupUIDs[group.Name] = uid
		if err := scope.client.Delete(ctx, group.Name, metav1.DeleteOptions{Preconditions: &metav1.Preconditions{UID: &uid}}); err != nil && !apierrors.IsNotFound(err) {
			errs = append(errs, fmt.Errorf("deleting LDAP group %q: %w", group.Name, err))
		} else {
			delete(scope.cleanupUIDs, group.Name)
		}
	}
	return utilerrors.NewAggregate(errs)
}

func (scope *ldapGroupScope) clearUsers(ctx context.Context, names ...string) error {
	groups, err := scope.groups(ctx)
	if err != nil {
		return err
	}
	if len(names) != 0 {
		groups = nil
		for _, name := range names {
			if !scope.names[name] {
				return fmt.Errorf("refusing to reset unowned group %q", name)
			}
			group, err := scope.client.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}
			groups = append(groups, *group)
		}
	}
	for _, group := range groups {
		if !strings.Contains(group.Name, scope.marker) || group.Labels["openshift.io/ldap.host"] != scope.host || group.Annotations["openshift.io/ldap.url"] != scope.address() {
			return fmt.Errorf("incorrect LDAP ownership metadata on %q", group.Name)
		}
		group.Users = nil
		if _, err := scope.client.Update(ctx, &group, metav1.UpdateOptions{}); err != nil {
			return err
		}
	}
	return nil
}

func (scope *ldapGroupScope) compare(ctx context.Context, expected []byte, names ...string) error {
	if len(names) != 0 {
		selected := map[string]bool{}
		for _, name := range names {
			selected[name] = true
		}
		var subset []byte
		for _, document := range strings.Split(string(expected), ldapGroupHeader)[1:] {
			var group userv1.Group
			if err := yaml.Unmarshal([]byte(ldapGroupHeader+document), &group); err != nil {
				return err
			}
			if selected[group.Name] {
				subset = append(subset, (ldapGroupHeader + document)...)
				delete(selected, group.Name)
			}
		}
		if len(selected) != 0 {
			return fmt.Errorf("expected group names missing from LDAP fixture: %v", selected)
		}
		expected = subset
	}
	groups, err := scope.groups(ctx)
	if err != nil {
		return err
	}
	var actual []byte
	for _, group := range groups {
		normalizeGroupMetadata(&group)
		data, err := json.Marshal(&group)
		if err != nil {
			return err
		}
		data, err = yaml.JSONToYAML(data)
		if err != nil {
			return err
		}
		actual = append(actual, ldapGroupHeader...)
		actual = append(actual, data...)
	}
	if !bytes.Equal(expected, actual) {
		return fmt.Errorf("LDAP group comparison: %s", kdiff.Diff(string(expected), string(actual)))
	}
	return nil
}
