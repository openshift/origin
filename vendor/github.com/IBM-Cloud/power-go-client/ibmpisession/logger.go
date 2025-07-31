package ibmpisession

import (
	"fmt"
	"os"
	"reflect"
	"regexp"

	"github.com/go-openapi/runtime/logger"
)

var _ logger.Logger = &IBMPILogger{}

type IBMPILogger struct{}

func (IBMPILogger) Printf(format string, args ...interface{}) {
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, args...)
}

func (IBMPILogger) Debugf(format string, args ...interface{}) {
	if len(format) == 0 || format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Fprintf(os.Stderr, format, sanatizeArgs(args)...)
}

func sanatizeArgs(args []interface{}) (out []interface{}) {
	for _, arg := range args {
		if reflect.TypeOf(arg).String() == "string" {
			arg = sanitize(fmt.Sprintf("%s", arg))
		}
		out = append(out, arg)
	}
	return
}

func sanitize(input string) string {
	re := regexp.MustCompile(`(?m)^Authorization: .*`)
	sanitized := re.ReplaceAllString(input, "Authorization: "+privateDataPlaceholder())

	re = regexp.MustCompile(`(?m)^X-Auth-Token: .*`)
	sanitized = re.ReplaceAllString(sanitized, "X-Auth-Token: "+privateDataPlaceholder())

	re = regexp.MustCompile(`(?m)^X-Auth-Refresh-Token: .*`)
	sanitized = re.ReplaceAllString(sanitized, "X-Auth-Refresh-Token: "+privateDataPlaceholder())

	re = regexp.MustCompile(`(?m)^X-Auth-Uaa-Token: .*`)
	sanitized = re.ReplaceAllString(sanitized, "X-Auth-Uaa-Token: "+privateDataPlaceholder())

	re = regexp.MustCompile(`(?m)^X-Auth-User-Token: .*`)
	sanitized = re.ReplaceAllString(sanitized, "X-Auth-User-Token: "+privateDataPlaceholder())

	re = regexp.MustCompile(`password=[^&]*&`)
	sanitized = re.ReplaceAllString(sanitized, "password="+privateDataPlaceholder()+"&")

	re = regexp.MustCompile(`refresh_token=[^&]*&`)
	sanitized = re.ReplaceAllString(sanitized, "refresh_token="+privateDataPlaceholder()+"&")

	re = regexp.MustCompile(`apikey=[^&]*&`)
	sanitized = re.ReplaceAllString(sanitized, "apikey="+privateDataPlaceholder()+"&")

	sanitized = sanitizeJSON("token", sanitized)
	sanitized = sanitizeJSON("password", sanitized)
	sanitized = sanitizeJSON("apikey", sanitized)
	sanitized = sanitizeJSON("passcode", sanitized)

	return sanitized
}

func sanitizeJSON(propertySubstring string, json string) string {
	regex := regexp.MustCompile(fmt.Sprintf(`(?i)"([^"]*%s[^"]*)":\s*"[^\,]*"`, propertySubstring))
	return regex.ReplaceAllString(json, fmt.Sprintf(`"$1":"%s"`, privateDataPlaceholder()))
}

// privateDataPlaceholder returns the text to replace the sentive data.
func privateDataPlaceholder() string {
	return "[PRIVATE DATA HIDDEN]"
}
