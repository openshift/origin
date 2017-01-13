package hash

func NewSaltedHasher(h Hasher) SaltedHasher {
	return &saltedHasher{h}
}

type saltedHasher struct {
	h Hasher
}

func (s *saltedHasher) SaltedHash(plaintext, salt string) string {
	return s.h.Hash(salt + plaintext)
}

func (s *saltedHasher) VerifySaltedHash(plaintext, salt, hash string) bool {
	return s.h.VerifyHash(salt+plaintext, hash)
}

func (s *saltedHasher) VerifySaltedHashFormat(hash string) bool {
	return s.h.VerifyHashFormat(hash)
}
