package extensions

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	kapierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/test/extended/util"
)

// TestBinary implements the openshift-tests extension interface (Info, ListTests, RunTests, etc).
type TestBinary struct {
	// The payload image tag in which an external binary path can be found
	imageTag string
	// The binary path to extract from the image
	binaryPath string
}

var extensionBinaries = []TestBinary{
	{
		imageTag:   "hyperkube",
		binaryPath: "/usr/bin/k8s-tests-ext.gz",
	},
}

// Info returns information about this particular extension.
func (b *TestBinary) Info(ctx context.Context) (*ExtensionInfo, error) {
	var info ExtensionInfo
	start := time.Now()
	binName := filepath.Base(b.binaryPath)

	logrus.Infof("Fetching info for %s", binName)
	command := exec.Command(b.binaryPath, "info")
	infoJson, err := runWithTimeout(ctx, command, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s info': %w", b.binaryPath, err)
	}
	err = json.Unmarshal(infoJson, &info)
	if err != nil {
		return nil, errors.Wrapf(err, "couldn't unmarshal extension info")
	}
	info.Source.SourceBinary = binName
	info.Source.SourceImage = b.imageTag
	logrus.Infof("Fetched info for %s %v", binName, time.Since(start))
	return &info, nil
}

// ListTests returns which tests this binary advertises.  Eventually, it should take an environment struct
// to provide to the binary so it can determine for itself which tests are relevant.
func (b *TestBinary) ListTests(ctx context.Context) (ExtensionTestSpecs, error) {
	var tests ExtensionTestSpecs
	start := time.Now()
	binName := filepath.Base(b.binaryPath)

	logrus.Infof("Listing tests for %s", binName)
	command := exec.Command(b.binaryPath, "list", "-o", "jsonl")
	testList, err := runWithTimeout(ctx, command, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s list': %w", b.binaryPath, err)
	}
	buf := bytes.NewBuffer(testList)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if !strings.HasPrefix(line, "{") {
			continue
		}

		extensionTestSpec := new(ExtensionTestSpec)
		err = json.Unmarshal([]byte(line), extensionTestSpec)
		if err != nil {
			return nil, errors.Wrapf(err, "line: %s", line)
		}
		extensionTestSpec.Binary = b
		tests = append(tests, extensionTestSpec)
	}
	logrus.Infof("Listed %d tests for %s in %v", len(tests), binName, time.Since(start))
	return tests, nil
}

// RunTests executes the named tests and returns the results.
func (b *TestBinary) RunTests(ctx context.Context, timeout time.Duration, env []string,
	names ...string) []*ExtensionTestResult {
	var results []*ExtensionTestResult
	expectedTests := sets.New[string](names...)
	binName := filepath.Base(b.binaryPath)

	// Build command
	args := []string{"run-test"}
	for _, name := range names {
		args = append(args, "-n", name)
	}
	args = append(args, "-o", "jsonl")
	command := exec.Command(b.binaryPath, args...)
	if len(env) == 0 {
		env = os.Environ()
	}
	command.Env = env

	// Run test
	testResult, _ := runWithTimeout(ctx, command, timeout) // error is ignored because external binaries return non-zero when a test fails, we only need to process the output
	buf := bytes.NewBuffer(testResult)
	for {
		line, err := buf.ReadString('\n')
		if err == io.EOF {
			break
		}
		if !strings.HasPrefix(line, "{") {
			continue
		}
		result := new(ExtensionTestResult)
		err = json.Unmarshal([]byte(line), &result)
		if err != nil {
			panic(fmt.Sprintf("test binary %q returned unmarshallable result", binName))
		}
		// expectedTests starts with the list of test names we expect, and as we see them, we
		// remove them from the set. If we encounter a test result that's not in expectedTests,
		// then it means either:
		//  - we already saw a result for this test, which breaks the invariant that run-test
		//    returns one result for each test
		//  - we got a test result we didn't expect at all (maybe the external binary improperly
		//    mutated the name, or otherwise did something weird)
		if !expectedTests.Has(result.Name) {
			result.Result = ResultFailed
			result.Error = fmt.Sprintf("test binary %q returned unexpected result: %s", binName, result.Name)
		}
		expectedTests.Delete(result.Name)
		results = append(results, result)
	}

	// If we end up with anything left in expected tests, generate failures for them because
	// we didn't get results for them.
	for _, expectedTest := range expectedTests.UnsortedList() {
		results = append(results, &ExtensionTestResult{
			Name:   expectedTest,
			Result: ResultFailed,
			Output: string(testResult),
			Error:  "external binary did not produce a result for this test",
		})
	}

	return results
}

