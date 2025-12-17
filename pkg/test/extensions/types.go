package extensions

import (
	"fmt"
	"strings"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift-eng/openshift-tests-extension/pkg/extension/extensiontests"
	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
)

// Extension represents an extension to openshift-tests.
type Extension struct {
	*extension.Extension

	// -- origin specific info --
	Binary               *TestBinary `json:"-"`
	Source               Source      `json:"source"`
	ExtensionArtifactDir string      `json:"extension_artifact_dir"`
}

// Source contains the details of the commit and source URL.
type Source struct {
	*extension.Source

	// -- origin specific info --

	// SourceImage contains the payload image it was extracted from.
	SourceImage string `json:"source_image,omitempty"`

	// SourceBinary contains the path in the source image for this extension.
	SourceBinary string `json:"source_binary,omitempty"`
}

type ExtensionTestSpecs []*ExtensionTestSpec

type EnvironmentSelector struct {
	Include string `json:"include,omitempty"`
	Exclude string `json:"exclude,omitempty"`
}

type ExtensionTestSpec struct {
	*extensiontests.ExtensionTestSpec

	// Binary invokes a link to the external binary that provided this test
	Binary *TestBinary
}

// FilterWrappedSpecs applies the upstream Filter method (defined on extensiontests.ExtensionTestSpecs)
// while preserving our local ExtensionTestSpec wrappers.
//
// This is a bit awkward because our ExtensionTestSpecs is a slice of wrappers around
// *extensiontests.ExtensionTestSpec, but the Filter method only exists on the upstream slice type.
// To work around this, we:
//  1. Extract the underlying *extensiontests.ExtensionTestSpec values.
//  2. Call the upstream Filter.
//  3. Map the filtered results back to the original wrapped specs using pointer identity.
//
// This preserves metadata like the Binary field stored in our wrapper.
func FilterWrappedSpecs(
	wrappedSpecs ExtensionTestSpecs,
	qualifiers []string,
) (ExtensionTestSpecs, error) {
	var baseSpecs extensiontests.ExtensionTestSpecs
	specMap := make(map[*extensiontests.ExtensionTestSpec]*ExtensionTestSpec)

	for _, spec := range wrappedSpecs {
		baseSpecs = append(baseSpecs, spec.ExtensionTestSpec)
		specMap[spec.ExtensionTestSpec] = spec
	}

	filtered, err := baseSpecs.Filter(qualifiers)
	if err != nil {
		return nil, err
	}

	var result ExtensionTestSpecs
	for _, f := range filtered {
		if orig, ok := specMap[f]; ok {
			result = append(result, orig)
		}
	}

	return result, nil
}

type ExtensionTestResults []*ExtensionTestResult

type ExtensionTestResult struct {
	*extensiontests.ExtensionTestResult

	// Source is the information from the extension binary (it's image tag, repo, commit sha, etc), reported
	// up by origin so it's easy to identify where a particular result came from in the overall combined result JSON.
	Source Source `json:"source"`
}

// ToHTML converts the extension test results to an HTML representation using the upstream ToHTML method.
func (results ExtensionTestResults) ToHTML(suiteName string) ([]byte, error) {
	var upstreamResults extensiontests.ExtensionTestResults
	for _, r := range results {
		if r != nil && r.ExtensionTestResult != nil {
			upstreamResults = append(upstreamResults, r.ExtensionTestResult)
		}
	}
	return upstreamResults.ToHTML(suiteName)
}

// EnvironmentFlagName enumerates each possible EnvironmentFlag's name to be passed to the external binary
type EnvironmentFlagName string

const (
	platform             EnvironmentFlagName = "platform"
	network              EnvironmentFlagName = "network"
	networkStack         EnvironmentFlagName = "network-stack"
	upgrade              EnvironmentFlagName = "upgrade"
	topology             EnvironmentFlagName = "topology"
	architecture         EnvironmentFlagName = "architecture"
	externalConnectivity EnvironmentFlagName = "external-connectivity"
	optionalCapability   EnvironmentFlagName = "optional-capability"
	fact                 EnvironmentFlagName = "fact"
	version              EnvironmentFlagName = "version"
	featureGate          EnvironmentFlagName = "feature-gate"
	apiGroup             EnvironmentFlagName = "api-group"
)

