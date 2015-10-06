package client

import (
	"regexp"
	"testing"

	"github.com/openshift/origin/pkg/diagnostics/types"
)

const fakeBuildLog = `
I1002 14:37:05.078169       1 sti.go:149] Using provided push secret for pushing 172.30.150.249:5000/test/nodejs-ex:latest image
I1002 14:37:05.078184       1 sti.go:151] Pushing 172.30.150.249:5000/test/nodejs-ex:latest image ...
F1002 14:37:06.981434       1 builder.go:54] Build error: Failed to push image: Error pushing to registry: Server error: unexpected 500 response status trying to initiate upload of test/nodejs-ex
I1002 14:37:05.078184       1 sti.go:151] This is some other stuff I made up.
F1002 14:37:05.078184       1 sti.go:151] WOW this looks really bad.
F1002 14:37:05.078184       1 sti.go:151] WOW this looks really bad. And it happened again, only warn me once.
`

// compares the expected error keys against the given errors/warnings.
func compareKeys(t *testing.T, errors []types.DiagnosticError, expErrKeys []string) {
	for _, expKey := range expErrKeys {
		found := false
		for _, diagErr := range errors {
			if diagErr.ID == expKey {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Unable to locate expected error key: %s", expKey)
		}
	}
}

func assertResults(t *testing.T, r types.DiagnosticResult, expErrKeys []string, expWarnKeys []string) {
	if len(r.Errors()) != len(expErrKeys) {
		t.Errorf("Expected %d errors, got %d.", len(expErrKeys), len(r.Errors()))
	}
	if len(r.Warnings()) != len(expWarnKeys) {
		t.Errorf("Expected %d warnings, got %d.", len(expWarnKeys), len(r.Warnings()))
	}
	compareKeys(t, r.Errors(), expErrKeys)
	compareKeys(t, r.Warnings(), expWarnKeys)
}

func TestReportsRegistrySelinuxProblem(t *testing.T) {
	r := types.NewDiagnosticResult(LatestBuildsName)
	needles := BuildNeedles()
	scanBuildLog(r, fakeBuildLog, needles)
	assertResults(t, r, []string{}, []string{registrySelinuxErrorKey})
}

func TestReportsMultipleProblems(t *testing.T) {
	r := types.NewDiagnosticResult(LatestBuildsName)
	needles := BuildNeedles()

	// Add an additional needle to look for, it will match twice but we want to only be warned once:
	regex, _ := regexp.Compile(".*WOW this looks really bad.*")
	fakeNeedle := &LogNeedle{
		ErrorKey:  "FAKE001",
		SearchFor: regex,
		Warning:   "Something looks really bad.",
	}
	needles = append(needles, fakeNeedle)

	scanBuildLog(r, fakeBuildLog, needles)
	assertResults(t, r, []string{}, []string{registrySelinuxErrorKey, "FAKE001"})
}
