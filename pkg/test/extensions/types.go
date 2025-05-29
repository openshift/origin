package extensions

import (
	"fmt"
	"strings"
	"time"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/sets"
)

// ExtensionInfo represents an extension to openshift-tests.
type ExtensionInfo struct {
	APIVersion string    `json:"apiVersion"`
	Source     Source    `json:"source"`
	Component  Component `json:"component"`

	// Suites that the extension wants to advertise/participate in.
	Suites []Suite `json:"suites"`

	// -- origin specific info --
	ExtensionArtifactDir string `json:"extension_artifact_dir"`
}

// Source contains the details of the commit and source URL.
type Source struct {
	// Commit from which this binary was compiled.
	Commit string `json:"commit"`
	// BuildDate ISO8601 string of when the binary was built
	BuildDate string `json:"build_date"`
	// GitTreeState lets you know the status of the git tree (clean/dirty)
	GitTreeState string `json:"git_tree_state"`
	// SourceURL contains the url of the git repository (if known) that this extension was built from.
	SourceURL string `json:"source_url,omitempty"`

	// -- origin specific info --

	// SourceImage contains the payload image it was extracted from.
	SourceImage string `json:"source_image,omitempty"`

	// SourceBinary contains the path in the source image for this extension.
	SourceBinary string `json:"source_binary,omitempty"`
}

// Component represents the component the binary acts on.
type Component struct {
	// The product this component is part of.
	Product string `json:"product"`
	// The type of the component.
	Kind string `json:"type"`
	// The name of the component.
	Name string `json:"name"`
}

// Suite represents additional suites the extension wants to advertise.
type Suite struct {
	// The name of the suite.
	Name string `json:"name"`
	// Parent suites this suite is part of.
	Parents []string `json:"parents,omitempty"`
	// Qualifiers are CEL expressions that are OR'd together for test selection that are members of the suite.
	Qualifiers []string `json:"qualifiers,omitempty"`
}

type Lifecycle string

var LifecycleInforming Lifecycle = "informing"
var LifecycleBlocking Lifecycle = "blocking"

type ExtensionTestSpecs []*ExtensionTestSpec

type EnvironmentSelector struct {
	Include string `json:"include,omitempty"`
	Exclude string `json:"exclude,omitempty"`
}

type ExtensionTestSpec struct {
	Name string `json:"name"`

	// OriginalName contains the very first name this test was ever known as, used to preserve
	// history across all names.
	OriginalName string `json:"originalName,omitempty"`

	// Labels are single string values to apply to the test spec
	Labels sets.Set[string] `json:"labels"`

	// Tags are key:value pairs
	Tags map[string]string `json:"tags,omitempty"`

	// Resources gives optional information about what's required to run this test.
	Resources Resources `json:"resources"`

	// Source is the origin of the test.
	Source string `json:"source"`

	// CodeLocations are the files where the spec originates from.
	CodeLocations []string `json:"codeLocations,omitempty"`

	// Lifecycle informs the executor whether the test is informing only, and should not cause the
	// overall job run to fail, or if it's blocking where a failure of the test is fatal.
	// Informing lifecycle tests can be used temporarily to gather information about a test's stability.
	// Tests must not remain informing forever.
	Lifecycle Lifecycle `json:"lifecycle"`

	// EnvironmentSelector allows for CEL expressions to be used to control test inclusion
	EnvironmentSelector EnvironmentSelector `json:"environmentSelector,omitempty"`

	// Binary invokes a link to the external binary that provided this test
	Binary *TestBinary
}

type Resources struct {
	Isolation Isolation `json:"isolation"`
	Memory    string    `json:"memory,omitempty"`
	Duration  string    `json:"duration,omitempty"`
	Timeout   string    `json:"timeout,omitempty"`
}

type Isolation struct {
	Mode     string   `json:"mode,omitempty"`
	Conflict []string `json:"conflict,omitempty"`
}

type ExtensionTestResults []*ExtensionTestResult

type Result string

var ResultPassed Result = "passed"
var ResultSkipped Result = "skipped"
var ResultFailed Result = "failed"

type ExtensionTestResult struct {
	Name      string    `json:"name"`
	Lifecycle Lifecycle `json:"lifecycle"`
	Duration  int64     `json:"duration"`
	StartTime *DBTime   `json:"startTime"`
	EndTime   *DBTime   `json:"endTime"`
	Result    Result    `json:"result"`
	Output    string    `json:"output"`
	Error     string    `json:"error,omitempty"`
	Details   []Details `json:"details,omitempty"`

	// Source is the information from the extension binary (it's image tag, repo, commit sha, etc), reported
	// up by origin so it's easy to identify where a particular result came from in the overall combined result JSON.
	Source Source `json:"source"`
}

// Details are human-readable messages to further explain skips, timeouts, etc.
// It can also be used to provide contemporaneous information about failures
// that may not be easily returned by must-gather. For larger artifacts (greater than
// 10KB, write them to $EXTENSION_ARTIFACTS_DIR.
type Details struct {
	Name  string      `json:"name"`
	Value interface{} `json:"value"`
}

// DBTime is a type suitable for direct importing into databases like BigQuery,
// formatted like 2006-01-02 15:04:05.000000 UTC.
type DBTime time.Time

func TimePtr(t time.Time) *DBTime {
	return (*DBTime)(&t)
}

func Time(t *DBTime) time.Time {
	if t == nil {
		return time.Time{}
	}
	return time.Time(*t)
}

func (dbt *DBTime) MarshalJSON() ([]byte, error) {
	formattedTime := time.Time(*dbt).Format(`"2006-01-02 15:04:05.000000 UTC"`)
	return []byte(formattedTime), nil
}

func (dbt *DBTime) UnmarshalJSON(b []byte) error {
	timeStr := string(b[1 : len(b)-1])
	parsedTime, err := time.Parse("2006-01-02 15:04:05.000000 UTC", timeStr)
	if err != nil {
		return err
	}
	*dbt = (DBTime)(parsedTime)
	return nil
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