type EnvironmentFlagsBuilder struct {
	flags EnvironmentFlags
}

func (e *EnvironmentFlagsBuilder) AddAPIGroups(values ...string) *EnvironmentFlagsBuilder {
	for _, value := range values {
		e.flags = append(e.flags, newEnvironmentFlag(apiGroup, value))
	}
	return e
}

func (e *EnvironmentFlagsBuilder) AddFeatureGates(values ...string) *EnvironmentFlagsBuilder {
	for _, value := range values {
		e.flags = append(e.flags, newEnvironmentFlag(featureGate, value))
	}
	return e
}
func (e *EnvironmentFlagsBuilder) AddPlatform(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(platform, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddNetwork(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(network, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddNetworkStack(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(networkStack, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddUpgrade(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(upgrade, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddTopology(value *configv1.TopologyMode) *EnvironmentFlagsBuilder {
	if value != nil {
		e.flags = append(e.flags, newEnvironmentFlag(topology, string(*value)))
	}
	return e
}

func (e *EnvironmentFlagsBuilder) AddArchitecture(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(architecture, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddExternalConnectivity(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(externalConnectivity, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddOptionalCapability(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(optionalCapability, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddFact(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(fact, value))
	return e
}

func (e *EnvironmentFlagsBuilder) AddVersion(value string) *EnvironmentFlagsBuilder {
	e.flags = append(e.flags, newEnvironmentFlag(version, value))
	return e
}

func (e *EnvironmentFlagsBuilder) Build() EnvironmentFlags {
	return e.flags
}

// EnvironmentFlag contains the info required to build an argument to pass to the external binary for the given Name
type EnvironmentFlag struct {
	Name         EnvironmentFlagName
	Value        string
	SinceVersion string
}

// newEnvironmentFlag creates an EnvironmentFlag including the determination of the SinceVersion
func newEnvironmentFlag(name EnvironmentFlagName, value string) EnvironmentFlag {
	return EnvironmentFlag{Name: name, Value: value, SinceVersion: EnvironmentFlagVersions[name]}
}

func (ef EnvironmentFlag) ArgString() string {
	return fmt.Sprintf("--%s=%s", ef.Name, ef.Value)
	//return fmt.Sprintf("%s=%s", ef.Name, ef.Value)
}

type EnvironmentFlags []EnvironmentFlag

// ArgStrings properly formats all EnvironmentFlags as a list of argument strings to pass to the external binary
func (ef EnvironmentFlags) ArgStrings() []string {
	var argStrings []string
	for _, flag := range ef {
		argStrings = append(argStrings, flag.ArgString())
	}
	return argStrings
}

func (ef EnvironmentFlags) String() string {
	return strings.Join(ef.ArgStrings(), " ")
}

func (ef EnvironmentFlags) LogFields() logrus.Fields {
	fields := logrus.Fields{}

	for _, flag := range ef {
		name := string(flag.Name)
		if val, ok := fields[name]; ok {
			fields[name] = append(val.([]string), flag.Value)
		} else {
			fields[name] = []string{flag.Value}
		}
	}

	return fields
}

// EnvironmentFlagVersions holds the "Since" version metadata for each flag.
var EnvironmentFlagVersions = map[EnvironmentFlagName]string{
	featureGate:          "v1.1",
	apiGroup:             "v1.1",
	platform:             "v1.0",
	network:              "v1.0",
	networkStack:         "v1.0",
	upgrade:              "v1.0",
	topology:             "v1.0",
	architecture:         "v1.0",
	externalConnectivity: "v1.0",
	optionalCapability:   "v1.0",
	fact:                 "v1.0", //TODO(sgoeddel): this will be set in a later version
	version:              "v1.0",
}
