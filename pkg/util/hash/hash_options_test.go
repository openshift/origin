package hash

import "testing"

func TestHashOptions(t *testing.T) {
	opts := NewHashOptions(NewSHA256Hasher(), true)
	hash := opts.Hash("mydata")
	salt, err := opts.Rand(32)
	if err != nil {
		t.Fatalf("Error generating hashes: %v", err)
	}
	saltedHash := opts.SaltedHash("mydata", salt)

	// Don't change this data. The output of the sha256 hasher must be stable release-to-release
	expectedHash := "Hu9RDYHupJFhzYIbMYqpmeYwvdKSsJOqmpMZ6fKCuYQ"
	if hash != expectedHash {
		t.Errorf("Expected %s, got %s for sha256 hash", expectedHash, hash)
	}

	if !opts.VerifyHash("mydata", hash) {
		t.Errorf("Got error or invalid verification of hash")
	}

	if !opts.VerifySaltedHash("mydata", salt, saltedHash) {
		t.Errorf("Got error or invalid verification of salted hash")
	}
}
