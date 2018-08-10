// +build !containers_image_storage_stub

package storage

import (
	"fmt"
	"testing"

	"github.com/containers/image/docker/reference"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sha256digestHex = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	sha256Digest2   = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
)

func TestTransportName(t *testing.T) {
	assert.Equal(t, "containers-storage", Transport.Name())
}

func TestTransportParseStoreReference(t *testing.T) {
	const digest3 = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

	store := newStore(t)

	Transport.SetStore(nil)
	for _, c := range []struct{ input, expectedRef, expectedID string }{
		{"", "", ""}, // Empty input
		// Handling of the store prefix
		// FIXME? Should we be silently discarding input like this?
		{"[unterminated", "", ""},                                    // Unterminated store specifier
		{"[garbage]busybox", "docker.io/library/busybox:latest", ""}, // Store specifier is overridden by the store we pass to ParseStoreReference

		{"UPPERCASEISINVALID", "", ""},                                                   // Invalid single-component name
		{"sha256:" + sha256digestHex, "docker.io/library/sha256:" + sha256digestHex, ""}, // Valid single-component name; the hex part is not an ID unless it has a "@" prefix, so it looks like a tag
		// FIXME: This test is now incorrect, this should not fail _if the image ID matches_
		{sha256digestHex, "", ""},                    // Invalid single-component ID; not an ID without a "@" prefix, so it's parsed as a name, but names aren't allowed to look like IDs
		{"@" + sha256digestHex, "", sha256digestHex}, // Valid single-component ID
		{"@sha256:" + sha256digestHex, "", ""},       // Invalid un-named @digest
		// "aaaa", either a valid image ID prefix, or a short form of docker.io/library/aaaa, untested
		{"sha256:ab", "docker.io/library/sha256:ab", ""},                                   // Valid single-component name, explicit tag
		{"busybox", "docker.io/library/busybox:latest", ""},                                // Valid single-component name, implicit tag
		{"busybox:notlatest", "docker.io/library/busybox:notlatest", ""},                   // Valid single-component name, explicit tag
		{"docker.io/library/busybox:notlatest", "docker.io/library/busybox:notlatest", ""}, // Valid single-component name, everything explicit

		{"UPPERCASEISINVALID@" + sha256digestHex, "", ""},                                                // Invalid name in name@digestOrID
		{"busybox@ab", "", ""},                                                                           // Invalid ID in name@digestOrID
		{"busybox@", "", ""},                                                                             // Empty ID in name@digestOrID
		{"busybox@sha256:ab", "", ""},                                                                    // Invalid digest in name@digestOrID
		{"busybox@sha256:" + sha256digestHex, "docker.io/library/busybox@sha256:" + sha256digestHex, ""}, // Valid name@digest, no tag
		{"busybox@" + sha256digestHex, "docker.io/library/busybox:latest", sha256digestHex},              // Valid name@ID, implicit tag
		// "busybox@aaaa", a valid image ID prefix, untested
		{"busybox:notlatest@" + sha256digestHex, "docker.io/library/busybox:notlatest", sha256digestHex},                     // Valid name@ID, explicit tag
		{"docker.io/library/busybox:notlatest@" + sha256digestHex, "docker.io/library/busybox:notlatest", sha256digestHex},   // Valid name@ID, everything explicit
		{"docker.io/library/busybox:notlatest@" + sha256Digest2, "docker.io/library/busybox:notlatest@" + sha256Digest2, ""}, // Valid name:tag@digest, everything explicit

		{"busybox@sha256:" + sha256digestHex + "@ab", "", ""},                                                                                                                       // Invalid ID in name@digest@ID
		{"busybox@ab@" + sha256digestHex, "", ""},                                                                                                                                   // Invalid digest in name@digest@ID
		{"busybox@@" + sha256digestHex, "", ""},                                                                                                                                     // Invalid digest in name@digest@ID
		{"busybox@" + sha256Digest2 + "@" + sha256digestHex, "docker.io/library/busybox@" + sha256Digest2, sha256digestHex},                                                         // name@digest@ID
		{"docker.io/library/busybox@" + sha256Digest2 + "@" + sha256digestHex, "docker.io/library/busybox@" + sha256Digest2, sha256digestHex},                                       // name@digest@ID, everything explicit
		{"docker.io/library/busybox:notlatest@sha256:" + sha256digestHex + "@" + sha256digestHex, "docker.io/library/busybox:notlatest@sha256:" + sha256digestHex, sha256digestHex}, // name:tag@digest@ID, everything explicit
		// "busybox@sha256:"+sha256digestHex+"@aaaa", a valid image ID prefix, untested
		{"busybox:notlatest@" + sha256Digest2 + "@" + digest3 + "@" + sha256digestHex, "", ""}, // name@digest@ID, with name containing another digest
	} {
		storageRef, err := Transport.ParseStoreReference(store, c.input)
		if c.expectedRef == "" && c.expectedID == "" {
			assert.Error(t, err, c.input)
		} else {
			require.NoError(t, err, c.input)
			assert.Equal(t, store, storageRef.transport.store, c.input)
			if c.expectedRef == "" {
				assert.Nil(t, storageRef.named, c.input)
			} else {
				dockerRef, err := reference.ParseNormalizedNamed(c.expectedRef)
				require.NoError(t, err)
				require.NotNil(t, storageRef.named, c.input)
				assert.Equal(t, dockerRef.String(), storageRef.named.String())
			}
			assert.Equal(t, c.expectedID, storageRef.id, c.input)
		}
	}
}

