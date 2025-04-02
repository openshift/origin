package k8s

import (
	"fmt"
	"hash/fnv"

	"k8s.io/apimachinery/pkg/util/rand"
)

func Hash(s string) string {
	hasher := fnv.New32a()
	hasher.Write([]byte(s))
	return rand.SafeEncodeString(fmt.Sprint(hasher.Sum32()))
}
