package extensions

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
)

// binaryKey uniquely identifies a test binary by its image and path.
type binaryKey struct {
	imageTag   string
	binaryPath string
}

// binaryResult holds the detection results for a single binary.
type binaryResult struct {
	binaryName      string // Just the filename, not the full path
	globalLocations []string
	totalTests      int
}

// imageResult groups all binary results for a single image.
type imageResult struct {
	imageTag string
	binaries []binaryResult
}

// CheckForGlobalNodes checks if any code locations appear in ALL tests for each binary,
// which indicates that BeforeEach/AfterEach nodes were registered at the global level
// (root of the Ginkgo tree). This is a serious bug because these hooks run for EVERY test,
// wasting resources and time. In some cases, these global nodes can
// interfere with test operations.
//
// Tests are grouped by their source image, and each binary within an image is checked separately.
//
// Returns JUnit test cases as flakes (failing + passing with same name) for each image
// that has global nodes detected. This allows the issue to be tracked in CI without
// blocking the test run for now.
func CheckForGlobalNodes(specs ExtensionTestSpecs) []*junitapi.JUnitTestCase {
	// Group tests by image tag and binary path
	specsByBinary := make(map[binaryKey]ExtensionTestSpecs)
	for _, spec := range specs {
		if spec.Binary == nil {
			continue
		}
		key := binaryKey{
			imageTag:   spec.Binary.imageTag,
			binaryPath: spec.Binary.binaryPath,
		}
		specsByBinary[key] = append(specsByBinary[key], spec)
	}

	// Check each binary for global nodes, grouped by image
	resultsByImage := make(map[string][]binaryResult)

	for key, binarySpecs := range specsByBinary {
		// Skip binaries with fewer than 25 tests - can't detect global nodes meaningfully
		// with small test counts as it would generate false positives
		if len(binarySpecs) < 25 {
			continue
		}

		totalTests := len(binarySpecs)

		// Count how many unique tests contain each code location
		locationToTests := make(map[string]map[string]struct{})
		for _, spec := range binarySpecs {
			for _, loc := range spec.CodeLocations {
				if locationToTests[loc] == nil {
					locationToTests[loc] = make(map[string]struct{})
				}
				locationToTests[loc][spec.Name] = struct{}{}
			}
		}

		// Find code locations that appear in ALL tests for this binary
		var globalLocations []string
		for loc, tests := range locationToTests {
			if len(tests) != totalTests {
				continue
			}

			// Skip locations that are expected to be in all tests
			if isExpectedGlobalLocation(loc) {
				continue
			}

			globalLocations = append(globalLocations, loc)
		}

		if len(globalLocations) > 0 {
			sort.Strings(globalLocations)
			// Use just the binary filename, not the full extracted path
			binaryName := filepath.Base(key.binaryPath)
			resultsByImage[key.imageTag] = append(resultsByImage[key.imageTag], binaryResult{
				binaryName:      binaryName,
				globalLocations: globalLocations,
				totalTests:      totalTests,
			})
		}
	}

	if len(resultsByImage) == 0 {
		return nil
	}

	// Build sorted list of image results
	var imageResults []imageResult
	for imageTag, binaries := range resultsByImage {
		// Sort binaries within each image for consistent output
		sort.Slice(binaries, func(i, j int) bool {
			return binaries[i].binaryName < binaries[j].binaryName
		})
		imageResults = append(imageResults, imageResult{
			imageTag: imageTag,
			binaries: binaries,
		})
	}

	// Sort by imageTag for consistent output
	sort.Slice(imageResults, func(i, j int) bool {
		return imageResults[i].imageTag < imageResults[j].imageTag
	})

	// Create JUnit test cases as flakes (one failing, one passing per image)
	var testCases []*junitapi.JUnitTestCase

	for _, result := range imageResults {
		testName := fmt.Sprintf("[sig-ci] image %s should not have global BeforeEach/AfterEach nodes", result.imageTag)

		// Build detailed failure message
		failureOutput := buildGlobalNodeFailureMessage(result.imageTag, result.binaries)

		// Count total global locations across all binaries
		totalLocations := 0
		for _, b := range result.binaries {
			totalLocations += len(b.globalLocations)
		}

		// Create failing test case
		failingCase := &junitapi.JUnitTestCase{
			Name: testName,
			FailureOutput: &junitapi.FailureOutput{
				Message: fmt.Sprintf("Found %d global BeforeEach/AfterEach code locations in image %s across %d binary(ies)", totalLocations, result.imageTag, len(result.binaries)),
				Output:  failureOutput,
			},
		}
		testCases = append(testCases, failingCase)

		// Create passing test case (same name = flake)
		passingCase := &junitapi.JUnitTestCase{
			Name: testName,
		}
		testCases = append(testCases, passingCase)
	}

	return testCases
}

