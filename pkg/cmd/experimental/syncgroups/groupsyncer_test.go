package syncgroups

import (
	"fmt"
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/go-ldap/ldap"
	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups/interfaces"
	userapi "github.com/openshift/origin/pkg/user/api"
)

const (
	Group1UID string = "group1"
	Group2UID string = "group2"
	Group3UID string = "group3"

	UserNameAttribute string = "cn"

	Member1UID string = "member1"
	Member2UID string = "member2"
	Member3UID string = "member3"
	Member4UID string = "member4"

	BaseDN string = "dc=example,dc=com"
)

var Member1 *ldap.Entry = &ldap.Entry{
	DN: UserNameAttribute + "=" + Member1UID + "," + BaseDN,
	Attributes: []*ldap.EntryAttribute{
		{
			Name:       UserNameAttribute,
			Values:     []string{Member1UID},
			ByteValues: [][]byte{[]byte(Member1UID)},
		},
	},
}
var Member2 *ldap.Entry = &ldap.Entry{
	DN: UserNameAttribute + "=" + Member2UID + "," + BaseDN,
	Attributes: []*ldap.EntryAttribute{
		{
			Name:       UserNameAttribute,
			Values:     []string{Member2UID},
			ByteValues: [][]byte{[]byte(Member2UID)},
		},
	},
}
var Member3 *ldap.Entry = &ldap.Entry{
	DN: UserNameAttribute + "=" + Member3UID + "," + BaseDN,
	Attributes: []*ldap.EntryAttribute{
		{
			Name:       UserNameAttribute,
			Values:     []string{Member3UID},
			ByteValues: [][]byte{[]byte(Member3UID)},
		},
	},
}
var Member4 *ldap.Entry = &ldap.Entry{
	DN: UserNameAttribute + "=" + Member4UID + "," + BaseDN,
	Attributes: []*ldap.EntryAttribute{
		{
			Name:       UserNameAttribute,
			Values:     []string{Member4UID},
			ByteValues: [][]byte{[]byte(Member4UID)},
		},
	},
}

var Group1Members []*ldap.Entry = []*ldap.Entry{Member1, Member2}
var Group2Members []*ldap.Entry = []*ldap.Entry{Member2, Member3}
var Group3Members []*ldap.Entry = []*ldap.Entry{Member3, Member4}

// TestSync ensures that data is exchanged and rearranged correctly during the sync process.
func TestSync(t *testing.T) {
	testGroupLister := TestGroupLister{
		GroupUIDs: []string{Group1UID, Group2UID, Group3UID},
	}
	testGroupMemberExtractor := TestGroupMemberExtractor{
		MemberMapping: map[string][]*ldap.Entry{
			Group1UID: Group1Members,
			Group2UID: Group2Members,
			Group3UID: Group3Members,
		},
	}
	testUserNameMapper := TestUserNameMapper{
		NameAttributes: []string{UserNameAttribute},
	}
	testGroupNameMapper := TestGroupNameMapper{
		NameMapping: map[string]string{
			Group1UID: "os" + Group1UID,
			Group2UID: "os" + Group2UID,
			Group3UID: "os" + Group3UID,
		},
	}
	testGroupClient := TestGroupClient{
		Storage: make(map[string]*userapi.Group),
	}
	testHost := "test.host:port"

	testGroupSyncer := LDAPGroupSyncer{
		GroupLister:          &testGroupLister,
		GroupMemberExtractor: &testGroupMemberExtractor,
		UserNameMapper:       &testUserNameMapper,
		GroupNameMapper:      &testGroupNameMapper,
		GroupClient:          &testGroupClient,
		Host:                 testHost,
		SyncExisting:         false,
	}

	errs := testGroupSyncer.Sync()
	for _, err := range errs {
		t.Errorf("unexpected sync error: %v", err)
	}

	expectedGroups := []*userapi.Group{
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "os" + Group1UID,
				Annotations: map[string]string{
					ldaputil.LDAPURLAnnotation: testHost,
					ldaputil.LDAPUIDAnnotation: Group1UID,
				},
			},
			Users: []string{Member1UID, Member2UID},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "os" + Group2UID,
				Annotations: map[string]string{
					ldaputil.LDAPURLAnnotation: testHost,
					ldaputil.LDAPUIDAnnotation: Group2UID,
				},
			},
			Users: []string{Member2UID, Member3UID},
		},
		{
			ObjectMeta: kapi.ObjectMeta{
				Name: "os" + Group3UID,
				Annotations: map[string]string{
					ldaputil.LDAPURLAnnotation: testHost,
					ldaputil.LDAPUIDAnnotation: Group3UID,
				},
			},
			Users: []string{Member3UID, Member4UID},
		},
	}

	for _, expectedGroup := range expectedGroups {
		group, err := (&testGroupClient).Get(expectedGroup.Name)
		if err != nil {
			t.Errorf("group did not exist after sync job:\n\texpected:\n%#v\n\t", expectedGroup)
		} else {
			if _, exists := group.Annotations[ldaputil.LDAPSyncTimeAnnotation]; !exists {
				t.Errorf("sycned group did not have %s annotation: %#v", ldaputil.LDAPSyncTimeAnnotation, group)
			}
			delete(group.Annotations, ldaputil.LDAPSyncTimeAnnotation)
			if !reflect.DeepEqual(expectedGroup, group) {
				t.Errorf("group was not synced correctly:\n\texpected:\n%#v\n\tgot:\n%#v", expectedGroup, group)
			}
		}
	}
}

