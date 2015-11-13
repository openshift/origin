package validation

import (
	"fmt"

	"github.com/go-ldap/ldap"

	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

func ValidateLDAPSyncConfig(config *api.LDAPSyncConfig) ValidationResults {
	validationResults := ValidateLDAPClientConfig(config.URL, config.BindDN, config.BindPassword, config.CA, config.Insecure)

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
		validationResults.AddErrors(fielderrors.NewFieldInvalid("", config, fmt.Sprintf("only one schema-specific config is allowed; found %v", schemaConfigsFound)))
	}
	if len(schemaConfigsFound) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("", config, fmt.Sprintf("exactly one schema-specific config is required;  one of %v", []string{"rfc2307", "activeDirectory", "augmentedActiveDirectory"})))
	}

	return validationResults
}

func ValidateLDAPClientConfig(url, bindDN, bindPassword, CA string, insecure bool) ValidationResults {
	validationResults := ValidationResults{}

	if len(url) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("url"))
		return validationResults
	}

	u, err := ldaputil.ParseURL(url)
	if err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("url", url, err.Error()))
		return validationResults
	}

	// Make sure bindDN and bindPassword are both set, or both unset
	// Both unset means an anonymous bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.1)
	// Both set means the name/password simple bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.3)
	if (len(bindDN) == 0) != (len(bindPassword) == 0) {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("bindDN", bindDN,
			"bindDN and bindPassword must both be specified, or both be empty"))
		validationResults.AddErrors(fielderrors.NewFieldInvalid("bindPassword", "<masked>",
			"bindDN and bindPassword must both be specified, or both be empty"))
	}

	if insecure {
		if u.Scheme == ldaputil.SchemeLDAPS {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("url", url,
				fmt.Sprintf("Cannot use %s scheme with insecure=true", u.Scheme)))
		}
		if len(CA) > 0 {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("ca", CA,
				"Cannot specify a ca with insecure=true"))
		}
	} else {
		if len(CA) > 0 {
			validationResults.AddErrors(ValidateFile(CA, "ca")...)
		}
	}

	// Warn if insecure
	if insecure {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid("insecure", insecure,
			"validating passwords over an insecure connection could allow them to be intercepted"))
	}

	return validationResults
}

func ValidateRFC2307Config(config *api.RFC2307Config) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllGroupsQuery).Prefix("groupsQuery"))
	if len(config.GroupUIDAttribute) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupUIDAttribute"))
	}
	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupNameAttributes"))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembershipAttributes"))
	}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery).Prefix("usersQuery"))
	if len(config.UserUIDAttribute) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userUIDAttribute"))
	}
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userNameAttributes"))
	}

	return validationResults
}

func ValidateActiveDirectoryConfig(config *api.ActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery).Prefix("usersQuery"))
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userNameAttributes"))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembershipAttributes"))
	}

	return validationResults
}

func ValidateAugmentedActiveDirectoryConfig(config *api.AugmentedActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery).Prefix("usersQuery"))
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userNameAttributes"))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembershipAttributes"))
	}

	validationResults.Append(ValidateLDAPQuery(config.AllGroupsQuery).Prefix("groupsQuery"))
	if len(config.GroupUIDAttribute) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupUIDAttribute"))
	}
	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupNameAttributes"))
	}

	return validationResults
}

func ValidateLDAPQuery(query api.LDAPQuery) ValidationResults {
	validationResults := ValidationResults{}

	if _, err := ldap.ParseDN(query.BaseDN); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("baseDN", query.BaseDN,
			fmt.Sprintf("invalid base DN for search: %v", err)))
	}

	if len(query.Scope) > 0 {
		if _, err := ldaputil.DetermineLDAPScope(query.Scope); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("scope", query.Scope,
				"invalid LDAP search scope"))
		}
	}

	if len(query.DerefAliases) > 0 {
		if _, err := ldaputil.DetermineDerefAliasesBehavior(query.DerefAliases); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("derefAliases",
				query.DerefAliases, "LDAP alias dereferencing instruction invalid"))
		}
	}

	if query.TimeLimit < 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("timeout", query.TimeLimit,
			"timeout must be equal to or greater than zero"))
	}

	if _, err := ldap.CompileFilter(query.Filter); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("filter", query.Filter,
			fmt.Sprintf("invalid query filter: %v", err)))
	}

	return validationResults
}
