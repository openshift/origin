package ginkgo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/origin/test/extended/util"
)

type serializedTest struct {
	Name   string
	Labels string
}

// externalTestsForSuite reads tests from external binaries
func externalTestsForSuite(ctx context.Context, externalTests []string) ([]*testCase, error) {
	var binaries []TestBinary
	for _, suite := range externalTests {
		parts := strings.Split(suite, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid suite %q", suite)
		}
		testType := parts[0]

		config := strings.Split(parts[1], ":")
		if len(config) != 2 {
			return nil, fmt.Errorf("invalid test %q", suite)
		}

		imageTag := config[0]
		path := config[1]

		switch testType {
		case "gingko":
			extractedBinary, err := extractGinkgoRunnerBinaryFromReleaseImage(imageTag, path)
			if err != nil {
				return nil, fmt.Errorf("failed to extract ginkgo binary from %q: %v", path, err)
			}
			binaries = append(binaries, extractedBinary)
		case "gotest":
			extractedBinaries, err := extractGoTestRunnerBinariesFromReleaseImage(imageTag, path)
			if err != nil {
				return nil, fmt.Errorf("failed to extract ginkgo binary from %q: %v", path, err)
			}
			for _, extractedBinary := range extractedBinaries {
				binaries = append(binaries, extractedBinary)
			}
		default:
			return nil, fmt.Errorf("invalid test %q", suite)
		}
	}

	var tests []*testCase
	for _, binary := range binaries {
		testCases, err := binary.ListTests(ctx)
		if err != nil {
			return nil, err
		}
		tests = append(tests, testCases...)
	}
	return tests, nil
}

// extractGinkgoRunnerBinaryFromReleaseImage is responsible for resolving the tag from
// release image and extracting binary, returns path to the binary or error
func extractGinkgoRunnerBinaryFromReleaseImage(tag, binary string) (GinkgoTestBinary, error) {
	tmpDir, err := extractPathFromReleaseImage(tag, binary)
	if err != nil {
		return "", fmt.Errorf("failed extracting %q from %q: %w", binary, tag, err)
	}

	extractedBinary := filepath.Join(tmpDir, filepath.Base(binary))
	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return "", fmt.Errorf("failed making the extracted binary executable: %w", err)
	}
	return GinkgoTestBinary(extractedBinary), nil
}

// extractGoTestRunnerBinariesFromReleaseImage is responsible for resolving the tag from
// release image and extracting go test binaries, returns path to the binary or error.
// The expectation is that a tree of directories containing test binaries and their package names
// exists:
// /tests/
// ├── a
// │ ├── b
// │ ├── b.package.txt
// │ ├── b.test
// │ ├── c
// │ │ ├── d
// │ │ ├── d.package.txt
// │ │ └── d.test
// │ ├── c.package.txt
// │ └── c.test
// ├── a.package.txt
// └── a.test
func extractGoTestRunnerBinariesFromReleaseImage(tag, path string) ([]GoTestBinary, error) {
	tmpDir, err := extractPathFromReleaseImage(tag, path)
	if err != nil {
		return nil, fmt.Errorf("failed extracting %q from %q: %w", path, tag, err)
	}

	var tests []GoTestBinary
	if err := filepath.WalkDir(tmpDir, func(path string, d fs.DirEntry, err error) error {
		if !d.IsDir() {
			return nil
		}
		name := filepath.Base(path)
		testBinary := filepath.Join(tmpDir, path, name+".test")
		if _, err := os.Stat(testBinary); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to search for test binary in %q: %w", path, err)
		}
		if err := os.Chmod(testBinary, 0755); err != nil {
			return fmt.Errorf("failed making the extracted path %q executable: %w", testBinary, err)
		}

		packageFile := filepath.Join(tmpDir, path, name+".package.txt")
		if _, err := os.Stat(packageFile); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to search for package file in %q: %w", path, err)
		}

		packageContents, err := os.ReadFile(packageFile)
		if err != nil {
			return fmt.Errorf("failed to read package file in %q: %w", packageFile, err)
		}

		tests = append(tests, GoTestBinary{
			binary: testBinary,
			module: string(bytes.TrimSpace(packageContents)),
		})

		return nil
	}); err != nil {
		return nil, fmt.Errorf("failed walking %q: %w", path, err)
	}
	return tests, nil
}

// extractPathFromReleaseImage is responsible for resolving the tag from
// release image and extracting the path, returning the local path for the extraction or an error
func extractPathFromReleaseImage(tag, path string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "release")
	if err != nil {
		return "", fmt.Errorf("cannot create temporary directory for extracted path: %w", err)
	}

	oc := util.NewCLIWithoutNamespace("default")
	cv, err := oc.AdminConfigClient().ConfigV1().ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed reading ClusterVersion/version: %w", err)
	}
	releaseImage := cv.Status.Desired.Image
	if len(releaseImage) == 0 {
		return "", fmt.Errorf("cannot determine release image from ClusterVersion resource")
	}

	if err := runImageExtract(releaseImage, "/release-manifests/image-references", tmpDir); err != nil {
		return "", fmt.Errorf("failed extracting image-references: %w", err)
	}
	jsonFile, err := os.Open(filepath.Join(tmpDir, "image-references"))
	if err != nil {
		return "", fmt.Errorf("failed reading image-references: %w", err)
	}
	defer jsonFile.Close()
	data, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return "", fmt.Errorf("unable to load release image-references: %w", err)
	}
	is := &imagev1.ImageStream{}
	if err := json.Unmarshal(data, &is); err != nil {
		return "", fmt.Errorf("unable to load release image-references: %w", err)
	}
	if is.Kind != "ImageStream" || is.APIVersion != "image.openshift.io/v1" {
		return "", fmt.Errorf("unrecognized image-references in release payload")
	}

	image := ""
	for _, t := range is.Spec.Tags {
		if t.Name == tag {
			image = t.From.Name
			break
		}
	}
	if len(image) == 0 {
		return "", fmt.Errorf("%s not found", tag)
	}
	if err := runImageExtract(image, path, tmpDir); err != nil {
		return "", fmt.Errorf("failed extracting %q from %q: %w", path, image, err)
	}

	return tmpDir, nil
}

