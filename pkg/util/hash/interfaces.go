package hash

type Rander interface {
	Rand(len int) (string, error)
}

type Hasher interface {
	Hash(plaintext string) string
	VerifyHash(plaintext, hash string) bool
	VerifyHashFormat(hash string) bool
}

type SaltedHasher interface {
	SaltedHash(plaintext, salt string) string
	VerifySaltedHash(plaintext, salt, hash string) bool
	VerifySaltedHashFormat(hash string) bool
}

type HashEnablement interface {
	HashOnWrite() bool
}

type HashOptions interface {
	Rander
	Hasher
	SaltedHasher
	HashEnablement
}
