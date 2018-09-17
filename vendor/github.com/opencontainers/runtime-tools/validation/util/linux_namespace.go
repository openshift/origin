package util

// ProcNamespaces defines a list of namespaces to be found under /proc/*/ns/.
// NOTE: it is not the same as generate.Namespaces, because of naming
// mismatches like "mnt" vs "mount" or "net" vs "network".
var ProcNamespaces = []string{
	"cgroup",
	"ipc",
	"mnt",
	"net",
	"pid",
	"user",
	"uts",
}

// GetRuntimeToolsNamespace converts a namespace type string for /proc into
// a string for runtime-tools. It deals with exceptional cases of "net" and
// "mnt", because those strings cannot be recognized by mapStrToNamespace(),
// which actually expects "network" and "mount" respectively.
func GetRuntimeToolsNamespace(ns string) string {
	switch ns {
	case "net":
		return "network"
	case "mnt":
		return "mount"
	}

	// In other cases, return just the original string
	return ns
}
