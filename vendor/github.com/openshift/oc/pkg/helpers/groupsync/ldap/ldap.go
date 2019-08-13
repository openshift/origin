package ldap

import (
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/openshift/library-go/pkg/certs"

	"gopkg.in/ldap.v2"

	"k8s.io/apimachinery/pkg/util/validation/field"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/validation"
	"github.com/openshift/library-go/pkg/security/ldaputil"
)

func GetStringSourceFileReferences(s *legacyconfigv1.StringSource) []*string {
	if s == nil {
		return nil
	}
	return []*string{
		&s.File,
		&s.KeyFile,
	}
}

func ResolveStringValue(s legacyconfigv1.StringSource) (string, error) {
	var value string
	switch {
	case len(s.Value) > 0:
		value = s.Value
	case len(s.Env) > 0:
		value = os.Getenv(s.Env)
	case len(s.File) > 0:
		data, err := ioutil.ReadFile(s.File)
		if err != nil {
			return "", err
		}
		value = string(data)
	default:
		value = ""
	}

	if len(s.KeyFile) == 0 {
		// value is cleartext, return
		return value, nil
	}

	keyData, err := ioutil.ReadFile(s.KeyFile)
	if err != nil {
		return "", err
	}

	secretBlock, ok := certs.BlockFromBytes([]byte(value), certs.StringSourceEncryptedBlockType)
	if !ok {
		return "", fmt.Errorf("no valid PEM block of type %q found in data", certs.StringSourceEncryptedBlockType)
	}

	keyBlock, ok := certs.BlockFromBytes(keyData, certs.StringSourceKeyBlockType)
	if !ok {
		return "", fmt.Errorf("no valid PEM block of type %q found in key", certs.StringSourceKeyBlockType)
	}

	data, err := x509.DecryptPEMBlock(secretBlock, keyBlock.Bytes)
	return string(data), err
}

func ValidateLDAPSyncConfig(config *legacyconfigv1.LDAPSyncConfig) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

	validationResults.Append(ValidateStringSource(config.BindPassword, field.NewPath("bindPassword")))
	bindPassword, _ := ResolveStringValue(config.BindPassword)
	validationResults.Append(ValidateLDAPClientConfig(config.URL, config.BindDN, bindPassword, config.CA, config.Insecure, nil))

	for ldapGroupUID, openShiftGroupName := range config.LDAPGroupUIDToOpenShiftGroupNameMapping {
		if len(ldapGroupUID) == 0 || len(openShiftGroupName) == 0 {
			validationResults.AddErrors(field.Invalid(field.NewPath("groupUIDNameMapping").Key(ldapGroupUID), openShiftGroupName, "has empty key or value"))
		}
	}

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

func ValidateLDAPClientConfig(url, bindDN, bindPassword, CA string, insecure bool, fldPath *field.Path) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

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
			validationResults.AddErrors(validation.ValidateFile(CA, fldPath.Child("ca"))...)
		}
	}

	// Warn if insecure
	if insecure {
		validationResults.AddWarnings(field.Invalid(fldPath.Child("insecure"), insecure,
			"validating passwords over an insecure connection could allow them to be intercepted"))
	}

	return validationResults
}

func ValidateRFC2307Config(config *legacyconfigv1.RFC2307Config) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

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

func ValidateActiveDirectoryConfig(config *legacyconfigv1.ActiveDirectoryConfig) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

	validationResults.Append(ValidateLDAPQuery(config.AllUsersQuery, field.NewPath("usersQuery")))
	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("userNameAttributes"), ""))
	}
	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(field.Required(field.NewPath("groupMembershipAttributes"), ""))
	}

	return validationResults
}

func ValidateAugmentedActiveDirectoryConfig(config *legacyconfigv1.AugmentedActiveDirectoryConfig) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

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

func ValidateLDAPQuery(query legacyconfigv1.LDAPQuery, fldPath *field.Path) validation.ValidationResults {
	return validateLDAPQuery(query, fldPath, false)
}
func validateLDAPQuery(query legacyconfigv1.LDAPQuery, fldPath *field.Path, isDNOnly bool) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

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

func ValidateStringSource(s legacyconfigv1.StringSource, fieldPath *field.Path) validation.ValidationResults {
	validationResults := validation.ValidationResults{}
	methods := 0
	if len(s.Value) > 0 {
		methods++
	}
	if len(s.File) > 0 {
		methods++
		fileErrors := validation.ValidateFile(s.File, fieldPath.Child("file"))
		validationResults.AddErrors(fileErrors...)

		// If the file was otherwise ok, and its value will be used verbatim, warn about trailing whitespace
		if len(fileErrors) == 0 && len(s.KeyFile) == 0 {
			if data, err := ioutil.ReadFile(s.File); err != nil {
				validationResults.AddErrors(field.Invalid(fieldPath.Child("file"), s.File, fmt.Sprintf("could not read file: %v", err)))
			} else if len(data) > 0 {
				r, _ := utf8.DecodeLastRune(data)
				if unicode.IsSpace(r) {
					validationResults.AddWarnings(field.Invalid(fieldPath.Child("file"), s.File, "contains trailing whitespace which will be included in the value"))
				}
			}
		}
	}
	if len(s.Env) > 0 {
		methods++
	}
	if methods > 1 {
		validationResults.AddErrors(field.Invalid(fieldPath, "", "only one of value, file, and env can be specified"))
	}

	if len(s.KeyFile) > 0 {
		validationResults.AddErrors(validation.ValidateFile(s.KeyFile, fieldPath.Child("keyFile"))...)
	}

	return validationResults
}