func TestTransportParseReference(t *testing.T) {
	store := newStore(t)
	driver := store.GraphDriverName()
	root := store.GraphRoot()

	for _, c := range []struct{ prefix, expectedDriver, expectedRoot, expectedRunRoot string }{
		{"", driver, root, ""},                                             // Implicit store location prefix
		{"[unterminated", "", "", ""},                                      // Unterminated store specifier
		{"[]", "", "", ""},                                                 // Empty store specifier
		{"[relative/path]", "", "", ""},                                    // Non-absolute graph root path
		{"[" + driver + "@relative/path]", "", "", ""},                     // Non-absolute graph root path
		{"[@" + root + "suffix2]", "", "", ""},                             // Empty graph driver
		{"[" + driver + "@]", "", "", ""},                                  // Empty root path
		{"[thisisunknown@" + root + "suffix2]", "", "", ""},                // Unknown graph driver
		{"[" + root + "suffix1]", "", "", ""},                              // A valid root path, but no run dir
		{"[" + driver + "@" + root + "suffix3+relative/path]", "", "", ""}, // Non-absolute run dir
		{"[" + driver + "@" + root + "suffix3+" + root + "suffix4]",
			driver,
			root + "suffix3",
			root + "suffix4"}, // A valid root@graph+run set
		{"[" + driver + "@" + root + "suffix3+" + root + "suffix4:options,options,options]",
			driver,
			root + "suffix3",
			root + "suffix4"}, // A valid root@graph+run+options set
	} {
		t.Logf("parsing %q", c.prefix+"busybox")
		ref, err := Transport.ParseReference(c.prefix + "busybox")
		if c.expectedDriver == "" {
			assert.Error(t, err, c.prefix)
		} else {
			require.NoError(t, err, c.prefix)
			storageRef, ok := ref.(*storageReference)
			require.True(t, ok, c.prefix)
			assert.Equal(t, c.expectedDriver, storageRef.transport.store.GraphDriverName(), c.prefix)
			assert.Equal(t, c.expectedRoot, storageRef.transport.store.GraphRoot(), c.prefix)
			if c.expectedRunRoot != "" {
				assert.Equal(t, c.expectedRunRoot, storageRef.transport.store.RunRoot(), c.prefix)
			}
		}
	}
}

func TestTransportValidatePolicyConfigurationScope(t *testing.T) {
	store := newStore(t)
	driver := store.GraphDriverName()
	root := store.GraphRoot()
	storeSpec := fmt.Sprintf("[%s@%s]", driver, root) // As computed in PolicyConfigurationNamespaces

	// Valid inputs
	for _, scope := range []string{
		"[" + root + "suffix1]",                                                                    // driverlessStoreSpec in PolicyConfigurationNamespaces
		"[" + driver + "@" + root + "suffix3]",                                                     // storeSpec in PolicyConfigurationNamespaces
		storeSpec + "@" + sha256digestHex,                                                          // ID only
		storeSpec + "docker.io",                                                                    // Host name only
		storeSpec + "docker.io/library",                                                            // A repository namespace
		storeSpec + "docker.io/library/busybox",                                                    // A repository name
		storeSpec + "docker.io/library/busybox:notlatest",                                          // name:tag
		storeSpec + "docker.io/library/busybox:notlatest@" + sha256digestHex,                       // name@ID
		storeSpec + "docker.io/library/busybox@" + sha256Digest2,                                   // name@digest
		storeSpec + "docker.io/library/busybox@" + sha256Digest2 + "@" + sha256digestHex,           // name@digest@ID
		storeSpec + "docker.io/library/busybox:notlatest@" + sha256Digest2,                         // name:tag@digest
		storeSpec + "docker.io/library/busybox:notlatest@" + sha256Digest2 + "@" + sha256digestHex, // name:tag@digest@ID
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.NoError(t, err, scope)
	}

	// Invalid inputs
	for _, scope := range []string{
		"busybox",                        // Unprefixed reference
		"[unterminated",                  // Unterminated store specifier
		"[]",                             // Empty store specifier
		"[relative/path]",                // Non-absolute graph root path
		"[" + driver + "@relative/path]", // Non-absolute graph root path
		// "[thisisunknown@" + root + "suffix2]", // Unknown graph driver FIXME: validate against storage.ListGraphDrivers() once that's available
		storeSpec + "@", // An incomplete two-component name

		storeSpec + "docker.io/library/busybox@sha256:ab",                    // Invalid digest in name@digest
		storeSpec + "docker.io/library/busybox@ab",                           // Invalid ID in name@ID
		storeSpec + "docker.io/library/busybox@",                             // Empty ID/digest in name@ID
		storeSpec + "docker.io/library/busybox@@" + sha256digestHex,          // Empty digest in name@digest@ID
		storeSpec + "docker.io/library/busybox@ab@" + sha256digestHex,        // Invalid digest in name@digest@ID
		storeSpec + "docker.io/library/busybox@sha256:ab@" + sha256digestHex, // Invalid digest in name@digest@ID
		storeSpec + "docker.io/library/busybox@" + sha256Digest2 + "@",       // Empty ID in name@digest@ID
		storeSpec + "docker.io/library/busybox@" + sha256Digest2 + "@ab",     // Invalid ID in name@digest@ID
	} {
		err := Transport.ValidatePolicyConfigurationScope(scope)
		assert.Error(t, err, scope)
	}
}
