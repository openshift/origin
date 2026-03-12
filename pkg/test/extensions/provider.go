package extensions

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/openshift/origin/test/extended/util"
)

// ExternalBinaryProvider handles extracting external test binaries from a given payload. By
// default, it uses a cache directory for extracted binaries assuming they'll be reused,
// especially when developing locally. Set OPENSHIFT_TESTS_DISABLE_CACHE to any non-empty
// value to use a temporary directory instead that will be removed at end of execution. When
// using caching, files older than 7 days will be removed.
type ExternalBinaryProvider struct {
	oc                   *util.CLI
	binPath              string
	tmpDir               string
	registryAuthFilePath string
	imageStream          *imagev1.ImageStream
}

func NewExternalBinaryProvider(releaseImage, registryAuthfilePath string) (*ExternalBinaryProvider, error) {
	oc := util.NewCLIWithoutNamespace("default")

	// Use a fixed cache or tmp directory for storing binaries
	tmpDir := ""
	binDir := pullSpecToDirName(releaseImage)
	if len(os.Getenv("OPENSHIFT_TESTS_DISABLE_CACHE")) == 0 {
		// Determine cache path
		cacheBase := os.Getenv("XDG_CACHE_HOME")
		if cacheBase == "" {
			cacheBase = path.Join(os.Getenv("HOME"), ".cache", "openshift-tests")
		}
		cleanOldCacheFiles(cacheBase)
		binDir = path.Join(cacheBase, binDir)
		logrus.WithField("cache_dir", cacheBase).Infof("External binary cache is enabled")
	} else {
		logrus.Infof("External binary cache is disabled, using a temp directory instead")
		var err error
		tmpDir, err = os.MkdirTemp("", "openshift-tests")
		if err != nil {
			return nil, errors.Wrap(err, "couldn't create temp directory")
		}
		binDir = path.Join(tmpDir, binDir)
	}
	logrus.Infof("Using path for binaries %s", binDir)

	if err := createBinPath(binDir); err != nil {
		return nil, errors.WithMessagef(err, "error creating cache path %s", binDir)
	}

	releasePayloadImageStream, releaseImage, err := ExtractReleaseImageStream(binDir, releaseImage, registryAuthfilePath)
	if err != nil {
		return nil, errors.WithMessage(err, "couldn't extract release payload image stream")
	}

	return &ExternalBinaryProvider{
		registryAuthFilePath: registryAuthfilePath,
		oc:                   oc,
		imageStream:          releasePayloadImageStream,
		binPath:              binDir,
		tmpDir:               tmpDir,
	}, nil
}

func (provider *ExternalBinaryProvider) Cleanup() {
	if provider.tmpDir != "" {
		if err := os.RemoveAll(provider.tmpDir); err != nil {
			logrus.Errorf("Failed to remove tmpDir %s: %v", provider.tmpDir, err)
		} else {
			logrus.Infof("Successfully removed tmpDir %s", provider.tmpDir)
		}
	}

	provider.tmpDir = ""
	provider.binPath = ""
}

