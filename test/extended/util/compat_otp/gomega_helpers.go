package compat_otp

import (
	"fmt"
	"os"
	"reflect"

	"github.com/onsi/gomega/types"
	logger "github.com/openshift/origin/test/extended/util/compat_otp/logext"
)

var secureMatchesMessage = fmt.Sprintf(
	"For security reasons we cannot print the compared values. If you need to debug this error you can `export %s=yes` and then the values will be printed",
	logger.EnableDebugLog,
)

// SecureMatcher it will not print the compared values when the matcher fails
type SecureMatcher struct {
	securedMatcher types.GomegaMatcher
}

// Match checks it the condition with the given type has the right value in the given field.
func (matcher *SecureMatcher) Match(actual interface{}) (success bool, err error) {
	return matcher.securedMatcher.Match(actual)
}

// FailureMessage returns the message in case of successful match
func (matcher *SecureMatcher) FailureMessage(actual interface{}) (message string) {
	if _, enabled := os.LookupEnv(logger.EnableDebugLog); enabled {
		return matcher.securedMatcher.FailureMessage(actual)
	}

	matcherType := reflect.TypeOf(matcher.securedMatcher).String()

	return fmt.Sprintf("%s did NOT match!! ", matcherType) + secureMatchesMessage
}

// NegatedFailureMessage returns the message in case of failed match
func (matcher *SecureMatcher) NegatedFailureMessage(actual interface{}) (message string) {
	if _, enabled := os.LookupEnv(logger.EnableDebugLog); enabled {
		return matcher.securedMatcher.NegatedFailureMessage(actual)
	}

	matcherType := reflect.TypeOf(matcher.securedMatcher).String()

	return fmt.Sprintf("%s matched, but should NOT match!! ", matcherType) + secureMatchesMessage
}

func Secure(securedMatcher types.GomegaMatcher) types.GomegaMatcher {
	return &SecureMatcher{
		securedMatcher: securedMatcher,
	}
}
