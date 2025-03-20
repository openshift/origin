package util

import "errors"

// suiteJUnitDir is used to add junit file directly from the test cases.
// openshift/usernamespace suite requires this to save the result of podman system test run in a container.
var suiteJUnitDir = make(map[string]string)

func GetSuiteJUnitDir(suiteName string) (string, error) {
	ret, ok := suiteJUnitDir[suiteName]
	if !ok {
		return "", errors.New("suite not found in JUnit directory map")
	}
	return ret, nil
}

func SetSuiteJUnitDir(suiteName, dir string) {
	if _, ok := suiteJUnitDir[suiteName]; ok {
		panic("suite already exists in JUnit directory map")
	}
	suiteJUnitDir[suiteName] = dir
}
