package hash

import "testing"

func TestSHA256(t *testing.T) {
	h := NewSHA256Hasher()

	expectedHash := "kKSQG99BVWYDEFBLjAKC53To7K0IUCmtPTVCRYRTeSI"
	// Don't change this data, this ensures sha256 outputs hashes old servers will recognize
	if hash := h.Hash("45b14bbe49b670e99c54aa4c122df23c26ad0865cc543d5b7ad6571415eb4abc2222222222-2222222222-2222222222-2222222222"); hash != expectedHash {
		t.Errorf("SHA256 hash returned unexpected hash: %v", hash)
	}

	// Don't change this data, this ensures old sha256 hashes can continue to be validated
	if !h.VerifyHash("45b14bbe49b670e99c54aa4c122df23c26ad0865cc543d5b7ad6571415eb4abc2222222222-2222222222-2222222222-2222222222", "kKSQG99BVWYDEFBLjAKC53To7K0IUCmtPTVCRYRTeSI") {
		t.Errorf("Got error or invalid verification of sha256 hash")
	}
}
