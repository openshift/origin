package externalbinary

import (
	"k8s.io/apimachinery/pkg/util/sets"
	"time"
)

type Lifecycle string

var LifecycleInforming Lifecycle = "informing"
var LifecycleBlocking Lifecycle = "blocking"

type ExtensionTestSpecs []*ExtensionTestSpec

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

	// Lifecycle informs the executor whether the test is informing only, and should not cause the
	// overall job run to fail, or if it's blocking where a failure of the test is fatal.
	// Informing lifecycle tests can be used temporarily to gather information about a test's stability.
	// Tests must not remain informing forever.
	Lifecycle Lifecycle `json:"lifecycle"`

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
