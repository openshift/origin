package extensions

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
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

func NewExternalBinaryProvider(releaseImage, registryAuthfilePath string) (*ExternalBinaryProvider,
	error) {
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

	// Allow overriding image path to an already existing local path, mostly useful
	// for development.
	if override := binaryPathOverride(tag, binary); override != "" {
		logrus.WithFields(logrus.Fields{
			"tag":      tag,
			"binary":   binary,
			"override": override,
		}).Info("Found override for this extension")
		return &TestBinary{
			imageTag:   tag,
			BinaryPath: override,
		}, nil
	}

	// Resolve the image tag from the image stream.
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

	// Define the path for the binary
	binPath := filepath.Join(provider.binPath, strings.TrimSuffix(filepath.Base(binary), ".gz"))

	// Check if the binary already exists in the path
	if _, err := os.Stat(binPath); err == nil {
		logrus.Infof("Using existing binary %s for tag %s", binPath, tag)
		return &TestBinary{
			imageTag:   tag,
			BinaryPath: binPath,
		}, nil
	}

	// Start the extraction process.
	startTime := time.Now()
	if err := runImageExtract(image, binary, provider.binPath, provider.registryAuthFilePath); err != nil {
		return nil, fmt.Errorf("failed extracting %q from %q: %w", binary, image, err)
	}
	extractDuration := time.Since(startTime)

	extractedBinary := filepath.Join(provider.binPath, filepath.Base(binary))

	// Verify that the extracted binary exists; "oc extract image" doesn't error when the path doesn't exist,
	// so we check that the extraction was successful via its existence
	_, err := os.Stat(extractedBinary)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("extracted binary at path %q does not exist. the src path %q doesn't exist in image %q. note the version of origin needs to match the version of the cluster under test",
				extractedBinary, binary, image)
		}
		return nil, fmt.Errorf("failed to stat external binary to check for existence %q: %w", binary, err)
	}

	// Support gzipped external binaries (handle decompression).
	extractedBinary, err = ungzipFile(extractedBinary)
	if err != nil {
		return nil, fmt.Errorf("failed to decompress external binary %q: %w", binary, err)
	}

	// Make the extracted binary executable.
	if err := os.Chmod(extractedBinary, 0755); err != nil {
		return nil, fmt.Errorf("failed making the extracted binary %q executable: %w", extractedBinary, err)
	}

	// Verify the binary actually exists
	fileInfo, err := os.Stat(extractedBinary)
	if err != nil {
		return nil, fmt.Errorf("failed stat on extracted binary %q: %w", extractedBinary, err)
	}

	// Verify the binary is compatible with our architecture
	if err := checkCompatibleArchitecture(extractedBinary); err != nil {
		return nil, errors.WithMessage(err, "error checking binary architecture compatability")
	}

	logrus.Infof("Extracted %s for tag %s from %s (disk size %v, extraction duration %v)",
		binary, tag, image, fileInfo.Size(), extractDuration)

	return &TestBinary{
		BinaryPath: extractedBinary,
	}, nil
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
	safeEnvVar := strings.NewReplacer("/", "_", "-", "_", ".", "_")

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