// runImageExtract extracts src from specified image to dst
func runImageExtract(image, src, dst string) error {
	cmd := exec.Command("oc", "--kubeconfig="+util.KubeConfigPath(), "image", "extract", image, fmt.Sprintf("--path=%s:%s", src, dst), "--confirm")
	return cmd.Run()
}

type TestBinary interface {
	ListTests(ctx context.Context) ([]*testCase, error)
	RunTest(ctx context.Context, timeout time.Duration, env []string, testName string) (TestState, []byte)
}

type GinkgoTestBinary string

func (testBinary GinkgoTestBinary) ListTests(ctx context.Context) ([]*testCase, error) {
	var tests []*testCase
	command := exec.Command(string(testBinary), "list")
	testList, err := runWithTimeout(ctx, command, 1*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s list': %w", testBinary, err)
	}
	buf := bytes.NewBuffer(testList)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if !strings.HasPrefix(line, "[{") {
			continue
		}
		var serializedTests []serializedTest
		err = json.Unmarshal([]byte(line), &serializedTests)
		if err != nil {
			return nil, err
		}
		for _, test := range serializedTests {
			tests = append(tests, &testCase{
				name:           test.Name + test.Labels,
				rawName:        test.Name,
				externalBinary: testBinary,
			})
		}
	}
	return tests, nil
}

func (testBinary GinkgoTestBinary) RunTest(ctx context.Context, timeout time.Duration, env []string, testName string) (TestState, []byte) {
	command := exec.Command(string(testBinary), "run-test", testName)
	command.Env = append(os.Environ(), updateEnvVars(env)...)

	testOutputBytes, err := runWithTimeout(ctx, command, timeout)
	if err == nil {
		return TestSucceeded, testOutputBytes
	}

	if ctx.Err() != nil {
		return TestSkipped, testOutputBytes
	}

	state := TestFailed
	if exitErr, ok := err.(*exec.ExitError); ok {
		switch exitErr.ProcessState.Sys().(syscall.WaitStatus).ExitStatus() {
		case 1:
			// failed
			state = TestFailed
			break
		case 2:
			// timeout (ABRT is an exit code 2)
			state = TestFailedTimeout
			break
		case 3:
			// skipped
			state = TestSkipped
			break
		case 4:
			// flaky, do not retry
			state = TestFlaked
			break
		default:
			state = TestUnknown
			break
		}
	}

	return state, testOutputBytes
}

var _ TestBinary = (*GinkgoTestBinary)(nil)

type GoTestBinary struct {
	binary string
	module string
}

func (testBinary GoTestBinary) ListTests(ctx context.Context) ([]*testCase, error) {
	var tests []*testCase
	command := exec.Command(testBinary.binary, "test.list", ".")
	testList, err := runWithTimeout(ctx, command, 1*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s list': %w", testBinary, err)
	}
	buf := bytes.NewBuffer(testList)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		tests = append(tests, &testCase{
			name:           line,
			rawName:        line,
			externalBinary: testBinary,
		})
	}
	return tests, nil
}

// TestEvent is the output of go tool test2json
type TestEvent struct {
	Time    time.Time // encodes as an RFC3339-format string
	Action  string
	Package string
	Test    string
	Elapsed float64 // seconds
	Output  string
}

func (testBinary GoTestBinary) RunTest(ctx context.Context, timeout time.Duration, env []string, testName string) (TestState, []byte) {
	command := exec.Command(
		"go", "tool", "test2json", "-p", testBinary.module, "-t",
		string(testBinary.binary), "-test.run", fmt.Sprintf(`^%s$`, testName), "-test.v=test2json",
	)
	command.Env = append(os.Environ(), updateEnvVars(env)...)

	testOutputBytes, err := runWithTimeout(ctx, command, timeout)
	// err will be non-nil (exit code 1) when a test fails, but the output will be parseable anyway, so attempt that first

	state := TestFailed
	var output []byte
	buf := bytes.NewBuffer(testOutputBytes)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		var event TestEvent
		err = json.Unmarshal([]byte(line), &event)
		if err != nil {
			return TestFailed, []byte(fmt.Errorf("failed unmarshalling test event from output %s: %w", string(testOutputBytes), err).Error())
		}

		// TODO: lots of events are possible here, and it's possible more than one test runs, etc
		// what do we do?
		if event.Action == "output" && event.Test == testName {
			output = append(output, []byte(event.Output)...)
			continue
		}

		switch event.Action {
		case "pass":
			state = TestSucceeded
		case "fail":
			state = TestFailed
		case "skip":
			state = TestSkipped
		}
	}

	if err != nil && len(output) == 0 {
		output = []byte(err.Error())
	}

	return state, output
}

var _ TestBinary = (*GoTestBinary)(nil)
