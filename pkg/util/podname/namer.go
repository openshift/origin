package podname

import (
	"fmt"
	"hash/fnv"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

// GetName returns a pod name given a base ("deployment-5") and a suffix ("deploy")
// It will first attempt to join them with a dash. If the resulting name is longer
// than a valid pod name, it will truncate the base name and add an 8-character hash
// of the [base]-[suffix] string.
func GetName(base, suffix string) string {
	name := fmt.Sprintf("%s-%s", base, suffix)
	if len(name) > util.DNS1123SubdomainMaxLength {
		prefix := base[0:min(len(base), util.DNS1123SubdomainMaxLength-9)]
		// Calculate hash on initial base-suffix string
		name = fmt.Sprintf("%s-%s", prefix, hash(name))
	}
	return name
}

// min returns the lesser of its 2 inputs
func min(a, b int) int {
	if b < a {
		return b
	}
	return a
}

// hash calculates the hexadecimal representation (8-chars)
// of the hash of the passed in string using the FNV-a algorithm
func hash(s string) string {
	hash := fnv.New32a()
	hash.Write([]byte(s))
	intHash := hash.Sum32()
	result := fmt.Sprintf("%08x", intHash)
	return result
}