// buildGlobalNodeFailureMessage creates a detailed message explaining the global node issue.
func buildGlobalNodeFailureMessage(imageTag string, binaries []binaryResult) string {
	var sb strings.Builder

	sb.WriteString("\n")
	sb.WriteString("╔══════════════════════════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║                    GLOBAL BEFOREEACH/AFTEREACH DETECTED                      ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════════════════════════╝\n")
	sb.WriteString("\n")
	sb.WriteString(fmt.Sprintf("IMAGE: %s\n", imageTag))
	sb.WriteString("\n")

	for _, binary := range binaries {
		sb.WriteString(fmt.Sprintf("BINARY: %s (%d tests)\n", binary.binaryName, binary.totalTests))
		sb.WriteString("GLOBAL CODE LOCATIONS:\n")
		for _, loc := range binary.globalLocations {
			sb.WriteString(fmt.Sprintf("  • %s\n", loc))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("PROBLEM: The code locations above appear in ALL tests for their respective binaries,\n")
	sb.WriteString("indicating that BeforeEach or AfterEach hooks were registered at the global level.\n")
	sb.WriteString("\n")
	sb.WriteString("This means these hooks run for EVERY SINGLE TEST even when not needed,\n")
	sb.WriteString("wasting CI resources and adding unnecessary test execution time.\n")
	sb.WriteString("\n")
	sb.WriteString("COMMON CAUSES:\n")
	sb.WriteString("\n")
	sb.WriteString("1. Package-level FixturePath() call:\n")
	sb.WriteString("   BAD:  var myFixture = exutil.FixturePath(\"testdata\", \"file.yaml\")\n")
	sb.WriteString("   GOOD: func myFixture() string { return exutil.FixturePath(\"testdata\", \"file.yaml\") }\n")
	sb.WriteString("\n")
	sb.WriteString("2. Package-level NewCLI/NewCLIWithoutNamespace call:\n")
	sb.WriteString("   BAD:  var oc = exutil.NewCLIWithoutNamespace(\"test\")\n")
	sb.WriteString("   GOOD: Inside g.Describe(): oc := exutil.NewCLIWithoutNamespace(\"test\")\n")
	sb.WriteString("\n")
	sb.WriteString("HOW TO FIX:\n")
	sb.WriteString("\n")
	sb.WriteString("1. Find the file and line number from the locations above\n")
	sb.WriteString("2. Look for package-level var declarations that call FixturePath() or NewCLI*()\n")
	sb.WriteString("3. Convert them to functions (for FixturePath) or move inside Describe blocks (for CLI)\n")
	sb.WriteString("\n")
	sb.WriteString("EXAMPLE FIX FOR FixturePath:\n")
	sb.WriteString("\n")
	sb.WriteString("  // BEFORE (causes global BeforeEach):\n")
	sb.WriteString("  var myFixture = exutil.FixturePath(\"testdata\", \"file.yaml\")\n")
	sb.WriteString("  func useFixture() { oc.Run(\"apply\").Args(\"-f\", myFixture) }\n")
	sb.WriteString("\n")
	sb.WriteString("  // AFTER (lazy evaluation, no global BeforeEach):\n")
	sb.WriteString("  func myFixture() string { return exutil.FixturePath(\"testdata\", \"file.yaml\") }\n")
	sb.WriteString("  func useFixture() { oc.Run(\"apply\").Args(\"-f\", myFixture()) }\n")
	sb.WriteString("\n")
	sb.WriteString("FOR AI ASSISTANTS:\n")
	sb.WriteString("If you're Claude or another AI helping fix this, search the codebase for the file\n")
	sb.WriteString("paths shown above. Look for package-level 'var' declarations that call\n")
	sb.WriteString("exutil.FixturePath() or exutil.NewCLI*(). Convert FixturePath vars to functions\n")
	sb.WriteString("that return the path. Move NewCLI* calls inside Ginkgo Describe/Context blocks.\n")
	sb.WriteString("\n")

	return sb.String()
}

// allowedGlobalLocations is an exception list of code locations that are known
// to register global BeforeEach/AfterEach nodes. These should be fixed
// unless it is intentional.
var allowedGlobalLocations = map[string]string{
	// Add exceptions here with an explanation, e.g.:
	// "github.com/openshift/origin/test/extended/networking/util.go:82": "FIXME: package-level FixturePath call",
}

// isExpectedGlobalLocation returns true for code locations that are expected
// to appear in all tests and should not trigger the global node detection.
func isExpectedGlobalLocation(loc string) bool {
	// Check exact match in allowlist
	if _, ok := allowedGlobalLocations[loc]; ok {
		return true
	}

	// Check pattern matches for framework infrastructure that's legitimately global
	expectedPatterns := []string{
		// None currently - if we find legitimate cases, add them here with comments
	}

	for _, pattern := range expectedPatterns {
		if strings.Contains(loc, pattern) {
			return true
		}
	}

	return false
}