// The following stub implementations allow us to build a test LDAPGroupSyncer

var _ interfaces.LDAPGroupLister = &TestGroupLister{}
var _ interfaces.LDAPMemberExtractor = &TestGroupMemberExtractor{}
var _ interfaces.LDAPUserNameMapper = &TestUserNameMapper{}
var _ interfaces.LDAPGroupNameMapper = &TestGroupNameMapper{}
var _ client.GroupInterface = &TestGroupClient{}

type TestGroupLister struct {
	GroupUIDs []string
}

func (l *TestGroupLister) ListGroups() ([]string, error) {
	return l.GroupUIDs, nil
}

type TestGroupMemberExtractor struct {
	MemberMapping map[string][]*ldap.Entry
}

func (e *TestGroupMemberExtractor) ExtractMembers(ldapGroupUID string) ([]*ldap.Entry, error) {
	members, exist := e.MemberMapping[ldapGroupUID]
	if !exist {
		return nil, fmt.Errorf("no members found for group: %s", ldapGroupUID)
	}
	return members, nil
}

type TestUserNameMapper struct {
	NameAttributes []string
}

func (m *TestUserNameMapper) UserNameFor(user *ldap.Entry) (string, error) {
	openShiftUserName := ldaputil.GetAttributeValue(user, m.NameAttributes)
	if len(openShiftUserName) == 0 {
		return "", fmt.Errorf("the user entry (%v) does not map to a OpenShift User name with the given mapping",
			user)
	}
	return openShiftUserName, nil
}

type TestGroupNameMapper struct {
	NameMapping map[string]string
}

func (m *TestGroupNameMapper) GroupNameFor(ldapGroupUID string) (string, error) {
	name, exists := m.NameMapping[ldapGroupUID]
	if !exists {
		return "", fmt.Errorf("no name found for group: %s", ldapGroupUID)
	}
	return name, nil
}

type TestGroupClient struct {
	Storage map[string]*userapi.Group
}

func (c *TestGroupClient) Update(group *userapi.Group) (*userapi.Group, error) {
	if _, exists := c.Storage[group.Name]; !exists {
		return nil, fmt.Errorf("cannot update group that does not exist: %v", group)
	}
	c.Storage[group.Name] = group
	return group, nil
}

func (c *TestGroupClient) Get(name string) (*userapi.Group, error) {
	group, exists := c.Storage[name]
	if !exists {
		return nil, kapierrors.NewNotFound("Group", name)
	}
	return group, nil
}

func (c *TestGroupClient) Create(group *userapi.Group) (*userapi.Group, error) {
	c.Storage[group.Name] = group
	return group, nil
}

// The following functions are not used during a sync and therefore have no implementation
func (c *TestGroupClient) List(_ labels.Selector, _ fields.Selector) (*userapi.GroupList, error) {
	return nil, nil
}

func (c *TestGroupClient) Delete(_ string) error {
	return nil
}
