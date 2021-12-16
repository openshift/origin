package disruption

// RequiresKubeNamespace is a marker to get a namespace created that starts with e2e-k8s-.  The e2e framework special
// cases these to create them as privileged and/or skipping SCC, which avoids default mutation and allows higher privileges
type RequiresKubeNamespace interface {
	// RequiresKubeNamespace is a marker to get a namespace created that starts with e2e-k8s-.  The e2e framework special
	// cases these to create them as privileged and/or skipping SCC, which avoids default mutation and allows higher privileges
	// return true to have it take effect.
	RequiresKubeNamespace() bool
}
