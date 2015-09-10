package validation

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/go-ldap/ldap"
	"github.com/spf13/pflag"

	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/auth/ldaputil"
	"github.com/openshift/origin/pkg/cmd/server/api"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ValidateHostPort(value string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(value) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if _, _, err := net.SplitHostPort(value); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, value, "must be a host:port"))
	}

	return allErrs
}

func ValidateCertInfo(certInfo api.CertInfo, required bool) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if required || len(certInfo.CertFile) > 0 || len(certInfo.KeyFile) > 0 {
		if len(certInfo.CertFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("certFile"))
		}
		if len(certInfo.KeyFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("keyFile"))
		}
	}

	if len(certInfo.CertFile) > 0 {
		allErrs = append(allErrs, ValidateFile(certInfo.CertFile, "certFile")...)
	}

	if len(certInfo.KeyFile) > 0 {
		allErrs = append(allErrs, ValidateFile(certInfo.KeyFile, "keyFile")...)
	}

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateHostPort(info.BindAddress, "bindAddress")...)
	allErrs = append(allErrs, ValidateCertInfo(info.ServerCert, false)...)

	switch info.BindNetwork {
	case "tcp", "tcp4", "tcp6":
	default:
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("bindNetwork", info.BindNetwork, "must be 'tcp', 'tcp4', or 'tcp6'"))
	}

	if len(info.ServerCert.CertFile) > 0 {
		if len(info.ClientCA) > 0 {
			allErrs = append(allErrs, ValidateFile(info.ClientCA, "clientCA")...)
		}
	} else {
		if len(info.ClientCA) > 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("clientCA", info.ClientCA, "cannot specify a clientCA without a certFile"))
		}
	}

	return allErrs
}

func ValidateHTTPServingInfo(info api.HTTPServingInfo) fielderrors.ValidationErrorList {
	allErrs := ValidateServingInfo(info.ServingInfo)

	if info.MaxRequestsInFlight < 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("maxRequestsInFlight", info.MaxRequestsInFlight, "must be zero (no limit) or greater"))
	}

	if info.RequestTimeoutSeconds < -1 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("requestTimeoutSeconds", info.RequestTimeoutSeconds, "must be -1 (no timeout), 0 (default timeout), or greater"))
	}

	return allErrs
}

func ValidateDisabledFeatures(disabledFeatures []string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	known := util.NewStringSet()
	for _, feature := range api.KnownOpenShiftFeatures {
		known.Insert(strings.ToLower(feature))
	}
	for i, feature := range disabledFeatures {
		if !known.Has(strings.ToLower(feature)) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("%s[%d]", field, i), disabledFeatures[i], fmt.Sprintf("not one of valid features: %s", strings.Join(api.KnownOpenShiftFeatures, ", "))))
		}
	}

	return allErrs
}

func ValidateKubeConfig(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(path, field)...)
	// TODO: load and parse

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo api.RemoteConnectionInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("url"))
	} else {
		_, urlErrs := ValidateURL(remoteConnectionInfo.URL, "url")
		allErrs = append(allErrs, urlErrs...)
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(remoteConnectionInfo.CA, "ca")...)
	}

	allErrs = append(allErrs, ValidateCertInfo(remoteConnectionInfo.ClientCert, false)...)

	return allErrs
}

func ValidatePodManifestConfig(podManifestConfig *api.PodManifestConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// the Path can be a file or a directory
	allErrs = append(allErrs, ValidateFile(podManifestConfig.Path, "path")...)
	if podManifestConfig.FileCheckIntervalSeconds < 1 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("fileCheckIntervalSeconds", podManifestConfig.FileCheckIntervalSeconds, "interval has to be positive"))
	}

	return allErrs
}

func ValidateSpecifiedIP(ipString string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "cannot be an unspecified IP"))
	}

	return allErrs
}

func ValidateSecureURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	url, urlErrs := ValidateURL(urlString, field)
	if len(urlErrs) == 0 && url.Scheme != "https" {
		urlErrs = append(urlErrs, fielderrors.NewFieldInvalid(field, urlString, "must use https scheme"))
	}
	return url, urlErrs
}

func ValidateURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	allErrs := fielderrors.ValidationErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a scheme (e.g. https://)"))
	}
	if len(urlObj.Host) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a host"))
	}
	return urlObj, allErrs
}

func ValidateNamespace(namespace, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if ok, _ := kvalidation.ValidateNamespaceName(namespace, false); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, namespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateFile(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "could not read file"))
	}

	return allErrs
}

func ValidateDir(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(path) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else {
		fileInfo, err := os.Stat(path)
		if err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "could not read info"))
		} else if !fileInfo.IsDir() {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "not a directory"))
		}
	}

	return allErrs
}

func ValidateExtendedArguments(config api.ExtendedArguments, flagFunc func(*pflag.FlagSet)) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// check extended arguments for errors
	for _, err := range cmdflags.Resolve(config, flagFunc) {
		switch t := err.(type) {
		case *fielderrors.ValidationError:
			allErrs = append(allErrs, t)
		default:
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("????", config, err.Error()))
		}
	}

	return allErrs
}

