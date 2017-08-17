package clientcmd

import (
	"encoding/json"
	"fmt"

	"github.com/blang/semver"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/version"
)

// Gate returns an error if the server is below minServerVersion or above/equal maxServerVersion.
// To test only for min or only max version, set the other string to the empty value.
func Gate(ocClient *client.Client, minServerVersion, maxServerVersion string) error {
	if len(minServerVersion) == 0 && len(maxServerVersion) == 0 {
		return fmt.Errorf("No version info passed to gate command")
	}

	ocVersionBody, err := ocClient.Get().AbsPath("/version/openshift").Do().Raw()
	if err != nil {
		return err
	}
	ocServerInfo := &version.Info{}
	if err := json.Unmarshal(ocVersionBody, ocServerInfo); err != nil {
		return err
	}
	ocVersion := ocServerInfo.String()
	// skip first chracter as Openshift returns a 'v' preceding the actual
	// version string which semver does not grok
	semVersion, err := semver.Parse(ocVersion[1:])
	if err != nil {
		return fmt.Errorf("Failed to parse server version %s: %v", ocVersion, err)
	}
	// ignore pre-release version info
	semVersion.Pre = nil

	if len(minServerVersion) > 0 {
		min, err := semver.Parse(minServerVersion)
		if err != nil {
			return fmt.Errorf("Failed to parse min gate version %s: %v", minServerVersion, err)
		}
		// ignore pre-release version info
		min.Pre = nil
		if semVersion.LT(min) {
			return fmt.Errorf("This command works only with server versions > %s, found %s", minServerVersion, ocVersion)
		}
	}

	if len(maxServerVersion) > 0 {
		max, err := semver.Parse(maxServerVersion)
		if err != nil {
			return fmt.Errorf("Failed to parse max gate version %s: %v", maxServerVersion, err)
		}
		// ignore pre-release version info
		max.Pre = nil
		if semVersion.GTE(max) {
			return fmt.Errorf("This command works only with server versions < %s, found %s", maxServerVersion, ocVersion)
		}
	}

	// OK this is within min/max all good!
	return nil
}