// extractBinary handles the common extraction logic with file locking and caching.
// It extracts binaryPath from imageRef into targetDir, ungzips, makes executable, and validates architecture.
func (provider *ExternalBinaryProvider) extractBinary(imageRef, binaryPath, targetDir, imageTag string) (extractedBinary string, extractDuration time.Duration, err error) {
	// Define the final path for the binary (without .gz extension)
	finalBinPath := filepath.Join(targetDir, strings.TrimSuffix(filepath.Base(binaryPath), ".gz"))

	// Acquire a file lock to prevent concurrent extraction of the same binary.
	// This is necessary when multiple openshift-tests processes share the same cache directory.
	lockPath := finalBinPath + ".lock"
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create lock file %q: %w", lockPath, err)
	}
	defer lockFile.Close()

	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX); err != nil {
		return "", 0, fmt.Errorf("failed to acquire lock on %q: %w", lockPath, err)
	}
	defer syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)

	// Check if the binary already exists in cache
	if _, err := os.Stat(finalBinPath); err == nil {
		// Revalidate architecture compatibility before returning cached binary
		if err := checkCompatibleArchitecture(finalBinPath); err != nil {
			logrus.Warnf("Cached binary %s for %s is incompatible with current architecture, removing: %v", finalBinPath, imageTag, err)
			if removeErr := os.Remove(finalBinPath); removeErr != nil {
				logrus.Warnf("Failed to remove incompatible cached binary %s: %v", finalBinPath, removeErr)
			}
			// Continue with normal extraction flow
		} else {
			logrus.Infof("Using existing binary %s for %s", finalBinPath, imageTag)
			return finalBinPath, 0, nil
		}
	}

	// Start the extraction process
	startTime := time.Now()
	if err := runImageExtract(imageRef, binaryPath, targetDir, provider.registryAuthFilePath); err != nil {
		return "", 0, fmt.Errorf("failed extracting %q from %q: %w", binaryPath, imageRef, err)
	}
	extractDuration = time.Since(startTime)

	extractedBinary = filepath.Join(targetDir, filepath.Base(binaryPath))

	// Verify that the extracted binary exists; "oc extract image" doesn't error when the path doesn't exist
	if _, err := os.Stat(extractedBinary); err != nil {
		if os.IsNotExist(err) {
			return "", 0, fmt.Errorf("extracted binary at path %q does not exist in image %q", extractedBinary, imageRef)
		}
		return "", 0, fmt.Errorf("failed to stat extracted binary %q: %w", extractedBinary, err)
	}

	// Support gzipped external binaries (handle decompression)
	extractedBinary, err = ungzipFile(extractedBinary)
	if err != nil {
		return "", 0, fmt.Errorf("failed to decompress external binary %q: %w", binaryPath, err)
	}

	// Make the extracted binary executable
	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return "", 0, fmt.Errorf("failed making the binary %q executable: %w", extractedBinary, err)
	}

	// Verify the binary is compatible with our architecture
	if err := checkCompatibleArchitecture(extractedBinary); err != nil {
		return "", 0, errors.WithMessage(err, "error checking binary architecture compatability")
	}

	return extractedBinary, extractDuration, nil
}

// ExtractBinaryFromReleaseImage resolves the tag from the release image and extracts the binary,
// checking if the binary is compatible with the current systems' architecture. It returns an error
// if extraction fails or if the binary is incompatible.
//
// Note: When developing openshift-tests on a non-Linux non-AMD64 computer (i.e. on Apple Silicon), external
// binaries won't work.  You would need to run it in a Linux environment (VM or container), and even then
// override the payload selection with an aarch64 payload unless x86 emulation is enabled.
func (provider *ExternalBinaryProvider) ExtractBinaryFromReleaseImage(tag, binary string) (*TestBinary, error) {
	if provider.binPath == "" {
		return nil, fmt.Errorf("extraction path is not set, cleanup was already run")
	}

	// Allow overriding image path to an already existing local path, mostly useful for development
	if override := binaryPathOverride(tag, binary); override != "" {
		logrus.WithFields(logrus.Fields{
			"tag":      tag,
			"binary":   binary,
			"override": override,
		}).Info("Found override for this extension")
		return &TestBinary{
			imageTag:   tag,
			binaryPath: override,
		}, nil
	}

	// Resolve the image tag from the image stream
	image := ""
	for _, t := range provider.imageStream.Spec.Tags {
		if t.Name == tag {
			image = t.From.Name
			break
		}
	}

	if len(image) == 0 {
		return nil, fmt.Errorf("%s not found", tag)
	}

	// Extract the binary using common logic
	extractedBinary, extractDuration, err := provider.extractBinary(image, binary, provider.binPath, tag)
	if err != nil {
		return nil, err
	}

	// Log extraction details (file size only if we actually extracted)
	if extractDuration > 0 {
		if fileInfo, statErr := os.Stat(extractedBinary); statErr == nil {
			logrus.Infof("Extracted %s for tag %s from %s (disk size %v, extraction duration %v)",
				binary, tag, image, fileInfo.Size(), extractDuration)
		}
	}

	return &TestBinary{
		imageTag:   tag,
		binaryPath: extractedBinary,
	}, nil
}