// ExtractAllTestBinaries determines the optimal release payload to use, and extracts all the external
// test binaries from it, and returns a slice of them.
func ExtractAllTestBinaries(ctx context.Context, parallelism int) (func(), TestBinaries, error) {
	if parallelism < 1 {
		return nil, nil, errors.New("parallelism must be greater than zero")
	}

	releaseImage, err := determineReleasePayloadImage()
	if err != nil {
		return nil, nil, errors.WithMessage(err, "couldn't determine release image")
	}

	oc := util.NewCLIWithoutNamespace("default")

	// To extract binaries bearing external tests, we must inspect the release
	// payload under tests as well as extract content from component images
	// referenced by that payload.
	// openshift-tests is frequently run in the context of a CI job, within a pod.
	// CI sets $RELEASE_IMAGE_LATEST to a pullspec for the release payload under test. This
	// pull spec resolve to:
	// 1. A build farm ci-op-* namespace / imagestream location (anonymous access permitted).
	// 2. A quay.io/openshift-release-dev location (for tests against promoted ART payloads -- anonymous access permitted).
	// 3. A registry.ci.openshift.org/ocp-<arch>/release:<tag> (request registry.ci.openshift.org token).
	// Within the pod, we don't necessarily have a pull-secret for #3 OR the component images
	// a payload references (which are private, unless in a ci-op-* imagestream).
	// We try the following options:
	// 1. If set, use the REGISTRY_AUTH_FILE environment variable to an auths file with
	//    pull secrets capable of reading appropriate payload & component image
	//    information.
	// 2. If it exists, use a file /run/secrets/ci.openshift.io/cluster-profile/pull-secret
	//    (conventional location for pull-secret information for CI cluster profile).
	// 3. Use openshift-config secret/pull-secret from the cluster-under-test, if it exists
	//    (Microshift does not).
	// 4. Use unauthenticated access to the payload image and component images.
	registryAuthFilePath := os.Getenv("REGISTRY_AUTH_FILE")

	// if the environment variable is not set, extract the target cluster's
	// platform pull secret.
	if len(registryAuthFilePath) != 0 {
		logrus.Infof("Using REGISTRY_AUTH_FILE environment variable: %v", registryAuthFilePath)
	} else {

		// See if the cluster-profile has stored a pull-secret at the conventional location.
		ciProfilePullSecretPath := "/run/secrets/ci.openshift.io/cluster-profile/pull-secret"
		_, err := os.Stat(ciProfilePullSecretPath)
		if !os.IsNotExist(err) {
			logrus.Infof("Detected %v; using cluster profile for image access", ciProfilePullSecretPath)
			registryAuthFilePath = ciProfilePullSecretPath
		} else {
			// Inspect the cluster-under-test and read its cluster pull-secret dockerconfigjson value.
			clusterPullSecret, err := oc.AdminKubeClient().CoreV1().Secrets("openshift-config").Get(context.Background(), "pull-secret", metav1.GetOptions{})
			if err != nil {
				if kapierrs.IsNotFound(err) {
					logrus.Warningf("Cluster has no openshift-config secret/pull-secret; falling back to unauthenticated image access")
				} else {
					return nil, nil, fmt.Errorf("unable to read ephemeral cluster pull secret: %w", err)
				}
			} else {
				tmpDir, err := os.MkdirTemp("", "external-binary")
				clusterDockerConfig := clusterPullSecret.Data[".dockerconfigjson"]
				registryAuthFilePath = filepath.Join(tmpDir, ".dockerconfigjson")
				err = os.WriteFile(registryAuthFilePath, clusterDockerConfig, 0600)
				if err != nil {
					return nil, nil, fmt.Errorf("unable to serialize target cluster pull-secret locally: %w", err)
				}

				defer os.RemoveAll(tmpDir)
				logrus.Infof("Using target cluster pull-secrets for registry auth")
			}
		}
	}

	externalBinaryProvider, err := NewExternalBinaryProvider(releaseImage, registryAuthFilePath)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "could not create external binary provider")
	}

	var (
		binaries []*TestBinary
		mu       sync.Mutex
		wg       sync.WaitGroup
		errCh    = make(chan error, len(extensionBinaries))
		jobCh    = make(chan TestBinary)
	)

	// Producer: sends jobs to the jobCh channel
	go func() {
		defer close(jobCh)
		for _, b := range extensionBinaries {
			select {
			case <-ctx.Done():
				return // Exit if context is cancelled
			case jobCh <- b:
			}
		}
	}()

	// Consumer workers: extract test binaries concurrently
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return // Context is cancelled
				case b, ok := <-jobCh:
					if !ok {
						return // Channel is closed
					}
					testBinary, err := externalBinaryProvider.ExtractBinaryFromReleaseImage(b.imageTag, b.binaryPath)
					if err != nil {
						errCh <- err
						continue
					}
					mu.Lock()
					binaries = append(binaries, testBinary)
					mu.Unlock()
				}
			}

		}()
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errCh)

	// Check if any errors were reported
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		externalBinaryProvider.Cleanup()
		return nil, nil, fmt.Errorf("encountered errors while extracting binaries: %s", strings.Join(errs, ";"))
	}

	return externalBinaryProvider.Cleanup, binaries, nil
}

