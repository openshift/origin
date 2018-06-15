package ostree

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	"path/filepath"

	"github.com/containers/image/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sha256digest    = "@sha256:" + sha256digestHex
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "ostree", Transport.Name())
}

// A helper to replace $TMP in a repo path with a real temporary directory
func withTmpDir(repo string, tmpDir string) string {
	return strings.Replace(repo, "$TMP", tmpDir, -1)
}

// A common list of repo suffixes to test for the various ImageReference methods.
var repoSuffixes = []struct{ repoSuffix, resolvedRepo string }{
	{"", "/ostree/repo"},
	{"@/ostree/repo", "/ostree/repo"}, // /ostree/repo is accepted even if neither /ostree/repo nor /ostree exists, as a special case.
	{"@$TMP/at@sign@repo", "$TMP/at@sign@repo"},
	// Rejected as ambiguous: /repo:with:colons could either be an (/repo, with:colons) policy configuration identity, or a (/repo:with, colons) policy configuration namespace.
	{"@$TMP/repo:with:colons", ""},
}

// A common list of cases for image name parsing and normalization
var imageNameTestcases = []struct{ input, normalized, branchName string }{
	{"busybox:notlatest", "busybox:notlatest", "busybox_3Anotlatest"},                                                  // Explicit tag
	{"busybox", "busybox:latest", "busybox_3Alatest"},                                                                  // Default tag
	{"docker.io/library/busybox:latest", "docker.io/library/busybox:latest", "docker.io_2Flibrary_2Fbusybox_3Alatest"}, // A hierarchical name
	{"UPPERCASEISINVALID", "", ""},                                                                                     // Invalid input
	{"busybox" + sha256digest, "", ""},                                                                                 // Digested references are not supported (parsed as invalid repository name)
	{"busybox:invalid+tag", "", ""},                                                                                    // Invalid tag value
	{"busybox:tag:with:colons", "", ""},                                                                                // Multiple colons - treated as a tag which contains a colon, which is invalid
	{"", "", ""},                                                                                                       // Empty input is rejected (invalid repository.Named)
}

func TestTransportParseReference(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreeParseReference")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range imageNameTestcases {
		for _, suffix := range repoSuffixes {
			fullInput := c.input + withTmpDir(suffix.repoSuffix, tmpDir)
			ref, err := Transport.ParseReference(fullInput)
			if c.normalized == "" || suffix.resolvedRepo == "" {
				assert.Error(t, err, fullInput)
			} else {
				require.NoError(t, err, fullInput)
				ostreeRef, ok := ref.(ostreeReference)
				require.True(t, ok, fullInput)
				assert.Equal(t, c.normalized, ostreeRef.image, fullInput)
				assert.Equal(t, c.branchName, ostreeRef.branchName, fullInput)
				assert.Equal(t, withTmpDir(suffix.resolvedRepo, tmpDir), ostreeRef.repo, fullInput)
			}
		}
	}
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	for _, scope := range []string{
		"/etc:docker.io/library/busybox:notlatest", // This also demonstrates that two colons are interpreted as repo:name:tag.
		"/etc:docker.io/library/busybox",
		"/etc:docker.io/library",
		"/etc:docker.io",
		"/etc:repo",
		"/this/does/not/exist:notlatest",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	for _, scope := range []string{
		"/colon missing as a path-reference delimiter",
		"relative/path:busybox",
		"/double//slashes:busybox",
		"/has/./dot:busybox",
		"/has/dot/../dot:busybox",
		"/trailing/slash/:busybox",
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}

func TestNewReference(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreeNewReference")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range imageNameTestcases {
		for _, suffix := range repoSuffixes {
			if suffix.repoSuffix == "" {
				continue
			}
			caseName := c.input + suffix.repoSuffix
			ref, err := NewReference(c.input, withTmpDir(strings.TrimPrefix(suffix.repoSuffix, "@"), tmpDir))
			if c.normalized == "" || suffix.resolvedRepo == "" {
				assert.Error(t, err, caseName)
			} else {
				require.NoError(t, err, caseName)
				ostreeRef, ok := ref.(ostreeReference)
				require.True(t, ok, caseName)
				assert.Equal(t, c.normalized, ostreeRef.image, caseName)
				assert.Equal(t, c.branchName, ostreeRef.branchName, caseName)
				assert.Equal(t, withTmpDir(suffix.resolvedRepo, tmpDir), ostreeRef.repo, caseName)
			}
		}
	}

	for _, path := range []string{
		"/",
		"/etc",
		tmpDir,
		"relativepath",
		tmpDir + "/thisdoesnotexist",
	} {
		_, err := NewReference("busybox", path)
		require.NoError(t, err, path)
	}

	_, err = NewReference("busybox", tmpDir+"/thisparentdoesnotexist/something")
	assert.Error(t, err)
}

