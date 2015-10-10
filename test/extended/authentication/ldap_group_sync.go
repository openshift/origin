package authentication

import (
	"fmt"
	"os"
	"reflect"

	g "github.com/onsi/ginkgo"
	o "github.com/onsi/gomega"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/experimental/syncgroups"
	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	userapi "github.com/openshift/origin/pkg/user/api"
	exutil "github.com/openshift/origin/test/extended/util"
)

var _ = g.Describe("authentication: OpenLDAP build and deployment", func() {
	defer g.GinkgoRecover()
	var (
		oc = exutil.NewCLI("openldap", exutil.KubeConfigPath())
	)

	g.Describe("Building and deploying an OpenLDAP server", func() {
		g.It("sync first-class groups, first-class users, membership on groups", func() {
			ldapIP := os.Getenv("LDAP_IP")

			var testCases = []struct {
				name       string
				options    syncgroups.SyncGroupsOptions
				expected   []string
				seedGroups []userapi.Group //allows for groups to exist prior to the sync
				preSync    bool            //determines whether a sync should be performed before the sync to be tested
			}{
				{
					name: "schema 1 all ldap",
					options: syncgroups.SyncGroupsOptions{
						Source: syncgroups.GroupSyncSourceLDAP,
					},
					expected:   []string{GroupName1, GroupName2, GroupName3},
					seedGroups: []userapi.Group{},
					preSync:    false,
				},
				{
					name: "schema 1 whitelist LDAP",
					options: syncgroups.SyncGroupsOptions{
						Source:            syncgroups.GroupSyncSourceLDAP,
						WhitelistContents: []string{GroupName1, GroupName2},
					},
					expected:   []string{GroupName1, GroupName2},
					seedGroups: []userapi.Group{},
					preSync:    false,
				},
				{
					name: "schema 1 all openshift no previous sync",
					options: syncgroups.SyncGroupsOptions{
						Source: syncgroups.GroupSyncSourceOpenShift,
					},
					expected:   []string{}, // cant sync OpenShift groups that haven't been linked to an LDAP entry
					seedGroups: []userapi.Group{},
					preSync:    false,
				},
				{
					name: "schema 1 all openshift with previous sync",
					options: syncgroups.SyncGroupsOptions{
						Source: syncgroups.GroupSyncSourceOpenShift,
					},
					expected:   []string{GroupName1, GroupName2, GroupName3},
					seedGroups: []userapi.Group{},
					preSync:    true,
				},
				{
					name: "schema 1 whitelist openshift no previous sync",
					options: syncgroups.SyncGroupsOptions{
						Source:            syncgroups.GroupSyncSourceOpenShift,
						WhitelistContents: []string{GroupName1, GroupName2},
					},
					expected:   []string{}, // cant sync OpenShift groups that haven't been linked to an LDAP entry
					seedGroups: []userapi.Group{},
					preSync:    false,
				},
				{
					name: "schema 1 whitelist openshift with previous sync",
					options: syncgroups.SyncGroupsOptions{
						Source:            syncgroups.GroupSyncSourceOpenShift,
						WhitelistContents: []string{GroupName1, GroupName2},
					},
					expected:   []string{GroupName1, GroupName2},
					seedGroups: []userapi.Group{},
					preSync:    true,
				},
				// TODO: seed a group that shares name but has not been synced, check for Existing correctness
			}

			for _, testCase := range testCases {
				g.By(fmt.Sprintf("Running test case: %s", testCase.name))
				// determine LDAP server host:port
				host := ldapIP + ":389"

				// determine expected groups
				expectedGroups := makeGroups(host, testCase.expected)

				// populate config with test-case data
				testCase.options.Config = makeConfig(host)
				testCase.options.GroupInterface = oc.AdminREST().Groups()
				testCase.options.Stderr = os.Stderr
				testCase.options.Out = os.Stdout

				// Check that we are in the correct starting state
				g.By("Checking that the test case starts in the correct state")
				groupList, err := oc.AdminREST().Groups().List(labels.Everything(), fields.Everything())
				o.Expect(err).NotTo(o.HaveOccurred())

				var stateErr error
				if len(groupList.Items) != 0 {
					stateErr = fmt.Errorf("test %s beginning in incorrect state: should have no groups, had: %d, (%v)",
						testCase.name, len(groupList.Items), groupList.Items)
				}
				o.Expect(stateErr).NotTo(o.HaveOccurred())

				// Add groups if necessary
				g.By("Adding seed groups as necessary")
				for _, groupToAdd := range testCase.seedGroups {
					_, err = oc.AdminREST().Groups().Create(&groupToAdd)
					o.Expect(err).NotTo(o.HaveOccurred())
				}

				// Preform "pre-sync" if required - this allows for OpenShift - sourced sync jobs to work
				// the OpenShift - sourced GroupListers look for the LDAPURLAnnotation annotation as well as the LDAPUIDAnnotation annotation
				g.By("Performing the pre-sync")
				if testCase.preSync {
					for _, group := range expectedGroups {
						bareGroup := createBareGroup(group)
						_, err = oc.AdminREST().Groups().Create(&bareGroup)
						o.Expect(err).NotTo(o.HaveOccurred())
					}
				}

				// Perform sync job
				g.By("Performing the sync job")
				errs := testCase.options.Run(nil, nil)
				o.Expect(errs).NotTo(o.HaveOccurred())

				// Check that the results are what we expected
				g.By("Validating results")
				newGroupList, err := oc.AdminREST().Groups().List(labels.Everything(), fields.Everything())
				o.Expect(err).NotTo(o.HaveOccurred())

				ok, err := checkSetEquality(newGroupList.Items, expectedGroups)
				if err != nil || !ok {
					stateErr = fmt.Errorf("group sync ended in incorrect state after test %s: %v", testCase.name, err)
				}
				o.Expect(stateErr).NotTo(o.HaveOccurred())

				// Clean up OpenShift etcd Group records
				cleanup(oc.AdminREST())
			}
		})
	})
})