// ExtractBinaryFromImage extracts a binary from an arbitrary image (e.g. from a non-payload ImageStreamTag).
// imageRef is the pull spec; binaryPath is the path inside the image; imageTag is the identifier for the
// TestBinary (e.g. namespace/imagestream:tag for non-payload). Uses the same lock/cache/ungzip/arch check as ExtractBinaryFromReleaseImage.
func (provider *ExternalBinaryProvider) ExtractBinaryFromImage(imageRef, binaryPath, imageTag string) (*TestBinary, error) {
	if provider.binPath == "" {
		return nil, fmt.Errorf("extraction path is not set, cleanup was already run")
	}

	// Allow overriding image path to an already existing local path, mostly useful for development
	if override := binaryPathOverride(imageTag, binaryPath); override != "" {
		logrus.WithFields(logrus.Fields{
			"tag":      imageTag,
			"binary":   binaryPath,
			"override": override,
		}).Info("Found override for this extension")
		return &TestBinary{
			imageTag:   imageTag,
			binaryPath: override,
		}, nil
	}

	// Create a subdirectory for non-payload binaries to keep them separate
	nonPayloadDir := filepath.Join(provider.binPath, "non-payload", safeNameForDir(imageTag))
	if err := createBinPath(nonPayloadDir); err != nil {
		return nil, errors.WithMessagef(err, "error creating cache path %s", nonPayloadDir)
	}

	// Extract the binary using common logic
	extractedBinary, extractDuration, err := provider.extractBinary(imageRef, binaryPath, nonPayloadDir, imageTag)
	if err != nil {
		return nil, err
	}

	// Log extraction details
	if extractDuration > 0 {
		logrus.Infof("Extracted %s for %s from %s in %v", binaryPath, imageTag, imageRef, extractDuration)
	}

	return &TestBinary{
		imageTag:   imageTag,
		binaryPath: extractedBinary,
	}, nil
}

func safeNameForDir(s string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(s)
}

func cleanOldCacheFiles(dir string) {
	maxAge := 24 * 7 * time.Hour // 7 days
	logrus.Infof("Cleaning up older cached data...")
	entries, err := os.ReadDir(dir)
	if err != nil {
		logrus.Warningf("Failed to read cache directory '%s': %v", dir, err)
		return
	}

	start := time.Now()
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil || start.Sub(info.ModTime()) < maxAge {
			continue
		}

		tgtPath := filepath.Join(dir, entry.Name())
		if err := os.RemoveAll(tgtPath); err != nil {
			logrus.Errorf("Failed to remove cache file '%s': %v", tgtPath, err)
		} else {
			logrus.Infof("Removed old cache file '%s'", tgtPath)
		}
	}
	logrus.Infof("Cleaned up old cached data in %v", time.Since(start))
}

func binaryPathOverride(imageTag, binaryPath string) string {
	safeEnvVar := strings.NewReplacer("/", "_", "-", "_", ".", "_", ":", "_")

	// Check for a specific override for this binary path, less common but allows supporting
	// images that have multiple test binaries.
	// 	Example: EXTENSION_BINARY_OVERRIDE_HYPERKUBE_USR_BIN_K8S_TESTS_EXT_GZ
	specificOverrideEnvVar := fmt.Sprintf("EXTENSION_BINARY_OVERRIDE_%s_%s",
		strings.ToUpper(safeEnvVar.Replace(imageTag)),
		strings.ToUpper(safeEnvVar.Replace(strings.TrimPrefix(binaryPath, "/"))),
	)
	if specificOverride := os.Getenv(specificOverrideEnvVar); specificOverride != "" {
		return specificOverride
	}

	// Check for a global override for all binaries in this image
	// 	Example: EXTENSION_BINARY_OVERRIDE_HYPERKUBE
	return os.Getenv(fmt.Sprintf("EXTENSION_BINARY_OVERRIDE_%s", strings.ToUpper(safeEnvVar.Replace(imageTag))))
}
