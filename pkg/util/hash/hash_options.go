package hash

import "crypto/rand"

type defaultHashOptions struct {
	Rander
	Hasher
	SaltedHasher
	HashEnablement
}

func NewHashOptions(hasher Hasher, hashOnWrite bool) HashOptions {
	return &defaultHashOptions{
		Rander:         NewRander(rand.Reader),
		Hasher:         hasher,
		SaltedHasher:   NewSaltedHasher(hasher),
		HashEnablement: StaticHashEnablement(hashOnWrite),
	}
}