// A common list of reference formats to test for the various ImageReference methods.
var validReferenceTestCases = []struct{ input, stringWithinTransport, policyConfigurationIdentity string }{
	{"busybox", "busybox:latest@/ostree/repo", "/ostree/repo:busybox:latest"},                                                         // Everything implied
	{"busybox:latest@/ostree/repo", "busybox:latest@/ostree/repo", "/ostree/repo:busybox:latest"},                                     // All implied values explicitly specified
	{"example.com/ns/foo:bar@$TMP/non-DEFAULT", "example.com/ns/foo:bar@$TMP/non-DEFAULT", "$TMP/non-DEFAULT:example.com/ns/foo:bar"}, // All values explicitly specified, a hierarchical name
	// A non-canonical path. Testing just one, the various other cases are tested in explicitfilepath.ResolvePathToFullyExplicit.
	{"busybox@$TMP/.", "busybox:latest@$TMP", "$TMP:busybox:latest"},
	// "/" as a corner case
	{"busybox@/", "busybox:latest@/", "/:busybox:latest"},
}

func TestReferenceTransport(t *testing.T) {
	ref, err := Transport.ParseReference("busybox")
	require.NoError(t, err)
	assert.Equal(t, Transport, ref.Transport())
}

func TestReferenceStringWithinTransport(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreeStringWithinTransport")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range validReferenceTestCases {
		ref, err := Transport.ParseReference(withTmpDir(c.input, tmpDir))
		require.NoError(t, err, c.input)
		stringRef := ref.StringWithinTransport()
		assert.Equal(t, withTmpDir(c.stringWithinTransport, tmpDir), stringRef, c.input)
		// Do one more round to verify that the output can be parsed, to an equal value.
		ref2, err := Transport.ParseReference(stringRef)
		require.NoError(t, err, c.input)
		stringRef2 := ref2.StringWithinTransport()
		assert.Equal(t, stringRef, stringRef2, c.input)
	}
}

func TestReferenceDockerReference(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreeDockerReference")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range validReferenceTestCases {
		ref, err := Transport.ParseReference(withTmpDir(c.input, tmpDir))
		require.NoError(t, err, c.input)
		dockerRef := ref.DockerReference()
		assert.Nil(t, dockerRef, c.input)
	}
}

func TestReferencePolicyConfigurationIdentity(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreePolicyConfigurationIdentity")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	for _, c := range validReferenceTestCases {
		ref, err := Transport.ParseReference(withTmpDir(c.input, tmpDir))
		require.NoError(t, err, c.input)
		assert.Equal(t, withTmpDir(c.policyConfigurationIdentity, tmpDir), ref.PolicyConfigurationIdentity(), c.input)
	}
}