func ValidateLDAPSyncConfig(config api.LDAPSyncConfig) ValidationResults {
	validationResults := ValidateLDAPClientConfig("config",
		config.Host,
		config.BindDN,
		config.BindPassword,
		config.CA,
		config.Insecure)

	var numConfigs int

	if config.RFC2307Config != nil {
		configResults := ValidateRFC2307Config(config.RFC2307Config)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		numConfigs++
	}
	if config.ActiveDirectoryConfig != nil {
		configResults := ValidateActiveDirectoryConfig(config.ActiveDirectoryConfig)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		numConfigs++
	}
	if config.AugmentedActiveDirectoryConfig != nil {
		configResults := ValidateAugmentedActiveDirectoryConfig(config.AugmentedActiveDirectoryConfig)
		validationResults.AddErrors(configResults.Errors...)
		validationResults.AddWarnings(configResults.Warnings...)
		numConfigs++
	}
	if numConfigs != 1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("", config.LDAPSchemaSpecificConfig,
			"only one schema-specific config is allowed"))
	}

	return validationResults
}

func ValidateLDAPClientConfig(parent, url, bindDN, bindPassword, CA string, insecure bool) ValidationResults {
	validationResults := ValidationResults{}

	if len(url) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired(parent + ".host"))
		return validationResults
	}

	u, err := ldaputil.ParseURL(url)
	if err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid(parent+".URL", url, err.Error()))
		return validationResults
	}

	// Make sure bindDN and bindPassword are both set, or both unset
	// Both unset means an anonymous bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.1)
	// Both set means the name/password simple bind is used for search (https://tools.ietf.org/html/rfc4513#section-5.1.3)
	if (len(bindDN) == 0) != (len(bindPassword) == 0) {
		validationResults.AddErrors(fielderrors.NewFieldInvalid(parent+".bindDN", bindDN,
			"bindDN and bindPassword must both be specified, or both be empty"))
		validationResults.AddErrors(fielderrors.NewFieldInvalid(parent+".bindPassword", "<masked>",
			"bindDN and bindPassword must both be specified, or both be empty"))
	}

	if insecure {
		if u.Scheme == ldaputil.SchemeLDAPS {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(parent+".url", url,
				fmt.Sprintf("Cannot use %s scheme with insecure=true", u.Scheme)))
		}
		if len(CA) > 0 {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(parent+".ca", CA,
				"Cannot specify a ca with insecure=true"))
		}
	} else {
		if len(CA) > 0 {
			validationResults.AddErrors(ValidateFile(CA, parent+".ca")...)
		}
	}

	// Warn if insecure
	if insecure {
		validationResults.AddWarnings(fielderrors.NewFieldInvalid(parent+".insecure", insecure,
			"validating passwords over an insecure connection could allow them to be intercepted"))
	}

	return validationResults
}

func ValidateRFC2307Config(config *api.RFC2307Config) ValidationResults {
	validationResults := ValidationResults{}

	groupQueryResults := ValidateLDAPQuery("groupQuery", config.GroupQuery)
	validationResults.AddErrors(groupQueryResults.Errors...)
	validationResults.AddWarnings(groupQueryResults.Warnings...)

	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupName"))
	}

	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembership"))
	}

	userQueryResults := ValidateLDAPQuery("userQuery", config.UserQuery)
	validationResults.AddErrors(userQueryResults.Errors...)
	validationResults.AddWarnings(userQueryResults.Warnings...)

	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userName"))
	}

	return validationResults
}

func ValidateActiveDirectoryConfig(config *api.ActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	userQueryResults := ValidateLDAPQuery("usersQuery", config.UsersQuery)
	validationResults.AddErrors(userQueryResults.Errors...)
	validationResults.AddWarnings(userQueryResults.Warnings...)

	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userName"))
	}

	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembership"))
	}

	return validationResults
}

func ValidateAugmentedActiveDirectoryConfig(config *api.AugmentedActiveDirectoryConfig) ValidationResults {
	validationResults := ValidationResults{}

	groupQueryResults := ValidateLDAPQuery("groupQuery", config.GroupQuery)
	validationResults.AddErrors(groupQueryResults.Errors...)
	validationResults.AddWarnings(groupQueryResults.Warnings...)

	if len(config.GroupNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupName"))
	}

	if len(config.GroupMembershipAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("groupMembership"))
	}

	userQueryResults := ValidateLDAPQuery("usersQuery", config.UsersQuery)
	validationResults.AddErrors(userQueryResults.Errors...)
	validationResults.AddWarnings(userQueryResults.Warnings...)

	if len(config.UserNameAttributes) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("userName"))
	}
	return validationResults
}

func ValidateLDAPQuery(queryName string, query api.LDAPQuery) ValidationResults {
	validationResults := ValidationResults{}

	if _, err := ldap.ParseDN(query.BaseDN); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid(queryName+".baseDN", query.BaseDN,
			fmt.Sprintf("invalid base DN for search: %v", err)))
	}

	if len(query.Scope) > 0 {
		if _, err := ldaputil.DetermineLDAPScope(query.Scope); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(queryName+".scope", query.Scope,
				"invalid LDAP search scope"))
		}
	}

	if len(query.DerefAliases) > 0 {
		if _, err := ldaputil.DetermineDerefAliasesBehavior(query.DerefAliases); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(queryName+".derefAliases",
				query.DerefAliases, "LDAP alias dereferencing instruction invalid"))
		}
	}

	if query.TimeLimit < 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid(queryName+".timeout", query.TimeLimit,
			"timeout must be equal to or greater than zero"))
	}

	if _, err := ldap.CompileFilter(query.Filter); err != nil {
		validationResults.AddErrors(fielderrors.NewFieldInvalid(queryName+".filter", query.Filter,
			fmt.Sprintf("invalid query filter: %v", err)))
	}

	return validationResults
}