type TestBinaries []*TestBinary

// Info fetches teh info from all TestBinaries using the specified parallelism.
func (binaries TestBinaries) Info(ctx context.Context, parallelism int) ([]*ExtensionInfo, error) {
	var (
		infos []*ExtensionInfo
		mu    sync.Mutex
		wg    sync.WaitGroup
		errCh = make(chan error, len(binaries))
		jobCh = make(chan *TestBinary)
	)

	// Producer: sends jobs to the jobCh channel
	go func() {
		defer close(jobCh)
		for _, binary := range binaries {
			select {
			case <-ctx.Done():
				return // Exit when context is cancelled
			case jobCh <- binary:
			}
		}
	}()

	// Consumer workers: extract tests concurrently
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return // Exit when context is cancelled
				case binary, ok := <-jobCh:
					if !ok {
						return // Channel was closed
					}
					info, err := binary.Info(ctx)
					if err != nil {
						errCh <- err
					}
					mu.Lock()
					infos = append(infos, info)
					mu.Unlock()
				}
			}
		}()
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errCh)

	// Check if any errors were reported
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("encountered errors while fetch info: %s", strings.Join(errs, ";"))
	}

	return infos, nil
}

// ListTests extracts the tests from all TestBinaries using the specified parallelism.
func (binaries TestBinaries) ListTests(ctx context.Context, parallelism int) (ExtensionTestSpecs, error) {
	var (
		allTests ExtensionTestSpecs
		mu       sync.Mutex
		wg       sync.WaitGroup
		errCh    = make(chan error, len(binaries))
		jobCh    = make(chan *TestBinary)
	)

	// Producer: sends jobs to the jobCh channel
	go func() {
		defer close(jobCh)
		for _, binary := range binaries {
			select {
			case <-ctx.Done():
				return // Exit when context is cancelled
			case jobCh <- binary:
			}
		}
	}()

	// Consumer workers: extract tests concurrently
	for i := 0; i < parallelism; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return // Exit when context is cancelled
				case binary, ok := <-jobCh:
					if !ok {
						return // Channel was closed
					}
					tests, err := binary.ListTests(ctx)
					if err != nil {
						errCh <- err
					}
					mu.Lock()
					allTests = append(allTests, tests...)
					mu.Unlock()
				}
			}
		}()
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errCh)

	// Check if any errors were reported
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("encountered errors while listing tests: %s", strings.Join(errs, ";"))
	}

	return allTests, nil
}

func runWithTimeout(ctx context.Context, c *exec.Cmd, timeout time.Duration) ([]byte, error) {
	if timeout > 0 {
		go func() {
			select {
			// interrupt tests after timeout, and abort if they don't complete quick enough
			case <-time.After(timeout):
				if c.Process != nil {
					c.Process.Signal(syscall.SIGINT)
				}
				// if the process appears to be hung a significant amount of time after the timeout
				// send an ABRT so we get a stack dump
				select {
				case <-time.After(time.Minute):
					if c.Process != nil {
						c.Process.Signal(syscall.SIGABRT)
					}
				}
			case <-ctx.Done():
				if c.Process != nil {
					c.Process.Signal(syscall.SIGINT)
				}
			}

		}()
	}
	return c.CombinedOutput()
}