const (
	LDAPScopeWholeSubtree string = "sub"
	LDAPNeverDerefAliases string = "never"
	LDAPQueryTimeout      int    = 10

	BaseDN      string = "dc=example,dc=com"
	GroupBaseDN string = "ou=groups," + BaseDN
	UserBaseDN  string = "ou=people," + BaseDN

	GroupFilter         string = "objectClass=groupOfNames"
	GroupQueryAttribute string = "cn"
	UserFilter          string = "objectClass=inetOrgPerson"
	UserQueryAttribute  string = "cn"

	GroupMembershipAttribute string = "member"

	GroupNameAttribute1 string = "missing"
	GroupNameAttribute2 string = "cn"

	UserNameAttribute1 string = "missing"
	UserNameAttribute2 string = "name"
	UserNameAttribute3 string = "cn"

	GroupName1 string = "group1"
	GroupName2 string = "group2"
	GroupName3 string = "group3"

	UserName1 string = "Person1"
	UserName2 string = "Person2"
	UserName3 string = "Person3"
	UserName4 string = "Person4"
	UserName5 string = "Person5"
)

// makeGroups injects the run-dependent host into the expected group records and returns those
// specified by the which string array
func makeGroups(host string, which []string) []userapi.Group {
	GroupRecord1 := userapi.Group{
		ObjectMeta: kapi.ObjectMeta{
			Name:      GroupName1,
			Namespace: "",
			Annotations: map[string]string{
				ldaputil.LDAPURLAnnotation: host,
				ldaputil.LDAPUIDAnnotation: GroupName1,
			},
		},
		Users: []string{
			UserName1,
			UserName2,
			UserName3,
			UserName4,
			UserName5,
		},
	}

	GroupRecord2 := userapi.Group{
		ObjectMeta: kapi.ObjectMeta{
			Name:      GroupName2,
			Namespace: "",
			Annotations: map[string]string{
				ldaputil.LDAPURLAnnotation: host,
				ldaputil.LDAPUIDAnnotation: GroupName2,
			},
		},
		Users: []string{
			UserName1,
			UserName2,
			UserName3,
		},
	}

	GroupRecord3 := userapi.Group{
		ObjectMeta: kapi.ObjectMeta{
			Name:      GroupName3,
			Namespace: "",
			Annotations: map[string]string{
				ldaputil.LDAPURLAnnotation: host,
				ldaputil.LDAPUIDAnnotation: GroupName3,
			},
		},
		Users: []string{
			UserName1,
			UserName5,
		},
	}

	expectedGroups := []userapi.Group{}
	for _, expectedGroup := range which {
		switch expectedGroup {
		case GroupName3:
			expectedGroups = append(expectedGroups, GroupRecord3)
		case GroupName2:
			expectedGroups = append(expectedGroups, GroupRecord2)
		case GroupName1:
			expectedGroups = append(expectedGroups, GroupRecord1)
		}
	}

	return expectedGroups
}