func TestReferencePolicyConfigurationNamespaces(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreePolicyConfigurationNamespaces")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Test both that DockerReferenceIdentity returns the expected value (fullName+suffix),
	// and that DockerReferenceNamespaces starts with the expected value (fullName), i.e. that the two functions are
	// consistent.
	for inputName, expectedNS := range map[string][]string{
		"example.com/ns/repo": {"example.com/ns/repo", "example.com/ns", "example.com"},
		"example.com/repo":    {"example.com/repo", "example.com"},
		"localhost/ns/repo":   {"localhost/ns/repo", "localhost/ns", "localhost"},
		"localhost/repo":      {"localhost/repo", "localhost"},
		"ns/repo":             {"ns/repo", "ns"},
		"repo":                {"repo"},
	} {
		// Test with a known path which should exist. Test just one non-canonical
		// path, the various other cases are tested in explicitfilepath.ResolvePathToFullyExplicit.
		for _, repoInput := range []string{tmpDir, tmpDir + "/./."} {
			fullName := inputName + ":notlatest"
			ref, err := NewReference(fullName, repoInput)
			require.NoError(t, err, fullName)

			identity := ref.PolicyConfigurationIdentity()
			assert.Equal(t, tmpDir+":"+expectedNS[0]+":notlatest", identity, fullName)

			ns := ref.PolicyConfigurationNamespaces()
			require.NotNil(t, ns, fullName)
			require.Len(t, ns, len(expectedNS), fullName)
			moreSpecific := identity
			for i := range expectedNS {
				assert.Equal(t, tmpDir+":"+expectedNS[i], ns[i], fmt.Sprintf("%s item %d", fullName, i))
				assert.True(t, strings.HasPrefix(moreSpecific, ns[i]))
				moreSpecific = ns[i]
			}
		}
	}
}

func TestReferenceNewImage(t *testing.T) {
	ref, err := Transport.ParseReference("busybox")
	require.NoError(t, err)
	_, err = ref.NewImage(nil)
	assert.Error(t, err)
}

func TestReferenceNewImageSource(t *testing.T) {
	ref, err := Transport.ParseReference("busybox")
	require.NoError(t, err)
	_, err = ref.NewImageSource(nil, nil)
	assert.Error(t, err)
}

func TestReferenceNewImageDestination(t *testing.T) {
	otherTmpDir, err := ioutil.TempDir("", "ostree-transport-test")
	require.NoError(t, err)
	defer os.RemoveAll(otherTmpDir)

	for _, c := range []struct {
		ctx    *types.SystemContext
		tmpDir string
	}{
		{nil, os.TempDir()},
		{&types.SystemContext{}, os.TempDir()},
		{&types.SystemContext{OSTreeTmpDirPath: otherTmpDir}, otherTmpDir},
	} {
		ref, err := Transport.ParseReference("busybox")
		require.NoError(t, err)
		dest, err := ref.NewImageDestination(c.ctx)
		require.NoError(t, err)
		ostreeDest, ok := dest.(*ostreeImageDestination)
		require.True(t, ok)
		assert.Equal(t, c.tmpDir+"/busybox_3Alatest", ostreeDest.tmpDirPath)
		defer dest.Close()
	}
}

func TestReferenceDeleteImage(t *testing.T) {
	tmpDir, err := ioutil.TempDir("", "ostreeDeleteImage")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ref, err := Transport.ParseReference(withTmpDir("busybox@$TMP/this-repo-does-not-exist", tmpDir))
	require.NoError(t, err)
	err = ref.DeleteImage(nil)
	assert.Error(t, err)
}

func TestEncodeOSTreeRef(t *testing.T) {
	// Just a smoke test
	assert.Equal(t, "busybox_3Alatest", encodeOStreeRef("busybox:latest"))
}

func TestReferenceManifestPath(t *testing.T) {
	ref, err := Transport.ParseReference("busybox")
	require.NoError(t, err)
	ostreeRef, ok := ref.(ostreeReference)
	require.True(t, ok)
	assert.Equal(t, fmt.Sprintf("manifest%cmanifest.json", filepath.Separator), ostreeRef.manifestPath())
}

func TestReferenceSignaturePath(t *testing.T) {
	ref, err := Transport.ParseReference("busybox")
	require.NoError(t, err)
	ostreeRef, ok := ref.(ostreeReference)
	require.True(t, ok)
	for _, c := range []struct {
		input  int
		suffix string
	}{
		{0, "-1"},
		{42, "-43"},
	} {
		assert.Equal(t, fmt.Sprintf("manifest%csignature%s", filepath.Separator, c.suffix), ostreeRef.signaturePath(c.input), string(c.input))
	}
}
