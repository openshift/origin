// Package osrelease package provides access to parsed os-release information
package osrelease

import (
	"errors"
	"io/ioutil"
	"reflect"
	"strings"
)

// etcPath is the default path to os-release in etc
var etcPath = "/etc/os-release"

// usrPath is the default path to os-release in usr
var usrPath = "/usr/lib/os-release"

// OSRelease implements the format noted at
// https://www.freedesktop.org/software/systemd/man/os-release.html
type OSRelease struct {
	NAME               string            // OS Identifier
	VERSION            string            // Full OS Version
	ID                 string            // Lowercase OS identifier
	ID_LIKE            string            // Coma seperated list of closely related OSes
	VERSION_ID         string            // Lowercase OS version
	VERSION_CODENAME   string            // Lowercase OS release codename
	PRETTY_NAME        string            // Human presented OS/Release name
	ANSI_COLOR         string            // Suggested color for showing OS name
	CPE_NAME           string            // See http://scap.nist.gov/specifications/cpe/
	HOME_URL           string            // Main link for the OS
	BUG_REPORT_URL     string            // Bug report link for the OS
	PRIVACY_POLICY_URL string            // Privacy policy link for the OS
	VARIANT            string            // Human presnted OS Variant
	VARIANT_ID         string            // Lowercase OS Variant identifier
	ADDITIONAL_FIELDS  map[string]string // Custom/unsupported fields
	supportedFields    []string          // List of supported fields
}

// getFields denerates a list of fields which are supported by
// the os-release spec.
func (o *OSRelease) getFields() []string {
	t := reflect.ValueOf(OSRelease{}).Type()
	var fields []string
	for x := 0; x < t.NumField(); x++ {
		fields = append(fields, t.Field(x).Name)
	}
	return fields
}

// SetField sets a field on an instance of OSRelease.
// If the field is not a supported field it will be
// added to ADDITIONAL_FIELDS.
func (o *OSRelease) SetField(key, value string) {
	supported := false
	for _, supportedKey := range o.supportedFields {
		if key == supportedKey {
			supported = true
			break
		}
	}
	if supported == true {
		field := reflect.ValueOf(o).Elem()
		field.FieldByName(key).SetString(value)
	} else {
		o.ADDITIONAL_FIELDS[key] = value
	}
}

// GetField returns a field from the instance. If the
// field is not present then an error us returned.
func (o *OSRelease) GetField(key string) (string, error) {
	supported := false
	for _, supportedKey := range o.supportedFields {
		if key == supportedKey {
			supported = true
			break
		}
	}

	if supported == true {
		return reflect.ValueOf(*o).FieldByName(key).String(), nil
	}
	for additionalKey := range o.ADDITIONAL_FIELDS {
		if key == additionalKey {
			return o.ADDITIONAL_FIELDS[additionalKey], nil
		}
	}
	// Fail case
	return "", errors.New("Field does not exist")
}

// Populate reads the os-release file, parses it, and
// sets fields for use.
func (o *OSRelease) Populate(paths []string) error {
	o.supportedFields = o.getFields()
	o.ADDITIONAL_FIELDS = make(map[string]string)

	// Iterate over our known file paths
	var rawContent []byte
	var err error
	for _, path := range paths {
		rawContent, err = ioutil.ReadFile(path)
		if err == nil {
			break
		}
	}
	// Error if we were unable to get any content
	if len(rawContent) == 0 {
		return errors.New("No content available from os-release files")
	}

	for _, line := range strings.Split(string(rawContent), "\n") {
		if strings.Contains(line, "=") {
			item := strings.Split(string(line), "=")
			// Remove prefix/suffix quotes from values
			item[1] = strings.TrimPrefix(item[1], "\"")
			item[1] = strings.TrimSuffix(item[1], "\"")
			// Set the field
			o.SetField(item[0], item[1])
		}
	}
	return nil
}

// New creates and returns a new insance of OSRelease.
// Generally New should be used to create a new instance.
func New() (OSRelease, error) {
	o, err := NewWithOverrides(etcPath, usrPath)
	if err != nil {
		return o, err
	}
	return o, nil
}

// NewWithOverrides creates and returns a new instance of OSRelease
// using the paths passed in.
func NewWithOverrides(etcOverridePath, usrOverridePath string) (OSRelease, error) {
	o := OSRelease{}
	err := o.Populate([]string{etcOverridePath, usrOverridePath})
	if err != nil {
		return o, err
	}
	return o, nil
}