func makeConfig(host string) configapi.LDAPSyncConfig {
	// hard-coded config until config-file parsing is hashed out
	return configapi.LDAPSyncConfig{
		URL:          "ldap://" + host + "/",
		BindDN:       "",
		BindPassword: "",
		Insecure:     true,
		CA:           "",

		LDAPGroupUIDToOpenShiftGroupNameMapping: make(map[string]string),

		RFC2307Config: &configapi.RFC2307Config{
			AllGroupsQuery: configapi.LDAPQuery{
				BaseDN:       GroupBaseDN,
				Scope:        LDAPScopeWholeSubtree,
				DerefAliases: LDAPNeverDerefAliases,
				TimeLimit:    LDAPQueryTimeout,
				Filter:       GroupFilter,
			},
			GroupUIDAttribute:         GroupQueryAttribute,
			GroupNameAttributes:       []string{GroupNameAttribute1, GroupNameAttribute2},
			GroupMembershipAttributes: []string{GroupMembershipAttribute},
			AllUsersQuery: configapi.LDAPQuery{
				BaseDN:       UserBaseDN,
				Scope:        LDAPScopeWholeSubtree,
				DerefAliases: LDAPNeverDerefAliases,
				TimeLimit:    LDAPQueryTimeout,
				Filter:       UserFilter,
			},
			UserUIDAttribute:   UserQueryAttribute,
			UserNameAttributes: []string{UserNameAttribute1, UserNameAttribute2, UserNameAttribute3},
		},
	}
}

// createBareGroup will create a new Group with only the data necessary for it to be accepted as having been previously
// synced from LDAP to allow us to add it to etcd and simulate a previous sync job
func createBareGroup(in userapi.Group) userapi.Group {
	return userapi.Group{
		ObjectMeta: kapi.ObjectMeta{
			Name:      in.Name,
			Namespace: in.Namespace,
			Annotations: map[string]string{
				ldaputil.LDAPUIDAnnotation: in.Annotations[ldaputil.LDAPUIDAnnotation],
				ldaputil.LDAPURLAnnotation: in.Annotations[ldaputil.LDAPURLAnnotation],
			},
		},
	}
}

// checkSetEquality treats the incoming slices as sets and returns true if the sets are equal
func checkSetEquality(have, want []userapi.Group) (bool, error) {
	// remove sync timestamp because it is not predictable and will cause DeepEqual to fail
	for _, obj := range have {
		_, ok := obj.Annotations[ldaputil.LDAPSyncTimeAnnotation]
		if !ok {
			return false, fmt.Errorf("synced group expected to have a %s annotation, but didn't",
				ldaputil.LDAPSyncTimeAnnotation)
		}
		delete(obj.Annotations, ldaputil.LDAPSyncTimeAnnotation)
	}

	if len(have) != len(want) {
		return false, fmt.Errorf("expected %v groups, got %v: wanted\n\t%#v\ngot\n\t%#v", len(want), len(have), want, have)
	}

	// if what we want and what we have are the same size and size 0, we're done
	if len(want) == 0 {
		return true, nil
	}

	// check that all entries in have exist in want
	for _, haveObj := range have {
		wantWhatWeHave := false
		for _, wantObj := range want {
			if reflect.DeepEqual(haveObj, wantObj) {
				wantWhatWeHave = true
			}
		}
		if !wantWhatWeHave {
			return false, fmt.Errorf("did not expect group record from sync job: %v", haveObj)
		}
	}
	return true, nil
}

// cleanup removes all Group records from the OpenShift cluster to ready it for the next test
func cleanup(client *client.Client) error {
	groupList, err := client.Groups().List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	for _, group := range groupList.Items {
		err = client.Groups().Delete(group.Name)
		if err != nil {
			return err
		}
	}

	return nil
}
