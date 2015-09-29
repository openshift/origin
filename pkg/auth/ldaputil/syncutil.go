package ldaputil

// These constants contain values for annotations and labels affixed to Groups by the LDAP sync job
const (
	// LDAPURLAnnotation is the Annotation value that stores the host:port of the LDAP server
	LDAPURLAnnotation string = "openshift.io/ldap.url"
	// LDAPUIDAnnotation is the Annotation value that stores the corresponding LDAP group UID for the Group
	LDAPUIDAnnotation string = "openshift.io/ldap.uid"
	// LDAPSyncTime is the Annotation value that stores the last time this Group was synced with LDAP
	LDAPSyncTimeAnnotation string = "openshift.io/ldap.sync-time"
)
