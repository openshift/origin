package hash

import "testing"

func TestSaltedHasher(t *testing.T) {
	h := NewSaltedHasher(NewSHA256Hasher())

	// Don't change this data, this ensures old salted hashes can continue to be validated
	expectedHash := "kKSQG99BVWYDEFBLjAKC53To7K0IUCmtPTVCRYRTeSI"
	if hash := h.SaltedHash("2222222222-2222222222-2222222222-2222222222", "45b14bbe49b670e99c54aa4c122df23c26ad0865cc543d5b7ad6571415eb4abc"); hash != expectedHash {
		t.Errorf("Salted hasher errored or returned unexpected hash: %v", hash)
	}
	// Don't change this data, this ensures old salted hashes can continue to be validated
	if !h.VerifySaltedHash("2222222222-2222222222-2222222222-2222222222", "45b14bbe49b670e99c54aa4c122df23c26ad0865cc543d5b7ad6571415eb4abc", "kKSQG99BVWYDEFBLjAKC53To7K0IUCmtPTVCRYRTeSI") {
		t.Errorf("Got error or invalid verification of salted hash")
	}

	// Invalid case
	if h.VerifySaltedHash("plaintext", "salt", "kKSQG99BVWYDEFBLjAKC53To7K0IUCmtPTVCRYRTeSI") {
		t.Errorf("Expected !ok")
	}
}
