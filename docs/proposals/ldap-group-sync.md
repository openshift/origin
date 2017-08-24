# Syncing LDAP Groups to OpenShift Groups

OpenShift cluster admins need a way to sync their OpenShift `Groups` to external records in an LDAP server. This requires that we have a way of retrieving the LDAP users that are the members of a group, determining the OpenShift `Group` name for this collection, updating the OpenShift record and optionally removing the record when the corresponding LDAP record is removed.

## Use Cases
1. I want to sync all LDAP groups minus some blacklist to OpenShift `Groups`
* I want to sync a specific set of LDAP groups to OpenShift `Groups`
* I want to sync all OpenShift groups belonging to a sync job minus some blacklist with the LDAP server
* I want to sync specific OpenShift `Groups` with the LDAP server records
* I want to remove OpenShift `Groups` that were previously backed by LDAP records that no longer exist
* I want to specify an explicit mapping from LDAP group identifiers to OpenShift `Group` resources

## Supported LDAP Group Schemas
1. Groups as first-class LDAP entries with an attribute containing a list of members:
  1. members listed with their DN in the attribute
  * members listed with some other UID in the attribute
* Users as first-class LDAP entries with an attribute containing a list of groups they are a member of:
  1. groups listed by a unique name, with no additional group metadata in the LDAP server
  * groups listed by DN, with the first-class group entries holding additional metadata
  * groups listed by some other UID, with the first-class group entries holding additional metadata

## Changes To OpenShift `Group` Metadata
OpenShift `Group`'s `Annotations` will be used to store metadata regarding the sync process. `Annotations` will store:

1. LDAP unique identifier (`openshift.io/ldap.uid`)
* last sync time (`openshift.io/ldap.sync-time`)

OpenShift `Group`'s `Labels` will also be used to store metadata regarding the sync process. `Labels` will store:

1. LDAP server URL (`openshift.io/external-record-provider-url`).

## Data Flow
1. Determine ordered list of LDAP groups to be sync
  1. For each group to sync:
  * Collect LDAP group members and group metadata
  * Determine OpenShift `Identities` from LDAP group member entries
  * Determine OpenShift `Users` from OpenShift `Identities`
  * Determine OpenShift `Group` from LDAP group metadata or explicit mapping
  * Populate new OpenShift `Group`'s `Users` from resulting OpenShift `UserIdentityMappings`, update OpenShift `Group` metadata fields tied to LDAP attributes, leave other OpenShift `Group` fields unchanged

##  Determining Ordered List of LDAP Groups To Sync
An `LDAPGroupLister` determines what LDAP groups needs to be synced and outputs the result as a set of unique identifier strings (called "LDAP group UIDs" in this document). The other objects that take LDAP group UIDs need to understand the format of this string (e.g. these objects will be tightly coupled). For example, an `LDAPGroupLister` for schema 1 above cannot be used with a `LDAPGroupMemberExtractor` for schema 2. The four `LDAPGroupLister` implementations that are necessary are:

1. List LDAP group UIDs for all OpenShift `Groups` matching a `Label` selector identifying them as pertaining to the sync job, minus a blacklist
* List LDAP group UIDs for some whitelist of OpenShift `Groups`
* List LDAP group UIDs for all LDAP groups, minus a blacklist
* List LDAP group UIDs for some whitelist of LDAP groups

```go
// LDAPGroupLister lists the LDAP groups that need to be synced by a job. The LDAPGroupLister needs to
// be paired with an LDAPGroupMemberExtractor that understands the format of the unique identifiers
// returned to represent the LDAP groups to be synced.
type LDAPGroupLister interface {
	ListGroups() (groupUIDs []string, err error)
}
```

## Collecting LDAP Members and Metadata
An `LDAPGroupMemberExtractor` gathers information on an LDAP group based on an LDAP group UID. It may cache LDAP responses for responsivity. The approach to implementing this structure will vary with LDAP schema as well as sync job request.

```go
// LDAPGroupMemberExtractor retrieves data about an LDAP group from the LDAP server.
type LDAPGroupMemberExtractor interface {
	// ExtractMembers returns the list of LDAP first-class user entries that are members of the LDAP
	// group specified by the groupUID
	ExtractMembers(groupUID string) (members []*ldap.Entry, err error)
}
```

