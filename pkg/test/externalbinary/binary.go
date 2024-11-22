package externalbinary

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/test/extended/util"
)

type externalBinaryStruct struct {
	// The payload image tag in which an external binary path can be found
	imageTag string
	// The binary path to extract from the image
	binaryPath string
}

var externalBinaries = []externalBinaryStruct{
	{
		imageTag:   "hyperkube",
		binaryPath: "/usr/bin/k8s-tests-ext.gz",
	},
}

// TestBinary is an abstraction around extracted test binaries that provides an interface for listing the available
// tests. In the future, it will implement the entire openshift-tests-extension interface.
type TestBinary struct {
	path   string
	logger *log.Logger
}

// ListTests returns which tests this binary advertises.  Eventually, it should take an environment struct
// to provide to the binary so it can determine for itself which tests are relevant.
func (b *TestBinary) ListTests(ctx context.Context) (ExtensionTestSpecs, error) {
	var tests ExtensionTestSpecs
	start := time.Now()
	binName := filepath.Base(b.path)

	b.logger.Printf("Listing tests for %q", binName)
	command := exec.Command(b.path, "list", "-o", "jsonl")
	testList, err := runWithTimeout(ctx, command, 10*time.Minute)
	if err != nil {
		return nil, fmt.Errorf("failed running '%s list': %w", b.path, err)
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
	b.logger.Printf("Listed %d tests for %q in %v", len(tests), binName, time.Since(start))
	return tests, nil
}

// RunTests executes the named tests and returns the results.
func (b *TestBinary) RunTests(ctx context.Context, timeout time.Duration, env []string,
	names ...string) []*ExtensionTestResult {
	var results []*ExtensionTestResult
	unseenTests := sets.New[string](names...)
	binName := filepath.Base(b.path)

	// Build command
	args := []string{"run-test"}
	for _, name := range names {
		args = append(args, "-n", name)
	}
	args = append(args, "-o", "jsonl")
	command := exec.Command(b.path, args...)
	if len(env) == 0 {
		env = os.Environ()
	}
	command.Env = env

	// Run test
	testResult, err := runWithTimeout(ctx, command, timeout)
	if err != nil {
		// If errored, generate failures for all tests and return
		for _, name := range names {
			results = append(results, &ExtensionTestResult{
				Name:   name,
				Result: ResultFailed,
				Error:  err.Error(),
			})
		}
		return results
	}
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
		if !unseenTests.Has(result.Name) {
			panic(fmt.Sprintf("test binary %q returned unexpected result: %s", binName, result.Name))
		}
		unseenTests.Delete(result.Name)
		results = append(results, result)
	}

	for _, unseenTest := range unseenTests.UnsortedList() {
		results = append(results, &ExtensionTestResult{
			Name:   unseenTest,
			Result: ResultFailed,
			Error:  "external binary did not produce a result for this test",
		})
	}

	return results
}

// ExtractAllTestBinaries determines the optimal release payload to use, and extracts all the external
// test binaries from it, and returns a slice of them.
func ExtractAllTestBinaries(ctx context.Context, logger *log.Logger, parallelism int) (func(), TestBinaries, error) {
	if parallelism < 1 {
		return nil, nil, errors.New("parallelism must be greater than zero")
	}

	releaseImage, err := determineReleasePayloadImage(logger)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "couldn't determine release image")
	}

	oc := util.NewCLIWithoutNamespace("default")
	registryAuthfilePath, err := getRegistryAuthFilePath(logger, oc)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "couldn't get registry auth file path")
	}

	externalBinaryProvider, err := NewExternalBinaryProvider(logger, releaseImage, registryAuthfilePath)
	if err != nil {
		return nil, nil, errors.WithMessage(err, "could not create external binary provider")
	}

	var (
		binaries []*TestBinary
		mu       sync.Mutex
		wg       sync.WaitGroup
		errCh    = make(chan error, len(externalBinaries))
		jobCh    = make(chan externalBinaryStruct)
	)

	// Producer: sends jobs to the jobCh channel
	go func() {
		defer close(jobCh)
		for _, b := range externalBinaries {
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
