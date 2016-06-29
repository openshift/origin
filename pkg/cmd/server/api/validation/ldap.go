package validation

import (
	"fmt"
	"strings"

	"gopkg.in/ldap.v2"

	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateLDAPSyncConfig(config *api.LDAPSyncConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateStringSource(config.BindPassword, field.NewPath("bindPassword")))
	bindPassword, _ := api.ResolveStringValue(config.BindPassword)
	validationResults.Append(ValidateLDAPClientConfig(config.URL, config.BindDN, bindPassword, config.CA, config.Insecure, nil))

	schemaConfigsFound := []string{}

	if config.RFC2307Config != nil {
		configResults := ValidateRFC2307Config(config.RFC2307Config)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		schemaConfigsFound = append(schemaConfigsFound, "rfc2307")
	}
	if config.ActiveDirectoryConfig != nil {
		configResults := ValidateActiveDirectoryConfig(config.ActiveDirectoryConfig)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		schemaConfigsFound = append(schemaConfigsFound, "activeDirectory")
	}
	if config.AugmentedActiveDirectoryConfig != nil {
		configResults := ValidateAugmentedActiveDirectoryConfig(config.AugmentedActiveDirectoryConfig)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		schemaConfigsFound = append(schemaConfigsFound, "augmentedActiveDirectory")
	}

	if len(schemaConfigsFound) > 1 {
		validationResults.AddErrors(field.Invalid(field.NewPath("schema"), config, fmt.Sprintf("only one schema-specific config is allowed; found %v", schemaConfigsFound)))
	}
	if len(schemaConfigsFound) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("schema"), fmt.Sprintf("exactly one schema-specific config is required;  one of %v", []string{"rfc2307", "activeDirectory", "augmentedActiveDirectory"})))
	}

	return validationResults
}

func ValidateLDAPClientConfig(url, bindDN, bindPassword, CA string, insecure bool, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	if len(url) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("url"), ""))
		return validationResults
	}

	u, err := ldaputil.ParseURL(url)
	if err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("url"), url, err.Error()))
		return validationResults
	}

	// Make sure bindDN and bindPassword are both set, or both unset
	// Both unset means an anonymous bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.1)
	// Both set means the name/password simple bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.3)
	if (len(bindDN) == 0) != (len(bindPassword) == 0) {
		validationResults.AddErrors(field.Invalid(fldPath.Child("bindDN"), bindDN,
			"bindDN and bindPassword must both be specified, or both be empty"))
		validationResults.AddErrors(field.Invalid(fldPath.Child("bindPassword"), "(masked)",
			"bindDN and bindPassword must both be specified, or both be empty"))
	}

	if insecure {
		if u.Scheme == ldaputil.SchemeLDAPS {
			validationResults.AddErrors(field.Invalid(fldPath.Child("url"), url,
				fmt.Sprintf("Cannot use %s scheme with insecure=true", u.Scheme)))
		}
		if len(CA) > 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("ca"), CA,
				"Cannot specify a ca with insecure=true"))
		}
	} else {
		if len(CA) > 0 {
			validationResults.AddErrors(ValidateFile(CA, fldPath.Child("ca"))...)
		}
	}

	// Warn if insecure
	if insecure {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("insecure"), insecure,
			"validating passwords over an insecure connection could allow them to be intercepted"))
	}

	return validationResults
}

func ValidateRFC2307Config(config *api.RFC2307Config) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllGroupsQuery, field.NewPath("groupsQuery")))
	if len(config.GroupUIDAttribute) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupUIDAttribute"), ""))
	}
	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupNameAttributes"), ""))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupMembershipAttributes"), ""))
	}

	isUserDNQuery := strings.TrimSpace(strings.ToLower(config.UserUIDAttribute)) == "dn"
	validationResults.Append(validateLDAPQuery(config.AllUsersQuery, field.NewPath("usersQuery"), isUserDNQuery))
	if len(config.UserUIDAttribute) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("userUIDAttribute"), ""))
	}
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("userNameAttributes"), ""))
	}

	return validationResults
}

func ValidateActiveDirectoryConfig(config *api.ActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery, field.NewPath("usersQuery")))
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("userNameAttributes"), ""))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupMembershipAttributes"), ""))
	}

	return validationResults
}

func ValidateAugmentedActiveDirectoryConfig(config *api.AugmentedActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery, field.NewPath("usersQuery")))
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("userNameAttributes"), ""))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupMembershipAttributes"), ""))
	}

	isGroupDNQuery := strings.TrimSpace(strings.ToLower(config.GroupUIDAttribute)) == "dn"
	validationResults.Append(validateLDAPQuery(config.AllGroupsQuery, field.NewPath("groupsQuery"), isGroupDNQuery))
	if len(config.GroupUIDAttribute) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupUIDAttribute"), ""))
	}
	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupNameAttributes"), ""))
	}

	return validationResults
}

func ValidateLDAPQuery(query api.LDAPQuery, fldPath *field.Path) ValidationResults {
	return validateLDAPQuery(query, fldPath, false)
}
func validateLDAPQuery(query api.LDAPQuery, fldPath *field.Path, isDNOnly bool) ValidationResults {
	validationResults := ValidationResults{}

	if _, err := ldap.ParseDN(query.BaseDN); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("baseDN"), query.BaseDN,
			fmt.Sprintf("invalid base DN for search: %v", err)))
	}

	if len(query.Scope) > 0 {
		if _, err := ldaputil.DetermineLDAPScope(query.Scope); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("scope"), query.Scope,
				"invalid LDAP search scope"))
		}
	}

	if len(query.DerefAliases) > 0 {
		if _, err := ldaputil.DetermineDerefAliasesBehavior(query.DerefAliases); err != nil {
			validationResults.AddErrors(field.Invalid(fldPath.Child("derefAliases"),
				query.DerefAliases, "LDAP alias dereferencing instruction invalid"))
		}
	}

	if query.TimeLimit < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("timeout"), query.TimeLimit,
			"timeout must be equal to or greater than zero"))
	}

	if isDNOnly {
		if len(query.Filter) != 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("filter"), query.Filter, `cannot specify a filter when using "dn" as the UID attribute`))
		}
		return validationResults
	}

	if _, err := ldap.CompileFilter(query.Filter); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("filter"), query.Filter,
			fmt.Sprintf("invalid query filter: %v", err)))
	}

	return validationResults
}