## Determining OpenShift `User` Names for LDAP Members
The mapping of a LDAP member entry to an OpenShift `User` Name will be deterministic and simple: whatever LDAP entry attribute is used for the OpenShift `User` Name field upon creation of OpenShift `Users` will be used as the OpenShift `User` Name. As long as the `DeterministicUserIdentityMapper` is used to introduce LDAP member entries to OpenShift `User` records and the `LDAPUserAttributeDefiner` used for the sync job and `DeterministicUserIdentityMapper` is the same, the mappings created by the `LDAPUserNameMapper` will be correct.

```go
// LDAPUserNameMapper maps an LDAP entry representing a user to the OpenShift User Name corresponding to it
type LDAPUserNameMapper interface {
	UserNameFor(ldapUser *ldap.Entry) (openShiftUserName string, err error)
}
```

## Determining OpenShift `Users` for OpenShift `Identities`
A new `UserIdentityMapper` will need to be created: the `DeterministicUserIdentityMapper`, which will assume that the unique identifier retrieved from the LDAP response and integrated into the OpenShift `Identity` will be deterministicically mapped to an OpenShift `User`. This is required as otherwise there is no deterministic mapping from the `Identity` created and any OpenShift `User`. This will not be used in the sync job itself but will be necessary for creating OpenShift `Users` when migrating them from an LDAP server.

```go
// DeterministicUserIdentityMapper is a UserIdentityMapper that forces a deterministic mapping from
// an Identity to a User.
type DeterministicUserIdentityMapper struct {
  // AllowIdentityCollisions determines if this UserIdentityMapper is allowed to make a mapping
  // between an Identity provided by LDAP and a User that already is mapped to another Identity.
  AllowIdentityCollisions bool
}

func (m *DeterministicUserIdentityMapper) UserFor(identity authapi.UserIdentityInfo) (user user.Info,
	err error) {
}
```

## Creating OpenShift `Groups`
1. Find the OpenShift `Group` or start one if it does not exist
* Populate `Annotations` and `Labels`
* Overwrite `Users` list
* Replace current OpenShift `Group` or create it

An `LDAPGroupNameMapper` will be used to determine the name of the OpenShift `Group` record that matches a given LDAP entry representing a group. This mapping can be done with a user-defined hard mapping from LDAP group UID to OpenShift `Group` Name or a dynamic user-defined mapping of LDAP group entry attributes to OpenShift `Group` Name.

```go
// LDAPGroupNameMapper maps an LDAP group UID identifying a group entry to the OpenShift Group name for it
type LDAPGroupNameMapper interface {
  GroupNameFor(ldapGroupUID string) (openShiftGroupName string, err error)
}
```

## Exposed Options for `oc adm sync-groups`
The sync command will be exposed as `oc adm sync-groups [<openshift-group-name>...] --sync-config=<location>` and invoked like:
* `oc adm sync-groups --all-openshift --sync-config=/etc/openshift/ldap-sync-config.yaml`
* `oc adm sync-groups <names> --sync-config=/etc/openshift/ldap-sync-config.yaml`
* `oc adm sync-groups --all-ldap --sync-config=/etc/openshift/ldap-sync-config.yaml`
* `oc adm sync-groups --whitelist-ldap --sync-config=/etc/openshift/ldap-sync-config.yaml`
* `oc adm sync-groups --prune --sync-config=/etc/openshift/ldap-sync-config.yaml`

The sync command will default to doing a dry-run.

`--sync-config=/path/to/file` determines where the config `yaml` is located.

`--all-openshift` would specify that all OpenShift `Groups` need to be synced

`--all-ldap` would specify that all LDAP groups need to be synced

`--whitelist-ldap=/path/to/file` would specify that an LDAP whitelist file containing LDAP group UIDs contains all the groups that need to be synced and determines where the file is located

`--prune` removes OpenShift `Group` records linked to an LDAP record that no longer exists

`--confirm` makes the proposed changes

`-o <json,yaml>` Format for the output of a `--dry-run` should be tuneable from human-readable, `json`, or `yaml`. The latter two formats should output in a format useable by `oc replace`
